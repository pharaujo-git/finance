using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>Savings goals and contributions.</summary>
public sealed class GoalService(AppDbContext db)
{
    private const string EntityName = "Goal";

    public async Task<IReadOnlyList<GoalDto>> ListAsync(Guid userId, CancellationToken cancellationToken)
    {
        var goals = await db.Goals
            .AsNoTracking()
            .OwnedBy(userId)
            .OrderBy(g => g.Name)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        return goals.Select(Map).ToList();
    }

    public async Task<GoalDto> CreateAsync(Guid userId, GoalRequest request, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var goal = new Goal { UserId = userId };
        Apply(goal, request);

        db.Goals.Add(goal);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(goal);
    }

    public async Task<GoalDto> UpdateAsync(
        Guid userId,
        Guid id,
        GoalRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var goal = await db.Goals.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        Apply(goal, request);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(goal);
    }

    public async Task DeleteAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var goal = await db.Goals.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        db.Goals.Remove(goal);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
    }

    public async Task<GoalDto> ContributeAsync(
        Guid userId,
        Guid id,
        ContributeRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        if (request.Amount <= 0m)
        {
            throw ApiException.BadRequest("Contribution amount must be greater than zero.");
        }

        var goal = await db.Goals.GetOwnedAsync(id, userId, EntityName, cancellationToken).ConfigureAwait(false);
        goal.CurrentAmount = decimal.Round(goal.CurrentAmount + request.Amount, 2, MidpointRounding.AwayFromZero);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(goal);
    }

    private static void Apply(Goal goal, GoalRequest request)
    {
        goal.Name = request.Name.Trim();
        goal.TargetAmount = decimal.Round(request.TargetAmount, 2, MidpointRounding.AwayFromZero);
        goal.CurrentAmount = decimal.Round(request.CurrentAmount, 2, MidpointRounding.AwayFromZero);
        goal.TargetDate = UtcDate.Normalize(request.TargetDate);
        goal.Color = request.Color.Trim();
    }

    private static GoalDto Map(Goal goal) =>
        new(goal.Id, goal.Name, goal.TargetAmount, goal.CurrentAmount, goal.TargetDate, goal.Color);
}
