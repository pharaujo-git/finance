using System.Globalization;
using System.Net;
using System.Net.Http.Json;
using System.Text;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Integration;

/// <summary>Covers the PUT/DELETE half of the contract and the import failure paths.</summary>
public sealed class UpdateEndpointsTests : IClassFixture<FinanceApiFactory>
{
    private readonly FinanceApiFactory _factory;

    public UpdateEndpointsTests(FinanceApiFactory factory) => _factory = factory;

    [Fact]
    public async Task CategoriesCanBeUpdatedAndDeletedButDefaultsAreProtected()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();

        var created = await ReadAsync<CategoryDto>(await client.PostAsJsonAsync(
            "/api/categories",
            new CategoryRequest { Name = "Plants", Type = CategoryType.Expense, Icon = "leaf", Color = "#00aa00" },
            FinanceApiFactory.Json));

        var updated = await ReadAsync<CategoryDto>(await client.PutAsJsonAsync(
            $"/api/categories/{created.Id}",
            new CategoryRequest { Name = "Garden", Type = CategoryType.Expense, Icon = "leaf", Color = "#00bb00" },
            FinanceApiFactory.Json));
        Assert.Equal("Garden", updated.Name);

        var all = await client.GetFromJsonAsync<List<CategoryDto>>("/api/categories", FinanceApiFactory.Json);
        Assert.NotNull(all);
        var seeded = all.First(c => c.IsDefault);

        var protectedDelete = await client.DeleteAsync(new Uri($"/api/categories/{seeded.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.BadRequest, protectedDelete.StatusCode);

        var removed = await client.DeleteAsync(new Uri($"/api/categories/{created.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NoContent, removed.StatusCode);
    }

    [Fact]
    public async Task TransactionBudgetGoalAndRuleUpdatesAreApplied()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();

        var account = await ReadAsync<AccountDto>(await client.PostAsJsonAsync(
            "/api/accounts",
            new CreateAccountRequest
            {
                Name = "Updates",
                Type = AccountType.Checking,
                InitialBalance = 0m,
                Currency = "USD",
            },
            FinanceApiFactory.Json));

        var category = await ReadAsync<CategoryDto>(await client.PostAsJsonAsync(
            "/api/categories",
            new CategoryRequest { Name = "Utilities", Type = CategoryType.Expense, Icon = "zap", Color = "#ffaa00" },
            FinanceApiFactory.Json));

        var transaction = await ReadAsync<TransactionDto>(await client.PostAsJsonAsync(
            "/api/transactions",
            NewTransaction(account.Id, category.Id, 30m, "Water bill"),
            FinanceApiFactory.Json));

        var updatedTransaction = await ReadAsync<TransactionDto>(await client.PutAsJsonAsync(
            $"/api/transactions/{transaction.Id}",
            NewTransaction(account.Id, category.Id, 45.75m, "Water bill corrected"),
            FinanceApiFactory.Json));
        Assert.Equal(45.75m, updatedTransaction.Amount);
        Assert.Equal("Water bill corrected", updatedTransaction.Description);

        var month = DateTime.UtcNow.ToString("yyyy-MM", CultureInfo.InvariantCulture);
        var budget = await ReadAsync<BudgetDto>(await client.PostAsJsonAsync(
            "/api/budgets",
            new CreateBudgetRequest { CategoryId = category.Id, Month = month, Limit = 100m },
            FinanceApiFactory.Json));

        var updatedBudget = await ReadAsync<BudgetDto>(await client.PutAsJsonAsync(
            $"/api/budgets/{budget.Id}",
            new UpdateBudgetRequest { Limit = 60m },
            FinanceApiFactory.Json));
        Assert.Equal(60m, updatedBudget.Limit);
        Assert.Equal(45.75m, updatedBudget.Spent);
        Assert.Equal(14.25m, updatedBudget.Remaining);

        var goal = await ReadAsync<GoalDto>(await client.PostAsJsonAsync(
            "/api/goals",
            new GoalRequest { Name = "Laptop", TargetAmount = 2_000m, Color = "#333333" },
            FinanceApiFactory.Json));

        var updatedGoal = await ReadAsync<GoalDto>(await client.PutAsJsonAsync(
            $"/api/goals/{goal.Id}",
            new GoalRequest
            {
                Name = "Laptop upgrade",
                TargetAmount = 2_500m,
                CurrentAmount = 100m,
                TargetDate = DateTime.UtcNow.AddYears(1),
                Color = "#444444",
            },
            FinanceApiFactory.Json));
        Assert.Equal("Laptop upgrade", updatedGoal.Name);
        Assert.NotNull(updatedGoal.TargetDate);

        var rule = await ReadAsync<RecurringRuleDto>(await client.PostAsJsonAsync(
            "/api/recurring",
            NewRule(account.Id, category.Id, Frequency.Weekly),
            FinanceApiFactory.Json));

        var updatedRule = await ReadAsync<RecurringRuleDto>(await client.PutAsJsonAsync(
            $"/api/recurring/{rule.Id}",
            NewRule(account.Id, category.Id, Frequency.Daily),
            FinanceApiFactory.Json));
        Assert.Equal(Frequency.Daily, updatedRule.Frequency);
    }

    [Fact]
    public async Task ImportRejectsAMissingOrEmptyFile()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();

        using var empty = new MultipartFormDataContent();
        using var file = new ByteArrayContent(Encoding.UTF8.GetBytes(string.Empty));
        empty.Add(file, "file", "empty.csv");

        var response = await client.PostAsync(new Uri("/api/transactions/import", UriKind.Relative), empty);

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }

    [Fact]
    public async Task UnknownResourcesReturnNotFound()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();
        var missing = Guid.NewGuid();

        Assert.Equal(
            HttpStatusCode.NotFound,
            (await client.DeleteAsync(new Uri($"/api/goals/{missing}", UriKind.Relative))).StatusCode);
        Assert.Equal(
            HttpStatusCode.NotFound,
            (await client.DeleteAsync(new Uri($"/api/budgets/{missing}", UriKind.Relative))).StatusCode);
        Assert.Equal(
            HttpStatusCode.NotFound,
            (await client.DeleteAsync(new Uri($"/api/recurring/{missing}", UriKind.Relative))).StatusCode);
    }

    private static TransactionRequest NewTransaction(Guid accountId, Guid categoryId, decimal amount, string description) =>
        new()
        {
            AccountId = accountId,
            CategoryId = categoryId,
            Type = TransactionType.Expense,
            Amount = amount,
            Date = DateTime.UtcNow,
            Description = description,
        };

    private static RecurringRuleRequest NewRule(Guid accountId, Guid categoryId, Frequency frequency) => new()
    {
        AccountId = accountId,
        CategoryId = categoryId,
        Type = TransactionType.Expense,
        Amount = 15m,
        Description = "Internet",
        Frequency = frequency,
        StartDate = DateTime.UtcNow.AddMonths(1),
        IsActive = true,
    };

    private static async Task<T> ReadAsync<T>(HttpResponseMessage response)
    {
        response.EnsureSuccessStatusCode();
        var value = await response.Content.ReadFromJsonAsync<T>(FinanceApiFactory.Json);
        Assert.NotNull(value);
        return value;
    }
}
