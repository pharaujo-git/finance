namespace FinanceTracker.Domain;

/// <summary>A template that a background worker materializes into transactions on a schedule.</summary>
public sealed class RecurringRule : IUserOwned
{
    public Guid Id { get; set; } = Guid.NewGuid();

    public Guid UserId { get; set; }

    public Guid AccountId { get; set; }

    public Guid? CategoryId { get; set; }

    public TransactionType Type { get; set; }

    public decimal Amount { get; set; }

    public string Description { get; set; } = string.Empty;

    public Frequency Frequency { get; set; }

    public DateTime StartDate { get; set; }

    public DateTime? EndDate { get; set; }

    /// <summary>Date of the next occurrence still to be materialized.</summary>
    public DateTime NextRunDate { get; set; }

    public bool IsActive { get; set; } = true;
}
