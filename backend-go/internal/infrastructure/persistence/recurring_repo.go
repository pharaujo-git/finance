package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// recurringColumns is the "RecurringRules" projection, in entity field order.
const recurringColumns = `"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Description", ` +
	`"Frequency", "StartDate", "EndDate", "NextRunDate", "IsActive"`

// RecurringRepository reads and writes the "RecurringRules" table.
type RecurringRepository struct {
	db Querier
}

// NewRecurringRepository binds a repository to a pool, a connection or a
// transaction; the worker binds it to the transaction that holds the pass lock.
func NewRecurringRepository(db Querier) *RecurringRepository {
	return &RecurringRepository{db: db}
}

var _ application.RecurringRepository = (*RecurringRepository)(nil)

// List returns the user's rules, soonest run first.
func (r *RecurringRepository) List(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.RecurringRule, error) {
	return r.query(ctx,
		`SELECT `+recurringColumns+` FROM "RecurringRules" WHERE "UserId" = $1
		 ORDER BY "NextRunDate"`, userID)
}

// Get reads one rule owned by the user.
func (r *RecurringRepository) Get(
	ctx context.Context,
	id, userID uuid.UUID,
) (*domain.RecurringRule, error) {
	rule, err := scanRecurring(r.db.QueryRow(ctx,
		`SELECT `+recurringColumns+` FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2`,
		id, userID))
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListDue returns every active rule of every user that is due at or before the
// cutoff. The pass is deliberately not scoped to one user.
func (r *RecurringRepository) ListDue(
	ctx context.Context,
	cutoff time.Time,
) ([]domain.RecurringRule, error) {
	return r.query(ctx,
		`SELECT `+recurringColumns+` FROM "RecurringRules"
		 WHERE "IsActive" = true AND "NextRunDate" <= $1
		 ORDER BY "NextRunDate", "Id"`, cutoff)
}

// Add inserts a rule.
func (r *RecurringRepository) Add(ctx context.Context, rule *domain.RecurringRule) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO "RecurringRules" (`+recurringColumns+`)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		rule.Id, rule.UserId, rule.AccountId, rule.CategoryId, int(rule.Type), rule.Amount,
		rule.Description, int(rule.Frequency), rule.StartDate, rule.EndDate, rule.NextRunDate,
		rule.IsActive)
	if err != nil {
		return fmt.Errorf("persistence: inserting recurring rule: %w", err)
	}
	return nil
}

// Update rewrites a rule, including the schedule the pass advances.
func (r *RecurringRepository) Update(ctx context.Context, rule *domain.RecurringRule) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE "RecurringRules"
		 SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5, "Amount" = $6, "Description" = $7,
		     "Frequency" = $8, "StartDate" = $9, "EndDate" = $10, "NextRunDate" = $11, "IsActive" = $12
		 WHERE "Id" = $1 AND "UserId" = $2`,
		rule.Id, rule.UserId, rule.AccountId, rule.CategoryId, int(rule.Type), rule.Amount,
		rule.Description, int(rule.Frequency), rule.StartDate, rule.EndDate, rule.NextRunDate,
		rule.IsActive)
	if err != nil {
		return fmt.Errorf("persistence: updating recurring rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

// Delete removes a rule the user owns.
func (r *RecurringRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("persistence: deleting recurring rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

func (r *RecurringRepository) query(
	ctx context.Context,
	sql string,
	args ...any,
) ([]domain.RecurringRule, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("persistence: listing recurring rules: %w", err)
	}
	defer rows.Close()

	rules := make([]domain.RecurringRule, 0, 8)
	for rows.Next() {
		rule, scanErr := scanRecurring(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: listing recurring rules: %w", err)
	}
	return rules, nil
}

func scanRecurring(row pgx.Row) (domain.RecurringRule, error) {
	var rule domain.RecurringRule
	var transactionType, frequency int

	err := row.Scan(&rule.Id, &rule.UserId, &rule.AccountId, &rule.CategoryId, &transactionType,
		&rule.Amount, &rule.Description, &frequency, &rule.StartDate, &rule.EndDate,
		&rule.NextRunDate, &rule.IsActive)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.RecurringRule{}, application.ErrRowNotFound
	case err != nil:
		return domain.RecurringRule{}, fmt.Errorf("persistence: reading recurring rule: %w", err)
	}

	rule.Type = domain.TransactionType(transactionType)
	rule.Frequency = domain.Frequency(frequency)
	rule.StartDate = rule.StartDate.UTC()
	rule.NextRunDate = rule.NextRunDate.UTC()
	rule.EndDate = domain.NormalizeUTCPtr(rule.EndDate)
	return rule, nil
}
