using FinanceTracker.Application.Abstractions;
using FinanceTracker.Application.Common;
using FinanceTracker.Application.Dtos;
using FinanceTracker.Domain;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Application.Services;

/// <summary>Registration, sign-in and profile maintenance.</summary>
public sealed class AuthService(IAppDbContext db, ITokenService tokens, IPasswordHasher hasher)
{
    public async Task<AuthResponse> RegisterAsync(RegisterRequest request, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var email = Normalize(request.Email);
        if (await db.Users.AnyAsync(u => u.Email == email, cancellationToken).ConfigureAwait(false))
        {
            throw AppException.Conflict("An account with that email already exists.");
        }

        var user = new User
        {
            Email = email,
            Name = request.Name.Trim(),
            CreatedAt = DateTime.UtcNow,
        };
        user.PasswordHash = hasher.Hash(user, request.Password);

        db.Users.Add(user);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return new AuthResponse(tokens.CreateToken(user), Map(user));
    }

    public async Task<AuthResponse> LoginAsync(LoginRequest request, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var email = Normalize(request.Email);
        var user = await db.Users
            .FirstOrDefaultAsync(u => u.Email == email, cancellationToken)
            .ConfigureAwait(false);

        if (user is null)
        {
            throw AppException.Unauthorized("Invalid email or password.");
        }

        var result = hasher.Verify(user, user.PasswordHash, request.Password);
        if (result == PasswordVerificationOutcome.Failed)
        {
            throw AppException.Unauthorized("Invalid email or password.");
        }

        if (result == PasswordVerificationOutcome.SuccessRehashNeeded)
        {
            user.PasswordHash = hasher.Hash(user, request.Password);
            await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
        }

        return new AuthResponse(tokens.CreateToken(user), Map(user));
    }

    public async Task<UserDto> GetProfileAsync(Guid userId, CancellationToken cancellationToken) =>
        Map(await LoadAsync(userId, cancellationToken).ConfigureAwait(false));

    public async Task<UserDto> UpdateProfileAsync(
        Guid userId,
        UpdateProfileRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var user = await LoadAsync(userId, cancellationToken).ConfigureAwait(false);
        user.Name = request.Name.Trim();
        user.Currency = request.Currency.Trim().ToUpperInvariant();
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(user);
    }

    private async Task<User> LoadAsync(Guid userId, CancellationToken cancellationToken)
    {
        var user = await db.Users
            .FirstOrDefaultAsync(u => u.Id == userId, cancellationToken)
            .ConfigureAwait(false);

        return user ?? throw AppException.NotFound("User");
    }

    private static string Normalize(string email) => email.Trim().ToLowerInvariant();

    private static UserDto Map(User user) => new(user.Id, user.Email, user.Name, user.Currency);
}
