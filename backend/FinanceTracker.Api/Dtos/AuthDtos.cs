using System.ComponentModel.DataAnnotations;

namespace FinanceTracker.Api.Dtos;

public sealed record UserDto(Guid Id, string Email, string Name, string Currency);

public sealed record AuthResponse(string Token, UserDto User);

public sealed class RegisterRequest
{
    [Required]
    [EmailAddress]
    [MaxLength(256)]
    public string Email { get; init; } = string.Empty;

    [Required]
    [MinLength(8)]
    [MaxLength(128)]
    public string Password { get; init; } = string.Empty;

    [Required]
    [MaxLength(200)]
    public string Name { get; init; } = string.Empty;
}

public sealed class LoginRequest
{
    [Required]
    [MaxLength(256)]
    public string Email { get; init; } = string.Empty;

    [Required]
    [MaxLength(128)]
    public string Password { get; init; } = string.Empty;
}

public sealed class UpdateProfileRequest
{
    [Required]
    [MaxLength(200)]
    public string Name { get; init; } = string.Empty;

    [Required]
    [MaxLength(8)]
    public string Currency { get; init; } = "USD";
}
