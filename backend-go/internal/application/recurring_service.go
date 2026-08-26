package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// The two 400s a malformed rule raises, byte for byte.
const (
	recurringTransferMessage = "Recurring transfers are not supported."
	recurringEndDateMessage  = "End date must not be before the start date."
)

// MaxOccurrencesPerPass is the upper bound on catch-up occurrences per rule per
// pass, so a rule left untouched for years cannot loop forever.
const MaxOccurrencesPerPass = 500

// recurringTag is the tag every materialized transaction carries.
const recurringTag = "recurring"

// RecurringService is recurring rule CRUD plus the materialization pass the
// background worker runs, the Go twin of FinanceTracker's RecurringService.
type RecurringService struct {
	rules        RecurringRepository
	transactions TransactionRepository
	accounts     AccountRepository
	categories   *CategoryService

	newID func() uuid.UUID
}

// NewRecurringService wires the service to its ports.
func NewRecurringService(
	rules RecurringRepository,
	transactions TransactionRepository,
	accounts AccountRepository,
	categories *CategoryService,
) *RecurringService {
	return &RecurringService{
		rules:        rules,
		transactions: transactions,
		accounts:     accounts,
		categories:   categories,
		newID:        uuid.New,
	}
}

// Advance moves a run date on by one period. Monthly and yearly steps clamp to
// the end of the target month, which is what DateTime.AddMonths does: a rule
// that starts on 31 January runs on 28 February and then on 28 March.
func Advance(from time.Time, frequency domain.Frequency) time.Time {
	switch frequency {
	case domain.FrequencyDaily:
		return from.AddDate(0, 0, 1)
	case domain.FrequencyWeekly:
		return from.AddDate(0, 0, 7)
	case domain.FrequencyMonthly:
		return domain.AddMonths(from, 1)
	case domain.FrequencyYearly:
		return domain.AddYears(from, 1)
	default:
		// An undefined ordinal falls through to monthly, as the C# switch does.
		return domain.AddMonths(from, 1)
	}
}

// List returns the caller's rules, soonest first.
func (s *RecurringService) List(ctx context.Context, userID uuid.UUID) ([]RecurringRuleDto, error) {
	rules, err := s.rules.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing recurring rules: %w", err)
	}

	dtos := make([]RecurringRuleDto, 0, len(rules))
	for _, rule := range rules {
		dtos = append(dtos, NewRecurringRuleDto(rule))
	}
	return dtos, nil
}

// Create adds a rule whose first occurrence is its start date.
func (s *RecurringService) Create(
	ctx context.Context,
	userID uuid.UUID,
	request RecurringRuleRequest,
) (RecurringRuleDto, error) {
	if err := request.Validate(); err != nil {
		return RecurringRuleDto{}, err
	}
	if err := s.check(ctx, userID, request); err != nil {
		return RecurringRuleDto{}, err
	}

	rule := &domain.RecurringRule{Id: s.newID(), UserId: userID}
	applyRecurring(rule, request)
	rule.NextRunDate = rule.StartDate

	if err := s.rules.Add(ctx, rule); err != nil {
		return RecurringRuleDto{}, fmt.Errorf("application: inserting recurring rule: %w", err)
	}
	return NewRecurringRuleDto(*rule), nil
}

// Update rewrites a rule, pulling its next run forward when the new start date
// is later than the run already scheduled.
func (s *RecurringService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	request RecurringRuleRequest,
) (RecurringRuleDto, error) {
	if err := request.Validate(); err != nil {
		return RecurringRuleDto{}, err
	}
	if err := s.check(ctx, userID, request); err != nil {
		return RecurringRuleDto{}, err
	}

	rule, err := s.load(ctx, userID, id)
	if err != nil {
		return RecurringRuleDto{}, err
	}

	applyRecurring(rule, request)
	if rule.NextRunDate.Before(rule.StartDate) {
		rule.NextRunDate = rule.StartDate
	}

	if err := s.rules.Update(ctx, rule); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return RecurringRuleDto{}, domain.NotFound(recurringEntityName)
		}
		return RecurringRuleDto{}, fmt.Errorf("application: updating recurring rule: %w", err)
	}
	return NewRecurringRuleDto(*rule), nil
}

