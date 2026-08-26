using System.Globalization;

namespace FinanceTracker.Application.Common;

/// <summary>Helpers for the <c>YYYY-MM</c> month keys used by budgets, dashboards and reports.</summary>
public static class MonthKey
{
    public const string Format = "yyyy-MM";

    public static string From(DateTime value) => value.ToString(Format, CultureInfo.InvariantCulture);

    public static bool TryParse(string? value, out DateTime firstDayUtc)
    {
        firstDayUtc = default;
        if (string.IsNullOrWhiteSpace(value)
            || !DateTime.TryParseExact(value, Format, CultureInfo.InvariantCulture, DateTimeStyles.None, out var parsed))
        {
            return false;
        }

        firstDayUtc = FirstDayUtc(parsed.Year, parsed.Month);
        return true;
    }

    /// <summary>Parses a month key or throws a 400.</summary>
    public static DateTime Parse(string? value) =>
        TryParse(value, out var start)
            ? start
            : throw AppException.BadRequest("Month must be in YYYY-MM format.");

    /// <summary>The first day of the month containing <paramref name="value"/>, in UTC.</summary>
    public static DateTime StartOfMonth(DateTime value) => FirstDayUtc(value.Year, value.Month);

    /// <summary>Midnight UTC on the first day of the given month.</summary>
    public static DateTime FirstDayUtc(int year, int month) =>
        new(year, month, 1, 0, 0, 0, DateTimeKind.Utc);

    /// <summary>The <paramref name="count"/> months ending with the month of <paramref name="reference"/>, oldest first.</summary>
    public static IReadOnlyList<DateTime> TrailingMonths(DateTime reference, int count)
    {
        var last = StartOfMonth(reference);
        return Enumerable.Range(0, count).Select(i => last.AddMonths(i - count + 1)).ToList();
    }
}
