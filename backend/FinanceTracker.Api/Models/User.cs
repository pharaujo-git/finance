namespace FinanceTracker.Api.Models;

/// <summary>An application user. Owns every other row in the database.</summary>
public sealed class User
{
    public Guid Id { get; set; } = Guid.NewGuid();

    public string Email { get; set; } = string.Empty;

    public string Name { get; set; } = string.Empty;

    public string PasswordHash { get; set; } = string.Empty;

    public string Currency { get; set; } = "USD";

    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
}
