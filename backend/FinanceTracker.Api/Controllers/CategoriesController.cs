using FinanceTracker.Application.Dtos;
using FinanceTracker.Application.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class CategoriesController(CategoryService categories) : ApiControllerBase
{
    [HttpGet]
    public async Task<ActionResult<IReadOnlyList<CategoryDto>>> List(CancellationToken cancellationToken) =>
        Ok(await categories.ListAsync(UserId, cancellationToken).ConfigureAwait(false));

    [HttpPost]
    public async Task<ActionResult<CategoryDto>> Create(
        [FromBody] CategoryRequest request,
        CancellationToken cancellationToken) =>
        Ok(await categories.CreateAsync(UserId, request, cancellationToken).ConfigureAwait(false));

    [HttpPut("{id:guid}")]
    public async Task<ActionResult<CategoryDto>> Update(
        Guid id,
        [FromBody] CategoryRequest request,
        CancellationToken cancellationToken) =>
        Ok(await categories.UpdateAsync(UserId, id, request, cancellationToken).ConfigureAwait(false));

    [HttpDelete("{id:guid}")]
    public async Task<IActionResult> Delete(Guid id, CancellationToken cancellationToken)
    {
        await categories.DeleteAsync(UserId, id, cancellationToken).ConfigureAwait(false);
        return NoContent();
    }
}
