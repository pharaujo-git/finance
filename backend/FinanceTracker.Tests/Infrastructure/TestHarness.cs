using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using FinanceTracker.Api.Services;
using Microsoft.AspNetCore.Identity;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Tests.Infrastructure;

/// <summary>
/// Wires the real services on top of a throwaway in-memory SQLite database so service
/// behaviour is exercised against genuine EF Core translation.
/// </summary>
internal sealed class TestHarness : IDisposable
{
    private readonly SqliteConnection _connection;

    public TestHarness()
    {
        _connection = new SqliteConnection("Data Source=:memory:");
        _connection.Open();

        var options = new DbContextOptionsBuilder<AppDbContext>()
            .UseSqlite(_connection)
            .EnableSensitiveDataLogging()
            .Options;

        Db = new AppDbContext(options);
        Db.Database.EnsureCreated();

        Categories = new CategoryService(Db);
        Accounts = new AccountService(Db);
        Transactions = new TransactionService(Db, Categories);
        Csv = new TransactionCsvService(Db);
        Recurring = new RecurringService(Db, Categories);
        Budgets = new BudgetService(Db, Categories);
        Goals = new GoalService(Db);
        Analytics = new AnalyticsService(Db, Transactions);
        Auth = new AuthService(Db, new TokenService(new JwtOptions()), new PasswordHasher<User>());
    }

    public AppDbContext Db { get; }

    public AuthService Auth { get; }

    public AccountService Accounts { get; }

    public CategoryService Categories { get; }

    public TransactionService Transactions { get; }

    public TransactionCsvService Csv { get; }

    public RecurringService Recurring { get; }

    public BudgetService Budgets { get; }

    public GoalService Goals { get; }

    public AnalyticsService Analytics { get; }

    public static DateTime Utc(int year, int month, int day) =>
        new(year, month, day, 0, 0, 0, DateTimeKind.Utc);

    public Task SeedDefaultCategoriesAsync() => DefaultCategorySeeder.SeedAsync(Db);

    public async Task<Guid> CreateUserAsync(string email = "owner@example.com")
    {
        var response = await Auth.RegisterAsync(
            new RegisterRequest { Email = email, Password = "correct horse battery", Name = "Owner" },
            CancellationToken.None);

        return response.User.Id;
    }

    public Task<AccountDto> CreateAccountAsync(
        Guid userId,
        string name = "Checking",
        decimal initialBalance = 0m,
        AccountType type = AccountType.Checking) =>
        Accounts.CreateAsync(
            userId,
            new CreateAccountRequest
            {
                Name = name,
                Type = type,
                InitialBalance = initialBalance,
                Currency = "USD",
            },
            CancellationToken.None);

    public Task<CategoryDto> CreateCategoryAsync(
        Guid userId,
        string name = "Coffee",
        CategoryType type = CategoryType.Expense) =>
        Categories.CreateAsync(
            userId,
            new CategoryRequest { Name = name, Type = type, Icon = "cup", Color = "#123456" },
            CancellationToken.None);

    public Task<TransactionDto> AddTransactionAsync(
        Guid userId,
        Guid accountId,
        TransactionType type,
        decimal amount,
        DateTime date,
        Guid? categoryId = null,
        Guid? transferAccountId = null,
        string description = "Test entry",
        IReadOnlyList<string>? tags = null,
        string? notes = null) =>
        Transactions.CreateAsync(
            userId,
            new TransactionRequest
            {
                AccountId = accountId,
                CategoryId = categoryId,
                Type = type,
                Amount = amount,
                Date = date,
                Description = description,
                Notes = notes,
                Tags = tags,
                TransferAccountId = transferAccountId,
            },
            CancellationToken.None);

    public void Dispose()
    {
        Db.Dispose();
        _connection.Dispose();
    }
}
