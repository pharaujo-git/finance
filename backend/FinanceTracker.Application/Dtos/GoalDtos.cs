using System.ComponentModel.DataAnnotations;

namespace FinanceTracker.Application.Dtos;

public sealed record GoalDto(
    Guid Id,
    string Name,
    decimal TargetAmount,
    decimal CurrentAmount,
    DateTime? TargetDate,
    string Color);

public sealed class GoalRequest
{
    [Required]
    [MaxLength(200)]
    public string Name { get; init; } = string.Empty;

    [Range(typeof(decimal), "0.01", "999999999999.99")]
    public required decimal TargetAmount { get; init; }

    /// <summary>Optional; an omitted balance means the goal starts empty.</summary>
    [Range(typeof(decimal), "0.00", "999999999999.99")]
    public decimal? CurrentAmount { get; init; }

    public DateTime? TargetDate { get; init; }

    [MaxLength(32)]
    public string Color { get; init; } = string.Empty;
}

public sealed class ContributeRequest
{
    [Range(typeof(decimal), "0.01", "999999999999.99")]
    public required decimal Amount { get; init; }
}
