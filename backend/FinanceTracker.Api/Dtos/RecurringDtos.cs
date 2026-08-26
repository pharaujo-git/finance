using System.ComponentModel.DataAnnotations;
using FinanceTracker.Api.Models;

namespace FinanceTracker.Api.Dtos;

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
    [Required]
    public Guid AccountId { get; init; }

    public Guid? CategoryId { get; init; }

    [Required]
    public TransactionType Type { get; init; }

    [Range(typeof(decimal), "0.01", "999999999999.99")]
    public decimal Amount { get; init; }

    [Required]
    [MaxLength(500)]
    public string Description { get; init; } = string.Empty;

    [Required]
    public Frequency Frequency { get; init; }

    [Required]
    public DateTime StartDate { get; init; }

    public DateTime? EndDate { get; init; }

    public bool IsActive { get; init; } = true;
}
