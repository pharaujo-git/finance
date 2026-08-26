using System.Text;
using Microsoft.IdentityModel.Tokens;

namespace FinanceTracker.Api.Services;

/// <summary>Signing configuration for the bearer tokens issued by this API.</summary>
public sealed class JwtOptions
{
    /// <summary>
    /// Placeholder used only when <c>JWT_SECRET</c> is absent, so a developer can run the API locally.
    /// Production deployments must supply the environment variable.
    /// </summary>
    public const string LocalDevelopmentSecret = "finance-tracker-local-development-signing-key-please-override";

    public const string SecretVariable = "JWT_SECRET";

    public string Secret { get; init; } = LocalDevelopmentSecret;

    public string Issuer { get; init; } = "finance-tracker";

    public string Audience { get; init; } = "finance-tracker";

    public int ExpiryDays { get; init; } = 7;

    public SymmetricSecurityKey SigningKey() => new(Encoding.UTF8.GetBytes(Secret));

    public static JwtOptions FromConfiguration(IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);

        var secret = configuration[SecretVariable];
        return new JwtOptions
        {
            Secret = string.IsNullOrWhiteSpace(secret) ? LocalDevelopmentSecret : secret,
        };
    }
}
