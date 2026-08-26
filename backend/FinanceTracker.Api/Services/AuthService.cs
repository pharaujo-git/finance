using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.AspNetCore.Identity;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>Registration, sign-in and profile maintenance.</summary>
public sealed class AuthService(AppDbContext db, ITokenService tokens, IPasswordHasher<User> hasher)
{
    public async Task<AuthResponse> RegisterAsync(RegisterRequest request, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var email = Normalize(request.Email);
        if (await db.Users.AnyAsync(u => u.Email == email, cancellationToken).ConfigureAwait(false))
        {
            throw ApiException.Conflict("An account with that email already exists.");
        }

        var user = new User
        {
            Email = email,
            Name = request.Name.Trim(),
            CreatedAt = DateTime.UtcNow,
        };
        user.PasswordHash = hasher.HashPassword(user, request.Password);

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
            throw ApiException.Unauthorized("Invalid email or password.");
        }

        var result = hasher.VerifyHashedPassword(user, user.PasswordHash, request.Password);
        if (result == PasswordVerificationResult.Failed)
        {
            throw ApiException.Unauthorized("Invalid email or password.");
        }

        if (result == PasswordVerificationResult.SuccessRehashNeeded)
        {
            user.PasswordHash = hasher.HashPassword(user, request.Password);
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

        return user ?? throw ApiException.NotFound("User");
    }

    private static string Normalize(string email) => email.Trim().ToLowerInvariant();

    private static UserDto Map(User user) => new(user.Id, user.Email, user.Name, user.Currency);
}
