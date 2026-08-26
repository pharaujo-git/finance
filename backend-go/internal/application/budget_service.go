package application

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// duplicateBudgetMessage is the 409 a second budget for one category and month
// raises, byte for byte.
const duplicateBudgetMessage = "A budget already exists for that category and month."

// BudgetService is monthly per-category budgets with computed spend, the Go
// twin of FinanceTracker.Application.Services.BudgetService.
type BudgetService struct {
	budgets      BudgetRepository
	transactions TransactionRepository
	categories   *CategoryService

	now   func() time.Time
	newID func() uuid.UUID
}

// NewBudgetService wires the service to its ports.
func NewBudgetService(
	budgets BudgetRepository,
	transactions TransactionRepository,
	categories *CategoryService,
) *BudgetService {
	return &BudgetService{
		budgets:      budgets,
		transactions: transactions,
		categories:   categories,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.New,
	}
}

// WithClock returns a copy that resolves "this month" from now.
func (s *BudgetService) WithClock(now func() time.Time) *BudgetService {
	clone := *s
	clone.now = now
	return &clone
}

// List returns the caller's budgets for one month, defaulting to the current
// one, each with the month's spend and what is left.
func (s *BudgetService) List(ctx context.Context, userID uuid.UUID, month *string) ([]BudgetDto, error) {
	key := MonthFrom(s.now())
	if month != nil {
		if _, ok := TryParseMonth(*month); !ok {
			return nil, domain.BadRequest(monthFormatMessage)
		}
		key = *month
	}

	start, err := ParseMonth(key)
	if err != nil {
		return nil, err
	}

	budgets, err := s.budgets.ListForMonth(ctx, userID, key)
	if err != nil {
		return nil, fmt.Errorf("application: listing budgets: %w", err)
	}
	if len(budgets) == 0 {
		return []BudgetDto{}, nil
	}

	spent, err := s.spentByCategory(ctx, userID, start)
	if err != nil {
		return nil, err
	}

	dtos := make([]BudgetDto, 0, len(budgets))
	for _, budget := range budgets {
		dtos = append(dtos, NewBudgetDto(budget, spent[budget.CategoryId]))
	}

	// The .NET service orders the mapped results in memory, so the comparison is
	// Guid.CompareTo rather than the byte order Postgres would use.
	sort.SliceStable(dtos, func(i, j int) bool {
		return compareGUID(dtos[i].CategoryID, dtos[j].CategoryID) < 0
	})
	return dtos, nil
}

// Create sets a limit for one category in one month.
func (s *BudgetService) Create(
	ctx context.Context,
	userID uuid.UUID,
	request CreateBudgetRequest,
) (BudgetDto, error) {
	if err := request.Validate(); err != nil {
		return BudgetDto{}, err
	}

	if _, ok := TryParseMonth(request.Month); !ok {
		return BudgetDto{}, domain.BadRequest(monthFormatMessage)
	}
	if err := s.categories.EnsureUsable(ctx, userID, request.CategoryID); err != nil {
		return BudgetDto{}, err
	}

	duplicate, err := s.budgets.Exists(ctx, userID, *request.CategoryID, request.Month)
	if err != nil {
		return BudgetDto{}, fmt.Errorf("application: checking for a duplicate budget: %w", err)
	}
	if duplicate {
		return BudgetDto{}, domain.Conflict(duplicateBudgetMessage)
	}

	budget := &domain.Budget{
		Id:         s.newID(),
		UserId:     userID,
		CategoryId: *request.CategoryID,
		Month:      request.Month,
		Limit:      domain.RoundMoney(*request.Limit),
	}

	if err := s.budgets.Add(ctx, budget); err != nil {
		return BudgetDto{}, fmt.Errorf("application: inserting budget: %w", err)
	}
	return s.build(ctx, userID, *budget)
}

