package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/persistence"
)

// owner seeds a user row so the rows below hang off something real, even though
// the schema declares no foreign keys.
func owner(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	user := newUser(uuid.NewString() + "@example.com")
	if err := persistence.NewUserRepository(pool).Add(context.Background(), user); err != nil {
		t.Fatalf("seeding a user: %v", err)
	}
	return user.Id
}

func amount(value string) domain.Money { return decimal.RequireFromString(value) }

func utc(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func newAccount(userID uuid.UUID, name, initialBalance string) *domain.Account {
	return &domain.Account{
		Id:             uuid.New(),
		UserId:         userID,
		Name:           name,
		Type:           domain.AccountChecking,
		InitialBalance: amount(initialBalance),
		Currency:       "USD",
		CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}
}

func newTransaction(
	userID, accountID uuid.UUID,
	kind domain.TransactionType,
	value string,
	when time.Time,
) domain.Transaction {
	return domain.Transaction{
		Id:          uuid.New(),
		UserId:      userID,
		AccountId:   accountID,
		Type:        kind,
		Amount:      amount(value),
		Date:        when,
		Description: "Entry",
	}
}

func TestAccountRepositoryRoundTripAndOrder(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewAccountRepository(pool)
	ctx := context.Background()
	userID := owner(t, pool)

	active := newAccount(userID, "Zebra", "1250.00")
	archived := newAccount(userID, "Aardvark", "0.00")
	archived.IsArchived = true
	archived.Type = domain.AccountCreditCard

	for _, account := range []*domain.Account{active, archived} {
		if err := repo.Add(ctx, account); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	accounts, err := repo.List(ctx, userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(accounts) != 2 || accounts[0].Name != "Zebra" || !accounts[1].IsArchived {
		t.Fatalf("accounts = %+v, want the active one first", accounts)
	}

	// numeric(18,2) hands the scale back, and the enum survives as its ordinal.
	if got := domain.NewAmount(accounts[0].InitialBalance).String(); got != "1250.00" {
		t.Errorf("initialBalance = %s, want 1250.00", got)
	}
	if accounts[1].Type != domain.AccountCreditCard {
		t.Errorf("type = %v, want creditCard", accounts[1].Type)
	}
	if accounts[0].CreatedAt.Location() != time.UTC {
		t.Errorf("createdAt zone = %s, want UTC", accounts[0].CreatedAt.Location())
	}

	exists, err := repo.Exists(ctx, active.Id, userID)
	if err != nil || !exists {
		t.Errorf("Exists = %v, %v", exists, err)
	}
	if exists, _ := repo.Exists(ctx, active.Id, uuid.New()); exists {
		t.Error("Exists reported another user's account")
	}

	if _, err := repo.Get(ctx, active.Id, uuid.New()); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("cross-user Get = %v, want ErrRowNotFound", err)
	}

	active.Name = "Renamed"
	if err := repo.Update(ctx, active); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := repo.Update(ctx, newAccount(uuid.New(), "Ghost", "0.00")); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("Update of a missing row = %v, want ErrRowNotFound", err)
	}
}

func TestCategoryRepositoryVisibilityAndCascade(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewCategoryRepository(pool)
	transactions := persistence.NewTransactionRepository(pool)
	budgets := persistence.NewBudgetRepository(pool)
	ctx := context.Background()

	userID := owner(t, pool)
	strangerID := owner(t, pool)

	shared := &domain.Category{
		Id: uuid.New(), Name: "Groceries", Type: domain.CategoryExpense, IsDefault: true, Color: "#16a34a",
	}
	mine := &domain.Category{Id: uuid.New(), UserId: &userID, Name: "Coffee", Type: domain.CategoryExpense}
	theirs := &domain.Category{Id: uuid.New(), UserId: &strangerID, Name: "Secret", Type: domain.CategoryExpense}

	for _, category := range []*domain.Category{shared, mine, theirs} {
		if err := repo.Add(ctx, category); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	visible, err := repo.ListVisible(ctx, userID)
	if err != nil {
		t.Fatalf("ListVisible: %v", err)
	}
	if len(visible) != 2 {
		t.Fatalf("visible = %+v, want the default and the caller's own", visible)
	}
	if _, err := repo.FindVisible(ctx, theirs.Id, userID); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("FindVisible of another user's row = %v, want ErrRowNotFound", err)
	}
	if _, err := repo.FindVisible(ctx, shared.Id, userID); err != nil {
		t.Errorf("FindVisible of a default = %v, want it visible", err)
	}

	account := newAccount(userID, "Checking", "0.00")
	if err := persistence.NewAccountRepository(pool).Add(ctx, account); err != nil {
		t.Fatalf("seeding an account: %v", err)
	}

	transaction := newTransaction(userID, account.Id, domain.TransactionExpense, "4.00", utc(2026, 8, 1))
	transaction.CategoryId = &mine.Id
	if err := transactions.Add(ctx, &transaction); err != nil {
		t.Fatalf("seeding a transaction: %v", err)
	}

	budget := &domain.Budget{
		Id: uuid.New(), UserId: userID, CategoryId: mine.Id, Month: "2026-08", Limit: amount("50.00"),
	}
	if err := budgets.Add(ctx, budget); err != nil {
		t.Fatalf("seeding a budget: %v", err)
	}

	if err := repo.Delete(ctx, mine.Id, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	detached, err := transactions.Get(ctx, transaction.Id, userID)
	if err != nil {
		t.Fatalf("reading the transaction back: %v", err)
	}
	if detached.CategoryId != nil {
		t.Errorf("categoryId = %v, want it detached by the delete", detached.CategoryId)
	}

	remaining, err := budgets.ListForMonth(ctx, userID, "2026-08")
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("budgets = %+v, want the category's budgets dropped", remaining)
	}

	if err := repo.Delete(ctx, shared.Id, userID); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("deleting a default = %v, want ErrRowNotFound (it has no owner)", err)
	}
}

