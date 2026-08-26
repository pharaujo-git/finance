using System.Globalization;
using System.Net;
using System.Net.Http.Json;
using System.Text;
using System.Text.Json;
using FinanceTracker.Application.Dtos;
using FinanceTracker.Domain;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Integration;

public sealed class FinanceEndpointsTests : IClassFixture<FinanceApiFactory>
{
    private readonly FinanceApiFactory _factory;

    public FinanceEndpointsTests(FinanceApiFactory factory) => _factory = factory;

    [Fact]
    public async Task CategoriesEndpointReturnsSeededDefaults()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();

        var categories = await client.GetFromJsonAsync<List<CategoryDto>>("/api/categories", FinanceApiFactory.Json);

        Assert.NotNull(categories);
        Assert.Contains(categories, c => c.IsDefault);
        Assert.Contains(categories, c => c.Type == CategoryType.Income);
    }

    [Fact]
    public async Task AccountLifecycleIsExposedOverHttp()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();

        var account = await CreateAccountAsync(client, "Everyday", 100m);
        Assert.Equal(100m, account.Balance);
        Assert.Equal(AccountType.Checking, account.Type);

        var update = await client.PutAsJsonAsync(
            $"/api/accounts/{account.Id}",
            new UpdateAccountRequest
            {
                Name = "Everyday renamed",
                Type = AccountType.Savings,
                Currency = "USD",
                IsArchived = false,
            },
            FinanceApiFactory.Json);
        update.EnsureSuccessStatusCode();

        var archive = await client.DeleteAsync(new Uri($"/api/accounts/{account.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NoContent, archive.StatusCode);

        var accounts = await client.GetFromJsonAsync<List<AccountDto>>("/api/accounts", FinanceApiFactory.Json);
        Assert.NotNull(accounts);
        Assert.True(accounts.Single(a => a.Id == account.Id).IsArchived);
    }

    [Fact]
    public async Task TransactionEndpointsSerializeTheAgreedShape()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();
        var account = await CreateAccountAsync(client, "Wallet", 0m);

        var created = await PostTransactionAsync(client, account.Id, TransactionType.Expense, 25.5m, "Books", ["reading"]);
        Assert.Equal(25.50m, created.Amount);
        Assert.Equal(["reading"], created.Tags);

        var raw = await client.GetStringAsync(new Uri("/api/transactions?page=1&pageSize=10", UriKind.Relative));
        using var document = JsonDocument.Parse(raw);
        var root = document.RootElement;

        Assert.Equal(1, root.GetProperty("total").GetInt32());
        Assert.Equal(1, root.GetProperty("page").GetInt32());
        Assert.Equal(10, root.GetProperty("pageSize").GetInt32());

        var item = root.GetProperty("items")[0];
        Assert.Equal("expense", item.GetProperty("type").GetString());
        Assert.Equal("Books", item.GetProperty("description").GetString());
        Assert.Equal(JsonValueKind.Null, item.GetProperty("categoryId").ValueKind);
        Assert.EndsWith("Z", item.GetProperty("date").GetString() ?? string.Empty, StringComparison.Ordinal);

        var deleted = await client.DeleteAsync(new Uri($"/api/transactions/{created.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NoContent, deleted.StatusCode);

        var missing = await client.GetAsync(new Uri($"/api/transactions/{created.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NotFound, missing.StatusCode);
    }

    [Fact]
    public async Task ExportReturnsCsvAndImportAcceptsItBack()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();
        var account = await CreateAccountAsync(client, "Import target", 0m);
        await PostTransactionAsync(client, account.Id, TransactionType.Income, 10m, "Refund", null);

        var export = await client.GetAsync(new Uri("/api/transactions/export", UriKind.Relative));
        Assert.Equal("text/csv", export.Content.Headers.ContentType?.MediaType);

        var csv = await export.Content.ReadAsStringAsync();
        Assert.StartsWith("Date,Type,Amount,Account,Category,Description,Notes,Tags", csv, StringComparison.Ordinal);

        using var form = new MultipartFormDataContent();
        using var file = new ByteArrayContent(Encoding.UTF8.GetBytes(csv));
        form.Add(file, "file", "transactions.csv");

        var import = await client.PostAsync(new Uri("/api/transactions/import", UriKind.Relative), form);
        import.EnsureSuccessStatusCode();

        var result = await import.Content.ReadFromJsonAsync<ImportResult>(FinanceApiFactory.Json);
        Assert.NotNull(result);
        Assert.Equal(1, result.Imported);
    }

    [Fact]
    public async Task BudgetsGoalsRecurringAndAnalyticsAreReachable()
    {
        var (client, _) = await _factory.CreateAuthenticatedClientAsync();
        var account = await CreateAccountAsync(client, "Analytics", 500m);

        var category = await ReadAsync<CategoryDto>(await client.PostAsJsonAsync(
            "/api/categories",
            new CategoryRequest { Name = "Hobbies", Type = CategoryType.Expense, Icon = "star", Color = "#ff00ff" },
            FinanceApiFactory.Json));

        await PostTransactionAsync(client, account.Id, TransactionType.Expense, 60m, "Paints", null, category.Id);

        var month = DateTime.UtcNow.ToString("yyyy-MM", CultureInfo.InvariantCulture);
        var budget = await ReadAsync<BudgetDto>(await client.PostAsJsonAsync(
            "/api/budgets",
            new CreateBudgetRequest { CategoryId = category.Id, Month = month, Limit = 200m },
            FinanceApiFactory.Json));

        Assert.Equal(60m, budget.Spent);
        Assert.Equal(140m, budget.Remaining);

        var budgets = await client.GetFromJsonAsync<List<BudgetDto>>(
            $"/api/budgets?month={month}", FinanceApiFactory.Json);
        Assert.NotNull(budgets);
        Assert.Single(budgets);

        var goal = await ReadAsync<GoalDto>(await client.PostAsJsonAsync(
            "/api/goals",
            new GoalRequest { Name = "New bike", TargetAmount = 1_000m, Color = "#00ffff" },
            FinanceApiFactory.Json));

        var contributed = await ReadAsync<GoalDto>(await client.PostAsJsonAsync(
            $"/api/goals/{goal.Id}/contribute",
            new ContributeRequest { Amount = 200m },
            FinanceApiFactory.Json));
        Assert.Equal(200m, contributed.CurrentAmount);

        var rule = await ReadAsync<RecurringRuleDto>(await client.PostAsJsonAsync(
            "/api/recurring",
            new RecurringRuleRequest
            {
                AccountId = account.Id,
                CategoryId = category.Id,
                Type = TransactionType.Expense,
                Amount = 9.99m,
                Description = "Magazine",
                Frequency = Frequency.Monthly,
                StartDate = DateTime.UtcNow.AddMonths(1),
                IsActive = true,
            },
            FinanceApiFactory.Json));

        Assert.Equal(Frequency.Monthly, rule.Frequency);

        var rules = await client.GetFromJsonAsync<List<RecurringRuleDto>>("/api/recurring", FinanceApiFactory.Json);
        Assert.NotNull(rules);
        Assert.Single(rules);

        var summary = await client.GetFromJsonAsync<DashboardSummaryDto>(
            "/api/dashboard/summary", FinanceApiFactory.Json);
        Assert.NotNull(summary);
        Assert.Equal(440m, summary.NetWorth);
        Assert.Equal(60m, summary.TotalExpenses);

        var netWorth = await client.GetFromJsonAsync<List<NetWorthPointDto>>(
            "/api/dashboard/networth?months=4", FinanceApiFactory.Json);
        Assert.NotNull(netWorth);
        Assert.Equal(4, netWorth.Count);

        var cashflow = await client.GetFromJsonAsync<List<CashflowPointDto>>(
            "/api/dashboard/cashflow?months=3", FinanceApiFactory.Json);
        Assert.NotNull(cashflow);
        Assert.Equal(3, cashflow.Count);

        var spending = await client.GetFromJsonAsync<List<SpendingSliceDto>>(
            $"/api/dashboard/spending?month={month}", FinanceApiFactory.Json);
        Assert.NotNull(spending);
        Assert.Equal("Hobbies", Assert.Single(spending).CategoryName);

        var monthly = await client.GetFromJsonAsync<List<MonthlyReportDto>>(
            $"/api/reports/monthly?year={DateTime.UtcNow.Year}", FinanceApiFactory.Json);
        Assert.NotNull(monthly);
        Assert.Equal(12, monthly.Count);

        var byCategory = await client.GetFromJsonAsync<List<CategoryReportDto>>(
            "/api/reports/categories", FinanceApiFactory.Json);
        Assert.NotNull(byCategory);
        Assert.Equal(60m, Assert.Single(byCategory).Amount);

        var deleteBudget = await client.DeleteAsync(new Uri($"/api/budgets/{budget.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NoContent, deleteBudget.StatusCode);

        var deleteGoal = await client.DeleteAsync(new Uri($"/api/goals/{goal.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NoContent, deleteGoal.StatusCode);

        var deleteRule = await client.DeleteAsync(new Uri($"/api/recurring/{rule.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NoContent, deleteRule.StatusCode);
    }

    [Fact]
    public async Task OneUsersDataIsInvisibleToAnother()
    {
        var (owner, _) = await _factory.CreateAuthenticatedClientAsync();
        var (stranger, _) = await _factory.CreateAuthenticatedClientAsync();

        var account = await CreateAccountAsync(owner, "Private", 42m);

        var accounts = await stranger.GetFromJsonAsync<List<AccountDto>>("/api/accounts", FinanceApiFactory.Json);
        Assert.NotNull(accounts);
        Assert.DoesNotContain(accounts, a => a.Id == account.Id);

        var probe = await stranger.GetAsync(new Uri($"/api/accounts/{account.Id}", UriKind.Relative));
        Assert.Equal(HttpStatusCode.NotFound, probe.StatusCode);
    }

    private static async Task<AccountDto> CreateAccountAsync(HttpClient client, string name, decimal initialBalance) =>
        await ReadAsync<AccountDto>(await client.PostAsJsonAsync(
            "/api/accounts",
            new CreateAccountRequest
            {
                Name = name,
                Type = AccountType.Checking,
                InitialBalance = initialBalance,
                Currency = "USD",
            },
            FinanceApiFactory.Json));

    private static async Task<TransactionDto> PostTransactionAsync(
        HttpClient client,
        Guid accountId,
        TransactionType type,
        decimal amount,
        string description,
        IReadOnlyList<string>? tags,
        Guid? categoryId = null) =>
        await ReadAsync<TransactionDto>(await client.PostAsJsonAsync(
            "/api/transactions",
            new TransactionRequest
            {
                AccountId = accountId,
                CategoryId = categoryId,
                Type = type,
                Amount = amount,
                Date = DateTime.UtcNow,
                Description = description,
                Tags = tags,
            },
            FinanceApiFactory.Json));

    private static async Task<T> ReadAsync<T>(HttpResponseMessage response)
    {
        response.EnsureSuccessStatusCode();
        var value = await response.Content.ReadFromJsonAsync<T>(FinanceApiFactory.Json);
        Assert.NotNull(value);
        return value;
    }
}
