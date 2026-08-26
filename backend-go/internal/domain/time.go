package domain

import "time"

// NormalizeUTC is UtcDate.Normalize: a wall clock with no zone is read as UTC,
// and anything else is converted to UTC. Go always carries a location, so the
// "unspecified" case is handled where a value is parsed, not here.
func NormalizeUTC(value time.Time) time.Time { return value.UTC() }

// NormalizeUTCPtr is the nullable overload.
func NormalizeUTCPtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

// AddMonths is DateTime.AddMonths, which clamps rather than overflows: 31
// January plus one month is 28 February, and the following month is 28 March.
// Go's AddDate normalizes instead (31 January plus one month is 3 March), so a
// monthly recurring rule started on a 31st would drift.
func AddMonths(value time.Time, months int) time.Time {
	year, month, day := value.Date()
	total := int(month) - 1 + months
	targetYear := year + floorDiv(total, 12)
	targetMonth := time.Month(floorMod(total, 12) + 1)

	if last := daysInMonth(targetYear, targetMonth); day > last {
		day = last
	}

	hour, minute, second := value.Clock()
	return time.Date(targetYear, targetMonth, day, hour, minute, second, value.Nanosecond(), value.Location())
}

// AddYears is DateTime.AddYears, which clamps 29 February onto 28 February in a
// common year.
func AddYears(value time.Time, years int) time.Time { return AddMonths(value, years*12) }

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func floorDiv(a, b int) int {
	quotient := a / b
	if a%b < 0 {
		quotient--
	}
	return quotient
}

func floorMod(a, b int) int {
	remainder := a % b
	if remainder < 0 {
		remainder += b
	}
	return remainder
}
