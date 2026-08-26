using FinanceTracker.Domain;

namespace FinanceTracker.Application.Abstractions;

/// <summary>Issues the bearer token a signed-in user presents on later requests.</summary>
public interface ITokenService
{
    string CreateToken(User user);
}
