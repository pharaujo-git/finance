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

// The two 400s a malformed transfer raises, byte for byte.
const (
	transferTargetMessage      = "A transfer requires a destination account."
	transferSameAccountMessage = "A transfer must use two different accounts."
)

// TransactionService is transaction CRUD and paged filtering, the Go twin of
// FinanceTracker.Application.Services.TransactionService.
type TransactionService struct {
	transactions TransactionRepository
	accounts     AccountRepository
	categories   *CategoryService

	newID func() uuid.UUID
}

// NewTransactionService wires the service to its ports.
func NewTransactionService(
	transactions TransactionRepository,
	accounts AccountRepository,
	categories *CategoryService,
) *TransactionService {
	return &TransactionService{
		transactions: transactions,
		accounts:     accounts,
		categories:   categories,
		newID:        uuid.New,
	}
}

// Search returns one page of the caller's transactions, newest first.
func (s *TransactionService) Search(
	ctx context.Context,
	userID uuid.UUID,
	query TransactionQuery,
) (PagedResult[TransactionDto], error) {
	if err := query.Validate(); err != nil {
		return PagedResult[TransactionDto]{}, err
	}

	rows, total, err := s.transactions.Search(ctx, userID, query.filter())
	if err != nil {
		return PagedResult[TransactionDto]{}, fmt.Errorf("application: searching transactions: %w", err)
	}

	items := make([]TransactionDto, 0, len(rows))
	for _, row := range rows {
		items = append(items, NewTransactionDto(row))
	}

	return PagedResult[TransactionDto]{
		Items:    items,
		Total:    total,
		Page:     query.EffectivePage(),
		PageSize: query.EffectivePageSize(),
	}, nil
}

// Get returns one transaction; another user's id reads as missing.
func (s *TransactionService) Get(ctx context.Context, userID, id uuid.UUID) (TransactionDto, error) {
	transaction, err := s.load(ctx, userID, id)
	if err != nil {
		return TransactionDto{}, err
	}
	return NewTransactionDto(*transaction), nil
}

// Create records a movement of money.
func (s *TransactionService) Create(
	ctx context.Context,
	userID uuid.UUID,
	request TransactionRequest,
) (TransactionDto, error) {
	if err := request.Validate(); err != nil {
		return TransactionDto{}, err
	}

	transaction := &domain.Transaction{Id: s.newID(), UserId: userID}
	if err := s.apply(ctx, userID, transaction, request); err != nil {
		return TransactionDto{}, err
	}

	if err := s.transactions.Add(ctx, transaction); err != nil {
		return TransactionDto{}, fmt.Errorf("application: inserting transaction: %w", err)
	}
	return NewTransactionDto(*transaction), nil
}

// Update rewrites a transaction the caller owns.
func (s *TransactionService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	request TransactionRequest,
) (TransactionDto, error) {
	if err := request.Validate(); err != nil {
		return TransactionDto{}, err
	}

	transaction, err := s.load(ctx, userID, id)
	if err != nil {
		return TransactionDto{}, err
	}

	if err := s.apply(ctx, userID, transaction, request); err != nil {
		return TransactionDto{}, err
	}

	if err := s.transactions.Update(ctx, transaction); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return TransactionDto{}, domain.NotFound(transactionEntityName)
		}
		return TransactionDto{}, fmt.Errorf("application: updating transaction: %w", err)
	}
	return NewTransactionDto(*transaction), nil
}

// Delete removes a transaction the caller owns.
func (s *TransactionService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.load(ctx, userID, id); err != nil {
		return err
	}

	if err := s.transactions.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return domain.NotFound(transactionEntityName)
		}
		return fmt.Errorf("application: deleting transaction: %w", err)
	}
	return nil
}

// Slices loads the projection the dashboard and report aggregations read. Both
// bounds are inclusive, as in LoadSlicesAsync.
func (s *TransactionService) Slices(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]TransactionSlice, error) {
	slices, err := s.transactions.Slices(ctx, userID, domain.NormalizeUTCPtr(from), domain.NormalizeUTCPtr(to))
	if err != nil {
		return nil, fmt.Errorf("application: loading transaction slices: %w", err)
	}
	return slices, nil
}

// apply validates the request against the caller's accounts and categories and
// then copies it onto the entity. The order of the checks is the .NET order,
// which decides which 404 a doubly-wrong request gets.
func (s *TransactionService) apply(
	ctx context.Context,
	userID uuid.UUID,
	transaction *domain.Transaction,
	request TransactionRequest,
) error {
	if err := s.ensureAccount(ctx, userID, *request.AccountID); err != nil {
		return err
	}
	if err := s.categories.EnsureUsable(ctx, userID, request.CategoryID); err != nil {
		return err
	}

	var transferAccountID *uuid.UUID
	if *request.Type == domain.TransactionTransfer {
		if request.TransferAccountID == nil {
			return domain.BadRequest(transferTargetMessage)
		}
		if *request.TransferAccountID == *request.AccountID {
			return domain.BadRequest(transferSameAccountMessage)
		}
		if err := s.ensureAccount(ctx, userID, *request.TransferAccountID); err != nil {
			return err
		}
		transferAccountID = request.TransferAccountID
	}

	transaction.AccountId = *request.AccountID
	transaction.CategoryId = request.CategoryID
	transaction.Type = *request.Type
	transaction.Amount = domain.RoundMoney(*request.Amount)
	transaction.Date = request.Date.Time.UTC()
	transaction.Description = strings.TrimSpace(request.Description)
	transaction.Notes = trimmedOrNil(request.Notes)
	transaction.TagsRaw = domain.JoinTags(request.Tags)
	transaction.TransferAccountId = transferAccountID

	return nil
}

func (s *TransactionService) ensureAccount(ctx context.Context, userID, accountID uuid.UUID) error {
	exists, err := s.accounts.Exists(ctx, accountID, userID)
	if err != nil {
		return fmt.Errorf("application: checking account ownership: %w", err)
	}
	if !exists {
		return domain.NotFound(accountEntityName)
	}
	return nil
}

func (s *TransactionService) load(
	ctx context.Context,
	userID, id uuid.UUID,
) (*domain.Transaction, error) {
	transaction, err := s.transactions.Get(ctx, id, userID)
	switch {
	case errors.Is(err, ErrRowNotFound):
		return nil, domain.NotFound(transactionEntityName)
	case err != nil:
		return nil, fmt.Errorf("application: reading transaction: %w", err)
	}
	return transaction, nil
}

// trimmedOrNil is `string.IsNullOrWhiteSpace(value) ? null : value.Trim()`.
func trimmedOrNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
