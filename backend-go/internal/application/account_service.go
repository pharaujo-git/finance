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

// Entity names feed domain.NotFound, which appends " was not found."; they are
// the strings the .NET services pass to AppException.NotFound.
const (
	accountEntityName     = "Account"
	categoryEntityName    = "Category"
	transactionEntityName = "Transaction"
	recurringEntityName   = "Recurring rule"
	budgetEntityName      = "Budget"
	goalEntityName        = "Goal"
)

// AccountService is account CRUD plus live balance computation, the Go twin of
// FinanceTracker.Application.Services.AccountService. Deleting archives.
type AccountService struct {
	accounts     AccountRepository
	transactions TransactionRepository

	now   func() time.Time
	newID func() uuid.UUID
}

// NewAccountService wires the service to its ports.
func NewAccountService(accounts AccountRepository, transactions TransactionRepository) *AccountService {
	return &AccountService{
		accounts:     accounts,
		transactions: transactions,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.New,
	}
}

// WithClock returns a copy that stamps CreatedAt from now.
func (s *AccountService) WithClock(now func() time.Time) *AccountService {
	clone := *s
	clone.now = now
	return &clone
}

// List returns the caller's accounts, active ones first and then by name, each
// carrying the balance implied by every transaction the caller owns.
func (s *AccountService) List(ctx context.Context, userID uuid.UUID) ([]AccountDto, error) {
	accounts, err := s.accounts.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing accounts: %w", err)
	}

	slices, err := s.transactions.Slices(ctx, userID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("application: loading transaction slices: %w", err)
	}

	dtos := make([]AccountDto, 0, len(accounts))
	for _, account := range accounts {
		dtos = append(dtos, NewAccountDto(account, BalanceOf(account, slices)))
	}
	return dtos, nil
}

// Get returns one account. Another user's id reads as missing.
func (s *AccountService) Get(ctx context.Context, userID, id uuid.UUID) (AccountDto, error) {
	account, err := s.load(ctx, userID, id)
	if err != nil {
		return AccountDto{}, err
	}

	slices, err := s.transactions.Slices(ctx, userID, nil, nil)
	if err != nil {
		return AccountDto{}, fmt.Errorf("application: loading transaction slices: %w", err)
	}
	return NewAccountDto(*account, BalanceOf(*account, slices)), nil
}

// Create opens an account. The response reports the opening balance rather than
// a recomputed one, as the .NET service does: a new account has no history.
func (s *AccountService) Create(
	ctx context.Context,
	userID uuid.UUID,
	request CreateAccountRequest,
) (AccountDto, error) {
	if err := request.Validate(); err != nil {
		return AccountDto{}, err
	}

	initialBalance := domain.Zero()
	if request.InitialBalance != nil {
		initialBalance = *request.InitialBalance
	}

	account := &domain.Account{
		Id:             s.newID(),
		UserId:         userID,
		Name:           strings.TrimSpace(request.Name),
		Type:           *request.Type,
		InitialBalance: initialBalance,
		Currency:       normalizeCurrency(resolveCurrency(request.Currency)),
		CreatedAt:      s.now(),
	}

	if err := s.accounts.Add(ctx, account); err != nil {
		return AccountDto{}, fmt.Errorf("application: inserting account: %w", err)
	}
	return NewAccountDto(*account, account.InitialBalance), nil
}

// Update renames, retypes and optionally archives an account.
func (s *AccountService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	request UpdateAccountRequest,
) (AccountDto, error) {
	if err := request.Validate(); err != nil {
		return AccountDto{}, err
	}

	account, err := s.load(ctx, userID, id)
	if err != nil {
		return AccountDto{}, err
	}

	account.Name = strings.TrimSpace(request.Name)
	account.Type = *request.Type
	account.Currency = normalizeCurrency(resolveCurrency(request.Currency))
	// An omitted flag un-archives, which is what `request.IsArchived ?? false`
	// does on the .NET side.
	account.IsArchived = request.IsArchived != nil && *request.IsArchived

	if err = s.accounts.Update(ctx, account); err != nil {
		return AccountDto{}, s.wrapWrite(err)
	}

	slices, err := s.transactions.Slices(ctx, userID, nil, nil)
	if err != nil {
		return AccountDto{}, fmt.Errorf("application: loading transaction slices: %w", err)
	}
	return NewAccountDto(*account, BalanceOf(*account, slices)), nil
}

// Archive is the DELETE handler's work: it flags the account rather than
// removing it, so historical transactions stay intact.
func (s *AccountService) Archive(ctx context.Context, userID, id uuid.UUID) error {
	account, err := s.load(ctx, userID, id)
	if err != nil {
		return err
	}

	account.IsArchived = true
	if err := s.accounts.Update(ctx, account); err != nil {
		return s.wrapWrite(err)
	}
	return nil
}

func (s *AccountService) load(ctx context.Context, userID, id uuid.UUID) (*domain.Account, error) {
	account, err := s.accounts.Get(ctx, id, userID)
	switch {
	case errors.Is(err, ErrRowNotFound):
		return nil, domain.NotFound(accountEntityName)
	case err != nil:
		return nil, fmt.Errorf("application: reading account: %w", err)
	}
	return account, nil
}

func (s *AccountService) wrapWrite(err error) error {
	if errors.Is(err, ErrRowNotFound) {
		return domain.NotFound(accountEntityName)
	}
	return fmt.Errorf("application: updating account: %w", err)
}

// normalizeCurrency is the .Trim().ToUpperInvariant() every service applies to
// a currency code.
func normalizeCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
