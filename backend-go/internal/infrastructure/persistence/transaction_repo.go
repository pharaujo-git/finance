package persistence

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// transactionColumns is the "Transactions" projection, in entity field order.
const transactionColumns = `"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Date", ` +
	`"Description", "Notes", "TagsRaw", "TransferAccountId"`

// transactionOrder is the newest-first order both list endpoints use. Ties on
// the date are broken by id, which Postgres compares as sixteen bytes.
const transactionOrder = ` ORDER BY "Date" DESC, "Id" DESC`

// insertChunk caps how many rows one multi-row INSERT carries: eleven
// parameters per row keeps a chunk well inside the 65535-parameter protocol
// limit while still importing a large CSV in a handful of round trips.
const insertChunk = 500

// TransactionRepository reads and writes the "Transactions" table.
type TransactionRepository struct {
	db Querier
}

// NewTransactionRepository binds a repository to a pool (or any other Querier).
func NewTransactionRepository(db Querier) *TransactionRepository {
	return &TransactionRepository{db: db}
}

var _ application.TransactionRepository = (*TransactionRepository)(nil)

// Search returns one page of matches plus the total number of them.
func (r *TransactionRepository) Search(
	ctx context.Context,
	userID uuid.UUID,
	filter application.TransactionFilter,
) ([]domain.Transaction, int, error) {
	where, args := searchPredicate(userID, filter)

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM "Transactions"`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("persistence: counting transactions: %w", err)
	}

	// Copied rather than appended in place: appending to args would either
	// alias its backing array or reallocate depending on its spare capacity.
	paged := make([]any, len(args), len(args)+2)
	copy(paged, args)
	paged = append(paged, filter.Limit, filter.Offset)
	query := `SELECT ` + transactionColumns + ` FROM "Transactions"` + where + transactionOrder +
		` LIMIT $` + strconv.Itoa(len(paged)-1) + ` OFFSET $` + strconv.Itoa(len(paged))

	transactions, err := r.query(ctx, query, paged...)
	if err != nil {
		return nil, 0, err
	}
	return transactions, total, nil
}

// Get reads one transaction owned by the user.
func (r *TransactionRepository) Get(
	ctx context.Context,
	id, userID uuid.UUID,
) (*domain.Transaction, error) {
	transaction, err := scanTransaction(r.db.QueryRow(ctx,
		`SELECT `+transactionColumns+` FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2`,
		id, userID))
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

// ListRange returns every transaction in an inclusive window, newest first.
func (r *TransactionRepository) ListRange(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]domain.Transaction, error) {
	where, args := windowPredicate(userID, from, to)
	return r.query(ctx, `SELECT `+transactionColumns+` FROM "Transactions"`+where+transactionOrder, args...)
}

// Slices reads the projection the aggregations use. The order is not part of
// the .NET query, which leaves it to the database; a deterministic one is used
// here so a tie in an aggregation's output ordering is at least stable.
func (r *TransactionRepository) Slices(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]application.TransactionSlice, error) {
	where, args := windowPredicate(userID, from, to)

	rows, err := r.db.Query(ctx,
		`SELECT "AccountId", "TransferAccountId", "CategoryId", "Type", "Amount", "Date"
		 FROM "Transactions"`+where+` ORDER BY "Date", "Id"`, args...)
	if err != nil {
		return nil, fmt.Errorf("persistence: loading transaction slices: %w", err)
	}
	defer rows.Close()

	slices := make([]application.TransactionSlice, 0, 64)
	for rows.Next() {
		var slice application.TransactionSlice
		var transactionType int

		if err := rows.Scan(&slice.AccountID, &slice.TransferAccountID, &slice.CategoryID,
			&transactionType, &slice.Amount, &slice.Date); err != nil {
			return nil, fmt.Errorf("persistence: reading transaction slice: %w", err)
		}

		slice.Type = domain.TransactionType(transactionType)
		slice.Date = slice.Date.UTC()
		slices = append(slices, slice)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: loading transaction slices: %w", err)
	}
	return slices, nil
}

// Add inserts one transaction.
func (r *TransactionRepository) Add(ctx context.Context, transaction *domain.Transaction) error {
	return r.AddMany(ctx, []domain.Transaction{*transaction})
}

// AddMany inserts a batch in chunks, which is how a CSV import and a
// materialization pass write their rows.
func (r *TransactionRepository) AddMany(ctx context.Context, transactions []domain.Transaction) error {
	for start := 0; start < len(transactions); start += insertChunk {
		end := min(start+insertChunk, len(transactions))
		if err := r.insertChunk(ctx, transactions[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r *TransactionRepository) insertChunk(ctx context.Context, transactions []domain.Transaction) error {
	const columnsPerRow = 11

	values := make([]string, 0, len(transactions))
	args := make([]any, 0, len(transactions)*columnsPerRow)

	for index, transaction := range transactions {
		base := index * columnsPerRow
		placeholders := make([]string, 0, columnsPerRow)
		for offset := 1; offset <= columnsPerRow; offset++ {
			placeholders = append(placeholders, "$"+strconv.Itoa(base+offset))
		}
		values = append(values, "("+strings.Join(placeholders, ", ")+")")

		args = append(args, transaction.Id, transaction.UserId, transaction.AccountId,
			transaction.CategoryId, int(transaction.Type), transaction.Amount, transaction.Date,
			transaction.Description, transaction.Notes, transaction.TagsRaw, transaction.TransferAccountId)
	}

	if _, err := r.db.Exec(ctx,
		`INSERT INTO "Transactions" (`+transactionColumns+`) VALUES `+strings.Join(values, ", "),
		args...); err != nil {
		return fmt.Errorf("persistence: inserting transactions: %w", err)
	}
	return nil
}

// Update rewrites a transaction the user owns.
func (r *TransactionRepository) Update(ctx context.Context, transaction *domain.Transaction) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE "Transactions"
		 SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5, "Amount" = $6, "Date" = $7,
		     "Description" = $8, "Notes" = $9, "TagsRaw" = $10, "TransferAccountId" = $11
		 WHERE "Id" = $1 AND "UserId" = $2`,
		transaction.Id, transaction.UserId, transaction.AccountId, transaction.CategoryId,
		int(transaction.Type), transaction.Amount, transaction.Date, transaction.Description,
		transaction.Notes, transaction.TagsRaw, transaction.TransferAccountId)
	if err != nil {
		return fmt.Errorf("persistence: updating transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

// Delete removes a transaction the user owns.
func (r *TransactionRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("persistence: deleting transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrRowNotFound
	}
	return nil
}

func (r *TransactionRepository) query(
	ctx context.Context,
	sql string,
	args ...any,
) ([]domain.Transaction, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("persistence: listing transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]domain.Transaction, 0, 32)
	for rows.Next() {
		transaction, scanErr := scanTransaction(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: listing transactions: %w", err)
	}
	return transactions, nil
}

// predicate accumulates a WHERE clause and its arguments.
type predicate struct {
	clauses []string
	args    []any
}

func (p *predicate) add(format string, value any) {
	p.args = append(p.args, value)
	p.clauses = append(p.clauses, fmt.Sprintf(format, len(p.args)))
}

func (p *predicate) where() (string, []any) {
	return " WHERE " + strings.Join(p.clauses, " AND "), p.args
}

// windowPredicate scopes to the user and an inclusive date window.
func windowPredicate(userID uuid.UUID, from, to *time.Time) (string, []any) {
	p := &predicate{}
	p.add(`"UserId" = $%d`, userID)
	if from != nil {
		p.add(`"Date" >= $%d`, *from)
	}
	if to != nil {
		p.add(`"Date" <= $%d`, *to)
	}
	return p.where()
}

// searchPredicate is ApplyFilters. The account filter matches either side of a
// transfer, and the search term is compared against a lowered column with
// strpos, which is how EF Core translates string.Contains for Npgsql — no LIKE,
// so a % or _ in the term is an ordinary character.
func searchPredicate(userID uuid.UUID, filter application.TransactionFilter) (string, []any) {
	p := &predicate{}
	p.add(`"UserId" = $%d`, userID)

	if filter.AccountID != nil {
		p.add(`("AccountId" = $%d OR "TransferAccountId" = $%[1]d)`, *filter.AccountID)
	}
	if filter.CategoryID != nil {
		p.add(`"CategoryId" = $%d`, *filter.CategoryID)
	}
	if filter.Type != nil {
		p.add(`"Type" = $%d`, int(*filter.Type))
	}
	if filter.From != nil {
		p.add(`"Date" >= $%d`, *filter.From)
	}
	if filter.To != nil {
		p.add(`"Date" <= $%d`, *filter.To)
	}
	if filter.Search != "" {
		p.add(`(strpos(lower("Description"), $%d) > 0
			 OR ("Notes" IS NOT NULL AND strpos(lower("Notes"), $%[1]d) > 0)
			 OR strpos(lower("TagsRaw"), $%[1]d) > 0)`, filter.Search)
	}

	return p.where()
}

func scanTransaction(row pgx.Row) (domain.Transaction, error) {
	var transaction domain.Transaction
	var transactionType int

	err := row.Scan(&transaction.Id, &transaction.UserId, &transaction.AccountId,
		&transaction.CategoryId, &transactionType, &transaction.Amount, &transaction.Date,
		&transaction.Description, &transaction.Notes, &transaction.TagsRaw,
		&transaction.TransferAccountId)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Transaction{}, application.ErrRowNotFound
	case err != nil:
		return domain.Transaction{}, fmt.Errorf("persistence: reading transaction: %w", err)
	}

	transaction.Type = domain.TransactionType(transactionType)
	transaction.Date = transaction.Date.UTC()
	return transaction, nil
}
