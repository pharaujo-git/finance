using FinanceTracker.Application.Services;
using FinanceTracker.Infrastructure.Persistence;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace FinanceTracker.Infrastructure.BackgroundJobs;

/// <summary>Materializes due recurring rules at startup and then every six hours.</summary>
public sealed class RecurringTransactionWorker(
    IServiceScopeFactory scopeFactory,
    ILogger<RecurringTransactionWorker> logger) : BackgroundService
{
    public static readonly TimeSpan Interval = TimeSpan.FromHours(6);

    /// <summary>
    /// Key of the Postgres advisory lock that serializes the pass. Any number would do; what
    /// matters is that every host taking it agrees, which is why the Go edition of this API
    /// (backend-go/internal/infrastructure/jobs) hard-codes the same one. Two instances — or the
    /// two backends deployed side by side — would otherwise materialize the same occurrence twice.
    /// </summary>
    public const long PassLockKey = 723441001L;

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using var timer = new PeriodicTimer(Interval);

        try
        {
            do
            {
                await RunOnceAsync(stoppingToken).ConfigureAwait(false);
            }
            while (await timer.WaitForNextTickAsync(stoppingToken).ConfigureAwait(false));
        }
        catch (OperationCanceledException ex)
        {
            logger.LogInformation(ex, "Recurring transaction worker stopping after host shutdown.");
        }
    }

    private async Task RunOnceAsync(CancellationToken cancellationToken)
    {
        try
        {
            await using var scope = scopeFactory.CreateAsyncScope();
            var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
            var service = scope.ServiceProvider.GetRequiredService<RecurringService>();

            var created = await MaterializeAsync(db, service, cancellationToken).ConfigureAwait(false);
            if (created is null)
            {
                logger.LogInformation("Recurring pass skipped: another instance holds the pass lock.");
                return;
            }

            logger.LogInformation("Recurring pass created {Created} transaction(s).", created);
        }
        catch (OperationCanceledException)
        {
            throw;
        }
        catch (Exception ex)
        {
            // A failed pass must not take the host down; the next tick retries.
            logger.LogError(ex, "Recurring transaction pass failed.");
        }
    }

    /// <summary>
    /// Runs one pass, guarded by a cluster-wide lock when the store supports one.
    /// </summary>
    /// <returns>
    /// The number of transactions created, or <see langword="null"/> when another instance held the
    /// lock and this pass did nothing.
    /// </returns>
    /// <remarks>
    /// The lock is an <c>xact</c> one, so it covers the whole pass and is released by the commit —
    /// or by the rollback the transaction performs if this process dies mid-pass. Only Npgsql
    /// offers it: on SQLite, which is the single-process development and test store, the pass runs
    /// exactly as it did before.
    /// </remarks>
    private static async Task<int?> MaterializeAsync(
        AppDbContext db,
        RecurringService service,
        CancellationToken cancellationToken)
    {
        if (!db.Database.IsNpgsql())
        {
            return await service.MaterializeDueAsync(DateTime.UtcNow, cancellationToken).ConfigureAwait(false);
        }

        await using var transaction = await db.Database
            .BeginTransactionAsync(cancellationToken)
            .ConfigureAwait(false);

        // EF Core projects a scalar query onto a column that has to be called "Value"; the key
        // itself travels as a parameter, not as text spliced into the statement.
        var acquired = await db.Database
            .SqlQuery<bool>($"""SELECT pg_try_advisory_xact_lock({PassLockKey}) AS "Value" """)
            .SingleAsync(cancellationToken)
            .ConfigureAwait(false);

        if (!acquired)
        {
            return null;
        }

        var created = await service.MaterializeDueAsync(DateTime.UtcNow, cancellationToken).ConfigureAwait(false);
        await transaction.CommitAsync(cancellationToken).ConfigureAwait(false);

        return created;
    }
}
