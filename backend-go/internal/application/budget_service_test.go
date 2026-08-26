package application_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestBudgetSpendCountsOnlyTheMonthsCategorizedExpenses(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	coffee := h.seedCategory("Coffee", domain.CategoryExpense)
	rent := h.seedCategory("Rent", domain.CategoryExpense)

	h.addTransaction(account.ID, domain.TransactionExpense, "4.50", date(2026, 8, 1), withCategory(coffee.ID))
	h.addTransaction(account.ID, domain.TransactionExpense, "5.50", date(2026, 8, 31), withCategory(coffee.ID))
	// Excluded: another category, another month, an income, and an
	// uncategorized expense.
	h.addTransaction(account.ID, domain.TransactionExpense, "900.00", date(2026, 8, 2), withCategory(rent.ID))
	h.addTransaction(account.ID, domain.TransactionExpense, "99.00", date(2026, 9, 1), withCategory(coffee.ID))
	h.addTransaction(account.ID, domain.TransactionIncome, "50.00", date(2026, 8, 3), withCategory(coffee.ID))
	h.addTransaction(account.ID, domain.TransactionExpense, "7.00", date(2026, 8, 4))

	budget, err := h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
		CategoryID: identifier(coffee.ID),
		Month:      "2026-08",
		Limit:      moneyPtr("50.00"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	requireMoney(t, budget.Limit, "50.00")
	requireMoney(t, budget.Spent, "10.00")
	requireMoney(t, budget.Remaining, "40.00")
}

func TestBudgetListDefaultsToTheCurrentMonth(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	for _, month := range []string{"2026-07", "2026-08"} {
		if _, err := h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
			CategoryID: identifier(category.ID),
			Month:      month,
			Limit:      moneyPtr("10.00"),
		}); err != nil {
			t.Fatalf("creating the %s budget: %v", month, err)
		}
	}

	current, err := h.budgets.List(h.ctx(), h.userID, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(current) != 1 || current[0].Month != "2026-08" {
		t.Errorf("budgets = %+v, want only August, the month of the fixed clock", current)
	}

	july, err := h.budgets.List(h.ctx(), h.userID, text("2026-07"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(july) != 1 || july[0].Month != "2026-07" {
		t.Errorf("budgets = %+v, want July", july)
	}
}

func TestBudgetMonthValidation(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	// The DTO's regular expression catches a shape that is not YYYY-MM.
	_, err := h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
		CategoryID: identifier(category.ID),
		Month:      "August",
		Limit:      moneyPtr("10.00"),
	})
	requireValidationError(t, err, "Month", "Month must be in YYYY-MM format.")

	// A well-shaped but impossible month gets past the regular expression and
	// fails in the service instead, as a problem document.
	_, err = h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
		CategoryID: identifier(category.ID),
		Month:      "2026-13",
		Limit:      moneyPtr("10.00"),
	})
	requireAppError(t, err, domain.KindValidation, "Month must be in YYYY-MM format.")

	_, err = h.budgets.List(h.ctx(), h.userID, text(""))
	requireAppError(t, err, domain.KindValidation, "Month must be in YYYY-MM format.")
}

func TestDuplicateBudgetIsAConflict(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	request := application.CreateBudgetRequest{
		CategoryID: identifier(category.ID),
		Month:      "2026-08",
		Limit:      moneyPtr("10.00"),
	}
	if _, err := h.budgets.Create(h.ctx(), h.userID, request); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err := h.budgets.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindConflict,
		"A budget already exists for that category and month.")

	// The same category in a different month is fine.
	request.Month = "2026-09"
	if _, err := h.budgets.Create(h.ctx(), h.userID, request); err != nil {
		t.Fatalf("Create for another month: %v", err)
	}
}

func TestBudgetRejectsAnInvisibleCategory(t *testing.T) {
	t.Parallel()

	h := newServices(t)

	_, err := h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
		CategoryID: identifier(uuid.New()),
		Month:      "2026-08",
		Limit:      moneyPtr("10.00"),
	})
	requireAppError(t, err, domain.KindNotFound, "Category was not found.")
}

func TestUpdateBudgetChangesOnlyTheLimit(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	category := h.seedCategory("Coffee", domain.CategoryExpense)
	h.addTransaction(account.ID, domain.TransactionExpense, "4.00", date(2026, 8, 1), withCategory(category.ID))

	budget, err := h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
		CategoryID: identifier(category.ID),
		Month:      "2026-08",
		Limit:      moneyPtr("10.00"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := h.budgets.Update(h.ctx(), h.userID, budget.ID, application.UpdateBudgetRequest{
		Limit: moneyPtr("25.005"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Rounded half away from zero to the column's two decimals.
	requireMoney(t, updated.Limit, "25.01")
	requireMoney(t, updated.Spent, "4.00")
	requireMoney(t, updated.Remaining, "21.01")
	if updated.Month != "2026-08" || updated.CategoryID != category.ID {
		t.Errorf("budget = %+v, want the category and month unchanged", updated)
	}
}

func TestBudgetsOfAnotherUserAreNotFound(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	budget, err := h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
		CategoryID: identifier(category.ID),
		Month:      "2026-08",
		Limit:      moneyPtr("10.00"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = h.budgets.Update(h.ctx(), h.otherID, budget.ID, application.UpdateBudgetRequest{
		Limit: moneyPtr("1.00"),
	})
	requireAppError(t, err, domain.KindNotFound, "Budget was not found.")

	err = h.budgets.Delete(h.ctx(), h.otherID, budget.ID)
	requireAppError(t, err, domain.KindNotFound, "Budget was not found.")

	if h.store.CountBudgets() != 1 {
		t.Error("a cross-user delete removed the row")
	}
}

func TestBudgetListIsEmptyRatherThanNull(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	budgets, err := h.budgets.List(h.ctx(), h.userID, text("2026-08"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if budgets == nil || len(budgets) != 0 {
		t.Errorf("budgets = %v, want an empty slice so the JSON is []", budgets)
	}
}

// The .NET service sorts the mapped budgets in memory, so the order is
// Guid.CompareTo's, which reads the first four bytes as a signed integer.
func TestBudgetListUsesTheDotNetGuidOrder(t *testing.T) {
	t.Parallel()

	h := newServices(t)

	// 0xff... sorts first under Guid.CompareTo (a negative first word) and last
	// under the byte order Postgres uses.
	high := uuid.MustParse("ff000000-0000-0000-0000-000000000000")
	low := uuid.MustParse("01000000-0000-0000-0000-000000000000")

	for _, categoryID := range []uuid.UUID{low, high} {
		h.store.SeedCategory(domain.Category{
			Id: categoryID, UserId: &h.userID, Name: categoryID.String(), Type: domain.CategoryExpense,
		})
		h.store.SeedBudget(domain.Budget{
			Id: uuid.New(), UserId: h.userID, CategoryId: categoryID, Month: "2026-08", Limit: money("10.00"),
		})
	}

	budgets, err := h.budgets.List(h.ctx(), h.userID, text("2026-08"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(budgets) != 2 {
		t.Fatalf("budgets = %d, want 2", len(budgets))
	}
	if budgets[0].CategoryID != high {
		t.Errorf("first categoryId = %s, want %s", budgets[0].CategoryID, high)
	}
}
