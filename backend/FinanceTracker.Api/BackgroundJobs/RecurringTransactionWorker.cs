using FinanceTracker.Api.Services;

namespace FinanceTracker.Api.BackgroundJobs;

/// <summary>Materializes due recurring rules at startup and then every six hours.</summary>
public sealed class RecurringTransactionWorker(
    IServiceScopeFactory scopeFactory,
    ILogger<RecurringTransactionWorker> logger) : BackgroundService
{
    public static readonly TimeSpan Interval = TimeSpan.FromHours(6);

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
        catch (OperationCanceledException)
        {
            logger.LogInformation("Recurring transaction worker stopping.");
        }
    }

    private async Task RunOnceAsync(CancellationToken cancellationToken)
    {
        try
        {
            await using var scope = scopeFactory.CreateAsyncScope();
            var service = scope.ServiceProvider.GetRequiredService<RecurringService>();
            var created = await service.MaterializeDueAsync(DateTime.UtcNow, cancellationToken).ConfigureAwait(false);

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
}
