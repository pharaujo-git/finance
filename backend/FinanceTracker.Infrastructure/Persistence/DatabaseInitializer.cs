using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.DependencyInjection;

namespace FinanceTracker.Infrastructure.Persistence;

/// <summary>Creates the schema and seeds the shared default categories at startup.</summary>
public static class DatabaseInitializer
{
    public static async Task InitializeAsync(
        IServiceProvider services,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(services);

        await using var scope = services.CreateAsyncScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();

        await db.Database.EnsureCreatedAsync(cancellationToken).ConfigureAwait(false);
        await DefaultCategorySeeder.SeedAsync(db, cancellationToken).ConfigureAwait(false);
    }
}
