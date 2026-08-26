namespace FinanceTracker.Api.Models;

/// <summary>A spending or income category. Defaults are shared and have no owner.</summary>
public sealed class Category
{
    public Guid Id { get; set; } = Guid.NewGuid();

    /// <summary>Null for the globally seeded default categories.</summary>
    public Guid? UserId { get; set; }

    public string Name { get; set; } = string.Empty;

    public CategoryType Type { get; set; }

    public string Icon { get; set; } = string.Empty;

    public string Color { get; set; } = string.Empty;

    public bool IsDefault { get; set; }
}
