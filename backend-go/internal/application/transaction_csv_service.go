package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// CSVDateFormat is TransactionCsvService.DateFormat ("yyyy-MM-dd"), used both
// for the Date column and for the download's file name.
const CSVDateFormat = "2006-01-02"

// tagDelimiter is the ';' the CSV column packs tags with; the storage column
// uses U+001F instead.
const tagDelimiter = ";"

// emptyCSVMessage is the 400 an unparseable upload raises.
const emptyCSVMessage = "The uploaded file contains no rows."

// csvHeader is the header row export writes and import recognises.
var csvHeader = []string{"Date", "Type", "Amount", "Account", "Category", "Description", "Notes", "Tags"}

// csvDateLayouts are the forms DateTime.TryParse accepts from an invariant
// culture that this API also accepts. .NET's parser recognises a longer list of
// exotic spellings; these are the ones a real export or spreadsheet produces.
var csvDateLayouts = []string{
	CSVDateFormat,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"01/02/2006",
	"1/2/2006",
	"01/02/2006 15:04:05",
	"1/2/2006 15:04:05",
	"1/2/2006 3:04:05 PM",
}

// TransactionCsvService is CSV export and import, the Go twin of
// FinanceTracker.Application.Services.TransactionCsvService.
type TransactionCsvService struct {
	transactions TransactionRepository
	accounts     AccountRepository
	categories   CategoryRepository

	newID func() uuid.UUID
}

// NewTransactionCsvService wires the service to its ports.
func NewTransactionCsvService(
	transactions TransactionRepository,
	accounts AccountRepository,
	categories CategoryRepository,
) *TransactionCsvService {
	return &TransactionCsvService{
		transactions: transactions,
		accounts:     accounts,
		categories:   categories,
		newID:        uuid.New,
	}
}

// ExportFileName is the name the download is offered under.
func ExportFileName(now time.Time) string {
	return "transactions-" + now.UTC().Format(CSVDateFormat) + ".csv"
}

// Export renders the caller's transactions in an inclusive date window, newest
// first, as one CSV document.
func (s *TransactionCsvService) Export(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) (string, error) {
	transactions, err := s.transactions.ListRange(
		ctx, userID, domain.NormalizeUTCPtr(from), domain.NormalizeUTCPtr(to))
	if err != nil {
		return "", fmt.Errorf("application: listing transactions for export: %w", err)
	}

	accountNames, err := s.accountNames(ctx, userID)
	if err != nil {
		return "", err
	}
	categoryNames, err := s.categoryNames(ctx, userID)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	AppendCSVRow(&builder, csvHeader)

	for _, transaction := range transactions {
		category := ""
		if transaction.CategoryId != nil {
			category = categoryNames[*transaction.CategoryId]
		}
		notes := ""
		if transaction.Notes != nil {
			notes = *transaction.Notes
		}

		AppendCSVRow(&builder, []string{
			transaction.Date.UTC().Format(CSVDateFormat),
			transaction.Type.String(),
			transaction.Amount.StringFixed(domain.MoneyScale),
			accountNames[transaction.AccountId],
			category,
			transaction.Description,
			notes,
			strings.Join(domain.SplitTags(transaction.TagsRaw), tagDelimiter),
		})
	}

	return builder.String(), nil
}

// Import reads a CSV document and inserts the rows it can make sense of. A row
// it cannot use is skipped rather than failing the upload, which is why the
// result reports both counts.
func (s *TransactionCsvService) Import(
	ctx context.Context,
	userID uuid.UUID,
	content []byte,
) (ImportResult, error) {
	rows := ParseCSV(strings.TrimPrefix(string(content), "\ufeff"))
	if len(rows) == 0 {
		return ImportResult{}, domain.BadRequest(emptyCSVMessage)
	}

	accountIDs, err := s.accountIDsByName(ctx, userID)
	if err != nil {
		return ImportResult{}, err
	}
	categoryIDs, err := s.categoryIDsByName(ctx, userID)
	if err != nil {
		return ImportResult{}, err
	}

	if isHeaderRow(rows[0]) {
		rows = rows[1:]
	}

	imported := make([]domain.Transaction, 0, len(rows))
	skipped := 0
	for _, row := range rows {
		transaction, ok := s.build(userID, row, accountIDs, categoryIDs)
		if !ok {
			skipped++
			continue
		}
		imported = append(imported, transaction)
	}

	if len(imported) > 0 {
		if err := s.transactions.AddMany(ctx, imported); err != nil {
			return ImportResult{}, fmt.Errorf("application: inserting imported transactions: %w", err)
		}
	}

	return ImportResult{Imported: len(imported), Skipped: skipped}, nil
}

