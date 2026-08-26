using FinanceTracker.Application.Abstractions;
using FinanceTracker.Domain;
using Microsoft.AspNetCore.Identity;

namespace FinanceTracker.Infrastructure.Identity;

/// <summary>
/// Adapts ASP.NET Core Identity's <see cref="PasswordHasher{TUser}"/> to the application's
/// <see cref="IPasswordHasher"/> port, keeping the Identity dependency out of the inner layers.
/// </summary>
public sealed class IdentityPasswordHasher : IPasswordHasher
{
    private readonly PasswordHasher<User> _hasher = new();

    public string Hash(User user, string password) => _hasher.HashPassword(user, password);

    public PasswordVerificationOutcome Verify(User user, string hash, string password) =>
        _hasher.VerifyHashedPassword(user, hash, password) switch
        {
            PasswordVerificationResult.Success => PasswordVerificationOutcome.Success,
            PasswordVerificationResult.SuccessRehashNeeded => PasswordVerificationOutcome.SuccessRehashNeeded,
            _ => PasswordVerificationOutcome.Failed,
        };
}
