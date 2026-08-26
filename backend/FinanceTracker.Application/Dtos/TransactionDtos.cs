using System.ComponentModel.DataAnnotations;
using FinanceTracker.Domain;

namespace FinanceTracker.Application.Dtos;

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
    public required Guid AccountId { get; init; }

    public Guid? CategoryId { get; init; }

    public required TransactionType Type { get; init; }

    [Range(typeof(decimal), "0.01", "999999999999.99")]
    public required decimal Amount { get; init; }

    public required DateTime Date { get; init; }

    [Required]
    [MaxLength(500)]
    public string Description { get; init; } = string.Empty;

    [MaxLength(2000)]
    public string? Notes { get; init; }

    public IReadOnlyList<string>? Tags { get; init; }

    public Guid? TransferAccountId { get; init; }
}

/// <summary>Query string filters for <c>GET /api/transactions</c>. Every value is optional.</summary>
public sealed class TransactionQuery
{
    public const int DefaultPage = 1;

    public const int DefaultPageSize = 20;

    public const int MaxPageSize = 200;

    [Range(1, int.MaxValue)]
    public int? Page { get; init; }

    [Range(1, MaxPageSize)]
    public int? PageSize { get; init; }

    public Guid? AccountId { get; init; }

    public Guid? CategoryId { get; init; }

    public TransactionType? Type { get; init; }

    public DateTime? From { get; init; }

    public DateTime? To { get; init; }

    [MaxLength(200)]
    public string? Search { get; init; }

    /// <summary>The requested page, or the default when the caller omitted it.</summary>
    public int EffectivePage => Page ?? DefaultPage;

    /// <summary>The requested page size, or the default when the caller omitted it.</summary>
    public int EffectivePageSize => PageSize ?? DefaultPageSize;
}

public sealed record ImportResult(int Imported, int Skipped);
