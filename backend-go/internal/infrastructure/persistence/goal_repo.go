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

// goalColumns is the "Goals" projection, in entity field order.
const goalColumns = `"Id", "UserId", "Name", "TargetAmount", "CurrentAmount", "TargetDate", "Color"`

// GoalRepository reads and writes the "Goals" table.
type GoalRepository struct {
	db Querier
}

// NewGoalRepository binds a repository to a pool (or any other Querier).
func NewGoalRepository(db Querier) *GoalRepository {
	return &GoalRepository{db: db}
}

var _ application.GoalRepository = (*GoalRepository)(nil)

// List returns the user's goals by name.
func (r *GoalRepository) List(ctx context.Context, userID uuid.UUID) ([]domain.Goal, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+goalColumns+` FROM "Goals" WHERE "UserId" = $1 ORDER BY "Name"`, userID)
	if err != nil {
		return nil, fmt.Errorf("persistence: listing goals: %w", err)
	}
	defer rows.Close()

	goals := make([]domain.Goal, 0, 8)
	for rows.Next() {
		goal, scanErr := scanGoal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		goals = append(goals, goal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: listing goals: %w", err)
	}
	return goals, nil
}

// Get reads one goal owned by the user.
func (r *GoalRepository) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Goal, error) {
	goal, err := scanGoal(r.db.QueryRow(ctx,
		`SELECT `+goalColumns+` FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2`, id, userID))
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

// Add inserts a goal.
func (r *GoalRepository) Add(ctx context.Context, goal *domain.Goal) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO "Goals" (`+goalColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		goal.Id, goal.UserId, goal.Name, goal.TargetAmount, goal.CurrentAmount,
		goal.TargetDate, goal.Color)
	if err != nil {
		return fmt.Errorf("persistence: inserting goal: %w", err)
	}
	return nil
}

// Update rewrites a goal, contributions included.
func (r *GoalRepository) Update(ctx context.Context, goal *domain.Goal) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE "Goals"
		 SET "Name" = $3, "TargetAmount" = $4, "CurrentAmount" = $5, "TargetDate" = $6, "Color" = $7
		 WHERE "Id" = $1 AND "UserId" = $2`,
		goal.Id, goal.UserId, goal.Name, goal.TargetAmount, goal.CurrentAmount,
		goal.TargetDate, goal.Color)
	if err != nil {
		return fmt.Errorf("persistence: updating goal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

// Delete removes a goal the user owns.
func (r *GoalRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("persistence: deleting goal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

func scanGoal(row pgx.Row) (domain.Goal, error) {
	var goal domain.Goal

	err := row.Scan(&goal.Id, &goal.UserId, &goal.Name, &goal.TargetAmount, &goal.CurrentAmount,
		&goal.TargetDate, &goal.Color)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Goal{}, application.ErrRowNotFound
	case err != nil:
		return domain.Goal{}, fmt.Errorf("persistence: reading goal: %w", err)
	}

	goal.TargetDate = domain.NormalizeUTCPtr(goal.TargetDate)
	return goal, nil
}
