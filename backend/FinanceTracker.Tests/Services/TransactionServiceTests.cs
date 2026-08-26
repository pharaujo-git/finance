using FinanceTracker.Api.Common;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Services;

public sealed class TransactionServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task SearchSortsByDateDescendingAndPages()
    {
        var (userId, accountId) = await SeedAsync();

        for (var day = 1; day <= 5; day++)
        {
            await _harness.AddTransactionAsync(
                userId,
                accountId,
                TransactionType.Expense,
                day,
                TestHarness.Utc(2026, 1, day),
                description: $"Day {day}");
        }

        var page = await _harness.Transactions.SearchAsync(
            userId,
            new TransactionQuery { Page = 1, PageSize = 2 },
            CancellationToken.None);

        Assert.Equal(5, page.Total);
        Assert.Equal(1, page.Page);
        Assert.Equal(2, page.PageSize);
        Assert.Equal(["Day 5", "Day 4"], page.Items.Select(i => i.Description));

        var second = await _harness.Transactions.SearchAsync(
            userId,
            new TransactionQuery { Page = 2, PageSize = 2 },
            CancellationToken.None);

        Assert.Equal(["Day 3", "Day 2"], second.Items.Select(i => i.Description));
    }

    [Fact]
    public async Task SearchFiltersByTypeCategoryDateRangeAndText()
    {
        var (userId, accountId) = await SeedAsync();
        var category = await _harness.CreateCategoryAsync(userId, "Coffee");

        await _harness.AddTransactionAsync(
            userId, accountId, TransactionType.Expense, 4m, TestHarness.Utc(2026, 1, 10), category.Id, description: "Flat white");
        await _harness.AddTransactionAsync(
            userId, accountId, TransactionType.Income, 900m, TestHarness.Utc(2026, 2, 1), description: "Payday");
        await _harness.AddTransactionAsync(
            userId, accountId, TransactionType.Expense, 60m, TestHarness.Utc(2026, 3, 4), description: "Shoes", tags: ["clothing"]);

        var expenses = await Search(userId, new TransactionQuery { Type = TransactionType.Expense });
        Assert.Equal(2, expenses.Total);

        var byCategory = await Search(userId, new TransactionQuery { CategoryId = category.Id });
        Assert.Equal("Flat white", Assert.Single(byCategory.Items).Description);

        var byRange = await Search(userId, new TransactionQuery
        {
            From = TestHarness.Utc(2026, 2, 1),
            To = TestHarness.Utc(2026, 2, 28),
        });
        Assert.Equal("Payday", Assert.Single(byRange.Items).Description);

        var byText = await Search(userId, new TransactionQuery { Search = "SHOE" });
        Assert.Equal("Shoes", Assert.Single(byText.Items).Description);

        var byTag = await Search(userId, new TransactionQuery { Search = "clothing" });
        Assert.Single(byTag.Items);
    }

    [Fact]
    public async Task SearchByAccountIncludesIncomingTransfers()
    {
        var (userId, accountId) = await SeedAsync();
        var savings = await _harness.CreateAccountAsync(userId, "Savings", 0m, AccountType.Savings);

        await _harness.AddTransactionAsync(
            userId,
            accountId,
            TransactionType.Transfer,
            100m,
            TestHarness.Utc(2026, 5, 1),
            transferAccountId: savings.Id);

        var results = await Search(userId, new TransactionQuery { AccountId = savings.Id });
        Assert.Single(results.Items);
    }

    [Fact]
    public async Task CreateRoundTripsTagsAndNotes()
    {
        var (userId, accountId) = await SeedAsync();

        var created = await _harness.AddTransactionAsync(
            userId,
            accountId,
            TransactionType.Expense,
            19.999m,
            TestHarness.Utc(2026, 6, 6),
            tags: ["food", " travel "],
            notes: " remember this ");

        Assert.Equal(20.00m, created.Amount);
        Assert.Equal(["food", "travel"], created.Tags);
        Assert.Equal("remember this", created.Notes);
        Assert.Equal(DateTimeKind.Utc, created.Date.Kind);
    }

    [Fact]
    public async Task TransferRequiresDistinctDestinationAccount()
    {
        var (userId, accountId) = await SeedAsync();

        var missingTarget = await Assert.ThrowsAsync<ApiException>(() => _harness.AddTransactionAsync(
            userId, accountId, TransactionType.Transfer, 10m, TestHarness.Utc(2026, 1, 1)));
        Assert.Equal(StatusCodes.Status400BadRequest, missingTarget.StatusCode);

        var sameAccount = await Assert.ThrowsAsync<ApiException>(() => _harness.AddTransactionAsync(
            userId, accountId, TransactionType.Transfer, 10m, TestHarness.Utc(2026, 1, 1), transferAccountId: accountId));
        Assert.Equal(StatusCodes.Status400BadRequest, sameAccount.StatusCode);
    }

    [Fact]
    public async Task CreateRejectsForeignAccountAndCategory()
    {
        var (userId, accountId) = await SeedAsync();
        var stranger = await _harness.CreateUserAsync("stranger@example.com");
        var strangerCategory = await _harness.CreateCategoryAsync(stranger, "Theirs");

        var badAccount = await Assert.ThrowsAsync<ApiException>(() => _harness.AddTransactionAsync(
            userId, Guid.NewGuid(), TransactionType.Expense, 5m, TestHarness.Utc(2026, 1, 1)));
        Assert.Equal(StatusCodes.Status404NotFound, badAccount.StatusCode);

        var badCategory = await Assert.ThrowsAsync<ApiException>(() => _harness.AddTransactionAsync(
            userId, accountId, TransactionType.Expense, 5m, TestHarness.Utc(2026, 1, 1), strangerCategory.Id));
        Assert.Equal(StatusCodes.Status404NotFound, badCategory.StatusCode);
    }

    [Fact]
    public async Task UpdateAndDeleteAreScopedToTheOwner()
    {
        var (userId, accountId) = await SeedAsync();
        var stranger = await _harness.CreateUserAsync("stranger@example.com");
        var created = await _harness.AddTransactionAsync(
            userId, accountId, TransactionType.Expense, 5m, TestHarness.Utc(2026, 7, 7));

        var request = new TransactionRequest
        {
            AccountId = accountId,
            Type = TransactionType.Income,
            Amount = 42m,
            Date = TestHarness.Utc(2026, 7, 8),
            Description = "Corrected",
        };

        var updated = await _harness.Transactions.UpdateAsync(userId, created.Id, request, CancellationToken.None);
        Assert.Equal(TransactionType.Income, updated.Type);
        Assert.Equal(42m, updated.Amount);
        Assert.Empty(updated.Tags);

        var foreignUpdate = await Assert.ThrowsAsync<ApiException>(
            () => _harness.Transactions.UpdateAsync(stranger, created.Id, request, CancellationToken.None));
        Assert.Equal(StatusCodes.Status404NotFound, foreignUpdate.StatusCode);

        await _harness.Transactions.DeleteAsync(userId, created.Id, CancellationToken.None);
        var afterDelete = await Search(userId, new TransactionQuery());
        Assert.Equal(0, afterDelete.Total);
    }

    private async Task<PagedResult<TransactionDto>> Search(Guid userId, TransactionQuery query) =>
        await _harness.Transactions.SearchAsync(userId, query, CancellationToken.None);

    private async Task<(Guid UserId, Guid AccountId)> SeedAsync()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);
        return (userId, account.Id);
    }

    public void Dispose() => _harness.Dispose();
}
