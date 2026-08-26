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

// accountColumns is the "Accounts" projection, in entity field order. The
// identifiers are quoted because the table keeps EF Core's PascalCase names.
const accountColumns = `"Id", "UserId", "Name", "Type", "InitialBalance", "Currency", "IsArchived", "CreatedAt"`

// AccountRepository reads and writes the "Accounts" table.
type AccountRepository struct {
	db Querier
}

// NewAccountRepository binds a repository to a pool (or any other Querier).
func NewAccountRepository(db Querier) *AccountRepository {
	return &AccountRepository{db: db}
}

var _ application.AccountRepository = (*AccountRepository)(nil)

// List returns the user's accounts, active ones first and then by name, which
// is the order AccountService.ListAsync asks the database for.
func (r *AccountRepository) List(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+accountColumns+` FROM "Accounts" WHERE "UserId" = $1
		 ORDER BY "IsArchived", "Name"`, userID)
	if err != nil {
		return nil, fmt.Errorf("persistence: listing accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]domain.Account, 0, 8)
	for rows.Next() {
		account, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: listing accounts: %w", err)
	}
	return accounts, nil
}

// Get reads one account owned by the user.
func (r *AccountRepository) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Account, error) {
	account, err := scanAccount(r.db.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2`, id, userID))
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// Exists is the ownership probe the transaction and recurring services run.
func (r *AccountRepository) Exists(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2)`,
		id, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("persistence: checking account: %w", err)
	}
	return exists, nil
}

// Add inserts a new account.
func (r *AccountRepository) Add(ctx context.Context, account *domain.Account) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO "Accounts" (`+accountColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		account.Id, account.UserId, account.Name, int(account.Type), account.InitialBalance,
		account.Currency, account.IsArchived, account.CreatedAt)
	if err != nil {
		return fmt.Errorf("persistence: inserting account: %w", err)
	}
	return nil
}

// Update writes the editable columns of an account the user owns.
func (r *AccountRepository) Update(ctx context.Context, account *domain.Account) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE "Accounts"
		 SET "Name" = $3, "Type" = $4, "InitialBalance" = $5, "Currency" = $6, "IsArchived" = $7
		 WHERE "Id" = $1 AND "UserId" = $2`,
		account.Id, account.UserId, account.Name, int(account.Type), account.InitialBalance,
		account.Currency, account.IsArchived)
	if err != nil {
		return fmt.Errorf("persistence: updating account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

// scanAccount reads one row of accountColumns. It takes the row interface both
// pgx.Row and pgx.Rows satisfy, so the single-row and many-row paths share it.
func scanAccount(row pgx.Row) (domain.Account, error) {
	var account domain.Account
	var accountType int

	err := row.Scan(&account.Id, &account.UserId, &account.Name, &accountType,
		&account.InitialBalance, &account.Currency, &account.IsArchived, &account.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Account{}, application.ErrRowNotFound
	case err != nil:
		return domain.Account{}, fmt.Errorf("persistence: reading account: %w", err)
	}

	account.Type = domain.AccountType(accountType)
	account.CreatedAt = account.CreatedAt.UTC()
	return account, nil
}