func TestTransactionRepositoryFilteringAndPaging(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewTransactionRepository(pool)
	ctx := context.Background()
	userID := owner(t, pool)

	accounts := persistence.NewAccountRepository(pool)
	checking := newAccount(userID, "Checking", "0.00")
	savings := newAccount(userID, "Savings", "0.00")
	for _, account := range []*domain.Account{checking, savings} {
		if err := accounts.Add(ctx, account); err != nil {
			t.Fatalf("seeding an account: %v", err)
		}
	}

	categoryID := uuid.New()
	if err := persistence.NewCategoryRepository(pool).Add(ctx, &domain.Category{
		Id: categoryID, UserId: &userID, Name: "Coffee", Type: domain.CategoryExpense,
	}); err != nil {
		t.Fatalf("seeding a category: %v", err)
	}

	rows := make([]domain.Transaction, 0, 8)
	for day := 1; day <= 5; day++ {
		row := newTransaction(userID, checking.Id, domain.TransactionExpense, "10.00", utc(2026, 8, day))
		row.CategoryId = &categoryID
		rows = append(rows, row)
	}

	searchable := newTransaction(userID, checking.Id, domain.TransactionIncome, "500.00", utc(2026, 7, 31))
	searchable.Description = "Monthly Paycheque"
	notes := "paid by BACS"
	searchable.Notes = &notes
	searchable.TagsRaw = domain.JoinTags([]string{"salary"})
	rows = append(rows, searchable)

	transfer := newTransaction(userID, checking.Id, domain.TransactionTransfer, "20.00", utc(2026, 8, 6))
	transfer.TransferAccountId = &savings.Id
	rows = append(rows, transfer)

	// Another user's row, which no filter may ever return.
	strangerID := owner(t, pool)
	strangerAccount := newAccount(strangerID, "Theirs", "0.00")
	if err := accounts.Add(ctx, strangerAccount); err != nil {
		t.Fatalf("seeding another user's account: %v", err)
	}
	rows = append(rows, newTransaction(strangerID, strangerAccount.Id, domain.TransactionExpense, "99.00", utc(2026, 8, 3)))

	if err := repo.AddMany(ctx, rows); err != nil {
		t.Fatalf("AddMany: %v", err)
	}

	cases := []struct {
		name   string
		filter application.TransactionFilter
		total  int
	}{
		{"everything the caller owns", application.TransactionFilter{Limit: 20}, 7},
		{"by account", application.TransactionFilter{AccountID: &checking.Id, Limit: 20}, 7},
		{"either side of a transfer", application.TransactionFilter{AccountID: &savings.Id, Limit: 20}, 1},
		{"by category", application.TransactionFilter{CategoryID: &categoryID, Limit: 20}, 5},
		{"by type", application.TransactionFilter{
			Type: &[]domain.TransactionType{domain.TransactionIncome}[0], Limit: 20,
		}, 1},
		{"inclusive window", application.TransactionFilter{
			From: &[]time.Time{utc(2026, 8, 2)}[0], To: &[]time.Time{utc(2026, 8, 4)}[0], Limit: 20,
		}, 3},
		{"search hits the description", application.TransactionFilter{Search: "paycheque", Limit: 20}, 1},
		{"search hits the notes", application.TransactionFilter{Search: "bacs", Limit: 20}, 1},
		{"search hits the tags", application.TransactionFilter{Search: "salary", Limit: 20}, 1},
		{"search matches nothing", application.TransactionFilter{Search: "nonexistent", Limit: 20}, 0},
		// A wildcard is an ordinary character: the SQL uses strpos, not LIKE.
		{"a percent sign is literal", application.TransactionFilter{Search: "%", Limit: 20}, 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			items, total, err := repo.Search(ctx, userID, testCase.filter)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if total != testCase.total {
				t.Errorf("total = %d, want %d", total, testCase.total)
			}
			if len(items) > testCase.filter.Limit {
				t.Errorf("items = %d, want at most the page size", len(items))
			}
		})
	}

	// Newest first, and the page window is offset-based.
	first, total, err := repo.Search(ctx, userID, application.TransactionFilter{Limit: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 7 || len(first) != 2 || !first[0].Date.Equal(utc(2026, 8, 6)) {
		t.Fatalf("first page = %+v (total %d)", first, total)
	}

	second, _, err := repo.Search(ctx, userID, application.TransactionFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !second[0].Date.Equal(utc(2026, 8, 4)) {
		t.Errorf("second page starts at %s, want 2026-08-04", second[0].Date)
	}

	beyond, _, err := repo.Search(ctx, userID, application.TransactionFilter{Limit: 20, Offset: 100})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(beyond) != 0 {
		t.Errorf("page past the end = %d rows, want none", len(beyond))
	}
}

func TestTransactionRepositoryWritesAndReads(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewTransactionRepository(pool)
	ctx := context.Background()
	userID := owner(t, pool)

	account := newAccount(userID, "Checking", "0.00")
	if err := persistence.NewAccountRepository(pool).Add(ctx, account); err != nil {
		t.Fatalf("seeding an account: %v", err)
	}

	transaction := newTransaction(userID, account.Id, domain.TransactionExpense, "12.34", utc(2026, 8, 17))
	notes := "with notes"
	transaction.Notes = &notes
	transaction.TagsRaw = domain.JoinTags([]string{"one", "two"})

	if err := repo.Add(ctx, &transaction); err != nil {
		t.Fatalf("Add: %v", err)
	}

	stored, err := repo.Get(ctx, transaction.Id, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Notes == nil || *stored.Notes != notes {
		t.Errorf("notes = %v", stored.Notes)
	}
	if tags := domain.SplitTags(stored.TagsRaw); len(tags) != 2 {
		t.Errorf("tags = %v, want two", tags)
	}
	if got := domain.NewAmount(stored.Amount).String(); got != "12.34" {
		t.Errorf("amount = %s, want 12.34", got)
	}
	if !stored.Date.Equal(utc(2026, 8, 17)) || stored.Date.Location() != time.UTC {
		t.Errorf("date = %s, want 2026-08-17 UTC", stored.Date)
	}

	slices, err := repo.Slices(ctx, userID, nil, nil)
	if err != nil {
		t.Fatalf("Slices: %v", err)
	}
	if len(slices) != 1 || slices[0].Type != domain.TransactionExpense {
		t.Errorf("slices = %+v", slices)
	}

	stored.Description = "Edited"
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := repo.Delete(ctx, transaction.Id, uuid.New()); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("cross-user Delete = %v, want ErrRowNotFound", err)
	}
	if err := repo.Delete(ctx, transaction.Id, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, transaction.Id, userID); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("Get after delete = %v, want ErrRowNotFound", err)
	}
}

func TestBudgetRepository(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewBudgetRepository(pool)
	ctx := context.Background()
	userID := owner(t, pool)
	categoryID := uuid.New()

	budget := &domain.Budget{
		Id: uuid.New(), UserId: userID, CategoryId: categoryID, Month: "2026-08", Limit: amount("50.00"),
	}
	if err := repo.Add(ctx, budget); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// "Limit" is a reserved word; the quoting is what makes the SQL legal.
	stored, err := repo.Get(ctx, budget.Id, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := domain.NewAmount(stored.Limit).String(); got != "50.00" {
		t.Errorf("limit = %s, want 50.00", got)
	}

	exists, err := repo.Exists(ctx, userID, categoryID, "2026-08")
	if err != nil || !exists {
		t.Errorf("Exists = %v, %v", exists, err)
	}
	if exists, _ := repo.Exists(ctx, userID, categoryID, "2026-09"); exists {
		t.Error("Exists reported another month")
	}
	if exists, _ := repo.Exists(ctx, uuid.New(), categoryID, "2026-08"); exists {
		t.Error("Exists reported another user's budget")
	}

	august, err := repo.ListForMonth(ctx, userID, "2026-08")
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	if len(august) != 1 {
		t.Errorf("budgets = %d, want 1", len(august))
	}
	september, err := repo.ListForMonth(ctx, userID, "2026-09")
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	if len(september) != 0 {
		t.Errorf("budgets = %d, want none", len(september))
	}

	budget.Limit = amount("75.00")
	if err := repo.Update(ctx, budget); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := repo.Delete(ctx, budget.Id, uuid.New()); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("cross-user Delete = %v, want ErrRowNotFound", err)
	}
	if err := repo.Delete(ctx, budget.Id, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestGoalAndRecurringRepositories(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	ctx := context.Background()
	userID := owner(t, pool)

	goals := persistence.NewGoalRepository(pool)
	targetDate := utc(2027, 3, 1)
	goal := &domain.Goal{
		Id: uuid.New(), UserId: userID, Name: "Bike",
		TargetAmount: amount("1200.00"), CurrentAmount: amount("100.00"),
		TargetDate: &targetDate, Color: "#ff8800",
	}
	if err := goals.Add(ctx, goal); err != nil {
		t.Fatalf("Add: %v", err)
	}

	storedGoal, err := goals.Get(ctx, goal.Id, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if storedGoal.TargetDate == nil || !storedGoal.TargetDate.Equal(targetDate) {
		t.Errorf("targetDate = %v", storedGoal.TargetDate)
	}

	goal.CurrentAmount = amount("125.50")
	if err := goals.Update(ctx, goal); err != nil {
		t.Fatalf("Update: %v", err)
	}
	listed, err := goals.List(ctx, userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 || domain.NewAmount(listed[0].CurrentAmount).String() != "125.50" {
		t.Errorf("goals = %+v", listed)
	}
	if err := goals.Delete(ctx, goal.Id, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	accounts := persistence.NewAccountRepository(pool)
	account := newAccount(userID, "Checking", "0.00")
	if err := accounts.Add(ctx, account); err != nil {
		t.Fatalf("seeding an account: %v", err)
	}

	rules := persistence.NewRecurringRepository(pool)
	end := utc(2027, 1, 1)
	rule := &domain.RecurringRule{
		Id: uuid.New(), UserId: userID, AccountId: account.Id,
		Type: domain.TransactionExpense, Amount: amount("12.00"), Description: "Streaming",
		Frequency: domain.FrequencyMonthly, StartDate: utc(2026, 1, 1), EndDate: &end,
		NextRunDate: utc(2026, 1, 1), IsActive: true,
	}
	if err := rules.Add(ctx, rule); err != nil {
		t.Fatalf("Add: %v", err)
	}

	due, err := rules.ListDue(ctx, utc(2026, 8, 26))
	if err != nil {
		t.Fatalf("ListDue: %v", err)
	}
	if len(due) != 1 || due[0].Frequency != domain.FrequencyMonthly {
		t.Fatalf("due = %+v", due)
	}
	if due[0].EndDate == nil || !due[0].EndDate.Equal(end) {
		t.Errorf("endDate = %v", due[0].EndDate)
	}

	rule.IsActive = false
	if err := rules.Update(ctx, rule); err != nil {
		t.Fatalf("Update: %v", err)
	}
	stillDue, err := rules.ListDue(ctx, utc(2026, 8, 26))
	if err != nil {
		t.Fatalf("ListDue: %v", err)
	}
	if len(stillDue) != 0 {
		t.Errorf("due = %+v, want an inactive rule skipped", stillDue)
	}

	if err := rules.Delete(ctx, rule.Id, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := rules.Get(ctx, rule.Id, userID); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("Get after delete = %v, want ErrRowNotFound", err)
	}
}

// The CSV export and the recurring list read through paths the paged search
// does not touch.
func TestListRangeAndRuleListing(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	ctx := context.Background()
	userID := owner(t, pool)

	account := newAccount(userID, "Checking", "0.00")
	if err := persistence.NewAccountRepository(pool).Add(ctx, account); err != nil {
		t.Fatalf("seeding an account: %v", err)
	}

	transactions := persistence.NewTransactionRepository(pool)
	for day := 1; day <= 3; day++ {
		row := newTransaction(userID, account.Id, domain.TransactionExpense, "1.00", utc(2026, 8, day))
		if err := transactions.Add(ctx, &row); err != nil {
			t.Fatalf("seeding a transaction: %v", err)
		}
	}

	all, err := transactions.ListRange(ctx, userID, nil, nil)
	if err != nil {
		t.Fatalf("ListRange: %v", err)
	}
	if len(all) != 3 || !all[0].Date.Equal(utc(2026, 8, 3)) {
		t.Errorf("rows = %+v, want three, newest first", all)
	}

	from := utc(2026, 8, 2)
	windowed, err := transactions.ListRange(ctx, userID, &from, nil)
	if err != nil {
		t.Fatalf("ListRange: %v", err)
	}
	if len(windowed) != 2 {
		t.Errorf("rows = %d, want 2 from the 2nd on", len(windowed))
	}

	rules := persistence.NewRecurringRepository(pool)
	for _, next := range []time.Time{utc(2026, 12, 1), utc(2026, 9, 1)} {
		if err := rules.Add(ctx, &domain.RecurringRule{
			Id: uuid.New(), UserId: userID, AccountId: account.Id,
			Type: domain.TransactionExpense, Amount: amount("12.00"), Description: "Rule",
			Frequency: domain.FrequencyMonthly, StartDate: next, NextRunDate: next, IsActive: true,
		}); err != nil {
			t.Fatalf("seeding a rule: %v", err)
		}
	}

	listed, err := rules.List(ctx, userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 || !listed[0].NextRunDate.Equal(utc(2026, 9, 1)) {
		t.Errorf("rules = %+v, want the soonest first", listed)
	}
	if others, err := rules.List(ctx, uuid.New()); err != nil || len(others) != 0 {
		t.Errorf("another user's rules = %+v, %v", others, err)
	}
}

func TestCategoryRepositoryUpdate(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewCategoryRepository(pool)
	ctx := context.Background()
	userID := owner(t, pool)

	category := &domain.Category{
		Id: uuid.New(), UserId: &userID, Name: "Coffee", Type: domain.CategoryExpense, Icon: "cup",
	}
	if err := repo.Add(ctx, category); err != nil {
		t.Fatalf("Add: %v", err)
	}

	category.Name = "Espresso"
	category.Type = domain.CategoryIncome
	if err := repo.Update(ctx, category); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stored, err := repo.FindVisible(ctx, category.Id, userID)
	if err != nil {
		t.Fatalf("FindVisible: %v", err)
	}
	if stored.Name != "Espresso" || stored.Type != domain.CategoryIncome {
		t.Errorf("category = %+v", stored)
	}

	stranger := uuid.New()
	category.UserId = &stranger
	if err := repo.Update(ctx, category); !errors.Is(err, application.ErrRowNotFound) {
		t.Errorf("cross-user Update = %v, want ErrRowNotFound", err)
	}
}
