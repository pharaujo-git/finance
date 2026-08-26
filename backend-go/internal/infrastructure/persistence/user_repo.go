package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// uniqueViolation is the SQLSTATE Postgres raises when IX_Users_Email rejects a
// duplicate address.
const uniqueViolation = "23505"

// Querier is the slice of *pgxpool.Pool the repositories use. Taking the
// interface rather than the concrete pool lets a test drive a repository from a
// single connection, and lets the recurring worker bind a whole set of
// repositories to one transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Column lists are spelled out rather than built at runtime: the table is the
// one db/migrations/0001_baseline.sql creates, with EF Core's quoted PascalCase
// identifiers, and an unquoted name would resolve to a lowercase column that
// does not exist.
const userColumns = `"Id", "Email", "Name", "PasswordHash", "Currency", "CreatedAt"`

// UserRepository reads and writes the "Users" table.
type UserRepository struct {
	db Querier
}

// NewUserRepository binds a repository to a pool (or any other Querier).
func NewUserRepository(db Querier) *UserRepository {
	return &UserRepository{db: db}
}

var _ application.UserRepository = (*UserRepository)(nil)

// FindByEmail looks a user up by their stored (already normalised) address.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.queryOne(ctx,
		`SELECT `+userColumns+` FROM "Users" WHERE "Email" = $1`, email)
}

// FindByID looks a user up by primary key.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return r.queryOne(ctx,
		`SELECT `+userColumns+` FROM "Users" WHERE "Id" = $1`, id)
}

// Add inserts a new user. A duplicate email is reported as
// application.ErrEmailTaken whether the caller checked first or not.
func (r *UserRepository) Add(ctx context.Context, user *domain.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO "Users" (`+userColumns+`) VALUES ($1, $2, $3, $4, $5, $6)`,
		user.Id, user.Email, user.Name, user.PasswordHash, user.Currency, user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return application.ErrEmailTaken
		}
		return fmt.Errorf("persistence: inserting user: %w", err)
	}
	return nil
}

// UpdatePasswordHash replaces a stored hash, as login does after a rehash.
func (r *UserRepository) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	return r.exec(ctx,
		`UPDATE "Users" SET "PasswordHash" = $2 WHERE "Id" = $1`, id, hash)
}

// UpdateProfile writes the editable profile fields.
func (r *UserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, name, currency string) error {
	return r.exec(ctx,
		`UPDATE "Users" SET "Name" = $2, "Currency" = $3 WHERE "Id" = $1`, id, name, currency)
}

// queryOne scans the single-row projection every lookup shares.
func (r *UserRepository) queryOne(ctx context.Context, sql string, args ...any) (*domain.User, error) {
	var user domain.User
	err := r.db.QueryRow(ctx, sql, args...).Scan(
		&user.Id, &user.Email, &user.Name, &user.PasswordHash, &user.Currency, &user.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, application.ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("persistence: reading user: %w", err)
	}
	return &user, nil
}

// exec runs an update that must touch exactly one user.
func (r *UserRepository) exec(ctx context.Context, sql string, args ...any) error {
	tag, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("persistence: updating user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrUserNotFound
	}
	return nil
}
