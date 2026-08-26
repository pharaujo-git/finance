using FinanceTracker.Application.Common;
using FinanceTracker.Domain;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Services;

public sealed class AnalyticsServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task SummaryUsesCurrentMonthIncomeAndExpenses()
    {
        var userId = await _harness.CreateUserAsync();
        var checking = await _harness.CreateAccountAsync(userId, "Checking", 1_000m);
        var savings = await _harness.CreateAccountAsync(userId, "Savings", 2_000m, AccountType.Savings);
        var now = DateTime.UtcNow;
        var thisMonth = MonthKey.StartOfMonth(now).AddDays(1);
        var lastMonth = MonthKey.StartOfMonth(now).AddMonths(-1).AddDays(2);

        await _harness.AddTransactionAsync(userId, checking.Id, TransactionType.Income, 4_000m, thisMonth);
        await _harness.AddTransactionAsync(userId, checking.Id, TransactionType.Expense, 1_000m, thisMonth);
        await _harness.AddTransactionAsync(userId, checking.Id, TransactionType.Expense, 500m, lastMonth);
        await _harness.AddTransactionAsync(
            userId, checking.Id, TransactionType.Transfer, 250m, thisMonth, transferAccountId: savings.Id);

        var summary = await _harness.Analytics.GetSummaryAsync(userId, CancellationToken.None);

        Assert.Equal(4_000m, summary.TotalIncome);
        Assert.Equal(1_000m, summary.TotalExpenses);
        Assert.Equal(0.75m, summary.SavingsRate);
        Assert.Equal(5_500m, summary.NetWorth);
    }

    [Fact]
    public async Task SavingsRateIsZeroWithoutIncome()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId, "Checking", 100m);
        await _harness.AddTransactionAsync(
            userId, account.Id, TransactionType.Expense, 20m, MonthKey.StartOfMonth(DateTime.UtcNow).AddDays(1));

        var summary = await _harness.Analytics.GetSummaryAsync(userId, CancellationToken.None);

        Assert.Equal(0m, summary.SavingsRate);
        Assert.Equal(80m, summary.NetWorth);
    }

    [Fact]
    public async Task NetWorthSeriesAccumulatesMonthByMonth()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId, "Checking", 100m);
        var thisMonthStart = MonthKey.StartOfMonth(DateTime.UtcNow);

        await _harness.AddTransactionAsync(
            userId, account.Id, TransactionType.Income, 50m, thisMonthStart.AddMonths(-1).AddDays(3));
        await _harness.AddTransactionAsync(
            userId, account.Id, TransactionType.Income, 25m, thisMonthStart.AddDays(1));

        var series = await _harness.Analytics.GetNetWorthAsync(userId, 3, CancellationToken.None);

        Assert.Equal(3, series.Count);
        Assert.Equal(MonthKey.From(thisMonthStart), series[^1].Month);
        Assert.Equal(100m, series[0].Value);
        Assert.Equal(150m, series[1].Value);
        Assert.Equal(175m, series[2].Value);
    }

    [Fact]
    public async Task CashflowSplitsIncomeAndExpensesPerMonth()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);
        var start = MonthKey.StartOfMonth(DateTime.UtcNow);

        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Income, 300m, start.AddDays(2));
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 120m, start.AddDays(3));
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Income, 90m, start.AddMonths(-1).AddDays(1));

        var series = await _harness.Analytics.GetCashflowAsync(userId, 2, CancellationToken.None);

        Assert.Equal(2, series.Count);
        Assert.Equal(90m, series[0].Income);
        Assert.Equal(0m, series[0].Expenses);
        Assert.Equal(300m, series[1].Income);
        Assert.Equal(120m, series[1].Expenses);
    }

    [Fact]
    public async Task SpendingGroupsExpensesByCategoryAndKeepsColours()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);
        var rent = await _harness.CreateCategoryAsync(userId, "Rent");

        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 800m, TestHarness.Utc(2026, 2, 1), rent.Id);
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 40m, TestHarness.Utc(2026, 2, 5));
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Income, 900m, TestHarness.Utc(2026, 2, 6), rent.Id);
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 10m, TestHarness.Utc(2026, 3, 1), rent.Id);

        var slices = await _harness.Analytics.GetSpendingAsync(userId, "2026-02", CancellationToken.None);

        Assert.Equal(2, slices.Count);
        Assert.Equal(800m, slices[0].Amount);
        Assert.Equal("Rent", slices[0].CategoryName);
        Assert.Equal("#123456", slices[0].Color);
        Assert.Equal("Uncategorized", slices[1].CategoryName);
        Assert.Null(slices[1].CategoryId);
    }

    [Fact]
    public async Task MonthlyReportCoversTwelveMonthsOfTheRequestedYear()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);

        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Income, 1_200m, TestHarness.Utc(2026, 1, 10));
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 200m, TestHarness.Utc(2026, 1, 20));
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 75m, TestHarness.Utc(2025, 12, 30));

        var report = await _harness.Analytics.GetMonthlyReportAsync(userId, 2026, CancellationToken.None);

        Assert.Equal(12, report.Count);
        Assert.Equal("2026-01", report[0].Month);
        Assert.Equal(1_200m, report[0].Income);
        Assert.Equal(200m, report[0].Expenses);
        Assert.Equal(1_000m, report[0].Net);
        Assert.All(report.Skip(1), m => Assert.Equal(0m, m.Income + m.Expenses));
    }

    [Fact]
    public async Task MonthlyReportRejectsImplausibleYears()
    {
        var userId = await _harness.CreateUserAsync();

        var error = await Assert.ThrowsAsync<AppException>(
            () => _harness.Analytics.GetMonthlyReportAsync(userId, 12, CancellationToken.None));

        Assert.Equal(ErrorKind.Validation, error.Kind);
    }

    [Fact]
    public async Task CategoryReportGroupsBothDirectionsAndExcludesTransfers()
    {
        var userId = await _harness.CreateUserAsync();
        var checking = await _harness.CreateAccountAsync(userId);
        var savings = await _harness.CreateAccountAsync(userId, "Savings", 0m, AccountType.Savings);
        var salary = await _harness.CreateCategoryAsync(userId, "Salary", CategoryType.Income);
        var food = await _harness.CreateCategoryAsync(userId, "Food");

        await _harness.AddTransactionAsync(userId, checking.Id, TransactionType.Income, 2_000m, TestHarness.Utc(2026, 1, 5), salary.Id);
        await _harness.AddTransactionAsync(userId, checking.Id, TransactionType.Expense, 150m, TestHarness.Utc(2026, 1, 6), food.Id);
        await _harness.AddTransactionAsync(
            userId, checking.Id, TransactionType.Transfer, 500m, TestHarness.Utc(2026, 1, 7), transferAccountId: savings.Id);

        var report = await _harness.Analytics.GetCategoryReportAsync(
            userId,
            TestHarness.Utc(2026, 1, 1),
            TestHarness.Utc(2026, 1, 31),
            CancellationToken.None);

        Assert.Equal(2, report.Count);
        Assert.Equal(2_000m, report[0].Amount);
        Assert.Equal(CategoryType.Income, report[0].Type);
        Assert.Equal("Food", report[1].CategoryName);
        Assert.Equal(CategoryType.Expense, report[1].Type);
    }

    public void Dispose() => _harness.Dispose();
}
