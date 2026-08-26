using System.Globalization;
using System.Text;
using FinanceTracker.Api.Common;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class TransactionsController(TransactionService transactions, TransactionCsvService csv)
    : ApiControllerBase
{
    private const long MaxUploadBytes = 5 * 1024 * 1024;

    [HttpGet]
    public async Task<ActionResult<PagedResult<TransactionDto>>> Search(
        [FromQuery] TransactionQuery query,
        CancellationToken cancellationToken) =>
        Ok(await transactions.SearchAsync(UserId, query, cancellationToken).ConfigureAwait(false));

    [HttpGet("{id:guid}")]
    public async Task<ActionResult<TransactionDto>> Get(Guid id, CancellationToken cancellationToken) =>
        Ok(await transactions.GetAsync(UserId, id, cancellationToken).ConfigureAwait(false));

    [HttpPost]
    public async Task<ActionResult<TransactionDto>> Create(
        [FromBody] TransactionRequest request,
        CancellationToken cancellationToken) =>
        Ok(await transactions.CreateAsync(UserId, request, cancellationToken).ConfigureAwait(false));

    [HttpPut("{id:guid}")]
    public async Task<ActionResult<TransactionDto>> Update(
        Guid id,
        [FromBody] TransactionRequest request,
        CancellationToken cancellationToken) =>
        Ok(await transactions.UpdateAsync(UserId, id, request, cancellationToken).ConfigureAwait(false));

    [HttpDelete("{id:guid}")]
    public async Task<IActionResult> Delete(Guid id, CancellationToken cancellationToken)
    {
        await transactions.DeleteAsync(UserId, id, cancellationToken).ConfigureAwait(false);
        return NoContent();
    }

    [HttpGet("export")]
    [Produces("text/csv")]
    public async Task<IActionResult> Export(
        [FromQuery] DateTime? from,
        [FromQuery] DateTime? to,
        CancellationToken cancellationToken)
    {
        var content = await csv.ExportAsync(UserId, from, to, cancellationToken).ConfigureAwait(false);
        var fileName = string.Create(
            CultureInfo.InvariantCulture,
            $"transactions-{DateTime.UtcNow.ToString(TransactionCsvService.DateFormat, CultureInfo.InvariantCulture)}.csv");

        return File(Encoding.UTF8.GetBytes(content), "text/csv", fileName);
    }

    [HttpPost("import")]
    [Consumes("multipart/form-data")]
    public async Task<ActionResult<ImportResult>> Import(IFormFile file, CancellationToken cancellationToken)
    {
        if (file is null || file.Length == 0)
        {
            throw ApiException.BadRequest("A non-empty CSV file is required.");
        }

        if (file.Length > MaxUploadBytes)
        {
            throw ApiException.BadRequest("The uploaded file is larger than 5 MB.");
        }

        await using var stream = file.OpenReadStream();
        return Ok(await csv.ImportAsync(UserId, stream, cancellationToken).ConfigureAwait(false));
    }
}
