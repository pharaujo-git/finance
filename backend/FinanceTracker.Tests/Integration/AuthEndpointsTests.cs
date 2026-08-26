using System.Net;
using System.Net.Http.Json;
using FinanceTracker.Application.Dtos;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Integration;

public sealed class AuthEndpointsTests : IClassFixture<FinanceApiFactory>
{
    private readonly FinanceApiFactory _factory;

    public AuthEndpointsTests(FinanceApiFactory factory) => _factory = factory;

    [Fact]
    public async Task HealthIsAnonymousAndReturnsOk()
    {
        var response = await _factory.CreateClient().GetAsync(new Uri("/health", UriKind.Relative));

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("ok", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task RootIsAnonymousAndPointsAtTheDocs()
    {
        var response = await _factory.CreateClient().GetAsync(new Uri("/", UriKind.Relative));

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        var body = await response.Content.ReadAsStringAsync();
        Assert.Contains("FinanceTracker API", body, StringComparison.Ordinal);
        Assert.Contains("/swagger", body, StringComparison.Ordinal);
    }

    [Fact]
    public async Task RegisterThenLoginReturnsATokenAndProfile()
    {
        var client = _factory.CreateClient();
        var email = $"flow-{Guid.NewGuid():N}@example.com";

        var register = await client.PostAsJsonAsync(
            "/api/auth/register",
            new RegisterRequest { Email = email, Password = "correct horse battery", Name = "Flow" },
            FinanceApiFactory.Json);

        Assert.Equal(HttpStatusCode.OK, register.StatusCode);

        var login = await client.PostAsJsonAsync(
            "/api/auth/login",
            new LoginRequest { Email = email, Password = "correct horse battery" },
            FinanceApiFactory.Json);

        Assert.Equal(HttpStatusCode.OK, login.StatusCode);
        var payload = await login.Content.ReadFromJsonAsync<AuthResponse>(FinanceApiFactory.Json);
        Assert.NotNull(payload);
        Assert.False(string.IsNullOrWhiteSpace(payload.Token));
        Assert.Equal(email, payload.User.Email);
    }

    [Fact]
    public async Task DuplicateRegistrationReturnsConflictProblemDetails()
    {
        var client = _factory.CreateClient();
        var request = new RegisterRequest
        {
            Email = $"dupe-{Guid.NewGuid():N}@example.com",
            Password = "correct horse battery",
            Name = "Dupe",
        };

        await client.PostAsJsonAsync("/api/auth/register", request, FinanceApiFactory.Json);
        var second = await client.PostAsJsonAsync("/api/auth/register", request, FinanceApiFactory.Json);

        Assert.Equal(HttpStatusCode.Conflict, second.StatusCode);
        Assert.Contains("problem+json", second.Content.Headers.ContentType?.MediaType ?? string.Empty, StringComparison.Ordinal);
    }

    [Fact]
    public async Task BadCredentialsReturnUnauthorized()
    {
        var client = _factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            "/api/auth/login",
            new LoginRequest { Email = "nobody@example.com", Password = "whatever it is" },
            FinanceApiFactory.Json);

        Assert.Equal(HttpStatusCode.Unauthorized, response.StatusCode);
    }

    [Fact]
    public async Task InvalidRegistrationPayloadReturnsValidationProblem()
    {
        var client = _factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            "/api/auth/register",
            new RegisterRequest { Email = "not-an-email", Password = "short", Name = string.Empty },
            FinanceApiFactory.Json);

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }

    [Fact]
    public async Task ProtectedEndpointsRejectAnonymousCallers()
    {
        var response = await _factory.CreateClient().GetAsync(new Uri("/api/accounts", UriKind.Relative));

        Assert.Equal(HttpStatusCode.Unauthorized, response.StatusCode);
    }

    [Fact]
    public async Task ProfileCanBeFetchedAndUpdatedWithABearerToken()
    {
        var (client, user) = await _factory.CreateAuthenticatedClientAsync();

        var me = await client.GetFromJsonAsync<UserDto>("/api/auth/me", FinanceApiFactory.Json);
        Assert.NotNull(me);
        Assert.Equal(user.Id, me.Id);

        var update = await client.PutAsJsonAsync(
            "/api/auth/me",
            new UpdateProfileRequest { Name = "Renamed", Currency = "EUR" },
            FinanceApiFactory.Json);

        var updated = await update.Content.ReadFromJsonAsync<UserDto>(FinanceApiFactory.Json);
        Assert.NotNull(updated);
        Assert.Equal("Renamed", updated.Name);
        Assert.Equal("EUR", updated.Currency);
    }
}
