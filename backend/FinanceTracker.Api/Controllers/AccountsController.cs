using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class AccountsController(AccountService accounts) : ApiControllerBase
{
    [HttpGet]
    public async Task<ActionResult<IReadOnlyList<AccountDto>>> List(CancellationToken cancellationToken) =>
        Ok(await accounts.ListAsync(UserId, cancellationToken).ConfigureAwait(false));

    [HttpGet("{id:guid}")]
    public async Task<ActionResult<AccountDto>> Get(Guid id, CancellationToken cancellationToken) =>
        Ok(await accounts.GetAsync(UserId, id, cancellationToken).ConfigureAwait(false));

    [HttpPost]
    public async Task<ActionResult<AccountDto>> Create(
        [FromBody] CreateAccountRequest request,
        CancellationToken cancellationToken) =>
        Ok(await accounts.CreateAsync(UserId, request, cancellationToken).ConfigureAwait(false));

    [HttpPut("{id:guid}")]
    public async Task<ActionResult<AccountDto>> Update(
        Guid id,
        [FromBody] UpdateAccountRequest request,
        CancellationToken cancellationToken) =>
        Ok(await accounts.UpdateAsync(UserId, id, request, cancellationToken).ConfigureAwait(false));

    /// <summary>Archives the account; transactions are preserved.</summary>
    [HttpDelete("{id:guid}")]
    public async Task<IActionResult> Archive(Guid id, CancellationToken cancellationToken)
    {
        await accounts.ArchiveAsync(UserId, id, cancellationToken).ConfigureAwait(false);
        return NoContent();
    }
}
