using FinanceTracker.Api.Common;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Data;

/// <summary>One place where "scope to the signed-in user, otherwise 404" is implemented.</summary>
public static class OwnedEntityQueries
{
    public static IQueryable<TEntity> OwnedBy<TEntity>(this IQueryable<TEntity> source, Guid userId)
        where TEntity : class, IUserOwned
    {
        ArgumentNullException.ThrowIfNull(source);
        return source.Where(e => e.UserId == userId);
    }

    public static async Task<TEntity> GetOwnedAsync<TEntity>(
        this DbSet<TEntity> set,
        Guid id,
        Guid userId,
        string entityName,
        CancellationToken cancellationToken)
        where TEntity : class, IUserOwned
    {
        ArgumentNullException.ThrowIfNull(set);

        var entity = await set
            .FirstOrDefaultAsync(e => e.Id == id && e.UserId == userId, cancellationToken)
            .ConfigureAwait(false);

        return entity ?? throw ApiException.NotFound(entityName);
    }
}
