using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Services;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

public sealed class ReportsController(AnalyticsService analytics) : ApiControllerBase
{
    [HttpGet("monthly")]
    public async Task<ActionResult<IReadOnlyList<MonthlyReportDto>>> Monthly(
        [FromQuery] int? year,
        CancellationToken cancellationToken) =>
        Ok(await analytics
            .GetMonthlyReportAsync(UserId, year ?? DateTime.UtcNow.Year, cancellationToken)
            .ConfigureAwait(false));

    [HttpGet("categories")]
    public async Task<ActionResult<IReadOnlyList<CategoryReportDto>>> Categories(
        [FromQuery] DateTime? from,
        [FromQuery] DateTime? to,
        CancellationToken cancellationToken) =>
        Ok(await analytics.GetCategoryReportAsync(UserId, from, to, cancellationToken).ConfigureAwait(false));
}
