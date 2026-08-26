using System.ComponentModel.DataAnnotations;
using FinanceTracker.Domain;

namespace FinanceTracker.Application.Dtos;

public sealed record CategoryDto(
    Guid Id,
    string Name,
    CategoryType Type,
    string Icon,
    string Color,
    bool IsDefault);

public sealed class CategoryRequest
{
    [Required]
    [MaxLength(200)]
    public string Name { get; init; } = string.Empty;

    public required CategoryType Type { get; init; }

    [MaxLength(64)]
    public string Icon { get; init; } = string.Empty;

    [MaxLength(32)]
    public string Color { get; init; } = string.Empty;
}
