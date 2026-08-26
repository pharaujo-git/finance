using FinanceTracker.Application.Common;
using FinanceTracker.Application.Dtos;
using FinanceTracker.Domain;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Services;

public sealed class BudgetServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task SpentCountsOnlyThatMonthsExpensesInThatCategory()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);
        var groceries = await _harness.CreateCategoryAsync(userId, "Groceries");
        var other = await _harness.CreateCategoryAsync(userId, "Transport");

        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 60m, TestHarness.Utc(2026, 3, 2), groceries.Id);
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 40.25m, TestHarness.Utc(2026, 3, 28), groceries.Id);
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 99m, TestHarness.Utc(2026, 4, 1), groceries.Id);
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Expense, 30m, TestHarness.Utc(2026, 3, 10), other.Id);
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Income, 500m, TestHarness.Utc(2026, 3, 10), groceries.Id);

        await _harness.Budgets.CreateAsync(
            userId,
            new CreateBudgetRequest { CategoryId = groceries.Id, Month = "2026-03", Limit = 150m },
            CancellationToken.None);

        var budget = Assert.Single(await _harness.Budgets.ListAsync(userId, "2026-03", CancellationToken.None));

        Assert.Equal(100.25m, budget.Spent);
        Assert.Equal(150m, budget.Limit);
        Assert.Equal(49.75m, budget.Remaining);
    }

    [Fact]
    public async Task DuplicateBudgetForSameCategoryAndMonthConflicts()
    {
        var userId = await _harness.CreateUserAsync();
        var category = await _harness.CreateCategoryAsync(userId);
        var request = new CreateBudgetRequest { CategoryId = category.Id, Month = "2026-05", Limit = 10m };

        await _harness.Budgets.CreateAsync(userId, request, CancellationToken.None);

        var error = await Assert.ThrowsAsync<AppException>(
            () => _harness.Budgets.CreateAsync(userId, request, CancellationToken.None));

        Assert.Equal(ErrorKind.Conflict, error.Kind);
    }

    [Fact]
    public async Task UpdateChangesLimitAndDeleteRemovesBudget()
    {
        var userId = await _harness.CreateUserAsync();
        var category = await _harness.CreateCategoryAsync(userId);
        var created = await _harness.Budgets.CreateAsync(
            userId,
            new CreateBudgetRequest { CategoryId = category.Id, Month = "2026-06", Limit = 100m },
            CancellationToken.None);

        var updated = await _harness.Budgets.UpdateAsync(
            userId,
            created.Id,
            new UpdateBudgetRequest { Limit = 250m },
            CancellationToken.None);

        Assert.Equal(250m, updated.Limit);
        Assert.Equal(250m, updated.Remaining);

        await _harness.Budgets.DeleteAsync(userId, created.Id, CancellationToken.None);
        Assert.Empty(await _harness.Budgets.ListAsync(userId, "2026-06", CancellationToken.None));
    }

    [Fact]
    public async Task InvalidMonthIsRejected()
    {
        var userId = await _harness.CreateUserAsync();

        var error = await Assert.ThrowsAsync<AppException>(
            () => _harness.Budgets.ListAsync(userId, "not-a-month", CancellationToken.None));

        Assert.Equal(ErrorKind.Validation, error.Kind);
    }

    [Fact]
    public async Task ListWithoutMonthUsesCurrentMonth()
    {
        var userId = await _harness.CreateUserAsync();
        var category = await _harness.CreateCategoryAsync(userId);
        await _harness.Budgets.CreateAsync(
            userId,
            new CreateBudgetRequest
            {
                CategoryId = category.Id,
                Month = MonthKey.From(DateTime.UtcNow),
                Limit = 500m,
            },
            CancellationToken.None);

        Assert.Single(await _harness.Budgets.ListAsync(userId, month: null, CancellationToken.None));
    }

    public void Dispose() => _harness.Dispose();
}
