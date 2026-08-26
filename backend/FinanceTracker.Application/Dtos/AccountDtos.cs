using System.ComponentModel.DataAnnotations;
using FinanceTracker.Domain;

namespace FinanceTracker.Application.Dtos;

public sealed record AccountDto(
    Guid Id,
    string Name,
    AccountType Type,
    decimal Balance,
    string Currency,
    bool IsArchived,
    DateTime CreatedAt);

public sealed class CreateAccountRequest
{
    [Required]
    [MaxLength(200)]
    public string Name { get; init; } = string.Empty;

    public required AccountType Type { get; init; }

    /// <summary>Optional; an omitted opening balance means zero.</summary>
    public decimal? InitialBalance { get; init; }

    [Required]
    [MaxLength(8)]
    public string Currency { get; init; } = "USD";
}

public sealed class UpdateAccountRequest
{
    [Required]
    [MaxLength(200)]
    public string Name { get; init; } = string.Empty;

    public required AccountType Type { get; init; }

    [Required]
    [MaxLength(8)]
    public string Currency { get; init; } = "USD";

    /// <summary>Optional; an omitted flag leaves the account active.</summary>
    public bool? IsArchived { get; init; }
}
