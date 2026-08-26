package application_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestCreateAccountTrimsAndUpperCases(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account, err := h.accounts.Create(h.ctx(), h.userID, application.CreateAccountRequest{
		Name:           "  Everyday  ",
		Type:           accountType(domain.AccountCreditCard),
		InitialBalance: moneyPtr("100.00"),
		Currency:       text(" eur "),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if account.Name != "Everyday" || account.Currency != "EUR" {
		t.Errorf("account = %+v, want Everyday / EUR", account)
	}
	if account.Type != domain.AccountCreditCard {
		t.Errorf("type = %s, want creditCard", account.Type)
	}
	// A new account reports its opening balance, not a recomputed one.
	requireMoney(t, account.Balance, "100.00")
	if !account.CreatedAt.Equal(fixedNow) {
		t.Errorf("createdAt = %s, want %s", account.CreatedAt, fixedNow)
	}
}

func TestCreateAccountDefaultsTheOptionalMembers(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account, err := h.accounts.Create(h.ctx(), h.userID, application.CreateAccountRequest{
		Name: "Wallet",
		Type: accountType(domain.AccountCash),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// An omitted currency keeps the "USD" the .NET DTO initialises, and an
	// omitted opening balance is zero.
	if account.Currency != application.DefaultCurrency {
		t.Errorf("currency = %q, want USD", account.Currency)
	}
	requireMoney(t, account.Balance, "0")
}

func TestCreateAccountValidation(t *testing.T) {
	t.Parallel()

	h := newServices(t)

	_, err := h.accounts.Create(h.ctx(), h.userID, application.CreateAccountRequest{Name: "No type"})
	requireValidationError(t, err, "$",
		"The JSON payload was missing required properties, including the following: type")

	_, err = h.accounts.Create(h.ctx(), h.userID, application.CreateAccountRequest{
		Name: "   ",
		Type: accountType(domain.AccountChecking),
	})
	requireValidationError(t, err, "Name", "The Name field is required.")
}

// The balance is the opening balance plus the signed sum of every transaction:
// income adds, expense subtracts, and a transfer moves the amount from the
// source account to the destination.
func TestAccountBalancesIncludeTransfers(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	checking := h.seedAccount("Checking", "1000.00")
	savings := h.seedAccount("Savings", "500.00")

	h.addTransaction(checking.ID, domain.TransactionIncome, "250.00", date(2026, 8, 1))
	h.addTransaction(checking.ID, domain.TransactionExpense, "75.50", date(2026, 8, 2))
	h.addTransaction(checking.ID, domain.TransactionTransfer, "200.00", date(2026, 8, 3),
		withTransferTo(savings.ID))

	accounts, err := h.accounts.List(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("accounts = %d, want 2", len(accounts))
	}

	// Ordered by name once both are active.
	requireMoney(t, accounts[0].Balance, "974.50")
	requireMoney(t, accounts[1].Balance, "700.00")
}

func TestAccountListPutsArchivedLast(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	first := h.seedAccount("Aardvark", "0.00")
	h.seedAccount("Zebra", "0.00")

	if err := h.accounts.Archive(h.ctx(), h.userID, first.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	accounts, err := h.accounts.List(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if accounts[0].Name != "Zebra" || !accounts[1].IsArchived {
		t.Errorf("accounts = %+v, want the archived Aardvark last", accounts)
	}
}

func TestArchiveKeepsTheRowAndItsTransactions(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "10.00")
	h.addTransaction(account.ID, domain.TransactionIncome, "5.00", date(2026, 8, 1))

	if err := h.accounts.Archive(h.ctx(), h.userID, account.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	stored, ok := h.store.StoredAccount(account.ID)
	if !ok || !stored.IsArchived {
		t.Fatalf("stored = %+v, want an archived row", stored)
	}
	if h.store.CountTransactions() != 1 {
		t.Errorf("transactions = %d, want the history kept", h.store.CountTransactions())
	}
}

func TestUpdateAccountUnarchivesWhenTheFlagIsOmitted(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	if err := h.accounts.Archive(h.ctx(), h.userID, account.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	updated, err := h.accounts.Update(h.ctx(), h.userID, account.ID, application.UpdateAccountRequest{
		Name:     "Checking",
		Type:     accountType(domain.AccountChecking),
		Currency: text("USD"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.IsArchived {
		t.Error("isArchived = true, want an omitted flag to leave the account active")
	}
}

func TestAccountsOfAnotherUserAreNotFound(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	if _, err := h.accounts.Get(h.ctx(), h.otherID, account.ID); err != nil {
		requireAppError(t, err, domain.KindNotFound, "Account was not found.")
	} else {
		t.Fatal("Get returned an account belonging to another user")
	}

	err := h.accounts.Archive(h.ctx(), h.otherID, account.ID)
	requireAppError(t, err, domain.KindNotFound, "Account was not found.")

	_, err = h.accounts.Get(h.ctx(), h.userID, uuid.New())
	requireAppError(t, err, domain.KindNotFound, "Account was not found.")
}

func TestAccountListIsEmptyRatherThanNull(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	accounts, err := h.accounts.List(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if accounts == nil || len(accounts) != 0 {
		t.Errorf("accounts = %v, want an empty slice so the JSON is []", accounts)
	}
}
