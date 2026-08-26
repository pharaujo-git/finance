using FinanceTracker.Application.Services;
using Microsoft.Extensions.DependencyInjection;

namespace FinanceTracker.Application;

/// <summary>Registers the application's use-case services.</summary>
public static class DependencyInjection
{
    public static IServiceCollection AddApplication(this IServiceCollection services)
    {
        ArgumentNullException.ThrowIfNull(services);

        services.AddScoped<AuthService>();
        services.AddScoped<AccountService>();
        services.AddScoped<CategoryService>();
        services.AddScoped<TransactionService>();
        services.AddScoped<TransactionCsvService>();
        services.AddScoped<RecurringService>();
        services.AddScoped<BudgetService>();
        services.AddScoped<GoalService>();
        services.AddScoped<AnalyticsService>();

        return services;
    }
}
