namespace FinanceTracker.Api.Models;

/// <summary>A bank/cash/credit account owned by a single user.</summary>
public sealed class Account : IUserOwned
{
    public Guid Id { get; set; } = Guid.NewGuid();

    public Guid UserId { get; set; }

    public string Name { get; set; } = string.Empty;

    public AccountType Type { get; set; }

    /// <summary>Opening balance; the live balance adds the signed sum of transactions.</summary>
    public decimal InitialBalance { get; set; }

    public string Currency { get; set; } = "USD";

    public bool IsArchived { get; set; }

    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
}
