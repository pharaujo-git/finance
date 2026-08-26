namespace FinanceTracker.Application.Common;

/// <summary>
/// Transport-neutral classification of an application failure. The API layer is the only place
/// that knows how these map onto HTTP status codes.
/// </summary>
public enum ErrorKind
{
    Validation,
    Unauthorized,
    NotFound,
    Conflict,
}

/// <summary>A failure the caller caused, carrying enough context for a transport to render it.</summary>
public sealed class AppException : Exception
{
    public AppException()
        : this(ErrorKind.Validation, "Request could not be processed.")
    {
    }

    public AppException(string message)
        : this(ErrorKind.Validation, message)
    {
    }

    public AppException(string message, Exception innerException)
        : base(message, innerException) => Kind = ErrorKind.Validation;

    public AppException(ErrorKind kind, string message)
        : base(message) => Kind = kind;

    public ErrorKind Kind { get; }

    public static AppException NotFound(string what) =>
        new(ErrorKind.NotFound, $"{what} was not found.");

    public static AppException BadRequest(string message) =>
        new(ErrorKind.Validation, message);

    public static AppException Conflict(string message) =>
        new(ErrorKind.Conflict, message);

    public static AppException Unauthorized(string message) =>
        new(ErrorKind.Unauthorized, message);
}
