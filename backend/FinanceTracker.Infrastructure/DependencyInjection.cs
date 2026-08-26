using FinanceTracker.Application.Abstractions;
using FinanceTracker.Infrastructure.BackgroundJobs;
using FinanceTracker.Infrastructure.Identity;
using FinanceTracker.Infrastructure.Persistence;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

namespace FinanceTracker.Infrastructure;

/// <summary>Binds the application's ports to their concrete adapters.</summary>
public static class DependencyInjection
{
    public static IServiceCollection AddInfrastructure(this IServiceCollection services)
    {
        ArgumentNullException.ThrowIfNull(services);

        // Resolved from DI so late-bound configuration (environment variables, test overrides)
        // is honoured rather than snapshotted at registration time.
        services.AddSingleton(sp => JwtOptions.FromConfiguration(sp.GetRequiredService<IConfiguration>()));
        services.AddAppDatabase();
        services.AddScoped<IAppDbContext>(sp => sp.GetRequiredService<AppDbContext>());
        services.AddSingleton<IPasswordHasher, IdentityPasswordHasher>();
        services.AddSingleton<ITokenService, TokenService>();
        services.AddHostedService<RecurringTransactionWorker>();

        return services;
    }
}
