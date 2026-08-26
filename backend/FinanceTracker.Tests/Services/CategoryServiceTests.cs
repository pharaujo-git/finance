using FinanceTracker.Api.Common;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using FinanceTracker.Tests.Infrastructure;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Tests.Services;

public sealed class CategoryServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task ListReturnsSeededDefaultsPlusOwnCategories()
    {
        await _harness.SeedDefaultCategoriesAsync();
        var userId = await _harness.CreateUserAsync();
        var stranger = await _harness.CreateUserAsync("stranger@example.com");

        await _harness.CreateCategoryAsync(userId, "Board games");
        await _harness.CreateCategoryAsync(stranger, "Private thing");

        var visible = await _harness.Categories.ListAsync(userId, CancellationToken.None);

        Assert.Contains(visible, c => c is { Name: "Groceries", IsDefault: true });
        Assert.Contains(visible, c => c is { Name: "Board games", IsDefault: false });
        Assert.DoesNotContain(visible, c => c.Name == "Private thing");
    }

    [Fact]
    public async Task SeedingIsIdempotent()
    {
        await _harness.SeedDefaultCategoriesAsync();
        await _harness.SeedDefaultCategoriesAsync();

        var names = await _harness.Db.Categories.Where(c => c.IsDefault).Select(c => c.Name).ToListAsync();

        Assert.Equal(names.Count, names.Distinct().Count());
        Assert.All(await _harness.Db.Categories.Where(c => c.IsDefault).ToListAsync(), c => Assert.Null(c.UserId));
    }

    [Fact]
    public async Task DefaultCategoriesCannotBeDeletedOrUpdated()
    {
        await _harness.SeedDefaultCategoriesAsync();
        var userId = await _harness.CreateUserAsync();
        var seeded = (await _harness.Categories.ListAsync(userId, CancellationToken.None)).First(c => c.IsDefault);

        var deleteError = await Assert.ThrowsAsync<ApiException>(
            () => _harness.Categories.DeleteAsync(userId, seeded.Id, CancellationToken.None));
        Assert.Equal(StatusCodes.Status400BadRequest, deleteError.StatusCode);

        var updateError = await Assert.ThrowsAsync<ApiException>(() => _harness.Categories.UpdateAsync(
            userId,
            seeded.Id,
            new CategoryRequest { Name = "Hijack", Type = CategoryType.Expense },
            CancellationToken.None));
        Assert.Equal(StatusCodes.Status400BadRequest, updateError.StatusCode);
    }

    [Fact]
    public async Task UpdateAndDeleteWorkForOwnedCategories()
    {
        var userId = await _harness.CreateUserAsync();
        var category = await _harness.CreateCategoryAsync(userId, "Snacks");

        var updated = await _harness.Categories.UpdateAsync(
            userId,
            category.Id,
            new CategoryRequest { Name = "Treats", Type = CategoryType.Expense, Icon = "candy", Color = "#abcdef" },
            CancellationToken.None);

        Assert.Equal("Treats", updated.Name);
        Assert.Equal("candy", updated.Icon);

        await _harness.Categories.DeleteAsync(userId, category.Id, CancellationToken.None);
        Assert.Empty(await _harness.Categories.ListAsync(userId, CancellationToken.None));
    }

    [Fact]
    public async Task DeletingCategoryDetachesTransactionsAndRemovesBudgets()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);
        var category = await _harness.CreateCategoryAsync(userId, "Doomed");

        var transaction = await _harness.AddTransactionAsync(
            userId,
            account.Id,
            TransactionType.Expense,
            25m,
            TestHarness.Utc(2026, 4, 3),
            category.Id);

        await _harness.Budgets.CreateAsync(
            userId,
            new CreateBudgetRequest { CategoryId = category.Id, Month = "2026-04", Limit = 100m },
            CancellationToken.None);

        await _harness.Categories.DeleteAsync(userId, category.Id, CancellationToken.None);

        var reloaded = await _harness.Transactions.GetAsync(userId, transaction.Id, CancellationToken.None);
        Assert.Null(reloaded.CategoryId);
        Assert.Empty(await _harness.Budgets.ListAsync(userId, "2026-04", CancellationToken.None));
    }

    [Fact]
    public async Task DeletingUnknownCategoryThrowsNotFound()
    {
        var userId = await _harness.CreateUserAsync();

        var error = await Assert.ThrowsAsync<ApiException>(
            () => _harness.Categories.DeleteAsync(userId, Guid.NewGuid(), CancellationToken.None));

        Assert.Equal(StatusCodes.Status404NotFound, error.StatusCode);
    }

    public void Dispose() => _harness.Dispose();
}
