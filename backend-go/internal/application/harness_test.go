package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/apptest"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// fixedNow is the instant every service test runs at, so "this month" is
// August 2026 whatever day the suite runs on.
var fixedNow = time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)

// services wires the real resource services onto an in-memory store, the way
// the .NET tests wire them onto a throwaway SQLite database.
type services struct {
	t     *testing.T
	store *apptest.Store

	userID  uuid.UUID
	otherID uuid.UUID

	accounts     *application.AccountService
	categories   *application.CategoryService
	transactions *application.TransactionService
	csv          *application.TransactionCsvService
	recurring    *application.RecurringService
	budgets      *application.BudgetService
	goals        *application.GoalService
	analytics    *application.AnalyticsService
}

func newServices(t *testing.T) *services {
	t.Helper()

	store := apptest.NewStore()
	clock := func() time.Time { return fixedNow }

	categories := application.NewCategoryService(store.Categories())
	transactions := application.NewTransactionService(store.Transactions(), store.Accounts(), categories)

	return &services{
		t:            t,
		store:        store,
		userID:       uuid.New(),
		otherID:      uuid.New(),
		accounts:     application.NewAccountService(store.Accounts(), store.Transactions()).WithClock(clock),
		categories:   categories,
		transactions: transactions,
		csv:          application.NewTransactionCsvService(store.Transactions(), store.Accounts(), store.Categories()),
		recurring: application.NewRecurringService(
			store.Rules(), store.Transactions(), store.Accounts(), categories),
		budgets: application.NewBudgetService(
			store.Budgets(), store.Transactions(), categories).WithClock(clock),
		goals: application.NewGoalService(store.Goals()),
		analytics: application.NewAnalyticsService(
			transactions, store.Accounts(), categories).WithClock(clock),
	}
}

func (h *services) ctx() context.Context { return context.Background() }

// money parses a decimal literal, keeping the scale it was written with.
func money(value string) domain.Money { return decimal.RequireFromString(value) }

func moneyPtr(value string) *domain.Money {
	parsed := money(value)
	return &parsed
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func timestamp(value time.Time) *application.Timestamp {
	stamp := application.NewTimestamp(value)
	return &stamp
}

func accountType(value domain.AccountType) *domain.AccountType             { return &value }
func categoryType(value domain.CategoryType) *domain.CategoryType          { return &value }
func transactionType(value domain.TransactionType) *domain.TransactionType { return &value }
func frequency(value domain.Frequency) *domain.Frequency                   { return &value }
func text(value string) *string                                            { return &value }
func identifier(value uuid.UUID) *uuid.UUID                                { return &value }

// seedAccount creates an account through the service, which is how a test gets
// an id it can point transactions at.
func (h *services) seedAccount(name, initialBalance string) application.AccountDto {
	h.t.Helper()

	account, err := h.accounts.Create(h.ctx(), h.userID, application.CreateAccountRequest{
		Name:           name,
		Type:           accountType(domain.AccountChecking),
		InitialBalance: moneyPtr(initialBalance),
		Currency:       text("usd"),
	})
	if err != nil {
		h.t.Fatalf("seeding account %q: %v", name, err)
	}
	return account
}

// seedCategory creates a category owned by the caller.
func (h *services) seedCategory(name string, kind domain.CategoryType) application.CategoryDto {
	h.t.Helper()

	category, err := h.categories.Create(h.ctx(), h.userID, application.CategoryRequest{
		Name:  name,
		Type:  categoryType(kind),
		Icon:  "cup",
		Color: "#123456",
	})
	if err != nil {
		h.t.Fatalf("seeding category %q: %v", name, err)
	}
	return category
}

// seedDefaultCategory inserts a shared default, which no user may edit.
func (h *services) seedDefaultCategory(name string) domain.Category {
	h.t.Helper()

	category := domain.Category{
		Id:        uuid.New(),
		Name:      name,
		Type:      domain.CategoryExpense,
		Icon:      "tag",
		Color:     "#16a34a",
		IsDefault: true,
	}
	h.store.SeedCategory(category)
	return category
}

// addTransaction records one movement through the service.
func (h *services) addTransaction(
	accountID uuid.UUID,
	kind domain.TransactionType,
	amount string,
	when time.Time,
	options ...func(*application.TransactionRequest),
) application.TransactionDto {
	h.t.Helper()

	request := application.TransactionRequest{
		AccountID:   identifier(accountID),
		Type:        transactionType(kind),
		Amount:      moneyPtr(amount),
		Date:        timestamp(when),
		Description: "Test entry",
	}
	for _, option := range options {
		option(&request)
	}

	transaction, err := h.transactions.Create(h.ctx(), h.userID, request)
	if err != nil {
		h.t.Fatalf("adding transaction: %v", err)
	}
	return transaction
}

func withCategory(categoryID uuid.UUID) func(*application.TransactionRequest) {
	return func(r *application.TransactionRequest) { r.CategoryID = identifier(categoryID) }
}

func withTransferTo(accountID uuid.UUID) func(*application.TransactionRequest) {
	return func(r *application.TransactionRequest) { r.TransferAccountID = identifier(accountID) }
}

func withDescription(value string) func(*application.TransactionRequest) {
	return func(r *application.TransactionRequest) { r.Description = value }
}

func withTags(values ...string) func(*application.TransactionRequest) {
	return func(r *application.TransactionRequest) { r.Tags = values }
}

func withNotes(value string) func(*application.TransactionRequest) {
	return func(r *application.TransactionRequest) { r.Notes = text(value) }
}

// requireMoney asserts a rendered amount, scale included: 12.50 and 12.5 are
// the same number but not the same JSON.
func requireMoney(t *testing.T, got domain.Amount, want string) {
	t.Helper()
	if got.String() != want {
		t.Errorf("amount = %s, want %s", got.String(), want)
	}
}

// requireValidationError asserts that one field failed with one message; the
// auth tests' requireFieldErrors asserts the whole dictionary instead.
func requireValidationError(t *testing.T, err error, field, message string) {
	t.Helper()

	var validation *domain.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v (%T), want *domain.ValidationError", err, err)
	}

	messages, ok := validation.Errors[field]
	if !ok {
		t.Fatalf("errors = %v, want a %q key", validation.Errors, field)
	}
	for _, candidate := range messages {
		if candidate == message {
			return
		}
	}
	t.Errorf("%s errors = %v, want %q", field, messages, message)
}
