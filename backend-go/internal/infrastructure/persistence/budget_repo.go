package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// budgetColumns is the "Budgets" projection, in entity field order. "Limit" is
// a reserved word, which the quoting already takes care of.
const budgetColumns = `"Id", "UserId", "CategoryId", "Month", "Limit"`

// BudgetRepository reads and writes the "Budgets" table.
type BudgetRepository struct {
	db Querier
}

// NewBudgetRepository binds a repository to a pool (or any other Querier).
func NewBudgetRepository(db Querier) *BudgetRepository {
	return &BudgetRepository{db: db}
}

var _ application.BudgetRepository = (*BudgetRepository)(nil)

// ListForMonth returns the user's budgets for one YYYY-MM key. The service puts
// them in order, because the .NET service sorts the mapped results in memory.
func (r *BudgetRepository) ListForMonth(
	ctx context.Context,
	userID uuid.UUID,
	month string,
) ([]domain.Budget, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+budgetColumns+` FROM "Budgets" WHERE "UserId" = $1 AND "Month" = $2`,
		userID, month)
	if err != nil {
		return nil, fmt.Errorf("persistence: listing budgets: %w", err)
	}
	defer rows.Close()

	budgets := make([]domain.Budget, 0, 8)
	for rows.Next() {
		budget, scanErr := scanBudget(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		budgets = append(budgets, budget)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: listing budgets: %w", err)
	}
	return budgets, nil
}

// Get reads one budget owned by the user.
func (r *BudgetRepository) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Budget, error) {
	budget, err := scanBudget(r.db.QueryRow(ctx,
		`SELECT `+budgetColumns+` FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2`, id, userID))
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

// Exists reports whether the user already budgeted that category in that month.
func (r *BudgetRepository) Exists(
	ctx context.Context,
	userID, categoryID uuid.UUID,
	month string,
) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2 AND "Month" = $3)`,
		userID, categoryID, month).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("persistence: checking for a duplicate budget: %w", err)
	}
	return exists, nil
}

// Add inserts a budget.
func (r *BudgetRepository) Add(ctx context.Context, budget *domain.Budget) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO "Budgets" (`+budgetColumns+`) VALUES ($1, $2, $3, $4, $5)`,
		budget.Id, budget.UserId, budget.CategoryId, budget.Month, budget.Limit)
	if err != nil {
		return fmt.Errorf("persistence: inserting budget: %w", err)
	}
	return nil
}

// Update changes the limit; the category and month are fixed at creation.
func (r *BudgetRepository) Update(ctx context.Context, budget *domain.Budget) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE "Budgets" SET "Limit" = $3 WHERE "Id" = $1 AND "UserId" = $2`,
		budget.Id, budget.UserId, budget.Limit)
	if err != nil {
		return fmt.Errorf("persistence: updating budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

// Delete removes a budget the user owns.
func (r *BudgetRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("persistence: deleting budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

func scanBudget(row pgx.Row) (domain.Budget, error) {
	var budget domain.Budget

	err := row.Scan(&budget.Id, &budget.UserId, &budget.CategoryId, &budget.Month, &budget.Limit)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Budget{}, application.ErrRowNotFound
	case err != nil:
		return domain.Budget{}, fmt.Errorf("persistence: reading budget: %w", err)
	}
	return budget, nil
}
