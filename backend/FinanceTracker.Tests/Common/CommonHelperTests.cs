using System.Security.Claims;
using FinanceTracker.Api.Common;
using FinanceTracker.Application.Common;
using FinanceTracker.Application.Services;
using FinanceTracker.Domain;
using FinanceTracker.Infrastructure.Hosting;
using FinanceTracker.Infrastructure.Identity;
using FinanceTracker.Infrastructure.Persistence;
using Microsoft.Extensions.Configuration;
using Microsoft.IdentityModel.JsonWebTokens;

namespace FinanceTracker.Tests.Common;

public sealed class CsvFileTests
{
    [Fact]
    public void ParseHandlesQuotesEmbeddedDelimitersAndNewlines()
    {
        var rows = CsvFile.Parse("a,b\n\"x,1\",\"he said \"\"hi\"\"\"\n\"multi\nline\",z\n");

        Assert.Equal(3, rows.Count);
        Assert.Equal(["a", "b"], rows[0]);
        Assert.Equal(["x,1", "he said \"hi\""], rows[1]);
        Assert.Equal(["multi\nline", "z"], rows[2]);
    }

    [Fact]
    public void ParseIgnoresTrailingBlankLinesAndCarriageReturns()
    {
        var rows = CsvFile.Parse("a,b\r\nc,d\r\n\r\n");

        Assert.Equal(2, rows.Count);
        Assert.Equal(["c", "d"], rows[1]);
    }

    [Fact]
    public void ParseOfNullOrEmptyYieldsNoRows() => Assert.Empty(CsvFile.Parse(null));

    [Theory]
    [InlineData("plain", "plain")]
    [InlineData("with,comma", "\"with,comma\"")]
    [InlineData("with\"quote", "\"with\"\"quote\"")]
    [InlineData(null, "")]
    public void EscapeFieldQuotesOnlyWhenNecessary(string? input, string expected) =>
        Assert.Equal(expected, CsvFile.EscapeField(input));
}

public sealed class MonthKeyTests
{
    [Fact]
    public void ParseReturnsFirstDayOfMonthInUtc()
    {
        var start = MonthKey.Parse("2026-07");

        Assert.Equal(new DateTime(2026, 7, 1, 0, 0, 0, DateTimeKind.Utc), start);
    }

    [Theory]
    [InlineData("2026-13")]
    [InlineData("26-07")]
    [InlineData("")]
    [InlineData(null)]
    public void ParseRejectsMalformedInput(string? value)
    {
        var error = Assert.Throws<AppException>(() => MonthKey.Parse(value));

        Assert.Equal(ErrorKind.Validation, error.Kind);
    }

    [Fact]
    public void TrailingMonthsEndsWithTheReferenceMonth()
    {
        var months = MonthKey.TrailingMonths(new DateTime(2026, 3, 20, 0, 0, 0, DateTimeKind.Utc), 3);

        Assert.Equal(["2026-01", "2026-02", "2026-03"], months.Select(MonthKey.From));
    }
}

public sealed class ServerBindingTests
{
    [Theory]
    [InlineData("9091", 9091)]
    [InlineData("1", 1)]
    [InlineData("65535", 65535)]
    public void ValidPortsAreHonoured(string configured, int expected) =>
        Assert.Equal(expected, ServerBinding.ResolvePort(Configuration(configured)));

    [Theory]
    [InlineData(null)]
    [InlineData("")]
    [InlineData("not-a-number")]
    [InlineData("0")]
    [InlineData("70000")]
    [InlineData("-1")]
    public void MissingOrOutOfRangePortsFallBackToTheDefault(string? configured) =>
        Assert.Equal(ServerBinding.DefaultPort, ServerBinding.ResolvePort(Configuration(configured)));

    private static IConfiguration Configuration(string? port) =>
        new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?> { [ServerBinding.PortVariable] = port })
            .Build();
}

public sealed class CorsOriginsTests
{
    [Fact]
    public void MissingVariableFallsBackToTheViteDevServer() =>
        Assert.Equal([CorsOrigins.Default], CorsOrigins.Read(Configuration(null)));

    [Fact]
    public void CommaSeparatedOriginsAreSplitAndTrimmed()
    {
        var origins = CorsOrigins.Read(Configuration(" https://app.example.com , https://staging.example.com "));

        Assert.Equal(["https://app.example.com", "https://staging.example.com"], origins);
    }

