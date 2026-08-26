package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestCreateTransactionNormalisesItsFields(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	transaction := h.addTransaction(account.ID, domain.TransactionExpense, "25.5", date(2026, 8, 17),
		withDescription("  Books  "),
		withNotes("   "),
		withTags(" reading ", "", "  "))

	// decimal.Round never lengthens a scale, so 25.5 stays 25.5 on the wire even
	// though the column stores 25.50.
	requireMoney(t, transaction.Amount, "25.5")
	if transaction.Description != "Books" {
		t.Errorf("description = %q, want %q", transaction.Description, "Books")
	}
	if transaction.Notes != nil {
		t.Errorf("notes = %v, want nil for a blank string", *transaction.Notes)
	}
	if len(transaction.Tags) != 1 || transaction.Tags[0] != "reading" {
		t.Errorf("tags = %v, want [reading]", transaction.Tags)
	}

	stored, _ := h.store.StoredTransaction(transaction.ID)
	if stored.TagsRaw != "reading" {
		t.Errorf("tagsRaw = %q, want the single trimmed tag", stored.TagsRaw)
	}
}

func TestCreateTransactionRoundsToTheColumnScale(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	transaction := h.addTransaction(account.ID, domain.TransactionExpense, "10.005", date(2026, 8, 1))
	// Half away from zero, as decimal.Round(value, 2, MidpointRounding.AwayFromZero).
	requireMoney(t, transaction.Amount, "10.01")
}

func TestTransferValidation(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	checking := h.seedAccount("Checking", "0.00")
	savings := h.seedAccount("Savings", "0.00")

	base := func() application.TransactionRequest {
		return application.TransactionRequest{
			AccountID:   identifier(checking.ID),
			Type:        transactionType(domain.TransactionTransfer),
			Amount:      moneyPtr("10.00"),
			Date:        timestamp(date(2026, 8, 1)),
			Description: "Move",
		}
	}

	request := base()
	_, err := h.transactions.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindValidation, "A transfer requires a destination account.")

	request = base()
	request.TransferAccountID = identifier(checking.ID)
	_, err = h.transactions.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindValidation, "A transfer must use two different accounts.")

	request = base()
	request.TransferAccountID = identifier(uuid.New())
	_, err = h.transactions.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindNotFound, "Account was not found.")

	request = base()
	request.TransferAccountID = identifier(savings.ID)
	transfer, err := h.transactions.Create(h.ctx(), h.userID, request)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if transfer.TransferAccountID == nil || *transfer.TransferAccountID != savings.ID {
		t.Errorf("transferAccountId = %v, want %s", transfer.TransferAccountID, savings.ID)
	}
}

// A destination account only matters for a transfer; the .NET service drops it
// for every other type.
func TestNonTransfersDropTheDestinationAccount(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	checking := h.seedAccount("Checking", "0.00")
	savings := h.seedAccount("Savings", "0.00")

	transaction := h.addTransaction(checking.ID, domain.TransactionExpense, "10.00", date(2026, 8, 1),
		withTransferTo(savings.ID))
	if transaction.TransferAccountID != nil {
		t.Errorf("transferAccountId = %v, want nil", transaction.TransferAccountID)
	}
}

func TestCreateTransactionChecksOwnership(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	_, err := h.transactions.Create(h.ctx(), h.userID, application.TransactionRequest{
		AccountID:   identifier(uuid.New()),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("1.00"),
		Date:        timestamp(date(2026, 8, 1)),
		Description: "Nowhere",
	})
	requireAppError(t, err, domain.KindNotFound, "Account was not found.")

	_, err = h.transactions.Create(h.ctx(), h.userID, application.TransactionRequest{
		AccountID:   identifier(account.ID),
		CategoryID:  identifier(uuid.New()),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("1.00"),
		Date:        timestamp(date(2026, 8, 1)),
		Description: "Unknown category",
	})
	requireAppError(t, err, domain.KindNotFound, "Category was not found.")
}

func TestTransactionValidation(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	_, err := h.transactions.Create(h.ctx(), h.userID, application.TransactionRequest{})
	requireValidationError(t, err, "$",
		"The JSON payload was missing required properties, including the following: accountId, type, amount, date")

	_, err = h.transactions.Create(h.ctx(), h.userID, application.TransactionRequest{
		AccountID:   identifier(account.ID),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("0.00"),
		Date:        timestamp(date(2026, 8, 1)),
		Description: "Free",
	})
	requireValidationError(t, err, "Amount",
		"The field Amount must be between 0.01 and 999999999999.99.")

	_, err = h.transactions.Create(h.ctx(), h.userID, application.TransactionRequest{
		AccountID: identifier(account.ID),
		Type:      transactionType(domain.TransactionExpense),
		Amount:    moneyPtr("1.00"),
		Date:      timestamp(date(2026, 8, 1)),
	})
	requireValidationError(t, err, "Description", "The Description field is required.")
}

