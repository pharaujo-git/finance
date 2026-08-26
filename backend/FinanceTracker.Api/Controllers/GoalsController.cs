using FinanceTracker.Application.Dtos;
using FinanceTracker.Application.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class GoalsController(GoalService goals) : ApiControllerBase
{
    [HttpGet]
    public async Task<ActionResult<IReadOnlyList<GoalDto>>> List(CancellationToken cancellationToken) =>
        Ok(await goals.ListAsync(UserId, cancellationToken).ConfigureAwait(false));

    [HttpPost]
    public async Task<ActionResult<GoalDto>> Create(
        [FromBody] GoalRequest request,
        CancellationToken cancellationToken) =>
        Ok(await goals.CreateAsync(UserId, request, cancellationToken).ConfigureAwait(false));

    [HttpPut("{id:guid}")]
    public async Task<ActionResult<GoalDto>> Update(
        Guid id,
        [FromBody] GoalRequest request,
        CancellationToken cancellationToken) =>
        Ok(await goals.UpdateAsync(UserId, id, request, cancellationToken).ConfigureAwait(false));

    [HttpDelete("{id:guid}")]
    public async Task<IActionResult> Delete(Guid id, CancellationToken cancellationToken)
    {
        await goals.DeleteAsync(UserId, id, cancellationToken).ConfigureAwait(false);
        return NoContent();
    }

    [HttpPost("{id:guid}/contribute")]
    public async Task<ActionResult<GoalDto>> Contribute(
        Guid id,
        [FromBody] ContributeRequest request,
        CancellationToken cancellationToken) =>
        Ok(await goals.ContributeAsync(UserId, id, request, cancellationToken).ConfigureAwait(false));
}
