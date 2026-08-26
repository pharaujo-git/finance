namespace FinanceTracker.Api.Common;

/// <summary>Reads the comma-separated <c>ALLOWED_ORIGINS</c> variable.</summary>
public static class CorsOrigins
{
    public const string Variable = "ALLOWED_ORIGINS";

    public const string Default = "http://localhost:5173";

    public static string[] Read(IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);

        var raw = configuration[Variable];
        var value = string.IsNullOrWhiteSpace(raw) ? Default : raw;
        return value.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
    }
}
