using Microsoft.AspNetCore.Diagnostics;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Common;

/// <summary>Renders <see cref="ApiException"/> as a ProblemDetails payload.</summary>
public sealed class ApiExceptionHandler(ILogger<ApiExceptionHandler> logger) : IExceptionHandler
{
    private static readonly Dictionary<int, string> Titles = new()
    {
        [StatusCodes.Status400BadRequest] = "Bad Request",
        [StatusCodes.Status401Unauthorized] = "Unauthorized",
        [StatusCodes.Status403Forbidden] = "Forbidden",
        [StatusCodes.Status404NotFound] = "Not Found",
        [StatusCodes.Status409Conflict] = "Conflict",
    };

    public async ValueTask<bool> TryHandleAsync(
        HttpContext httpContext,
        Exception exception,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(httpContext);

        if (exception is not ApiException apiException)
        {
            return false;
        }

        logger.LogInformation(
            "Request {Path} rejected with {StatusCode}: {Reason}",
            httpContext.Request.Path,
            apiException.StatusCode,
            apiException.Message);

        var problem = new ProblemDetails
        {
            Status = apiException.StatusCode,
            Title = Titles.TryGetValue(apiException.StatusCode, out var title) ? title : "Error",
            Detail = apiException.Message,
            Instance = httpContext.Request.Path,
        };

        httpContext.Response.StatusCode = apiException.StatusCode;
        await httpContext.Response
            .WriteAsJsonAsync(problem, options: null, contentType: "application/problem+json", cancellationToken)
            .ConfigureAwait(false);

        return true;
    }
}
