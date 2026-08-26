namespace FinanceTracker.Api.Models;

/// <summary>A single movement of money. Amount is always stored positive; <see cref="Type"/> carries the sign.</summary>
public sealed class Transaction : IUserOwned
{
    /// <summary>Unit separator (U+001F): safe because it cannot appear in user-entered tag text.</summary>
    public const char TagSeparator = (char)31;

    public Guid Id { get; set; } = Guid.NewGuid();

    public Guid UserId { get; set; }

    public Guid AccountId { get; set; }

    public Guid? CategoryId { get; set; }

    public TransactionType Type { get; set; }

    public decimal Amount { get; set; }

    public DateTime Date { get; set; }

    public string Description { get; set; } = string.Empty;

    public string? Notes { get; set; }

    /// <summary>Tags joined by <see cref="TagSeparator"/>; exposed as an array by the API.</summary>
    public string TagsRaw { get; set; } = string.Empty;

    /// <summary>Destination account for transfers.</summary>
    public Guid? TransferAccountId { get; set; }

    public static string JoinTags(IEnumerable<string>? tags) =>
        tags is null
            ? string.Empty
            : string.Join(TagSeparator, tags.Where(t => !string.IsNullOrWhiteSpace(t)).Select(t => t.Trim()));

    public static string[] SplitTags(string? raw) =>
        string.IsNullOrEmpty(raw) ? [] : raw.Split(TagSeparator, StringSplitOptions.RemoveEmptyEntries);
}
