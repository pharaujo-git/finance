using FinanceTracker.Api.Common;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using FinanceTracker.Api.Services;
using FinanceTracker.Tests.Infrastructure;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Tests.Services;

public sealed class RecurringServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Theory]
    [InlineData(Frequency.Daily, 2)]
    [InlineData(Frequency.Weekly, 8)]
    [InlineData(Frequency.Monthly, 1)]
    [InlineData(Frequency.Yearly, 1)]
    public void AdvanceMovesToTheNextOccurrence(Frequency frequency, int expectedDayOrMonth)
    {
        var next = RecurringService.Advance(TestHarness.Utc(2026, 1, 1), frequency);

        var actual = frequency switch
        {
            Frequency.Daily or Frequency.Weekly => next.Day,
            Frequency.Monthly => next.Month - 1,
            _ => next.Year - 2026,
        };

        Assert.Equal(expectedDayOrMonth, actual);
    }

    [Fact]
    public async Task MaterializeCreatesEveryDueOccurrenceAndAdvancesNextRunDate()
    {
        var (userId, accountId) = await SeedAsync();

        await _harness.Recurring.CreateAsync(
            userId,
            BuildRequest(accountId, Frequency.Monthly, TestHarness.Utc(2026, 1, 15)),
            CancellationToken.None);

        var created = await _harness.Recurring.MaterializeDueAsync(TestHarness.Utc(2026, 4, 1), CancellationToken.None);

        Assert.Equal(3, created);

        var rule = Assert.Single(await _harness.Recurring.ListAsync(userId, CancellationToken.None));
        Assert.Equal(TestHarness.Utc(2026, 4, 15), rule.NextRunDate);
        Assert.True(rule.IsActive);

        var transactions = await _harness.Transactions.SearchAsync(
            userId,
            new TransactionQuery { PageSize = 50 },
            CancellationToken.None);

        Assert.Equal(3, transactions.Total);
        Assert.All(transactions.Items, t => Assert.Contains("recurring", t.Tags));
        Assert.Equal(
            [TestHarness.Utc(2026, 3, 15), TestHarness.Utc(2026, 2, 15), TestHarness.Utc(2026, 1, 15)],
            transactions.Items.Select(t => t.Date));
    }

    [Fact]
    public async Task MaterializeStopsAndDeactivatesAfterTheEndDate()
    {
        var (userId, accountId) = await SeedAsync();
        var request = BuildRequest(accountId, Frequency.Weekly, TestHarness.Utc(2026, 1, 1));

        await _harness.Recurring.CreateAsync(
            userId,
            new RecurringRuleRequest
            {
                AccountId = request.AccountId,
                Type = request.Type,
                Amount = request.Amount,
                Description = request.Description,
                Frequency = request.Frequency,
                StartDate = request.StartDate,
                EndDate = TestHarness.Utc(2026, 1, 20),
                IsActive = true,
            },
            CancellationToken.None);

        var created = await _harness.Recurring.MaterializeDueAsync(TestHarness.Utc(2026, 3, 1), CancellationToken.None);

        Assert.Equal(3, created);
        var rule = Assert.Single(await _harness.Recurring.ListAsync(userId, CancellationToken.None));
        Assert.False(rule.IsActive);
    }

    [Fact]
    public async Task InactiveAndFutureRulesAreLeftAlone()
    {
        var (userId, accountId) = await SeedAsync();

        await _harness.Recurring.CreateAsync(
            userId,
            BuildRequest(accountId, Frequency.Daily, TestHarness.Utc(2027, 1, 1)),
            CancellationToken.None);

        var inactive = await _harness.Recurring.CreateAsync(
            userId,
            BuildRequest(accountId, Frequency.Daily, TestHarness.Utc(2020, 1, 1)),
            CancellationToken.None);

        var stored = await _harness.Db.RecurringRules.SingleAsync(r => r.Id == inactive.Id);
        stored.IsActive = false;
        await _harness.Db.SaveChangesAsync();

        var created = await _harness.Recurring.MaterializeDueAsync(TestHarness.Utc(2026, 1, 1), CancellationToken.None);

        Assert.Equal(0, created);
    }

    [Fact]
    public async Task UpdateAndDeleteAreScopedToTheOwner()
    {
        var (userId, accountId) = await SeedAsync();
        var stranger = await _harness.CreateUserAsync("stranger@example.com");
        var rule = await _harness.Recurring.CreateAsync(
            userId,
            BuildRequest(accountId, Frequency.Monthly, TestHarness.Utc(2026, 1, 1)),
            CancellationToken.None);

        var updated = await _harness.Recurring.UpdateAsync(
            userId,
            rule.Id,
            BuildRequest(accountId, Frequency.Yearly, TestHarness.Utc(2026, 6, 1)),
            CancellationToken.None);

        Assert.Equal(Frequency.Yearly, updated.Frequency);
        Assert.Equal(TestHarness.Utc(2026, 6, 1), updated.NextRunDate);

        var error = await Assert.ThrowsAsync<ApiException>(() => _harness.Recurring.DeleteAsync(
            stranger, rule.Id, CancellationToken.None));
        Assert.Equal(StatusCodes.Status404NotFound, error.StatusCode);

        await _harness.Recurring.DeleteAsync(userId, rule.Id, CancellationToken.None);
        Assert.Empty(await _harness.Recurring.ListAsync(userId, CancellationToken.None));
    }

    [Fact]
    public async Task InvalidRulesAreRejected()
    {
        var (userId, accountId) = await SeedAsync();

        var transferRule = BuildRequest(accountId, Frequency.Monthly, TestHarness.Utc(2026, 1, 1));
        var transferError = await Assert.ThrowsAsync<ApiException>(() => _harness.Recurring.CreateAsync(
            userId,
            new RecurringRuleRequest
            {
                AccountId = transferRule.AccountId,
                Type = TransactionType.Transfer,
                Amount = 10m,
                Description = "Nope",
                Frequency = Frequency.Monthly,
                StartDate = transferRule.StartDate,
            },
            CancellationToken.None));
        Assert.Equal(StatusCodes.Status400BadRequest, transferError.StatusCode);

        var badDates = await Assert.ThrowsAsync<ApiException>(() => _harness.Recurring.CreateAsync(
            userId,
            new RecurringRuleRequest
            {
                AccountId = accountId,
                Type = TransactionType.Expense,
                Amount = 10m,
                Description = "Backwards",
                Frequency = Frequency.Monthly,
                StartDate = TestHarness.Utc(2026, 5, 1),
                EndDate = TestHarness.Utc(2026, 4, 1),
            },
            CancellationToken.None));
        Assert.Equal(StatusCodes.Status400BadRequest, badDates.StatusCode);

        var unknownAccount = await Assert.ThrowsAsync<ApiException>(() => _harness.Recurring.CreateAsync(
            userId,
            BuildRequest(Guid.NewGuid(), Frequency.Monthly, TestHarness.Utc(2026, 1, 1)),
            CancellationToken.None));
        Assert.Equal(StatusCodes.Status404NotFound, unknownAccount.StatusCode);
    }

    private static RecurringRuleRequest BuildRequest(Guid accountId, Frequency frequency, DateTime startDate) => new()
    {
        AccountId = accountId,
        Type = TransactionType.Expense,
        Amount = 12.50m,
        Description = "Streaming service",
        Frequency = frequency,
        StartDate = startDate,
        IsActive = true,
    };

    private async Task<(Guid UserId, Guid AccountId)> SeedAsync()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);
        return (userId, account.Id);
    }

    public void Dispose() => _harness.Dispose();
}
