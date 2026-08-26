using System.ComponentModel.DataAnnotations;
using FinanceTracker.Api.Models;

namespace FinanceTracker.Api.Dtos;

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

    [Required]
    public AccountType Type { get; init; }

    public decimal InitialBalance { get; init; }

    [Required]
    [MaxLength(8)]
    public string Currency { get; init; } = "USD";
}

public sealed class UpdateAccountRequest
{
    [Required]
    [MaxLength(200)]
    public string Name { get; init; } = string.Empty;

    [Required]
    public AccountType Type { get; init; }

    [Required]
    [MaxLength(8)]
    public string Currency { get; init; } = "USD";

    public bool IsArchived { get; init; }
}