// Update changes only the limit; the category and month are fixed at creation.
func (s *BudgetService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	request UpdateBudgetRequest,
) (BudgetDto, error) {
	if err := request.Validate(); err != nil {
		return BudgetDto{}, err
	}

	budget, err := s.load(ctx, userID, id)
	if err != nil {
		return BudgetDto{}, err
	}

	budget.Limit = domain.RoundMoney(*request.Limit)
	if err := s.budgets.Update(ctx, budget); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return BudgetDto{}, domain.NotFound(budgetEntityName)
		}
		return BudgetDto{}, fmt.Errorf("application: updating budget: %w", err)
	}
	return s.build(ctx, userID, *budget)
}

// Delete removes a budget the caller owns.
func (s *BudgetService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.load(ctx, userID, id); err != nil {
		return err
	}

	if err := s.budgets.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return domain.NotFound(budgetEntityName)
		}
		return fmt.Errorf("application: deleting budget: %w", err)
	}
	return nil
}

func (s *BudgetService) build(
	ctx context.Context,
	userID uuid.UUID,
	budget domain.Budget,
) (BudgetDto, error) {
	start, err := ParseMonth(budget.Month)
	if err != nil {
		return BudgetDto{}, err
	}

	spent, err := s.spentByCategory(ctx, userID, start)
	if err != nil {
		return BudgetDto{}, err
	}
	return NewBudgetDto(budget, spent[budget.CategoryId]), nil
}

// spentByCategory totals the caller's categorized expenses inside one month.
// The window is half-open, so a transaction stamped midnight on the first of
// the next month belongs to that month, not this one.
func (s *BudgetService) spentByCategory(
	ctx context.Context,
	userID uuid.UUID,
	start time.Time,
) (map[uuid.UUID]domain.Money, error) {
	end := domain.AddMonths(start, 1)

	slices, err := s.transactions.Slices(ctx, userID, &start, nil)
	if err != nil {
		return nil, fmt.Errorf("application: loading transaction slices: %w", err)
	}

	totals := make(map[uuid.UUID]domain.Money)
	for _, slice := range slices {
		if slice.Type != domain.TransactionExpense || slice.CategoryID == nil || !slice.Date.Before(end) {
			continue
		}

		running, seen := totals[*slice.CategoryID]
		if !seen {
			running = domain.Zero()
		}
		totals[*slice.CategoryID] = running.Add(slice.Amount)
	}
	return totals, nil
}

func (s *BudgetService) load(ctx context.Context, userID, id uuid.UUID) (*domain.Budget, error) {
	budget, err := s.budgets.Get(ctx, id, userID)
	switch {
	case errors.Is(err, ErrRowNotFound):
		return nil, domain.NotFound(budgetEntityName)
	case err != nil:
		return nil, fmt.Errorf("application: reading budget: %w", err)
	}
	return budget, nil
}

// compareGUID reproduces System.Guid.CompareTo, which is neither byte order nor
// string order: the first four bytes are compared as one signed 32-bit integer,
// the next two pairs as signed 16-bit integers, and only the last eight bytes
// one at a time. A uuid whose first byte is 0x80 or higher therefore sorts
// before one that starts with 0x7f.
func compareGUID(left, right uuid.UUID) int {
	if diff := compareInt(int64(int32(binary.BigEndian.Uint32(left[0:4]))),
		int64(int32(binary.BigEndian.Uint32(right[0:4])))); diff != 0 {
		return diff
	}
	if diff := compareInt(int64(int16(binary.BigEndian.Uint16(left[4:6]))),
		int64(int16(binary.BigEndian.Uint16(right[4:6])))); diff != 0 {
		return diff
	}
	if diff := compareInt(int64(int16(binary.BigEndian.Uint16(left[6:8]))),
		int64(int16(binary.BigEndian.Uint16(right[6:8])))); diff != 0 {
		return diff
	}

	for index := 8; index < len(left); index++ {
		if diff := compareInt(int64(left[index]), int64(right[index])); diff != 0 {
			return diff
		}
	}
	return 0
}

func compareInt(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
