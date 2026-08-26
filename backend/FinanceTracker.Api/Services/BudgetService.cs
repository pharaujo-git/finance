using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>Monthly per-category budgets with computed spend.</summary>
public sealed class BudgetService(AppDbContext db, CategoryService categories)
{
    private const string EntityName = "Budget";

    public async Task<IReadOnlyList<BudgetDto>> ListAsync(
        Guid userId,
        string? month,
        CancellationToken cancellationToken)
    {
        var key = month is null ? MonthKey.From(DateTime.UtcNow) : Validate(month);
        var start = MonthKey.Parse(key);
        var end = start.AddMonths(1);

        var budgets = await db.Budgets
            .AsNoTracking()
            .OwnedBy(userId)
            .Where(b => b.Month == key)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        if (budgets.Count == 0)
        {
            return [];
        }

        var spentByCategory = await SpentByCategoryAsync(userId, start, end, cancellationToken).ConfigureAwait(false);

        return budgets
            .Select(b => Map(b, spentByCategory.GetValueOrDefault(b.CategoryId)))
            .OrderBy(b => b.CategoryId)
            .ToList();
    }

    public async Task<BudgetDto> CreateAsync(
        Guid userId,
        CreateBudgetRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var key = Validate(request.Month);
        await categories.EnsureUsableAsync(userId, request.CategoryId, cancellationToken).ConfigureAwait(false);

        var duplicate = await db.Budgets
            .AsNoTracking()
            .OwnedBy(userId)
            .AnyAsync(b => b.CategoryId == request.CategoryId && b.Month == key, cancellationToken)
            .ConfigureAwait(false);

        if (duplicate)
        {
            throw ApiException.Conflict("A budget already exists for that category and month.");
        }

        var budget = new Budget
        {
            UserId = userId,
            CategoryId = request.CategoryId,
            Month = key,
            Limit = decimal.Round(request.Limit, 2, MidpointRounding.AwayFromZero),
        };

        db.Budgets.Add(budget);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return await BuildAsync(userId, budget, cancellationToken).ConfigureAwait(false);
    }

    public async Task<BudgetDto> UpdateAsync(
        Guid userId,
        Guid id,
        UpdateBudgetRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var budget = await db.Budgets.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        budget.Limit = decimal.Round(request.Limit, 2, MidpointRounding.AwayFromZero);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return await BuildAsync(userId, budget, cancellationToken).ConfigureAwait(false);
    }

    public async Task DeleteAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var budget = await db.Budgets.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        db.Budgets.Remove(budget);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
    }

    private async Task<BudgetDto> BuildAsync(Guid userId, Budget budget, CancellationToken cancellationToken)
    {
        var start = MonthKey.Parse(budget.Month);
        var spent = await SpentByCategoryAsync(userId, start, start.AddMonths(1), cancellationToken)
            .ConfigureAwait(false);

        return Map(budget, spent.GetValueOrDefault(budget.CategoryId));
    }

    private async Task<Dictionary<Guid, decimal>> SpentByCategoryAsync(
        Guid userId,
        DateTime start,
        DateTime end,
        CancellationToken cancellationToken)
    {
        var rows = await db.Transactions
            .AsNoTracking()
            .OwnedBy(userId)
            .Where(t => t.Type == TransactionType.Expense
                        && t.CategoryId != null
                        && t.Date >= start
                        && t.Date < end)
            .Select(t => new { CategoryId = t.CategoryId!.Value, t.Amount })
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        return rows
            .GroupBy(r => r.CategoryId)
            .ToDictionary(g => g.Key, g => g.Sum(r => r.Amount));
    }

    private static string Validate(string month) => MonthKey.TryParse(month, out _)
        ? month
        : throw ApiException.BadRequest("Month must be in YYYY-MM format.");

    private static BudgetDto Map(Budget budget, decimal spent) =>
        new(budget.Id, budget.CategoryId, budget.Month, budget.Limit, spent, budget.Limit - spent);
}
