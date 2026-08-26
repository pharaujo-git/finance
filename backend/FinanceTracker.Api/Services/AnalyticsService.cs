using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>
/// Aggregations behind the dashboard and report endpoints. Every figure is derived from the
/// same <see cref="TransactionSlice"/> projection so the numbers cannot drift apart.
/// </summary>
public sealed class AnalyticsService(AppDbContext db, TransactionService transactions)
{
    private const string UncategorizedName = "Uncategorized";

    private const string UncategorizedColor = "#94a3b8";

    public async Task<DashboardSummaryDto> GetSummaryAsync(Guid userId, CancellationToken cancellationToken)
    {
        var slices = await transactions.LoadSlicesAsync(userId, null, null, cancellationToken).ConfigureAwait(false);
        var openingBalance = await OpeningBalanceAsync(userId, cancellationToken).ConfigureAwait(false);

        var monthStart = MonthKey.StartOfMonth(DateTime.UtcNow);
        var monthEnd = monthStart.AddMonths(1);
        var thisMonth = slices.Where(s => s.Date >= monthStart && s.Date < monthEnd).ToList();

        var income = Total(thisMonth, TransactionType.Income);
        var expenses = Total(thisMonth, TransactionType.Expense);
        var netWorth = openingBalance + slices.Sum(BalanceCalculator.NetWorthDelta);
        var savingsRate = income > 0m ? decimal.Round((income - expenses) / income, 4, MidpointRounding.AwayFromZero) : 0m;

        return new DashboardSummaryDto(netWorth, income, expenses, savingsRate);
    }

    public async Task<IReadOnlyList<NetWorthPointDto>> GetNetWorthAsync(
        Guid userId,
        int months,
        CancellationToken cancellationToken)
    {
        var window = MonthKey.TrailingMonths(DateTime.UtcNow, Clamp(months));
        var slices = await transactions.LoadSlicesAsync(userId, null, null, cancellationToken).ConfigureAwait(false);
        var openingBalance = await OpeningBalanceAsync(userId, cancellationToken).ConfigureAwait(false);

        return window
            .Select(start =>
            {
                var end = start.AddMonths(1);
                var value = openingBalance + slices
                    .Where(s => s.Date < end)
                    .Sum(BalanceCalculator.NetWorthDelta);
                return new NetWorthPointDto(MonthKey.From(start), value);
            })
            .ToList();
    }

    public async Task<IReadOnlyList<CashflowPointDto>> GetCashflowAsync(
        Guid userId,
        int months,
        CancellationToken cancellationToken)
    {
        var window = MonthKey.TrailingMonths(DateTime.UtcNow, Clamp(months));
        var slices = await transactions
            .LoadSlicesAsync(userId, window[0], null, cancellationToken)
            .ConfigureAwait(false);

        return window.Select(start => Bucket(slices, start)).Select(b =>
            new CashflowPointDto(b.Month, b.Income, b.Expenses)).ToList();
    }

    public async Task<IReadOnlyList<SpendingSliceDto>> GetSpendingAsync(
        Guid userId,
        string? month,
        CancellationToken cancellationToken)
    {
        var start = month is null ? MonthKey.StartOfMonth(DateTime.UtcNow) : MonthKey.Parse(month);
        var end = start.AddMonths(1);

        var slices = await transactions.LoadSlicesAsync(userId, start, null, cancellationToken).ConfigureAwait(false);
        var lookup = await CategoryLookupAsync(userId, cancellationToken).ConfigureAwait(false);

        return slices
            .Where(s => s.Type == TransactionType.Expense && s.Date < end)
            .GroupBy(s => s.CategoryId)
            .Select(g =>
            {
                var info = Describe(lookup, g.Key);
                return new SpendingSliceDto(g.Key, info.Name, info.Color, g.Sum(s => s.Amount));
            })
            .OrderByDescending(s => s.Amount)
            .ToList();
    }

    public async Task<IReadOnlyList<MonthlyReportDto>> GetMonthlyReportAsync(
        Guid userId,
        int year,
        CancellationToken cancellationToken)
    {
        if (year is < 1900 or > 9999)
        {
            throw ApiException.BadRequest("Year must be between 1900 and 9999.");
        }

        var start = DateTime.SpecifyKind(new DateTime(year, 1, 1), DateTimeKind.Utc);
        var slices = await transactions
            .LoadSlicesAsync(userId, start, start.AddYears(1).AddTicks(-1), cancellationToken)
            .ConfigureAwait(false);

        return Enumerable.Range(0, 12)
            .Select(offset => Bucket(slices, start.AddMonths(offset)))
            .Select(b => new MonthlyReportDto(b.Month, b.Income, b.Expenses, b.Income - b.Expenses))
            .ToList();
    }

    public async Task<IReadOnlyList<CategoryReportDto>> GetCategoryReportAsync(
        Guid userId,
        DateTime? from,
        DateTime? to,
        CancellationToken cancellationToken)
    {
        var slices = await transactions.LoadSlicesAsync(userId, from, to, cancellationToken).ConfigureAwait(false);
        var lookup = await CategoryLookupAsync(userId, cancellationToken).ConfigureAwait(false);

        return slices
            .Where(s => s.Type != TransactionType.Transfer)
            .GroupBy(s => s.CategoryId)
            .Select(g =>
            {
                var info = Describe(lookup, g.Key);
                var type = g.Key is null && g.Any(s => s.Type == TransactionType.Income)
                    ? CategoryType.Income
                    : info.Type;
                return new CategoryReportDto(g.Key, info.Name, type, info.Color, g.Sum(s => s.Amount));
            })
            .OrderByDescending(r => r.Amount)
            .ToList();
    }

    private static int Clamp(int months) => Math.Clamp(months, 1, 120);

    private static decimal Total(IEnumerable<TransactionSlice> slices, TransactionType type) =>
        slices.Where(s => s.Type == type).Sum(s => s.Amount);

    private static MonthBucket Bucket(IReadOnlyList<TransactionSlice> slices, DateTime start)
    {
        var end = start.AddMonths(1);
        var inMonth = slices.Where(s => s.Date >= start && s.Date < end).ToList();
        return new MonthBucket(MonthKey.From(start), Total(inMonth, TransactionType.Income), Total(inMonth, TransactionType.Expense));
    }

    private static CategoryInfo Describe(IReadOnlyDictionary<Guid, CategoryInfo> lookup, Guid? categoryId) =>
        categoryId is { } id && lookup.TryGetValue(id, out var info)
            ? info
            : new CategoryInfo(UncategorizedName, UncategorizedColor, CategoryType.Expense);

    private async Task<decimal> OpeningBalanceAsync(Guid userId, CancellationToken cancellationToken)
    {
        var balances = await db.Accounts
            .AsNoTracking()
            .OwnedBy(userId)
            .Select(a => a.InitialBalance)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        return balances.Sum();
    }

    private async Task<Dictionary<Guid, CategoryInfo>> CategoryLookupAsync(
        Guid userId,
        CancellationToken cancellationToken)
    {
        var rows = await CategoryService
            .VisibleTo(db.Categories.AsNoTracking(), userId)
            .Select(c => new { c.Id, c.Name, c.Color, c.Type })
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        return rows.ToDictionary(
            r => r.Id,
            r => new CategoryInfo(r.Name, string.IsNullOrEmpty(r.Color) ? UncategorizedColor : r.Color, r.Type));
    }

    private sealed record CategoryInfo(string Name, string Color, CategoryType Type);

    private sealed record MonthBucket(string Month, decimal Income, decimal Expenses);
}
