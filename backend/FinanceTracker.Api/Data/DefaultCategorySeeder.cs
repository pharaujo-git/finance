using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Data;

/// <summary>Inserts the globally shared default categories exactly once.</summary>
public static class DefaultCategorySeeder
{
    private static readonly (string Name, CategoryType Type, string Icon, string Color)[] Defaults =
    [
        ("Salary", CategoryType.Income, "wallet", "#16a34a"),
        ("Freelance", CategoryType.Income, "briefcase", "#22c55e"),
        ("Investments", CategoryType.Income, "trending-up", "#0ea5e9"),
        ("Gifts", CategoryType.Income, "gift", "#a855f7"),
        ("Other Income", CategoryType.Income, "plus-circle", "#64748b"),
        ("Groceries", CategoryType.Expense, "shopping-cart", "#f97316"),
        ("Rent", CategoryType.Expense, "home", "#ef4444"),
        ("Utilities", CategoryType.Expense, "zap", "#eab308"),
        ("Transport", CategoryType.Expense, "car", "#3b82f6"),
        ("Dining", CategoryType.Expense, "utensils", "#f43f5e"),
        ("Entertainment", CategoryType.Expense, "film", "#8b5cf6"),
        ("Health", CategoryType.Expense, "heart-pulse", "#14b8a6"),
        ("Shopping", CategoryType.Expense, "shopping-bag", "#ec4899"),
        ("Education", CategoryType.Expense, "graduation-cap", "#6366f1"),
        ("Travel", CategoryType.Expense, "plane", "#06b6d4"),
        ("Insurance", CategoryType.Expense, "shield", "#0f766e"),
        ("Subscriptions", CategoryType.Expense, "repeat", "#7c3aed"),
        ("Other Expense", CategoryType.Expense, "minus-circle", "#64748b"),
    ];

    public static async Task SeedAsync(AppDbContext db, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(db);

        var existing = await db.Categories
            .Where(c => c.IsDefault)
            .Select(c => c.Name)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        var missing = Defaults
            .Where(d => !existing.Contains(d.Name))
            .Select(d => new Category
            {
                Name = d.Name,
                Type = d.Type,
                Icon = d.Icon,
                Color = d.Color,
                IsDefault = true,
                UserId = null,
            })
            .ToList();

        if (missing.Count == 0)
        {
            return;
        }

        db.Categories.AddRange(missing);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
    }
}
