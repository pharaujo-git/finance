using FinanceTracker.Application.Dtos;
using FinanceTracker.Application.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class RecurringController(RecurringService recurring) : ApiControllerBase
{
    [HttpGet]
    public async Task<ActionResult<IReadOnlyList<RecurringRuleDto>>> List(CancellationToken cancellationToken) =>
        Ok(await recurring.ListAsync(UserId, cancellationToken).ConfigureAwait(false));

    [HttpPost]
    public async Task<ActionResult<RecurringRuleDto>> Create(
        [FromBody] RecurringRuleRequest request,
        CancellationToken cancellationToken) =>
        Ok(await recurring.CreateAsync(UserId, request, cancellationToken).ConfigureAwait(false));

    [HttpPut("{id:guid}")]
    public async Task<ActionResult<RecurringRuleDto>> Update(
        Guid id,
        [FromBody] RecurringRuleRequest request,
        CancellationToken cancellationToken) =>
        Ok(await recurring.UpdateAsync(UserId, id, request, cancellationToken).ConfigureAwait(false));

    [HttpDelete("{id:guid}")]
    public async Task<IActionResult> Delete(Guid id, CancellationToken cancellationToken)
    {
        await recurring.DeleteAsync(UserId, id, cancellationToken).ConfigureAwait(false);
        return NoContent();
    }
}
