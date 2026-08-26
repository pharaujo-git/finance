using System.Security.Claims;
using FinanceTracker.Application.Common;
using Microsoft.IdentityModel.JsonWebTokens;

namespace FinanceTracker.Api.Common;

/// <summary>Reads the authenticated user id from the bearer token claims.</summary>
public static class ClaimsPrincipalExtensions
{
    public static Guid GetUserId(this ClaimsPrincipal principal)
    {
        ArgumentNullException.ThrowIfNull(principal);

        var raw = principal.FindFirstValue(JwtRegisteredClaimNames.Sub)
                  ?? principal.FindFirstValue(ClaimTypes.NameIdentifier);

        return Guid.TryParse(raw, out var userId)
            ? userId
            : throw AppException.Unauthorized("The access token does not identify a user.");
    }
}
