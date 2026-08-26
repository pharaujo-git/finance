using System.ComponentModel.DataAnnotations;

namespace FinanceTracker.Api.Dtos;

public sealed record BudgetDto(
    Guid Id,
    Guid CategoryId,
    string Month,
    decimal Limit,
    decimal Spent,
    decimal Remaining);

public sealed class CreateBudgetRequest
{
    public required Guid CategoryId { get; init; }

    [Required]
    [RegularExpression(@"^\d{4}-\d{2}$", ErrorMessage = "Month must be in YYYY-MM format.")]
    public string Month { get; init; } = string.Empty;

    [Range(typeof(decimal), "0.00", "999999999999.99")]
    public required decimal Limit { get; init; }
}

public sealed class UpdateBudgetRequest
{
    [Range(typeof(decimal), "0.00", "999999999999.99")]
    public required decimal Limit { get; init; }
}
