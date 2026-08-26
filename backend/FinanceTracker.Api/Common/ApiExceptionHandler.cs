using FinanceTracker.Application.Common;
using Microsoft.AspNetCore.Diagnostics;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Common;

/// <summary>
/// The single place where an application <see cref="ErrorKind"/> becomes an HTTP status code,
/// rendered as a ProblemDetails payload.
/// </summary>
public sealed class ApiExceptionHandler(ILogger<ApiExceptionHandler> logger) : IExceptionHandler
{
    private static readonly Dictionary<ErrorKind, (int Status, string Title)> Responses = new()
    {
        [ErrorKind.Validation] = (StatusCodes.Status400BadRequest, "Bad Request"),
        [ErrorKind.Unauthorized] = (StatusCodes.Status401Unauthorized, "Unauthorized"),
        [ErrorKind.NotFound] = (StatusCodes.Status404NotFound, "Not Found"),
        [ErrorKind.Conflict] = (StatusCodes.Status409Conflict, "Conflict"),
    };

    public async ValueTask<bool> TryHandleAsync(
        HttpContext httpContext,
        Exception exception,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(httpContext);

        if (exception is not AppException appException)
        {
            return false;
        }

        var (status, title) = Responses.TryGetValue(appException.Kind, out var mapped)
            ? mapped
            : (StatusCodes.Status400BadRequest, "Error");

        logger.LogInformation(
            "Request {Path} rejected with {StatusCode}: {Reason}",
            httpContext.Request.Path,
            status,
            appException.Message);

        var problem = new ProblemDetails
        {
            Status = status,
            Title = title,
            Detail = appException.Message,
            Instance = httpContext.Request.Path,
        };

        httpContext.Response.StatusCode = status;
        await httpContext.Response
            .WriteAsJsonAsync(problem, options: null, contentType: "application/problem+json", cancellationToken)
            .ConfigureAwait(false);

        return true;
    }
}
