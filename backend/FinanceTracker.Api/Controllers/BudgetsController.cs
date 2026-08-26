using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class BudgetsController(BudgetService budgets) : ApiControllerBase
{
    [HttpGet]
    public async Task<ActionResult<IReadOnlyList<BudgetDto>>> List(
        [FromQuery] string? month,
        CancellationToken cancellationToken) =>
        Ok(await budgets.ListAsync(UserId, month, cancellationToken).ConfigureAwait(false));

    [HttpPost]
    public async Task<ActionResult<BudgetDto>> Create(
        [FromBody] CreateBudgetRequest request,
        CancellationToken cancellationToken) =>
        Ok(await budgets.CreateAsync(UserId, request, cancellationToken).ConfigureAwait(false));

    [HttpPut("{id:guid}")]
    public async Task<ActionResult<BudgetDto>> Update(
        Guid id,
        [FromBody] UpdateBudgetRequest request,
        CancellationToken cancellationToken) =>
        Ok(await budgets.UpdateAsync(UserId, id, request, cancellationToken).ConfigureAwait(false));

    [HttpDelete("{id:guid}")]
    public async Task<IActionResult> Delete(Guid id, CancellationToken cancellationToken)
    {
        await budgets.DeleteAsync(UserId, id, cancellationToken).ConfigureAwait(false);
        return NoContent();
    }
}
