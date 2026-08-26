using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>Categories visible to a user: the shared defaults plus the ones they created.</summary>
public sealed class CategoryService(AppDbContext db)
{
    public static IQueryable<Category> VisibleTo(IQueryable<Category> source, Guid userId)
    {
        ArgumentNullException.ThrowIfNull(source);
        return source.Where(c => c.IsDefault || c.UserId == userId);
    }

    public async Task<IReadOnlyList<CategoryDto>> ListAsync(Guid userId, CancellationToken cancellationToken)
    {
        var categories = await VisibleTo(db.Categories.AsNoTracking(), userId)
            .OrderBy(c => c.Type)
            .ThenBy(c => c.Name)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        return categories.Select(Map).ToList();
    }

    public async Task<CategoryDto> CreateAsync(Guid userId, CategoryRequest request, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var category = new Category
        {
            UserId = userId,
            IsDefault = false,
            Name = request.Name.Trim(),
            Type = request.Type,
            Icon = request.Icon.Trim(),
            Color = request.Color.Trim(),
        };

        db.Categories.Add(category);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(category);
    }

    public async Task<CategoryDto> UpdateAsync(
        Guid userId,
        Guid id,
        CategoryRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var category = await LoadEditableAsync(userId, id, cancellationToken).ConfigureAwait(false);
        category.Name = request.Name.Trim();
        category.Type = request.Type;
        category.Icon = request.Icon.Trim();
        category.Color = request.Color.Trim();
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(category);
    }

    public async Task DeleteAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var category = await LoadEditableAsync(userId, id, cancellationToken).ConfigureAwait(false);

        var orphaned = await db.Transactions
            .Where(t => t.UserId == userId && t.CategoryId == id)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        foreach (var transaction in orphaned)
        {
            transaction.CategoryId = null;
        }

        var budgets = await db.Budgets
            .Where(b => b.UserId == userId && b.CategoryId == id)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        db.Budgets.RemoveRange(budgets);
        db.Categories.Remove(category);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
    }

    /// <summary>Throws when the category does not exist or is not usable by this user.</summary>
    public async Task EnsureUsableAsync(Guid userId, Guid? categoryId, CancellationToken cancellationToken)
    {
        if (categoryId is null)
        {
            return;
        }

        var exists = await VisibleTo(db.Categories.AsNoTracking(), userId)
            .AnyAsync(c => c.Id == categoryId, cancellationToken)
            .ConfigureAwait(false);

        if (!exists)
        {
            throw ApiException.NotFound("Category");
        }
    }

    private async Task<Category> LoadEditableAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var category = await db.Categories
            .FirstOrDefaultAsync(c => c.Id == id && (c.IsDefault || c.UserId == userId), cancellationToken)
            .ConfigureAwait(false);

        if (category is null)
        {
            throw ApiException.NotFound("Category");
        }

        if (category.IsDefault)
        {
            throw ApiException.BadRequest("Default categories cannot be modified.");
        }

        return category;
    }

    private static CategoryDto Map(Category category) =>
        new(category.Id, category.Name, category.Type, category.Icon, category.Color, category.IsDefault);
}
