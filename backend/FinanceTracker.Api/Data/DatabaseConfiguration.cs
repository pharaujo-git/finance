using Microsoft.EntityFrameworkCore;
using Npgsql;

namespace FinanceTracker.Api.Data;

/// <summary>
/// Chooses the EF Core provider: Postgres when <c>DATABASE_URL</c> is present
/// (Render/Neon style), SQLite otherwise.
/// </summary>
public static class DatabaseConfiguration
{
    public const string DatabaseUrlVariable = "DATABASE_URL";

    public const string DefaultSqliteConnectionString = "Data Source=finance.db";

    /// <summary>Translates a <c>postgres://user:pass@host:port/db?sslmode=require</c> URL into an ADO.NET connection string.</summary>
    /// <exception cref="ArgumentException">The URL is not a well-formed absolute URI.</exception>
    public static string BuildNpgsqlConnectionString(string databaseUrl)
    {
        if (!Uri.TryCreate(databaseUrl, UriKind.Absolute, out var uri))
        {
            throw new ArgumentException("DATABASE_URL is not a valid absolute URI.", nameof(databaseUrl));
        }

        var credentials = uri.UserInfo.Split(':', 2);
        var builder = new NpgsqlConnectionStringBuilder
        {
            Host = uri.Host,
            Port = uri.Port > 0 ? uri.Port : 5432,
            Username = Uri.UnescapeDataString(credentials[0]),
            Password = credentials.Length > 1 ? Uri.UnescapeDataString(credentials[1]) : string.Empty,
            Database = Uri.UnescapeDataString(uri.AbsolutePath.TrimStart('/')),
        };

        foreach (var pair in ParseQuery(uri.Query))
        {
            ApplyQueryOption(builder, pair.Key, pair.Value);
        }

        return builder.ConnectionString;
    }

    /// <summary>Registers <see cref="AppDbContext"/> with the provider implied by the environment.</summary>
    public static IServiceCollection AddAppDatabase(this IServiceCollection services)
    {
        ArgumentNullException.ThrowIfNull(services);

        services.AddDbContext<AppDbContext>((provider, options) =>
        {
            var configuration = provider.GetRequiredService<IConfiguration>();
            var databaseUrl = configuration[DatabaseUrlVariable];

            if (string.IsNullOrWhiteSpace(databaseUrl))
            {
                options.UseSqlite(configuration.GetConnectionString("Sqlite") ?? DefaultSqliteConnectionString);
            }
            else
            {
                options.UseNpgsql(BuildNpgsqlConnectionString(databaseUrl));
            }
        });

        return services;
    }

    private static void ApplyQueryOption(NpgsqlConnectionStringBuilder builder, string key, string value)
    {
        if (string.Equals(key, "sslmode", StringComparison.OrdinalIgnoreCase)
            && Enum.TryParse<SslMode>(value, ignoreCase: true, out var sslMode))
        {
            builder.SslMode = sslMode;
        }
        else if (string.Equals(key, "options", StringComparison.OrdinalIgnoreCase))
        {
            builder.Options = value;
        }
    }

    private static IEnumerable<KeyValuePair<string, string>> ParseQuery(string query)
    {
        foreach (var part in query.TrimStart('?').Split('&', StringSplitOptions.RemoveEmptyEntries))
        {
            var pieces = part.Split('=', 2);
            if (pieces.Length == 2)
            {
                yield return new KeyValuePair<string, string>(
                    Uri.UnescapeDataString(pieces[0]),
                    Uri.UnescapeDataString(pieces[1]));
            }
        }
    }
}
