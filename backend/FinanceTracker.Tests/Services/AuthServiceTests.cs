using FinanceTracker.Api.Common;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Services;

public sealed class AuthServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task RegisterIssuesTokenAndNormalizesEmail()
    {
        var response = await _harness.Auth.RegisterAsync(
            new RegisterRequest { Email = " Owner@Example.COM ", Password = "correct horse battery", Name = " Ada " },
            CancellationToken.None);

        Assert.False(string.IsNullOrWhiteSpace(response.Token));
        Assert.Equal("owner@example.com", response.User.Email);
        Assert.Equal("Ada", response.User.Name);
        Assert.Equal("USD", response.User.Currency);
    }

    [Fact]
    public async Task RegisterRejectsDuplicateEmailWithConflict()
    {
        await _harness.CreateUserAsync("dup@example.com");

        var error = await Assert.ThrowsAsync<ApiException>(() => _harness.Auth.RegisterAsync(
            new RegisterRequest { Email = "DUP@example.com", Password = "another password", Name = "Copy" },
            CancellationToken.None));

        Assert.Equal(StatusCodes.Status409Conflict, error.StatusCode);
    }

    [Fact]
    public async Task LoginSucceedsWithCorrectCredentials()
    {
        await _harness.CreateUserAsync("login@example.com");

        var response = await _harness.Auth.LoginAsync(
            new LoginRequest { Email = "login@example.com", Password = "correct horse battery" },
            CancellationToken.None);

        Assert.False(string.IsNullOrWhiteSpace(response.Token));
        Assert.Equal("login@example.com", response.User.Email);
    }

    [Theory]
    [InlineData("login@example.com", "wrong password")]
    [InlineData("missing@example.com", "correct horse battery")]
    public async Task LoginRejectsBadCredentialsWithUnauthorized(string email, string password)
    {
        await _harness.CreateUserAsync("login@example.com");

        var error = await Assert.ThrowsAsync<ApiException>(() => _harness.Auth.LoginAsync(
            new LoginRequest { Email = email, Password = password },
            CancellationToken.None));

        Assert.Equal(StatusCodes.Status401Unauthorized, error.StatusCode);
    }

    [Fact]
    public async Task ProfileCanBeReadAndUpdated()
    {
        var userId = await _harness.CreateUserAsync();

        var loaded = await _harness.Auth.GetProfileAsync(userId, CancellationToken.None);
        Assert.Equal("Owner", loaded.Name);

        var updated = await _harness.Auth.UpdateProfileAsync(
            userId,
            new UpdateProfileRequest { Name = " Grace ", Currency = "eur" },
            CancellationToken.None);

        Assert.Equal("Grace", updated.Name);
        Assert.Equal("EUR", updated.Currency);
    }

    [Fact]
    public async Task GetProfileForUnknownUserThrowsNotFound()
    {
        var error = await Assert.ThrowsAsync<ApiException>(
            () => _harness.Auth.GetProfileAsync(Guid.NewGuid(), CancellationToken.None));

        Assert.Equal(StatusCodes.Status404NotFound, error.StatusCode);
    }

    public void Dispose() => _harness.Dispose();
}
