package application

import (
	"time"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// MonthFormat is the layout of the YYYY-MM keys budgets, dashboards and reports
// use. It is the Go spelling of MonthKey.Format ("yyyy-MM").
const MonthFormat = "2006-01"

// monthFormatMessage is the 400 MonthKey.Parse raises, byte for byte.
const monthFormatMessage = "Month must be in YYYY-MM format."

// MonthFrom renders the month key of a timestamp.
func MonthFrom(value time.Time) string { return value.UTC().Format(MonthFormat) }

// TryParseMonth returns midnight UTC on the first day of the named month.
func TryParseMonth(value string) (time.Time, bool) {
	parsed, err := time.ParseInLocation(MonthFormat, value, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return FirstDayUTC(parsed.Year(), parsed.Month()), true
}

// ParseMonth is TryParseMonth or a 400.
func ParseMonth(value string) (time.Time, error) {
	start, ok := TryParseMonth(value)
	if !ok {
		return time.Time{}, domain.BadRequest(monthFormatMessage)
	}
	return start, nil
}

// StartOfMonth is the first day of the month containing value, in UTC.
func StartOfMonth(value time.Time) time.Time {
	utc := value.UTC()
	return FirstDayUTC(utc.Year(), utc.Month())
}

// FirstDayUTC is midnight UTC on the first day of the given month.
func FirstDayUTC(year int, month time.Month) time.Time {
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
}

// TrailingMonths returns the count months ending with the month of reference,
// oldest first.
func TrailingMonths(reference time.Time, count int) []time.Time {
	last := StartOfMonth(reference)
	months := make([]time.Time, 0, count)
	for i := range count {
		months = append(months, domain.AddMonths(last, i-count+1))
	}
	return months
}
