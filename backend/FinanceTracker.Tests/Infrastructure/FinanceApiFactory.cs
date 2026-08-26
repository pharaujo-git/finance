using System.Globalization;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using FinanceTracker.Api.Dtos;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.Data.Sqlite;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Hosting;

namespace FinanceTracker.Tests.Infrastructure;

/// <summary>Boots the real HTTP pipeline against a private SQLite file.</summary>
public sealed class FinanceApiFactory : WebApplicationFactory<Program>
{
    private readonly string _databasePath = Path.Combine(
        Path.GetTempPath(),
        string.Create(CultureInfo.InvariantCulture, $"finance-tests-{Guid.NewGuid():N}.db"));

    public static JsonSerializerOptions Json { get; } = new(JsonSerializerDefaults.Web)
    {
        Converters = { new JsonStringEnumConverter(JsonNamingPolicy.CamelCase) },
    };

    /// <summary>Registers a fresh user and returns a client that already carries their bearer token.</summary>
    public async Task<(HttpClient Client, UserDto User)> CreateAuthenticatedClientAsync(string? email = null)
    {
        var client = CreateClient();
        var request = new RegisterRequest
        {
            Email = email ?? string.Create(CultureInfo.InvariantCulture, $"user-{Guid.NewGuid():N}@example.com"),
            Password = "correct horse battery",
            Name = "Integration User",
        };

        var response = await client.PostAsJsonAsync("/api/auth/register", request, Json);
        response.EnsureSuccessStatusCode();

        var payload = await response.Content.ReadFromJsonAsync<AuthResponse>(Json);
        Assert.NotNull(payload);

        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", payload.Token);
        return (client, payload.User);
    }

    protected override void ConfigureWebHost(IWebHostBuilder builder)
    {
        ArgumentNullException.ThrowIfNull(builder);

        builder.UseEnvironment(Environments.Development);
        builder.ConfigureAppConfiguration((_, configuration) => configuration.AddInMemoryCollection(
            new Dictionary<string, string?>
            {
                ["ConnectionStrings:Sqlite"] = $"Data Source={_databasePath}",
                ["JWT_SECRET"] = "integration-test-signing-key-not-used-in-production",
                ["ALLOWED_ORIGINS"] = "http://localhost:5173,http://localhost:4173",
            }));
    }

    protected override void Dispose(bool disposing)
    {
        base.Dispose(disposing);

        if (!disposing)
        {
            return;
        }

        SqliteConnection.ClearAllPools();

        foreach (var suffix in new[] { string.Empty, "-shm", "-wal" })
        {
            var path = _databasePath + suffix;
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }
    }
}
