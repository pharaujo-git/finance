using System.Globalization;
using System.Text;
using FinanceTracker.Application.Abstractions;
using FinanceTracker.Application.Common;
using FinanceTracker.Application.Dtos;
using FinanceTracker.Domain;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Application.Services;

/// <summary>CSV export and import for transactions.</summary>
public sealed class TransactionCsvService(IAppDbContext db)
{
    public const string DateFormat = "yyyy-MM-dd";

    private const char TagDelimiter = ';';

    private static readonly string[] Header =
        ["Date", "Type", "Amount", "Account", "Category", "Description", "Notes", "Tags"];

    public async Task<string> ExportAsync(
        Guid userId,
        DateTime? from,
        DateTime? to,
        CancellationToken cancellationToken)
    {
        var query = db.Transactions.AsNoTracking().OwnedBy(userId);

        if (from.HasValue)
        {
            var fromUtc = UtcDate.Normalize(from.Value);
            query = query.Where(t => t.Date >= fromUtc);
        }

        if (to.HasValue)
        {
            var toUtc = UtcDate.Normalize(to.Value);
            query = query.Where(t => t.Date <= toUtc);
        }

        var transactions = await query
            .OrderByDescending(t => t.Date)
            .ThenByDescending(t => t.Id)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        var accountNames = await LoadAccountNamesAsync(userId, cancellationToken).ConfigureAwait(false);
        var categoryNames = await LoadCategoryNamesAsync(userId, cancellationToken).ConfigureAwait(false);

        var builder = new StringBuilder();
        CsvFile.AppendRow(builder, Header);

        foreach (var t in transactions)
        {
            CsvFile.AppendRow(builder,
            [
                t.Date.ToString(DateFormat, CultureInfo.InvariantCulture),
                ToWireName(t.Type),
                t.Amount.ToString("0.00", CultureInfo.InvariantCulture),
                Lookup(accountNames, t.AccountId),
                t.CategoryId is { } categoryId ? Lookup(categoryNames, categoryId) : string.Empty,
                t.Description,
                t.Notes ?? string.Empty,
                string.Join(TagDelimiter, Transaction.SplitTags(t.TagsRaw)),
            ]);
        }

        return builder.ToString();
    }

    public async Task<ImportResult> ImportAsync(Guid userId, Stream csv, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(csv);

        using var reader = new StreamReader(csv, Encoding.UTF8, detectEncodingFromByteOrderMarks: true, leaveOpen: true);
        var content = await reader.ReadToEndAsync(cancellationToken).ConfigureAwait(false);

        var rows = CsvFile.Parse(content);
        if (rows.Count == 0)
        {
            throw AppException.BadRequest("The uploaded file contains no rows.");
        }

        var accounts = await db.Accounts
            .AsNoTracking()
            .OwnedBy(userId)
            .ToDictionaryAsync(a => a.Name.ToUpperInvariant(), a => a.Id, cancellationToken)
            .ConfigureAwait(false);

        var categories = await CategoryService
            .VisibleTo(db.Categories.AsNoTracking(), userId)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        var categoryIds = categories
            .GroupBy(c => c.Name.ToUpperInvariant())
            .ToDictionary(g => g.Key, g => g.First().Id);

        var imported = new List<Transaction>();
        var skipped = 0;

        foreach (var row in rows.Skip(IsHeaderRow(rows[0]) ? 1 : 0))
        {
            var transaction = TryBuild(userId, row, accounts, categoryIds);
            if (transaction is null)
            {
                skipped++;
            }
            else
            {
                imported.Add(transaction);
            }
        }

        if (imported.Count > 0)
        {
            db.Transactions.AddRange(imported);
            await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
        }

        return new ImportResult(imported.Count, skipped);
    }

    private static Transaction? TryBuild(
        Guid userId,
        List<string> row,
        Dictionary<string, Guid> accounts,
        Dictionary<string, Guid> categories)
    {
        if (row.Count < 6)
        {
            return null;
        }

        if (!DateTime.TryParse(
                Field(row, 0),
                CultureInfo.InvariantCulture,
                DateTimeStyles.AdjustToUniversal | DateTimeStyles.AssumeUniversal,
                out var date))
        {
            return null;
        }

        if (!TryParseType(Field(row, 1), out var type) || type == TransactionType.Transfer)
        {
            // Transfers need a destination account, which the CSV shape does not carry.
            return null;
        }

        if (!decimal.TryParse(Field(row, 2), NumberStyles.Currency, CultureInfo.InvariantCulture, out var amount)
            || amount <= 0m)
        {
            return null;
        }

        if (!accounts.TryGetValue(Field(row, 3).ToUpperInvariant(), out var accountId))
        {
            return null;
        }

        var categoryName = Field(row, 4).ToUpperInvariant();
        Guid? categoryId = categories.TryGetValue(categoryName, out var found) ? found : null;
        var notes = Field(row, 6);
        var tags = Field(row, 7).Split(TagDelimiter, StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);

        return new Transaction
        {
            UserId = userId,
            AccountId = accountId,
            CategoryId = categoryId,
            Type = type,
            Amount = decimal.Round(amount, 2, MidpointRounding.AwayFromZero),
            Date = DateTime.SpecifyKind(date, DateTimeKind.Utc),
            Description = Field(row, 5),
            Notes = string.IsNullOrWhiteSpace(notes) ? null : notes,
            TagsRaw = Transaction.JoinTags(tags),
        };
    }

    private static bool IsHeaderRow(List<string> row) =>
        row.Count > 0 && string.Equals(row[0].Trim(), "Date", StringComparison.OrdinalIgnoreCase);

    private static bool TryParseType(string value, out TransactionType type) =>
        Enum.TryParse(value.Trim(), ignoreCase: true, out type) && Enum.IsDefined(type);

    private static string Field(List<string> row, int index) =>
        index < row.Count ? row[index].Trim() : string.Empty;

    private static string ToWireName(TransactionType type) =>
        char.ToLowerInvariant(type.ToString()[0]) + type.ToString()[1..];

    private static string Lookup(Dictionary<Guid, string> names, Guid id) =>
        names.TryGetValue(id, out var name) ? name : string.Empty;

    private async Task<Dictionary<Guid, string>> LoadAccountNamesAsync(Guid userId, CancellationToken cancellationToken) =>
        await db.Accounts
            .AsNoTracking()
            .OwnedBy(userId)
            .ToDictionaryAsync(a => a.Id, a => a.Name, cancellationToken)
            .ConfigureAwait(false);

    private async Task<Dictionary<Guid, string>> LoadCategoryNamesAsync(Guid userId, CancellationToken cancellationToken) =>
        await CategoryService
            .VisibleTo(db.Categories.AsNoTracking(), userId)
            .ToDictionaryAsync(c => c.Id, c => c.Name, cancellationToken)
            .ConfigureAwait(false);
}
