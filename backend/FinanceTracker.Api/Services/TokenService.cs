using System.Globalization;
using System.Security.Claims;
using FinanceTracker.Api.Models;
using Microsoft.IdentityModel.JsonWebTokens;
using Microsoft.IdentityModel.Tokens;

namespace FinanceTracker.Api.Services;

public interface ITokenService
{
    string CreateToken(User user);
}

/// <summary>Issues HS256 bearer tokens that expire after <see cref="JwtOptions.ExpiryDays"/>.</summary>
public sealed class TokenService(JwtOptions options) : ITokenService
{
    private readonly JsonWebTokenHandler _handler = new();

    public string CreateToken(User user)
    {
        ArgumentNullException.ThrowIfNull(user);

        var now = DateTime.UtcNow;
        var descriptor = new SecurityTokenDescriptor
        {
            Issuer = options.Issuer,
            Audience = options.Audience,
            IssuedAt = now,
            NotBefore = now,
            Expires = now.AddDays(options.ExpiryDays),
            Subject = new ClaimsIdentity(
            [
                new Claim(JwtRegisteredClaimNames.Sub, user.Id.ToString()),
                new Claim(JwtRegisteredClaimNames.Email, user.Email),
                new Claim(JwtRegisteredClaimNames.Jti, Guid.NewGuid().ToString()),
                new Claim(
                    JwtRegisteredClaimNames.Iat,
                    new DateTimeOffset(now).ToUnixTimeSeconds().ToString(CultureInfo.InvariantCulture),
                    ClaimValueTypes.Integer64),
            ]),
            SigningCredentials = new SigningCredentials(options.SigningKey(), SecurityAlgorithms.HmacSha256),
        };

        return _handler.CreateToken(descriptor);
    }
}
