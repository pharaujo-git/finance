using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>Transaction CRUD, paged filtering and CSV import/export.</summary>
public sealed class TransactionService(AppDbContext db, CategoryService categories)
{
    private const string EntityName = "Transaction";

    public async Task<PagedResult<TransactionDto>> SearchAsync(
        Guid userId,
        TransactionQuery query,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(query);

        var page = query.EffectivePage;
        var pageSize = query.EffectivePageSize;

        var filtered = ApplyFilters(db.Transactions.AsNoTracking().OwnedBy(userId), query);
        var total = await filtered.CountAsync(cancellationToken).ConfigureAwait(false);

        var items = await filtered
            .OrderByDescending(t => t.Date)
            .ThenByDescending(t => t.Id)
            .Skip((page - 1) * pageSize)
            .Take(pageSize)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        return new PagedResult<TransactionDto>(items.Select(Map).ToList(), total, page, pageSize);
    }

    public async Task<TransactionDto> GetAsync(Guid userId, Guid id, CancellationToken cancellationToken) =>
        Map(await db.Transactions.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false));

    public async Task<TransactionDto> CreateAsync(
        Guid userId,
        TransactionRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var transaction = new Transaction { Id = Guid.NewGuid(), UserId = userId };
        await ApplyAsync(userId, transaction, request, cancellationToken).ConfigureAwait(false);

        db.Transactions.Add(transaction);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(transaction);
    }

    public async Task<TransactionDto> UpdateAsync(
        Guid userId,
        Guid id,
        TransactionRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var transaction = await db.Transactions
            .GetOwnedAsync(id, userId, EntityName, cancellationToken)
            .ConfigureAwait(false);

        await ApplyAsync(userId, transaction, request, cancellationToken).ConfigureAwait(false);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(transaction);
    }

    public async Task DeleteAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var transaction = await db.Transactions
            .GetOwnedAsync(id, userId, EntityName, cancellationToken)
            .ConfigureAwait(false);

        db.Transactions.Remove(transaction);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
    }

    /// <summary>Loads the slices used by dashboard and report aggregations.</summary>
    public async Task<List<TransactionSlice>> LoadSlicesAsync(
        Guid userId,
        DateTime? from,
        DateTime? to,
        CancellationToken cancellationToken)
    {
        var query = db.Transactions.AsNoTracking().OwnedBy(userId);

        if (from.HasValue)
        {
            var fromUtc = UtcDate.Normalize(from.Value);
            query = query.Where(t => t.Date >= fromUtc);
        }

        if (to.HasValue)
        {
            var toUtc = UtcDate.Normalize(to.Value);
            query = query.Where(t => t.Date <= toUtc);
        }

        return await query
            .Select(t => new TransactionSlice(t.AccountId, t.TransferAccountId, t.CategoryId, t.Type, t.Amount, t.Date))
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);
    }

    private static IQueryable<Transaction> ApplyFilters(IQueryable<Transaction> query, TransactionQuery filter)
    {
        if (filter.AccountId is { } accountId)
        {
            query = query.Where(t => t.AccountId == accountId || t.TransferAccountId == accountId);
        }

        if (filter.CategoryId is { } categoryId)
        {
            query = query.Where(t => t.CategoryId == categoryId);
        }

        if (filter.Type is { } type)
        {
            query = query.Where(t => t.Type == type);
        }

        if (filter.From is { } from)
        {
            var fromUtc = UtcDate.Normalize(from);
            query = query.Where(t => t.Date >= fromUtc);
        }

        if (filter.To is { } to)
        {
            var toUtc = UtcDate.Normalize(to);
            query = query.Where(t => t.Date <= toUtc);
        }

        if (!string.IsNullOrWhiteSpace(filter.Search))
        {
            var term = filter.Search.Trim().ToLowerInvariant();
            query = query.Where(t =>
                t.Description.ToLower().Contains(term)
                || (t.Notes != null && t.Notes.ToLower().Contains(term))
                || t.TagsRaw.ToLower().Contains(term));
        }

        return query;
    }

    private async Task ApplyAsync(
        Guid userId,
        Transaction transaction,
        TransactionRequest request,
        CancellationToken cancellationToken)
    {
        await EnsureAccountAsync(userId, request.AccountId, cancellationToken).ConfigureAwait(false);
        await categories.EnsureUsableAsync(userId, request.CategoryId, cancellationToken).ConfigureAwait(false);

        Guid? transferAccountId = null;
        if (request.Type == TransactionType.Transfer)
        {
            if (request.TransferAccountId is not { } target)
            {
                throw ApiException.BadRequest("A transfer requires a destination account.");
            }

            if (target == request.AccountId)
            {
                throw ApiException.BadRequest("A transfer must use two different accounts.");
            }

            await EnsureAccountAsync(userId, target, cancellationToken).ConfigureAwait(false);
            transferAccountId = target;
        }

        transaction.AccountId = request.AccountId;
        transaction.CategoryId = request.CategoryId;
        transaction.Type = request.Type;
        transaction.Amount = decimal.Round(request.Amount, 2, MidpointRounding.AwayFromZero);
        transaction.Date = UtcDate.Normalize(request.Date);
        transaction.Description = request.Description.Trim();
        transaction.Notes = string.IsNullOrWhiteSpace(request.Notes) ? null : request.Notes.Trim();
        transaction.TagsRaw = Transaction.JoinTags(request.Tags);
        transaction.TransferAccountId = transferAccountId;
    }

    private async Task EnsureAccountAsync(Guid userId, Guid accountId, CancellationToken cancellationToken)
    {
        var exists = await db.Accounts
            .AsNoTracking()
            .AnyAsync(a => a.Id == accountId && a.UserId == userId, cancellationToken)
            .ConfigureAwait(false);

        if (!exists)
        {
            throw ApiException.NotFound("Account");
        }
    }

    internal static TransactionDto Map(Transaction transaction) => new(
        transaction.Id,
        transaction.AccountId,
        transaction.CategoryId,
        transaction.Type,
        transaction.Amount,
        transaction.Date,
        transaction.Description,
        transaction.Notes,
        Transaction.SplitTags(transaction.TagsRaw),
        transaction.TransferAccountId);
}
