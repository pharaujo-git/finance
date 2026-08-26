namespace FinanceTracker.Domain;

/// <summary>A monthly spending limit for one category.</summary>
public sealed class Budget : IUserOwned
{
    public Guid Id { get; set; } = Guid.NewGuid();

    public Guid UserId { get; set; }

    public Guid CategoryId { get; set; }

    /// <summary>Calendar month in <c>YYYY-MM</c> form.</summary>
    public string Month { get; set; } = string.Empty;

    public decimal Limit { get; set; }
}
