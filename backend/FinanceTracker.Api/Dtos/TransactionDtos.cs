using System.ComponentModel.DataAnnotations;
using FinanceTracker.Api.Models;

namespace FinanceTracker.Api.Dtos;

public sealed record TransactionDto(
    Guid Id,
    Guid AccountId,
    Guid? CategoryId,
    TransactionType Type,
    decimal Amount,
    DateTime Date,
    string Description,
    string? Notes,
    IReadOnlyList<string> Tags,
    Guid? TransferAccountId);

public sealed record PagedResult<T>(IReadOnlyList<T> Items, int Total, int Page, int PageSize);

public sealed class TransactionRequest
{
    [Required]
    public Guid AccountId { get; init; }

    public Guid? CategoryId { get; init; }

    [Required]
    public TransactionType Type { get; init; }

    [Range(typeof(decimal), "0.01", "999999999999.99")]
    public decimal Amount { get; init; }

    [Required]
    public DateTime Date { get; init; }

    [Required]
    [MaxLength(500)]
    public string Description { get; init; } = string.Empty;

    [MaxLength(2000)]
    public string? Notes { get; init; }

    public IReadOnlyList<string>? Tags { get; init; }

    public Guid? TransferAccountId { get; init; }
}

/// <summary>Query string filters for <c>GET /api/transactions</c>.</summary>
public sealed class TransactionQuery
{
    [Range(1, int.MaxValue)]
    public int Page { get; init; } = 1;

    [Range(1, 200)]
    public int PageSize { get; init; } = 20;

    public Guid? AccountId { get; init; }

    public Guid? CategoryId { get; init; }

    public TransactionType? Type { get; init; }

    public DateTime? From { get; init; }

    public DateTime? To { get; init; }

    [MaxLength(200)]
    public string? Search { get; init; }
}

public sealed record ImportResult(int Imported, int Skipped);
