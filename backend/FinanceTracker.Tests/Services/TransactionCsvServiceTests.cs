using System.Text;
using FinanceTracker.Api.Common;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Services;

public sealed class TransactionCsvServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task ExportWritesTheAgreedHeaderAndEscapesFields()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId, "Main, Checking");
        var category = await _harness.CreateCategoryAsync(userId, "Food");

        await _harness.AddTransactionAsync(
            userId,
            account.Id,
            TransactionType.Expense,
            12.5m,
            TestHarness.Utc(2026, 1, 9),
            category.Id,
            description: "Lunch \"special\"",
            tags: ["work", "team"],
            notes: "line one");

        var csv = await _harness.Csv.ExportAsync(userId, null, null, CancellationToken.None);
        var lines = csv.Split('\n', StringSplitOptions.RemoveEmptyEntries);

        Assert.Equal("Date,Type,Amount,Account,Category,Description,Notes,Tags", lines[0]);
        Assert.Contains("2026-01-09", lines[1], StringComparison.Ordinal);
        Assert.Contains("\"Main, Checking\"", lines[1], StringComparison.Ordinal);
        Assert.Contains("\"Lunch \"\"special\"\"\"", lines[1], StringComparison.Ordinal);
        Assert.Contains("work;team", lines[1], StringComparison.Ordinal);
        Assert.Contains("12.50", lines[1], StringComparison.Ordinal);
    }

    [Fact]
    public async Task ExportHonoursTheDateWindow()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId);

        await _harness.AddTransactionAsync(
            userId, account.Id, TransactionType.Expense, 10m, TestHarness.Utc(2026, 1, 1), description: "Early");
        await _harness.AddTransactionAsync(
            userId, account.Id, TransactionType.Expense, 20m, TestHarness.Utc(2026, 6, 1), description: "Late");

        var csv = await _harness.Csv.ExportAsync(
            userId,
            TestHarness.Utc(2026, 5, 1),
            TestHarness.Utc(2026, 7, 1),
            CancellationToken.None);

        Assert.Contains("Late", csv, StringComparison.Ordinal);
        Assert.DoesNotContain("Early", csv, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ImportCountsAcceptedAndRejectedRows()
    {
        var userId = await _harness.CreateUserAsync();
        await _harness.CreateAccountAsync(userId, "Checking");
        await _harness.CreateCategoryAsync(userId, "Food");

        var csv = string.Join('\n',
        [
            "Date,Type,Amount,Account,Category,Description,Notes,Tags",
            "2026-02-01,expense,12.34,Checking,Food,Groceries,weekly run,food;home",
            "2026-02-02,income,1000.00,checking,,Salary,,",
            "2026-02-03,expense,50.00,Unknown Account,Food,Nope,,",
            "2026-02-04,transfer,50.00,Checking,,Moving money,,",
            "not-a-date,expense,10.00,Checking,,Bad date,,",
            "2026-02-05,expense,-5.00,Checking,,Negative,,",
            "2026-02-06,teleport,10.00,Checking,,Bad type,,",
        ]);

        var result = await ImportAsync(userId, csv);

        Assert.Equal(2, result.Imported);
        Assert.Equal(5, result.Skipped);

        var stored = await _harness.Transactions.SearchAsync(
            userId, new TransactionQuery { PageSize = 50 }, CancellationToken.None);

        Assert.Equal(2, stored.Total);
        var groceries = stored.Items.Single(t => t.Description == "Groceries");
        Assert.Equal(12.34m, groceries.Amount);
        Assert.Equal(["food", "home"], groceries.Tags);
        Assert.Equal("weekly run", groceries.Notes);
        Assert.NotNull(groceries.CategoryId);

        var salary = stored.Items.Single(t => t.Description == "Salary");
        Assert.Equal(TransactionType.Income, salary.Type);
        Assert.Null(salary.CategoryId);
    }

    [Fact]
    public async Task ExportedFileCanBeImportedBack()
    {
        var userId = await _harness.CreateUserAsync();
        var account = await _harness.CreateAccountAsync(userId, "Checking");

        await _harness.AddTransactionAsync(
            userId, account.Id, TransactionType.Expense, 33.33m, TestHarness.Utc(2026, 8, 8), description: "Round trip");

        var csv = await _harness.Csv.ExportAsync(userId, null, null, CancellationToken.None);
        var result = await ImportAsync(userId, csv);

        Assert.Equal(1, result.Imported);
        Assert.Equal(0, result.Skipped);
    }

    [Fact]
    public async Task ImportRejectsAnEmptyFile()
    {
        var userId = await _harness.CreateUserAsync();

        var error = await Assert.ThrowsAsync<ApiException>(() => ImportAsync(userId, string.Empty));

        Assert.Equal(StatusCodes.Status400BadRequest, error.StatusCode);
    }

    private async Task<ImportResult> ImportAsync(Guid userId, string csv)
    {
        using var stream = new MemoryStream(Encoding.UTF8.GetBytes(csv));
        return await _harness.Csv.ImportAsync(userId, stream, CancellationToken.None);
    }

    public void Dispose() => _harness.Dispose();
}