func TestSearchPagesAndFilters(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	checking := h.seedAccount("Checking", "0.00")
	savings := h.seedAccount("Savings", "0.00")
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	for day := 1; day <= 5; day++ {
		h.addTransaction(checking.ID, domain.TransactionExpense, "10.00", date(2026, 8, day),
			withCategory(category.ID))
	}
	h.addTransaction(checking.ID, domain.TransactionIncome, "500.00", date(2026, 7, 31),
		withDescription("Paycheque"))
	h.addTransaction(checking.ID, domain.TransactionTransfer, "20.00", date(2026, 8, 6),
		withTransferTo(savings.ID))

	page, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if page.Total != 7 || page.Page != 1 || page.PageSize != 20 {
		t.Errorf("page = %+v, want 7 items on page 1 of 20", page)
	}
	// Newest first.
	if !page.Items[0].Date.Equal(date(2026, 8, 6)) {
		t.Errorf("first date = %s, want 2026-08-06", page.Items[0].Date)
	}

	size := 2
	second, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		Page:     &[]int{2}[0],
		PageSize: &size,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(second.Items) != 2 || second.Total != 7 {
		t.Errorf("second page = %+v, want two of seven", second)
	}
	if !second.Items[0].Date.Equal(date(2026, 8, 4)) {
		t.Errorf("second page starts at %s, want 2026-08-04", second.Items[0].Date)
	}

	// The account filter matches either side of a transfer.
	byAccount, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		AccountID: identifier(savings.ID),
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if byAccount.Total != 1 {
		t.Errorf("transfers into savings = %d, want 1", byAccount.Total)
	}

	byType, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		Type: transactionType(domain.TransactionIncome),
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if byType.Total != 1 {
		t.Errorf("income = %d, want 1", byType.Total)
	}

	byCategory, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		CategoryID: identifier(category.ID),
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if byCategory.Total != 5 {
		t.Errorf("category matches = %d, want 5", byCategory.Total)
	}

	// Both date bounds are inclusive.
	from, to := date(2026, 8, 2), date(2026, 8, 4)
	window, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		From: &from,
		To:   &to,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if window.Total != 3 {
		t.Errorf("window matches = %d, want 3", window.Total)
	}
}

func TestSearchMatchesDescriptionNotesAndTags(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	h.addTransaction(account.ID, domain.TransactionExpense, "1.00", date(2026, 8, 1),
		withDescription("Espresso machine"))
	h.addTransaction(account.ID, domain.TransactionExpense, "2.00", date(2026, 8, 2),
		withDescription("Other"), withNotes("Paid by ESPRESSO voucher"))
	h.addTransaction(account.ID, domain.TransactionExpense, "3.00", date(2026, 8, 3),
		withDescription("Other"), withTags("espresso"))
	h.addTransaction(account.ID, domain.TransactionExpense, "4.00", date(2026, 8, 4),
		withDescription("Unrelated"))

	page, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		Search: "  ESPRESSO  ",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("matches = %d, want 3 (description, notes and tags)", page.Total)
	}
}

func TestSearchValidation(t *testing.T) {
	t.Parallel()

	h := newServices(t)

	zero := 0
	_, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{Page: &zero})
	requireValidationError(t, err, "Page", "The field Page must be between 1 and 2147483647.")

	tooBig := application.MaxPageSize + 1
	_, err = h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{PageSize: &tooBig})
	requireValidationError(t, err, "PageSize", "The field PageSize must be between 1 and 200.")

	_, err = h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{
		Search: string(make([]byte, 201)),
	})
	requireValidationError(t, err, "Search",
		"The field Search must be a string or array type with a maximum length of '200'.")
}

func TestTransactionsOfAnotherUserAreNotFound(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	transaction := h.addTransaction(account.ID, domain.TransactionExpense, "1.00", date(2026, 8, 1))

	_, err := h.transactions.Get(h.ctx(), h.otherID, transaction.ID)
	requireAppError(t, err, domain.KindNotFound, "Transaction was not found.")

	err = h.transactions.Delete(h.ctx(), h.otherID, transaction.ID)
	requireAppError(t, err, domain.KindNotFound, "Transaction was not found.")

	if h.store.CountTransactions() != 1 {
		t.Error("a cross-user delete removed the row")
	}
}

func TestUpdateAndDeleteTransaction(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	transaction := h.addTransaction(account.ID, domain.TransactionExpense, "1.00", date(2026, 8, 1))

	updated, err := h.transactions.Update(h.ctx(), h.userID, transaction.ID, application.TransactionRequest{
		AccountID:   identifier(account.ID),
		Type:        transactionType(domain.TransactionIncome),
		Amount:      moneyPtr("2.50"),
		Date:        timestamp(date(2026, 8, 9)),
		Description: "Refund",
		Tags:        []string{"tax"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Type != domain.TransactionIncome || updated.Description != "Refund" {
		t.Errorf("updated = %+v, want an income called Refund", updated)
	}
	requireMoney(t, updated.Amount, "2.50")

	if err := h.transactions.Delete(h.ctx(), h.userID, transaction.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if h.store.CountTransactions() != 0 {
		t.Errorf("transactions = %d, want none", h.store.CountTransactions())
	}
}

func TestSlicesRespectTheInclusiveWindow(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	for day := 1; day <= 3; day++ {
		h.addTransaction(account.ID, domain.TransactionExpense, "1.00", date(2026, 8, day))
	}

	from, to := date(2026, 8, 2), date(2026, 8, 3)
	slices, err := h.transactions.Slices(h.ctx(), h.userID, &from, &to)
	if err != nil {
		t.Fatalf("Slices: %v", err)
	}
	if len(slices) != 2 {
		t.Errorf("slices = %d, want 2", len(slices))
	}

	var none *time.Time
	all, err := h.transactions.Slices(h.ctx(), h.userID, none, none)
	if err != nil {
		t.Fatalf("Slices: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("slices = %d, want 3", len(all))
	}
}
