package application

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Paging bounds from FinanceTracker's TransactionQuery.
const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
)

// TransactionDto is the wire shape of a transaction.
type TransactionDto struct {
	ID                uuid.UUID              `json:"id"`
	AccountID         uuid.UUID              `json:"accountId"`
	CategoryID        *uuid.UUID             `json:"categoryId"`
	Type              domain.TransactionType `json:"type"`
	Amount            domain.Amount          `json:"amount"`
	Date              time.Time              `json:"date"`
	Description       string                 `json:"description"`
	Notes             *string                `json:"notes"`
	Tags              []string               `json:"tags"`
	TransferAccountID *uuid.UUID             `json:"transferAccountId"`
}

// NewTransactionDto projects a transaction onto the wire, splitting the packed
// tag column back into an array.
func NewTransactionDto(transaction domain.Transaction) TransactionDto {
	return TransactionDto{
		ID:                transaction.Id,
		AccountID:         transaction.AccountId,
		CategoryID:        transaction.CategoryId,
		Type:              transaction.Type,
		Amount:            domain.NewAmount(transaction.Amount),
		Date:              transaction.Date.UTC(),
		Description:       transaction.Description,
		Notes:             transaction.Notes,
		Tags:              domain.SplitTags(transaction.TagsRaw),
		TransferAccountID: transaction.TransferAccountId,
	}
}

// PagedResult is the envelope GET /api/transactions returns.
type PagedResult[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// ImportResult is the body POST /api/transactions/import returns.
type ImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// TransactionRequest is the body of POST and PUT /api/transactions.
type TransactionRequest struct {
	AccountID   *uuid.UUID              `json:"accountId"`
	CategoryID  *uuid.UUID              `json:"categoryId"`
	Type        *domain.TransactionType `json:"type"`
	Amount      *domain.Money           `json:"amount"`
	Date        *Timestamp              `json:"date"`
	Description string                  `json:"description"`
	Notes       *string                 `json:"notes"`
	Tags        []string                `json:"tags"`
	// TransferAccountID is only read for a transfer; other types ignore it.
	TransferAccountID *uuid.UUID `json:"transferAccountId"`
}

// Validate mirrors the attributes on FinanceTracker's TransactionRequest.
func (r TransactionRequest) Validate() error {
	errs := domain.NewValidationError()

	var missing []string
	if r.AccountID == nil {
		missing = append(missing, "accountId")
	}
	if r.Type == nil {
		missing = append(missing, "type")
	}
	if r.Amount == nil {
		missing = append(missing, "amount")
	}
	if r.Date == nil {
		missing = append(missing, "date")
	}
	if requiredMembers(errs, missing) {
		return errs.OrNil()
	}

	decimalRange(errs, FieldAmount, r.Amount, MoneyMinPositive)

	required(errs, FieldDescription, r.Description)
	maxLength(errs, FieldDescription, r.Description, descriptionMaxLength)

	if r.Notes != nil {
		maxLength(errs, FieldNotes, *r.Notes, notesMaxLength)
	}

	return errs.OrNil()
}

// TransactionQuery is the query string of GET /api/transactions. Every value is
// optional; the two paging members fall back to the .NET defaults.
type TransactionQuery struct {
	Page       *int
	PageSize   *int
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Type       *domain.TransactionType
	From       *time.Time
	To         *time.Time
	Search     string
}

// Validate mirrors the attributes on FinanceTracker's TransactionQuery.
func (q TransactionQuery) Validate() error {
	errs := domain.NewValidationError()

	intRange(errs, FieldPage, q.Page, 1, maxInt32)
	intRange(errs, FieldPageSize, q.PageSize, 1, MaxPageSize)
	maxLength(errs, FieldSearch, q.Search, searchMaxLength)

	return errs.OrNil()
}

// maxInt32 is int.MaxValue, the upper bound of [Range(1, int.MaxValue)] on Page.
const maxInt32 = 2147483647

// EffectivePage is the requested page, or the default when it was omitted.
func (q TransactionQuery) EffectivePage() int {
	if q.Page == nil {
		return DefaultPage
	}
	return *q.Page
}

// EffectivePageSize is the requested page size, or the default.
func (q TransactionQuery) EffectivePageSize() int {
	if q.PageSize == nil {
		return DefaultPageSize
	}
	return *q.PageSize
}

// filter resolves the query into the shape the repository takes.
func (q TransactionQuery) filter() TransactionFilter {
	page := q.EffectivePage()
	pageSize := q.EffectivePageSize()

	filter := TransactionFilter{
		AccountID:  q.AccountID,
		CategoryID: q.CategoryID,
		Type:       q.Type,
		From:       domain.NormalizeUTCPtr(q.From),
		To:         domain.NormalizeUTCPtr(q.To),
		Offset:     (page - 1) * pageSize,
		Limit:      pageSize,
	}

	// The .NET filter lowercases the trimmed term and compares it against a
	// lowered column, so the match is case-insensitive on both sides.
	if term := strings.TrimSpace(q.Search); term != "" {
		filter.Search = strings.ToLower(term)
	}
	return filter
}
