using FinanceTracker.Application.Common;
using FinanceTracker.Application.Dtos;
using FinanceTracker.Tests.Infrastructure;

namespace FinanceTracker.Tests.Services;

public sealed class GoalServiceTests : IDisposable
{
    private readonly TestHarness _harness = new();

    [Fact]
    public async Task CreateUpdateContributeAndDelete()
    {
        var userId = await _harness.CreateUserAsync();

        var created = await _harness.Goals.CreateAsync(
            userId,
            new GoalRequest
            {
                Name = " Emergency fund ",
                TargetAmount = 5_000m,
                CurrentAmount = 250m,
                TargetDate = TestHarness.Utc(2027, 1, 1),
                Color = "#00ff00",
            },
            CancellationToken.None);

        Assert.Equal("Emergency fund", created.Name);
        Assert.Equal(250m, created.CurrentAmount);

        var contributed = await _harness.Goals.ContributeAsync(
            userId,
            created.Id,
            new ContributeRequest { Amount = 125.50m },
            CancellationToken.None);
        Assert.Equal(375.50m, contributed.CurrentAmount);

        var updated = await _harness.Goals.UpdateAsync(
            userId,
            created.Id,
            new GoalRequest { Name = "Rainy day", TargetAmount = 6_000m, CurrentAmount = 375.50m, Color = "#0000ff" },
            CancellationToken.None);
        Assert.Equal("Rainy day", updated.Name);
        Assert.Equal(6_000m, updated.TargetAmount);
        Assert.Null(updated.TargetDate);

        await _harness.Goals.DeleteAsync(userId, created.Id, CancellationToken.None);
        Assert.Empty(await _harness.Goals.ListAsync(userId, CancellationToken.None));
    }

    [Fact]
    public async Task ContributionMustBePositive()
    {
        var userId = await _harness.CreateUserAsync();
        var goal = await _harness.Goals.CreateAsync(
            userId,
            new GoalRequest { Name = "Car", TargetAmount = 100m, Color = "#123456" },
            CancellationToken.None);

        var error = await Assert.ThrowsAsync<AppException>(() => _harness.Goals.ContributeAsync(
            userId,
            goal.Id,
            new ContributeRequest { Amount = 0m },
            CancellationToken.None));

        Assert.Equal(ErrorKind.Validation, error.Kind);
    }

    [Fact]
    public async Task GoalsAreScopedToTheOwner()
    {
        var userId = await _harness.CreateUserAsync();
        var stranger = await _harness.CreateUserAsync("stranger@example.com");
        var goal = await _harness.Goals.CreateAsync(
            userId,
            new GoalRequest { Name = "Trip", TargetAmount = 900m, Color = "#abcdef" },
            CancellationToken.None);

        Assert.Empty(await _harness.Goals.ListAsync(stranger, CancellationToken.None));

        var error = await Assert.ThrowsAsync<AppException>(
            () => _harness.Goals.DeleteAsync(stranger, goal.Id, CancellationToken.None));

        Assert.Equal(ErrorKind.NotFound, error.Kind);
    }

    public void Dispose() => _harness.Dispose();
}
