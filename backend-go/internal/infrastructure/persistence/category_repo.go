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

// categoryColumns is the "Categories" projection, in entity field order.
const categoryColumns = `"Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault"`

// beginner is the Begin method a pool and a transaction both have. The category
// delete needs it: it rewrites three tables and must not half-apply.
type beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// CategoryRepository reads and writes the "Categories" table.
type CategoryRepository struct {
	db Querier
}

// NewCategoryRepository binds a repository to a pool (or any other Querier).
func NewCategoryRepository(db Querier) *CategoryRepository {
	return &CategoryRepository{db: db}
}

var _ application.CategoryRepository = (*CategoryRepository)(nil)

// ListVisible returns the categories the user may use, by type then name.
func (r *CategoryRepository) ListVisible(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.Category, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+categoryColumns+` FROM "Categories" WHERE "IsDefault" = true OR "UserId" = $1
		 ORDER BY "Type", "Name"`, userID)
	if err != nil {
		return nil, fmt.Errorf("persistence: listing categories: %w", err)
	}
	defer rows.Close()

	categories := make([]domain.Category, 0, 24)
	for rows.Next() {
		category, scanErr := scanCategory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: listing categories: %w", err)
	}
	return categories, nil
}

// FindVisible reads one category the user may use, default or their own.
func (r *CategoryRepository) FindVisible(
	ctx context.Context,
	id, userID uuid.UUID,
) (*domain.Category, error) {
	category, err := scanCategory(r.db.QueryRow(ctx,
		`SELECT `+categoryColumns+` FROM "Categories"
		 WHERE "Id" = $1 AND ("IsDefault" = true OR "UserId" = $2)`, id, userID))
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// Add inserts a category owned by a user.
func (r *CategoryRepository) Add(ctx context.Context, category *domain.Category) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO "Categories" (`+categoryColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		category.Id, category.UserId, category.Name, int(category.Type),
		category.Icon, category.Color, category.IsDefault)
	if err != nil {
		return fmt.Errorf("persistence: inserting category: %w", err)
	}
	return nil
}

// Update rewrites a category the user owns. The defaults have no owner, so this
// can never touch one.
func (r *CategoryRepository) Update(ctx context.Context, category *domain.Category) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE "Categories" SET "Name" = $3, "Type" = $4, "Icon" = $5, "Color" = $6
		 WHERE "Id" = $1 AND "UserId" = $2`,
		category.Id, category.UserId, category.Name, int(category.Type), category.Icon, category.Color)
	if err != nil {
		return fmt.Errorf("persistence: updating category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

// Delete removes the category, detaches it from the user's transactions and
// drops their budgets for it, all in one transaction — the three writes the
// .NET service commits together in a single SaveChanges.
func (r *CategoryRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	pool, ok := r.db.(beginner)
	if !ok {
		return r.deleteWith(ctx, r.db, id, userID)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("persistence: opening a transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.deleteWith(ctx, tx, id, userID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("persistence: committing category delete: %w", err)
	}
	return nil
}

func (r *CategoryRepository) deleteWith(ctx context.Context, db Querier, id, userID uuid.UUID) error {
	if _, err := db.Exec(ctx,
		`UPDATE "Transactions" SET "CategoryId" = NULL WHERE "UserId" = $1 AND "CategoryId" = $2`,
		userID, id); err != nil {
		return fmt.Errorf("persistence: detaching transactions from a category: %w", err)
	}

	if _, err := db.Exec(ctx,
		`DELETE FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2`, userID, id); err != nil {
		return fmt.Errorf("persistence: deleting budgets of a category: %w", err)
	}

	tag, err := db.Exec(ctx,
		`DELETE FROM "Categories" WHERE "Id" = $1 AND "UserId" = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("persistence: deleting category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

func scanCategory(row pgx.Row) (domain.Category, error) {
	var category domain.Category
	var categoryType int

	err := row.Scan(&category.Id, &category.UserId, &category.Name, &categoryType,
		&category.Icon, &category.Color, &category.IsDefault)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Category{}, application.ErrRowNotFound
	case err != nil:
		return domain.Category{}, fmt.Errorf("persistence: reading category: %w", err)
	}

	category.Type = domain.CategoryType(categoryType)
	return category, nil
}
