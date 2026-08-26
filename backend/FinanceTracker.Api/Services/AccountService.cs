using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>Account CRUD plus live balance computation.</summary>
public sealed class AccountService(AppDbContext db)
{
    private const string EntityName = "Account";

    public async Task<IReadOnlyList<AccountDto>> ListAsync(Guid userId, CancellationToken cancellationToken)
    {
        var accounts = await db.Accounts
            .AsNoTracking()
            .OwnedBy(userId)
            .OrderBy(a => a.IsArchived)
            .ThenBy(a => a.Name)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        var slices = await LoadSlicesAsync(userId, cancellationToken).ConfigureAwait(false);
        return accounts.Select(a => Map(a, BalanceCalculator.BalanceOf(a, slices))).ToList();
    }

    public async Task<AccountDto> GetAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var account = await db.Accounts.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        var slices = await LoadSlicesAsync(userId, cancellationToken).ConfigureAwait(false);
        return Map(account, BalanceCalculator.BalanceOf(account, slices));
    }

    public async Task<AccountDto> CreateAsync(
        Guid userId,
        CreateAccountRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var account = new Account
        {
            UserId = userId,
            Name = request.Name.Trim(),
            Type = request.Type,
            InitialBalance = request.InitialBalance ?? 0m,
            Currency = request.Currency.Trim().ToUpperInvariant(),
            CreatedAt = DateTime.UtcNow,
        };

        db.Accounts.Add(account);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(account, account.InitialBalance);
    }

    public async Task<AccountDto> UpdateAsync(
        Guid userId,
        Guid id,
        UpdateAccountRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var account = await db.Accounts.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        account.Name = request.Name.Trim();
        account.Type = request.Type;
        account.Currency = request.Currency.Trim().ToUpperInvariant();
        account.IsArchived = request.IsArchived ?? false;
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        var slices = await LoadSlicesAsync(userId, cancellationToken).ConfigureAwait(false);
        return Map(account, BalanceCalculator.BalanceOf(account, slices));
    }

    /// <summary>Archives rather than deletes, so historical transactions stay intact.</summary>
    public async Task ArchiveAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var account = await db.Accounts.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        account.IsArchived = true;
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
    }

    private async Task<List<TransactionSlice>> LoadSlicesAsync(Guid userId, CancellationToken cancellationToken) =>
        await db.Transactions
            .AsNoTracking()
            .OwnedBy(userId)
            .Select(t => new TransactionSlice(t.AccountId, t.TransferAccountId, t.CategoryId, t.Type, t.Amount, t.Date))
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

    private static AccountDto Map(Account account, decimal balance) => new(
        account.Id,
        account.Name,
        account.Type,
        balance,
        account.Currency,
        account.IsArchived,
        account.CreatedAt);
}
