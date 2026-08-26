using FinanceTracker.Domain;

namespace FinanceTracker.Application.Abstractions;

/// <summary>Result of checking a supplied password against a stored hash.</summary>
public enum PasswordVerificationOutcome
{
    Failed,
    Success,

    /// <summary>Correct password, but the stored hash uses outdated parameters and should be replaced.</summary>
    SuccessRehashNeeded,
}

/// <summary>
/// Password hashing, kept behind an interface so the application layer does not depend on
/// ASP.NET Core Identity.
/// </summary>
public interface IPasswordHasher
{
    string Hash(User user, string password);

    PasswordVerificationOutcome Verify(User user, string hash, string password);
}
