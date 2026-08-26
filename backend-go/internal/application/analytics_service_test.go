package application_test

import (
	"testing"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestSummaryUsesTheCurrentMonthForFlowAndAllTimeForNetWorth(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	checking := h.seedAccount("Checking", "1000.00")
	savings := h.seedAccount("Savings", "500.00")

	// Last month, which counts towards net worth but not the month's flow.
	h.addTransaction(checking.ID, domain.TransactionIncome, "300.00", date(2026, 7, 15))
	// This month.
	h.addTransaction(checking.ID, domain.TransactionIncome, "2000.00", date(2026, 8, 1))
	h.addTransaction(checking.ID, domain.TransactionExpense, "500.00", date(2026, 8, 2))
	// A transfer moves money between the caller's own accounts, so it cancels
	// out of net worth and counts as neither income nor expense.
	h.addTransaction(checking.ID, domain.TransactionTransfer, "250.00", date(2026, 8, 3),
		withTransferTo(savings.ID))

	summary, err := h.analytics.Summary(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	requireMoney(t, summary.TotalIncome, "2000.00")
	requireMoney(t, summary.TotalExpenses, "500.00")
	requireMoney(t, summary.NetWorth, "3300.00")
	// (2000 - 500) / 2000 = 0.75, and a .NET decimal division keeps no trailing
	// zeros, so the JSON is 0.75 rather than 0.7500.
	requireMoney(t, summary.SavingsRate, "0.75")
}

func TestSummaryRoundsTheSavingsRateToFourPlaces(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	h.addTransaction(account.ID, domain.TransactionIncome, "3.00", date(2026, 8, 1))
	h.addTransaction(account.ID, domain.TransactionExpense, "1.00", date(2026, 8, 2))

	summary, err := h.analytics.Summary(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	requireMoney(t, summary.SavingsRate, "0.6667")
}

func TestSummaryWithoutIncomeReportsAZeroRate(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	h.addTransaction(account.ID, domain.TransactionExpense, "10.00", date(2026, 8, 1))

	summary, err := h.analytics.Summary(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	requireMoney(t, summary.SavingsRate, "0")
	requireMoney(t, summary.NetWorth, "-10.00")
}

func TestNetWorthWalksTheTrailingMonths(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "100.00")
	h.addTransaction(account.ID, domain.TransactionIncome, "50.00", date(2026, 7, 10))
	h.addTransaction(account.ID, domain.TransactionExpense, "20.00", date(2026, 8, 10))

	points, err := h.analytics.NetWorth(h.ctx(), h.userID, 3)
	if err != nil {
		t.Fatalf("NetWorth: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("points = %d, want 3", len(points))
	}

	if points[0].Month != "2026-06" || points[2].Month != "2026-08" {
		t.Errorf("months = %s..%s, want 2026-06..2026-08", points[0].Month, points[2].Month)
	}
	requireMoney(t, points[0].Value, "100.00")
	requireMoney(t, points[1].Value, "150.00")
	requireMoney(t, points[2].Value, "130.00")
}

func TestNetWorthClampsTheWindow(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	h.seedAccount("Checking", "0.00")

	short, err := h.analytics.NetWorth(h.ctx(), h.userID, -5)
	if err != nil {
		t.Fatalf("NetWorth: %v", err)
	}
	if len(short) != 1 {
		t.Errorf("points = %d, want the window clamped up to 1", len(short))
	}

	long, err := h.analytics.NetWorth(h.ctx(), h.userID, 500)
	if err != nil {
		t.Fatalf("NetWorth: %v", err)
	}
	if len(long) != 120 {
		t.Errorf("points = %d, want the window clamped down to 120", len(long))
	}
}

func TestCashflowBucketsByMonth(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	h.addTransaction(account.ID, domain.TransactionIncome, "100.00", date(2026, 7, 1))
	h.addTransaction(account.ID, domain.TransactionExpense, "40.00", date(2026, 7, 2))
	h.addTransaction(account.ID, domain.TransactionIncome, "200.00", date(2026, 8, 1))

	points, err := h.analytics.Cashflow(h.ctx(), h.userID, 2)
	if err != nil {
		t.Fatalf("Cashflow: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("points = %d, want 2", len(points))
	}

	requireMoney(t, points[0].Income, "100.00")
	requireMoney(t, points[0].Expenses, "40.00")
	requireMoney(t, points[1].Income, "200.00")
	// An empty bucket renders as 0, the way default(decimal) does.
	requireMoney(t, points[1].Expenses, "0")
}

func TestSpendingGroupsAMonthsExpensesLargestFirst(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	coffee := h.seedCategory("Coffee", domain.CategoryExpense)
	rent := h.seedCategory("Rent", domain.CategoryExpense)

	h.addTransaction(account.ID, domain.TransactionExpense, "4.00", date(2026, 8, 1), withCategory(coffee.ID))
	h.addTransaction(account.ID, domain.TransactionExpense, "6.00", date(2026, 8, 2), withCategory(coffee.ID))
	h.addTransaction(account.ID, domain.TransactionExpense, "900.00", date(2026, 8, 3), withCategory(rent.ID))
	h.addTransaction(account.ID, domain.TransactionExpense, "3.00", date(2026, 8, 4))
	// Excluded: income, and an expense from another month.
	h.addTransaction(account.ID, domain.TransactionIncome, "50.00", date(2026, 8, 5), withCategory(coffee.ID))
	h.addTransaction(account.ID, domain.TransactionExpense, "77.00", date(2026, 9, 1), withCategory(coffee.ID))

	slices, err := h.analytics.Spending(h.ctx(), h.userID, nil)
	if err != nil {
		t.Fatalf("Spending: %v", err)
	}
	if len(slices) != 3 {
		t.Fatalf("slices = %d, want 3", len(slices))
	}

	requireMoney(t, slices[0].Amount, "900.00")
	if slices[0].CategoryName != "Rent" {
		t.Errorf("largest slice = %q, want Rent", slices[0].CategoryName)
	}
	requireMoney(t, slices[1].Amount, "10.00")

	uncategorized := slices[2]
	if uncategorized.CategoryID != nil ||
		uncategorized.CategoryName != "Uncategorized" ||
		uncategorized.Color != "#94a3b8" {
		t.Errorf("last slice = %+v, want the uncategorized fallback", uncategorized)
	}
}

func TestSpendingAcceptsAMonthKey(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	h.addTransaction(account.ID, domain.TransactionExpense, "12.00", date(2026, 7, 4))

	slices, err := h.analytics.Spending(h.ctx(), h.userID, text("2026-07"))
	if err != nil {
		t.Fatalf("Spending: %v", err)
	}
	if len(slices) != 1 {
		t.Fatalf("slices = %d, want 1", len(slices))
	}
	requireMoney(t, slices[0].Amount, "12.00")

	_, err = h.analytics.Spending(h.ctx(), h.userID, text("nope"))
	requireAppError(t, err, domain.KindValidation, "Month must be in YYYY-MM format.")
}

func TestMonthlyReportCoversTwelveMonths(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	h.addTransaction(account.ID, domain.TransactionIncome, "100.00", date(2026, 1, 1))
	h.addTransaction(account.ID, domain.TransactionExpense, "30.00", date(2026, 1, 31))
	// The last instant of the year is inside the window.
	h.addTransaction(account.ID, domain.TransactionIncome, "10.00",
		date(2026, 12, 31).Add(23*60*60*1e9))
	// The next year is not.
	h.addTransaction(account.ID, domain.TransactionIncome, "999.00", date(2027, 1, 1))

	months, err := h.analytics.MonthlyReport(h.ctx(), h.userID, 2026)
	if err != nil {
		t.Fatalf("MonthlyReport: %v", err)
	}
	if len(months) != 12 {
		t.Fatalf("months = %d, want 12", len(months))
	}

	requireMoney(t, months[0].Income, "100.00")
	requireMoney(t, months[0].Expenses, "30.00")
	requireMoney(t, months[0].Net, "70.00")
	requireMoney(t, months[11].Income, "10.00")
	if months[5].Month != "2026-06" {
		t.Errorf("sixth month = %q, want 2026-06", months[5].Month)
	}
}

func TestMonthlyReportRejectsAnImpossibleYear(t *testing.T) {
	t.Parallel()

	h := newServices(t)

	for _, year := range []int{1899, 10000} {
		_, err := h.analytics.MonthlyReport(h.ctx(), h.userID, year)
		requireAppError(t, err, domain.KindValidation, "Year must be between 1900 and 9999.")
	}
}

func TestCategoryReportExcludesTransfersAndTypesTheUncategorized(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	checking := h.seedAccount("Checking", "0.00")
	savings := h.seedAccount("Savings", "0.00")
	coffee := h.seedCategory("Coffee", domain.CategoryExpense)

	h.addTransaction(checking.ID, domain.TransactionExpense, "10.00", date(2026, 8, 1), withCategory(coffee.ID))
	h.addTransaction(checking.ID, domain.TransactionIncome, "500.00", date(2026, 8, 2))
	h.addTransaction(checking.ID, domain.TransactionTransfer, "99.00", date(2026, 8, 3),
		withTransferTo(savings.ID))

	report, err := h.analytics.CategoryReport(h.ctx(), h.userID, nil, nil)
	if err != nil {
		t.Fatalf("CategoryReport: %v", err)
	}
	if len(report) != 2 {
		t.Fatalf("rows = %d, want 2 (the transfer is excluded)", len(report))
	}

	// Largest first: the uncategorized income.
	if report[0].CategoryID != nil || report[0].Type != domain.CategoryIncome {
		t.Errorf("first row = %+v, want an uncategorized income row", report[0])
	}
	requireMoney(t, report[0].Amount, "500.00")

	if report[1].CategoryName != "Coffee" || report[1].Type != domain.CategoryExpense {
		t.Errorf("second row = %+v, want the Coffee expense", report[1])
	}
}

func TestCategoryReportHonoursTheWindow(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	h.addTransaction(account.ID, domain.TransactionExpense, "1.00", date(2026, 1, 1))
	h.addTransaction(account.ID, domain.TransactionExpense, "2.00", date(2026, 8, 1))

	from := date(2026, 8, 1)
	report, err := h.analytics.CategoryReport(h.ctx(), h.userID, &from, nil)
	if err != nil {
		t.Fatalf("CategoryReport: %v", err)
	}
	if len(report) != 1 {
		t.Fatalf("rows = %d, want 1", len(report))
	}
	requireMoney(t, report[0].Amount, "2.00")
}

func TestAnalyticsAreScopedToTheCaller(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "1000.00")
	h.addTransaction(account.ID, domain.TransactionIncome, "100.00", date(2026, 8, 1))

	summary, err := h.analytics.Summary(h.ctx(), h.otherID)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	requireMoney(t, summary.NetWorth, "0")
	requireMoney(t, summary.TotalIncome, "0")
}