    private static IConfiguration Configuration(string? value) =>
        new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?> { [CorsOrigins.Variable] = value })
            .Build();
}

public sealed class UtcDateTests
{
    [Fact]
    public void UnspecifiedIsTreatedAsUtc()
    {
        var normalized = UtcDate.Normalize(new DateTime(2026, 1, 1, 12, 0, 0, DateTimeKind.Unspecified));

        Assert.Equal(DateTimeKind.Utc, normalized.Kind);
        Assert.Equal(12, normalized.Hour);
    }

    [Fact]
    public void LocalIsConvertedAndNullPassesThrough()
    {
        var local = new DateTime(2026, 1, 1, 12, 0, 0, DateTimeKind.Local);

        Assert.Equal(local.ToUniversalTime(), UtcDate.Normalize(local));
        Assert.Null(UtcDate.Normalize((DateTime?)null));
    }
}

public sealed class ClaimsPrincipalExtensionsTests
{
    [Fact]
    public void GetUserIdReadsTheSubjectClaim()
    {
        var userId = Guid.NewGuid();
        var principal = new ClaimsPrincipal(new ClaimsIdentity(
            [new Claim(JwtRegisteredClaimNames.Sub, userId.ToString())]));

        Assert.Equal(userId, principal.GetUserId());
    }

    [Fact]
    public void GetUserIdFallsBackToNameIdentifier()
    {
        var userId = Guid.NewGuid();
        var principal = new ClaimsPrincipal(new ClaimsIdentity(
            [new Claim(ClaimTypes.NameIdentifier, userId.ToString())]));

        Assert.Equal(userId, principal.GetUserId());
    }

    [Fact]
    public void GetUserIdThrowsUnauthorizedWhenTheClaimIsMissing()
    {
        var error = Assert.Throws<AppException>(() => new ClaimsPrincipal(new ClaimsIdentity()).GetUserId());

        Assert.Equal(ErrorKind.Unauthorized, error.Kind);
    }
}

public sealed class DatabaseConfigurationTests
{
    [Fact]
    public void RenderStyleUrlBecomesAnNpgsqlConnectionString()
    {
        var connectionString = DatabaseConfiguration.BuildNpgsqlConnectionString(
            "postgres://finance_user:s3cr3t%40@db.example.com:6543/finance?sslmode=require");

        Assert.Contains("Host=db.example.com", connectionString, StringComparison.Ordinal);
        Assert.Contains("Port=6543", connectionString, StringComparison.Ordinal);
        Assert.Contains("Database=finance", connectionString, StringComparison.Ordinal);
        Assert.Contains("Username=finance_user", connectionString, StringComparison.Ordinal);
        Assert.Contains("Require", connectionString, StringComparison.Ordinal);
    }

    [Fact]
    public void MissingPortFallsBackToThePostgresDefault()
    {
        var connectionString = DatabaseConfiguration.BuildNpgsqlConnectionString(
            "postgresql://user:pass@neon.example.com/finance");

        Assert.Contains("Port=5432", connectionString, StringComparison.Ordinal);
    }

    [Fact]
    public void MalformedUrlIsRejected() =>
        Assert.Throws<ArgumentException>(() => DatabaseConfiguration.BuildNpgsqlConnectionString("not a url"));
}

public sealed class TokenServiceTests
{
    [Fact]
    public void TokenCarriesTheUserIdAndAnExpiry()
    {
        var options = new JwtOptions();
        var user = new User { Email = "token@example.com", Name = "Token" };

        var token = new TokenService(options).CreateToken(user);
        var parsed = new JsonWebTokenHandler().ReadJsonWebToken(token);

        Assert.Equal(user.Id.ToString(), parsed.Subject);
        Assert.Equal(options.Issuer, parsed.Issuer);
        Assert.True(parsed.ValidTo > DateTime.UtcNow.AddDays(6));
    }

    [Fact]
    public void ConfigurationSecretWinsOverTheDevelopmentFallback()
    {
        var configured = JwtOptions.FromConfiguration(Configuration(JwtOptions.SecretVariable, "a-configured-secret-value"));
        var fallback = JwtOptions.FromConfiguration(Configuration(JwtOptions.SecretVariable, null));

        Assert.Equal("a-configured-secret-value", configured.Secret);
        Assert.Equal(JwtOptions.LocalDevelopmentSecret, fallback.Secret);
    }

    private static IConfiguration Configuration(string key, string? value) =>
        new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?> { [key] = value })
            .Build();
}
