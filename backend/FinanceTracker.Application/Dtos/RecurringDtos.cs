using System.ComponentModel.DataAnnotations;
using FinanceTracker.Domain;

namespace FinanceTracker.Application.Dtos;

public sealed record RecurringRuleDto(
    Guid Id,
    Guid AccountId,
    Guid? CategoryId,
    TransactionType Type,
    decimal Amount,
    string Description,
    Frequency Frequency,
    DateTime StartDate,
    DateTime? EndDate,
    DateTime NextRunDate,
    bool IsActive);

public sealed class RecurringRuleRequest
{
    public required Guid AccountId { get; init; }

    public Guid? CategoryId { get; init; }

    public required TransactionType Type { get; init; }

    [Range(typeof(decimal), "0.01", "999999999999.99")]
    public required decimal Amount { get; init; }

    [Required]
    [MaxLength(500)]
    public string Description { get; init; } = string.Empty;

    public required Frequency Frequency { get; init; }

    public required DateTime StartDate { get; init; }

    public DateTime? EndDate { get; init; }

    /// <summary>Optional; an omitted flag leaves the rule running.</summary>
    public bool? IsActive { get; init; }
}
