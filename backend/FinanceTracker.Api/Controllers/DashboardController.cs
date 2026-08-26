using FinanceTracker.Application.Dtos;
using FinanceTracker.Application.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class DashboardController(AnalyticsService analytics) : ApiControllerBase
{
    [HttpGet("summary")]
    public async Task<ActionResult<DashboardSummaryDto>> Summary(CancellationToken cancellationToken) =>
        Ok(await analytics.GetSummaryAsync(UserId, cancellationToken).ConfigureAwait(false));

    [HttpGet("networth")]
    public async Task<ActionResult<IReadOnlyList<NetWorthPointDto>>> NetWorth(
        [FromQuery] int months = 12,
        CancellationToken cancellationToken = default) =>
        Ok(await analytics.GetNetWorthAsync(UserId, months, cancellationToken).ConfigureAwait(false));

    [HttpGet("cashflow")]
    public async Task<ActionResult<IReadOnlyList<CashflowPointDto>>> Cashflow(
        [FromQuery] int months = 6,
        CancellationToken cancellationToken = default) =>
        Ok(await analytics.GetCashflowAsync(UserId, months, cancellationToken).ConfigureAwait(false));

    [HttpGet("spending")]
    public async Task<ActionResult<IReadOnlyList<SpendingSliceDto>>> Spending(
        [FromQuery] string? month,
        CancellationToken cancellationToken) =>
        Ok(await analytics.GetSpendingAsync(UserId, month, cancellationToken).ConfigureAwait(false));
}
