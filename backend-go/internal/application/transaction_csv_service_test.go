package application_test

import (
	"strings"
	"testing"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestExportWritesTheAgreedHeaderAndColumns(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	h.addTransaction(account.ID, domain.TransactionExpense, "4.5", date(2026, 8, 17),
		withCategory(category.ID),
		withDescription(`Beans, "single origin"`),
		withNotes("With a\nline break"),
		withTags("morning", "treat"))

	content, err := h.csv.Export(h.ctx(), h.userID, nil, nil)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if lines[0] != "Date,Type,Amount,Account,Category,Description,Notes,Tags" {
		t.Fatalf("header = %q", lines[0])
	}

	row := strings.Join(lines[1:], "\n")
	for _, want := range []string{
		"2026-08-17",
		"expense",
		// The Amount column is always written with two decimals, whatever scale
		// the stored value carries.
		"4.50",
		"Checking",
		"Coffee",
		`"Beans, ""single origin"""`,
		`"With a` + "\n" + `line break"`,
		"morning;treat",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("row %q is missing %q", row, want)
		}
	}
}

func TestExportHonoursTheWindowAndOrder(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	for day := 1; day <= 3; day++ {
		h.addTransaction(account.ID, domain.TransactionExpense, "1.00", date(2026, 8, day))
	}

	from, to := date(2026, 8, 2), date(2026, 8, 3)
	content, err := h.csv.Export(h.ctx(), h.userID, &from, &to)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want a header and two rows", len(lines))
	}
	// Newest first.
	if !strings.HasPrefix(lines[1], "2026-08-03") || !strings.HasPrefix(lines[2], "2026-08-02") {
		t.Errorf("rows = %v, want 08-03 then 08-02", lines[1:])
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	h.addTransaction(account.ID, domain.TransactionExpense, "4.50", date(2026, 8, 17),
		withCategory(category.ID), withDescription("Beans"), withTags("morning"))
	h.addTransaction(account.ID, domain.TransactionIncome, "10.00", date(2026, 8, 18),
		withDescription("Refund"))

	content, err := h.csv.Export(h.ctx(), h.userID, nil, nil)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	result, err := h.csv.Import(h.ctx(), h.userID, []byte(content))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 0 {
		t.Fatalf("result = %+v, want two imported and none skipped", result)
	}

	// Import appends: there is no duplicate detection on either side.
	if h.store.CountTransactions() != 4 {
		t.Errorf("transactions = %d, want the originals plus the imports", h.store.CountTransactions())
	}

	page, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		Search: "beans",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("beans rows = %d, want 2", page.Total)
	}
	for _, item := range page.Items {
		requireMoney(t, item.Amount, "4.50")
		if item.CategoryID == nil || *item.CategoryID != category.ID {
			t.Errorf("categoryId = %v, want the Coffee category resolved by name", item.CategoryID)
		}
		if len(item.Tags) != 1 || item.Tags[0] != "morning" {
			t.Errorf("tags = %v, want [morning]", item.Tags)
		}
	}
}

func TestImportSkipRules(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	h.seedAccount("Checking", "0.00")

	content := strings.Join([]string{
		"Date,Type,Amount,Account,Category,Description,Notes,Tags",
		// Kept: an unknown category simply leaves the row uncategorized.
		"2026-08-01,expense,10.00,Checking,Nonexistent,Groceries,,",
		// Kept: the type name is matched case-insensitively.
		"2026-08-02,INCOME,20.00,Checking,,Salary,,",
		// Skipped: too few columns.
		"2026-08-03,expense,1.00",
		// Skipped: an unparseable date.
		"not-a-date,expense,1.00,Checking,,Bad date,,",
		// Skipped: an unknown type.
		"2026-08-04,teleport,1.00,Checking,,Bad type,,",
		// Skipped: a transfer, which the CSV shape cannot carry a destination for.
		"2026-08-05,transfer,1.00,Checking,,Move,,",
		// Skipped: a non-positive amount.
		"2026-08-06,expense,0,Checking,,Free,,",
		// Skipped: an unparseable amount.
		"2026-08-07,expense,abc,Checking,,Bad amount,,",
		// Skipped: an unknown account.
		"2026-08-08,expense,1.00,Nowhere,,Bad account,,",
		"",
	}, "\n")

	result, err := h.csv.Import(h.ctx(), h.userID, []byte(content))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 7 {
		t.Errorf("result = %+v, want 2 imported and 7 skipped", result)
	}
}

func TestImportAcceptsCurrencyFormatting(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	h.seedAccount("Checking", "0.00")

	content := "Date,Type,Amount,Account,Category,Description,Notes,Tags\n" +
		`2026-08-01,expense,"$1,234.50",Checking,,Formatted,,` + "\n"

	result, err := h.csv.Import(h.ctx(), h.userID, []byte(content))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("result = %+v, want one imported row", result)
	}

	page, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	requireMoney(t, page.Items[0].Amount, "1234.50")
}

func TestImportWithoutAHeaderReadsTheFirstRow(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	h.seedAccount("Checking", "0.00")

	content := "2026-08-01,expense,10.00,Checking,,No header,,\n"
	result, err := h.csv.Import(h.ctx(), h.userID, []byte(content))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Imported != 1 {
		t.Errorf("result = %+v, want the first row treated as data", result)
	}
}

func TestImportOfAnEmptyDocumentIsRejected(t *testing.T) {
	t.Parallel()

	h := newServices(t)

	_, err := h.csv.Import(h.ctx(), h.userID, []byte("\n"))
	requireAppError(t, err, domain.KindValidation, "The uploaded file contains no rows.")
}

func TestImportStripsAByteOrderMark(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	h.seedAccount("Checking", "0.00")

	content := "\ufeffDate,Type,Amount,Account,Category,Description,Notes,Tags\n" +
		"2026-08-01,expense,10.00,Checking,,With a BOM,,\n"

	result, err := h.csv.Import(h.ctx(), h.userID, []byte(content))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Imported != 1 || result.Skipped != 0 {
		t.Errorf("result = %+v, want the header recognised despite the BOM", result)
	}
}

func TestExportFileName(t *testing.T) {
	t.Parallel()

	if got := application.ExportFileName(fixedNow); got != "transactions-2026-08-26.csv" {
		t.Errorf("file name = %q, want transactions-2026-08-26.csv", got)
	}
}

func TestCSVParsingHandlesQuotedFields(t *testing.T) {
	t.Parallel()

	rows := application.ParseCSV("a,\"b,c\",\"d\"\"e\"\n\"multi\nline\",f,g\n")
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][1] != "b,c" || rows[0][2] != `d"e` {
		t.Errorf("first row = %q", rows[0])
	}
	if rows[1][0] != "multi\nline" {
		t.Errorf("second row = %q", rows[1])
	}

	if got := application.EscapeCSVField(`say "hi"`); got != `"say ""hi"""` {
		t.Errorf("escaped = %s", got)
	}
	if got := application.EscapeCSVField("plain"); got != "plain" {
		t.Errorf("escaped = %s, want no quotes", got)
	}
}
