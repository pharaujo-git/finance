using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace FinanceTracker.Infrastructure.Persistence;

/// <summary>
/// Creates the schema and seeds the shared default categories at startup, but only for the
/// throwaway SQLite database used by local development and the test suite.
/// </summary>
/// <remarks>
/// Postgres deployments (anything with <c>DATABASE_URL</c> set) own their schema through the
/// backend-neutral dbmate migrations in <c>db/migrations</c>, so no backend gets to issue DDL
/// against them. Letting <c>EnsureCreated</c> touch a migrated database would be at best a
/// no-op and at worst a silent divergence between what EF believes and what actually shipped.
/// </remarks>
public static class DatabaseInitializer
{
    public static async Task InitializeAsync(
        IServiceProvider services,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(services);

        await using var scope = services.CreateAsyncScope();
        var configuration = scope.ServiceProvider.GetRequiredService<IConfiguration>();

        if (!string.IsNullOrWhiteSpace(configuration[DatabaseConfiguration.DatabaseUrlVariable]))
        {
            scope.ServiceProvider
                .GetRequiredService<ILoggerFactory>()
                .CreateLogger(typeof(DatabaseInitializer))
                .LogInformation(
                    "DATABASE_URL is set; skipping schema creation and seeding. "
                    + "The database schema is managed by dbmate (see db/README.md).");
            return;
        }

        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();

        await db.Database.EnsureCreatedAsync(cancellationToken).ConfigureAwait(false);
        await DefaultCategorySeeder.SeedAsync(db, cancellationToken).ConfigureAwait(false);
    }
}
