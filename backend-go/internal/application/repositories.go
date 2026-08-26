package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// ErrRowNotFound is what every repository below reports when a lookup matches
// nothing. The services turn it into the 404 the transport renders, so the
// "scope to the signed-in user, otherwise 404" rule of OwnedEntityQueries lives
// in one place per service rather than in the SQL.
var ErrRowNotFound = errors.New("application: row not found")

// AccountRepository is the persistence port for the "Accounts" table. List is
// ordered active-first then by name, as AccountService.ListAsync asks for.
type AccountRepository interface {
	List(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	Get(ctx context.Context, id, userID uuid.UUID) (*domain.Account, error)
	Exists(ctx context.Context, id, userID uuid.UUID) (bool, error)
	Add(ctx context.Context, account *domain.Account) error
	Update(ctx context.Context, account *domain.Account) error
}

// CategoryRepository is the persistence port for the "Categories" table.
// "Visible" is CategoryService.VisibleTo: the shared defaults plus the caller's
// own rows, ordered by type then name.
type CategoryRepository interface {
	ListVisible(ctx context.Context, userID uuid.UUID) ([]domain.Category, error)
	FindVisible(ctx context.Context, id, userID uuid.UUID) (*domain.Category, error)
	Add(ctx context.Context, category *domain.Category) error
	Update(ctx context.Context, category *domain.Category) error
	// Delete removes the category and, in the same transaction, detaches it from
	// the user's transactions and drops their budgets for it — the three writes
	// CategoryService.DeleteAsync commits together.
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

// TransactionFilter is the resolved form of TransactionQuery: the page has
// already been turned into an offset and the search term trimmed and lowered.
type TransactionFilter struct {
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Type       *domain.TransactionType
	// From and To are both inclusive, as the .NET filters are.
	From   *time.Time
	To     *time.Time
	Search string
	Offset int
	Limit  int
}

// TransactionRepository is the persistence port for the "Transactions" table.
type TransactionRepository interface {
	// Search returns one page ordered by date then id, both descending, plus the
	// total number of matches.
	Search(ctx context.Context, userID uuid.UUID, filter TransactionFilter) ([]domain.Transaction, int, error)
	Get(ctx context.Context, id, userID uuid.UUID) (*domain.Transaction, error)
	// ListRange returns every transaction in an inclusive date window, ordered
	// like Search. It backs CSV export.
	ListRange(ctx context.Context, userID uuid.UUID, from, to *time.Time) ([]domain.Transaction, error)
	// Slices is the projection every aggregation reads.
	Slices(ctx context.Context, userID uuid.UUID, from, to *time.Time) ([]TransactionSlice, error)
	Add(ctx context.Context, transaction *domain.Transaction) error
	AddMany(ctx context.Context, transactions []domain.Transaction) error
	Update(ctx context.Context, transaction *domain.Transaction) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

// RecurringRepository is the persistence port for the "RecurringRules" table.
type RecurringRepository interface {
	List(ctx context.Context, userID uuid.UUID) ([]domain.RecurringRule, error)
	Get(ctx context.Context, id, userID uuid.UUID) (*domain.RecurringRule, error)
	// ListDue returns every active rule of every user whose next occurrence is
	// due at or before the cutoff; the materialization pass is not user-scoped.
	ListDue(ctx context.Context, cutoff time.Time) ([]domain.RecurringRule, error)
	Add(ctx context.Context, rule *domain.RecurringRule) error
	Update(ctx context.Context, rule *domain.RecurringRule) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

// BudgetRepository is the persistence port for the "Budgets" table.
type BudgetRepository interface {
	ListForMonth(ctx context.Context, userID uuid.UUID, month string) ([]domain.Budget, error)
	Get(ctx context.Context, id, userID uuid.UUID) (*domain.Budget, error)
	Exists(ctx context.Context, userID, categoryID uuid.UUID, month string) (bool, error)
	Add(ctx context.Context, budget *domain.Budget) error
	Update(ctx context.Context, budget *domain.Budget) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

// GoalRepository is the persistence port for the "Goals" table; List is ordered
// by name.
type GoalRepository interface {
	List(ctx context.Context, userID uuid.UUID) ([]domain.Goal, error)
	Get(ctx context.Context, id, userID uuid.UUID) (*domain.Goal, error)
	Add(ctx context.Context, goal *domain.Goal) error
	Update(ctx context.Context, goal *domain.Goal) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}
