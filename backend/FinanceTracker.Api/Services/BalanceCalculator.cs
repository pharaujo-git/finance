using FinanceTracker.Api.Models;

namespace FinanceTracker.Api.Services;

/// <summary>The minimum a transaction contributes to any balance calculation.</summary>
public sealed record TransactionSlice(
    Guid AccountId,
    Guid? TransferAccountId,
    Guid? CategoryId,
    TransactionType Type,
    decimal Amount,
    DateTime Date);

/// <summary>
/// Single source of truth for how a transaction moves money. Aggregation runs in memory so
/// decimal arithmetic stays exact on both SQLite and Postgres.
/// </summary>
public static class BalanceCalculator
{
    /// <summary>Signed effect of <paramref name="slice"/> on one specific account.</summary>
    public static decimal DeltaFor(Guid accountId, TransactionSlice slice)
    {
        ArgumentNullException.ThrowIfNull(slice);

        if (slice.Type == TransactionType.Transfer && slice.TransferAccountId == accountId)
        {
            return slice.Amount;
        }

        if (slice.AccountId != accountId)
        {
            return 0m;
        }

        return slice.Type == TransactionType.Income ? slice.Amount : -slice.Amount;
    }

    /// <summary>Signed effect on total net worth: transfers between own accounts cancel out.</summary>
    public static decimal NetWorthDelta(TransactionSlice slice)
    {
        ArgumentNullException.ThrowIfNull(slice);

        return slice.Type switch
        {
            TransactionType.Income => slice.Amount,
            TransactionType.Expense => -slice.Amount,
            _ => 0m,
        };
    }

    public static decimal BalanceOf(Account account, IEnumerable<TransactionSlice> slices)
    {
        ArgumentNullException.ThrowIfNull(account);
        ArgumentNullException.ThrowIfNull(slices);

        return account.InitialBalance + slices.Sum(s => DeltaFor(account.Id, s));
    }
}
