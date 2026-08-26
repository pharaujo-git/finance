namespace FinanceTracker.Domain;

/// <summary>Normalizes incoming dates to UTC so both Npgsql and SQLite accept them.</summary>
public static class UtcDate
{
    public static DateTime Normalize(DateTime value) => value.Kind switch
    {
        DateTimeKind.Utc => value,
        DateTimeKind.Local => value.ToUniversalTime(),
        _ => DateTime.SpecifyKind(value, DateTimeKind.Utc),
    };

    public static DateTime? Normalize(DateTime? value) => value.HasValue ? Normalize(value.Value) : null;
}
