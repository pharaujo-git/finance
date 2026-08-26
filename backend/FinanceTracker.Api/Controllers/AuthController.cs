using FinanceTracker.Application.Dtos;
using FinanceTracker.Application.Services;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

[Route("api/auth")]
public sealed class AuthController(AuthService auth) : ApiControllerBase
{
    [HttpPost("register")]
    [AllowAnonymous]
    [ProducesResponseType(typeof(AuthResponse), StatusCodes.Status200OK)]
    public async Task<ActionResult<AuthResponse>> Register(
        [FromBody] RegisterRequest request,
        CancellationToken cancellationToken) =>
        Ok(await auth.RegisterAsync(request, cancellationToken).ConfigureAwait(false));

    [HttpPost("login")]
    [AllowAnonymous]
    [ProducesResponseType(typeof(AuthResponse), StatusCodes.Status200OK)]
    public async Task<ActionResult<AuthResponse>> Login(
        [FromBody] LoginRequest request,
        CancellationToken cancellationToken) =>
        Ok(await auth.LoginAsync(request, cancellationToken).ConfigureAwait(false));

    [HttpGet("me")]
    public async Task<ActionResult<UserDto>> Me(CancellationToken cancellationToken) =>
        Ok(await auth.GetProfileAsync(UserId, cancellationToken).ConfigureAwait(false));

    [HttpPut("me")]
    public async Task<ActionResult<UserDto>> UpdateMe(
        [FromBody] UpdateProfileRequest request,
        CancellationToken cancellationToken) =>
        Ok(await auth.UpdateProfileAsync(UserId, request, cancellationToken).ConfigureAwait(false));
}