// Delete removes a rule the caller owns. Transactions it already produced stay.
func (s *RecurringService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.load(ctx, userID, id); err != nil {
		return err
	}

	if err := s.rules.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return domain.NotFound(recurringEntityName)
		}
		return fmt.Errorf("application: deleting recurring rule: %w", err)
	}
	return nil
}

// MaterializeDue creates a transaction for every occurrence due on or before
// now, across every user, and reports how many it created. The caller decides
// what atomicity it runs under: the worker wraps the whole pass in one
// transaction so a crash cannot leave a rule advanced without its transaction.
func (s *RecurringService) MaterializeDue(ctx context.Context, now time.Time) (int, error) {
	cutoff := now.UTC()

	due, err := s.rules.ListDue(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("application: listing due recurring rules: %w", err)
	}

	created := make([]domain.Transaction, 0, len(due))
	for index := range due {
		created = append(created, s.materialize(&due[index], cutoff)...)
	}

	if len(created) > 0 {
		if err := s.transactions.AddMany(ctx, created); err != nil {
			return 0, fmt.Errorf("application: inserting materialized transactions: %w", err)
		}
	}

	// Every due rule changed: it either advanced past the cutoff or was
	// deactivated by its end date.
	for index := range due {
		if err := s.rules.Update(ctx, &due[index]); err != nil {
			return 0, fmt.Errorf("application: advancing recurring rule: %w", err)
		}
	}

	return len(created), nil
}

// materialize walks one rule forward, mutating it as it goes.
func (s *RecurringService) materialize(rule *domain.RecurringRule, cutoff time.Time) []domain.Transaction {
	created := make([]domain.Transaction, 0, 1)

	for occurrence := 0; occurrence < MaxOccurrencesPerPass; occurrence++ {
		if !rule.IsActive || rule.NextRunDate.After(cutoff) {
			break
		}
		if rule.EndDate != nil && rule.NextRunDate.After(*rule.EndDate) {
			rule.IsActive = false
			break
		}

		created = append(created, domain.Transaction{
			Id:          s.newID(),
			UserId:      rule.UserId,
			AccountId:   rule.AccountId,
			CategoryId:  rule.CategoryId,
			Type:        rule.Type,
			Amount:      rule.Amount,
			Date:        rule.NextRunDate,
			Description: rule.Description,
			TagsRaw:     domain.JoinTags([]string{recurringTag}),
		})

		rule.NextRunDate = Advance(rule.NextRunDate, rule.Frequency)
	}

	return created
}

// check is ValidateAsync: the order decides which failure a doubly-wrong
// request reports.
func (s *RecurringService) check(
	ctx context.Context,
	userID uuid.UUID,
	request RecurringRuleRequest,
) error {
	if *request.Type == domain.TransactionTransfer {
		return domain.BadRequest(recurringTransferMessage)
	}

	exists, err := s.accounts.Exists(ctx, *request.AccountID, userID)
	if err != nil {
		return fmt.Errorf("application: checking account ownership: %w", err)
	}
	if !exists {
		return domain.NotFound(accountEntityName)
	}

	if request.EndDate != nil && request.EndDate.Time.Before(request.StartDate.Time) {
		return domain.BadRequest(recurringEndDateMessage)
	}

	return s.categories.EnsureUsable(ctx, userID, request.CategoryID)
}

func (s *RecurringService) load(
	ctx context.Context,
	userID, id uuid.UUID,
) (*domain.RecurringRule, error) {
	rule, err := s.rules.Get(ctx, id, userID)
	switch {
	case errors.Is(err, ErrRowNotFound):
		return nil, domain.NotFound(recurringEntityName)
	case err != nil:
		return nil, fmt.Errorf("application: reading recurring rule: %w", err)
	}
	return rule, nil
}

// applyRecurring copies the request onto the entity; NextRunDate is the
// caller's business.
func applyRecurring(rule *domain.RecurringRule, request RecurringRuleRequest) {
	rule.AccountId = *request.AccountID
	rule.CategoryId = request.CategoryID
	rule.Type = *request.Type
	rule.Amount = domain.RoundMoney(*request.Amount)
	rule.Description = strings.TrimSpace(request.Description)
	rule.Frequency = *request.Frequency
	rule.StartDate = request.StartDate.Time.UTC()
	rule.EndDate = TimePtr(request.EndDate)
	rule.IsActive = request.IsActive == nil || *request.IsActive
}
