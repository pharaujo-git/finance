using FinanceTracker.Application.Common;
using FinanceTracker.Application.Dtos;
using FinanceTracker.Domain;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Services;

public sealed class AccountServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task BalanceCombinesInitialBalanceIncomeExpensesAndTransfers()
    {
        var userId = await _harness.CreateUserAsync();
        var checking = await _harness.CreateAccountAsync(userId, "Checking", 1_000m);
        var savings = await _harness.CreateAccountAsync(userId, "Savings", 500m, AccountType.Savings);

        await _harness.AddTransactionAsync(userId, checking.Id, TransactionType.Income, 200m, TestHarness.Utc(2026, 1, 5));
        await _harness.AddTransactionAsync(userId, checking.Id, TransactionType.Expense, 75.50m, TestHarness.Utc(2026, 1, 6));
        await _harness.AddTransactionAsync(
            userId,
            checking.Id,
            TransactionType.Transfer,
            300m,
            TestHarness.Utc(2026, 1, 7),
            transferAccountId: savings.Id);

        var accounts = await _harness.Accounts.ListAsync(userId, CancellationToken.None);

        Assert.Equal(824.50m, accounts.Single(a => a.Id == checking.Id).Balance);
        Assert.Equal(800m, accounts.Single(a => a.Id == savings.Id).Balance);
    }

    [Fact]
    public async Task BalancesNeverLeakBetweenUsers()
    {
        var owner = await _harness.CreateUserAsync("owner@example.com");
        var stranger = await _harness.CreateUserAsync("stranger@example.com");
        var account = await _harness.CreateAccountAsync(owner, "Checking", 100m);

        await _harness.AddTransactionAsync(owner, account.Id, TransactionType.Income, 50m, TestHarness.Utc(2026, 2, 1));

        Assert.Empty(await _harness.Accounts.ListAsync(stranger, CancellationToken.None));

        var error = await Assert.ThrowsAsync<AppException>(
            () => _harness.Accounts.GetAsync(stranger, account.Id, CancellationToken.None));

        Assert.Equal(ErrorKind.NotFound, error.Kind);
    }

    [Fact]
    public async Task UpdateChangesMetadataAndKeepsBalance()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId, "Old name", 40m);
        await _harness.AddTransactionAsync(userId, account.Id, TransactionType.Income, 10m, TestHarness.Utc(2026, 3, 1));

        var updated = await _harness.Accounts.UpdateAsync(
            userId,
            account.Id,
            new UpdateAccountRequest
            {
                Name = "New name",
                Type = AccountType.Investment,
                Currency = "gbp",
                IsArchived = false,
            },
            CancellationToken.None);

        Assert.Equal("New name", updated.Name);
        Assert.Equal(AccountType.Investment, updated.Type);
        Assert.Equal("GBP", updated.Currency);
        Assert.Equal(50m, updated.Balance);
    }

    [Fact]
    public async Task DeleteArchivesInsteadOfRemoving()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId, "Closing", 10m);

        await _harness.Accounts.ArchiveAsync(userId, account.Id, CancellationToken.None);

        var reloaded = await _harness.Accounts.GetAsync(userId, account.Id, CancellationToken.None);
        Assert.True(reloaded.IsArchived);
    }

    [Fact]
    public async Task CreateReturnsOpeningBalanceAndUppercaseCurrency()
    {
        var userId = await _harness.CreateUserAsync();

        var account = await _harness.Accounts.CreateAsync(
            userId,
            new CreateAccountRequest
            {
                Name = " Wallet ",
                Type = AccountType.Cash,
                InitialBalance = 12.34m,
                Currency = "usd",
            },
            CancellationToken.None);

        Assert.Equal("Wallet", account.Name);
        Assert.Equal("USD", account.Currency);
        Assert.Equal(12.34m, account.Balance);
        Assert.False(account.IsArchived);
    }

    public void Dispose() => _harness.Dispose();
}
