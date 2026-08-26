namespace FinanceTracker.Api.Common;

/// <summary>Domain failure that maps directly onto an HTTP status code.</summary>
public sealed class ApiException : Exception
{
    public ApiException()
        : this(StatusCodes.Status400BadRequest, "Request could not be processed.")
    {
    }

    public ApiException(string message)
        : this(StatusCodes.Status400BadRequest, message)
    {
    }

    public ApiException(string message, Exception innerException)
        : base(message, innerException) => StatusCode = StatusCodes.Status400BadRequest;

    public ApiException(int statusCode, string message)
        : base(message) => StatusCode = statusCode;

    public int StatusCode { get; }

    public static ApiException NotFound(string what) =>
        new(StatusCodes.Status404NotFound, $"{what} was not found.");

    public static ApiException BadRequest(string message) =>
        new(StatusCodes.Status400BadRequest, message);

    public static ApiException Conflict(string message) =>
        new(StatusCodes.Status409Conflict, message);

    public static ApiException Unauthorized(string message) =>
        new(StatusCodes.Status401Unauthorized, message);
}
