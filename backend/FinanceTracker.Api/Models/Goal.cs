namespace FinanceTracker.Api.Models;

/// <summary>A savings target the user contributes towards.</summary>
public sealed class Goal : IUserOwned
{
    public Guid Id { get; set; } = Guid.NewGuid();

    public Guid UserId { get; set; }

    public string Name { get; set; } = string.Empty;

    public decimal TargetAmount { get; set; }

    public decimal CurrentAmount { get; set; }

    public DateTime? TargetDate { get; set; }

    public string Color { get; set; } = string.Empty;
}