// build is TryBuild: every check that fails means "skip this row", never "fail
// the upload".
func (s *TransactionCsvService) build(
	userID uuid.UUID,
	row []string,
	accountIDs, categoryIDs map[string]uuid.UUID,
) (domain.Transaction, bool) {
	if len(row) < 6 {
		return domain.Transaction{}, false
	}

	date, ok := parseCSVDate(csvField(row, 0))
	if !ok {
		return domain.Transaction{}, false
	}

	transactionType, ok := domain.ParseTransactionType(csvField(row, 1))
	// Transfers need a destination account, which the CSV shape does not carry.
	if !ok || !transactionType.Valid() || transactionType == domain.TransactionTransfer {
		return domain.Transaction{}, false
	}

	amount, ok := parseCurrencyAmount(csvField(row, 2))
	if !ok || !amount.IsPositive() {
		return domain.Transaction{}, false
	}

	accountID, ok := accountIDs[strings.ToUpper(csvField(row, 3))]
	if !ok {
		return domain.Transaction{}, false
	}

	var categoryID *uuid.UUID
	if found, ok := categoryIDs[strings.ToUpper(csvField(row, 4))]; ok {
		categoryID = &found
	}

	notes := csvField(row, 6)
	tags := strings.Split(csvField(row, 7), tagDelimiter)

	return domain.Transaction{
		Id:          s.newID(),
		UserId:      userID,
		AccountId:   accountID,
		CategoryId:  categoryID,
		Type:        transactionType,
		Amount:      domain.RoundMoney(amount),
		Date:        date,
		Description: csvField(row, 5),
		Notes:       trimmedOrNil(&notes),
		TagsRaw:     domain.JoinTags(tags),
	}, true
}

func (s *TransactionCsvService) accountNames(
	ctx context.Context,
	userID uuid.UUID,
) (map[uuid.UUID]string, error) {
	accounts, err := s.accounts.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing accounts: %w", err)
	}

	names := make(map[uuid.UUID]string, len(accounts))
	for _, account := range accounts {
		names[account.Id] = account.Name
	}
	return names, nil
}

func (s *TransactionCsvService) categoryNames(
	ctx context.Context,
	userID uuid.UUID,
) (map[uuid.UUID]string, error) {
	categories, err := s.categories.ListVisible(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing categories: %w", err)
	}

	names := make(map[uuid.UUID]string, len(categories))
	for _, category := range categories {
		names[category.Id] = category.Name
	}
	return names, nil
}

// accountIDsByName keys accounts by upper-cased name, as the import lookup
// does. Two accounts sharing a name make the .NET dictionary throw; here the
// later row wins, so the upload still completes.
func (s *TransactionCsvService) accountIDsByName(
	ctx context.Context,
	userID uuid.UUID,
) (map[string]uuid.UUID, error) {
	accounts, err := s.accounts.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing accounts: %w", err)
	}

	ids := make(map[string]uuid.UUID, len(accounts))
	for _, account := range accounts {
		ids[strings.ToUpper(account.Name)] = account.Id
	}
	return ids, nil
}

// categoryIDsByName keeps the first category of each name, which is what the
// .NET GroupBy(...).First() does over the visible set.
func (s *TransactionCsvService) categoryIDsByName(
	ctx context.Context,
	userID uuid.UUID,
) (map[string]uuid.UUID, error) {
	categories, err := s.categories.ListVisible(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing categories: %w", err)
	}

	ids := make(map[string]uuid.UUID, len(categories))
	for _, category := range categories {
		key := strings.ToUpper(category.Name)
		if _, taken := ids[key]; !taken {
			ids[key] = category.Id
		}
	}
	return ids, nil
}

func isHeaderRow(row []string) bool {
	return len(row) > 0 && strings.EqualFold(strings.TrimSpace(row[0]), "Date")
}

// csvField reads one column, trimmed, treating a short row as blanks.
func csvField(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

// parseCSVDate mirrors DateTime.TryParse with AssumeUniversal: a value with no
// zone is read as UTC and one with an offset is converted.
func parseCSVDate(value string) (time.Time, bool) {
	for _, layout := range csvDateLayouts {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// parseCurrencyAmount mirrors decimal.TryParse with NumberStyles.Currency:
// thousands separators, a leading currency symbol and parenthesised or trailing
// negatives are all accepted.
func parseCurrencyAmount(value string) (domain.Money, bool) {
	text := strings.TrimSpace(value)
	negative := false

	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		negative = true
		text = text[1 : len(text)-1]
	}

	text = strings.TrimSpace(text)
	text = strings.TrimLeft(text, "¤$£€")
	text = strings.ReplaceAll(text, ",", "")
	text = strings.TrimSpace(text)

	if strings.HasSuffix(text, "-") {
		negative = true
		text = strings.TrimSuffix(text, "-")
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(text))
	if err != nil {
		return domain.Zero(), false
	}
	if negative {
		amount = amount.Neg()
	}
	return amount, true
}
