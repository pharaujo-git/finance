package domain_test

import (
	"testing"
	"time"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

// AddMonths clamps onto the last day of the target month rather than spilling
// into the next one, which is where Go's own AddDate differs.
func TestAddMonthsClamps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		from   time.Time
		months int
		want   time.Time
	}{
		{"31 January plus one month", day(2026, time.January, 31), 1, day(2026, time.February, 28)},
		{"31 January in a leap year", day(2028, time.January, 31), 1, day(2028, time.February, 29)},
		{"31 March back one month", day(2026, time.March, 31), -1, day(2026, time.February, 28)},
		{"across the new year", day(2026, time.November, 30), 2, day(2027, time.January, 30)},
		{"back across the new year", day(2026, time.January, 15), -1, day(2025, time.December, 15)},
		{"a full year", day(2026, time.August, 26), 12, day(2027, time.August, 26)},
		{"no move", day(2026, time.August, 26), 0, day(2026, time.August, 26)},
		{"back a full year", day(2026, time.August, 26), -12, day(2025, time.August, 26)},
		{"back thirteen months", day(2026, time.March, 31), -13, day(2025, time.February, 28)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := domain.AddMonths(testCase.from, testCase.months)
			if !got.Equal(testCase.want) {
				t.Errorf("AddMonths(%s, %d) = %s, want %s",
					testCase.from.Format(time.DateOnly), testCase.months,
					got.Format(time.DateOnly), testCase.want.Format(time.DateOnly))
			}
		})
	}
}

func TestAddMonthsKeepsTheClock(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.January, 31, 13, 45, 30, 500, time.UTC)
	got := domain.AddMonths(from, 1)

	if got.Hour() != 13 || got.Minute() != 45 || got.Second() != 30 || got.Nanosecond() != 500 {
		t.Errorf("AddMonths dropped the time of day: %s", got)
	}
}

func TestAddYearsClampsALeapDay(t *testing.T) {
	t.Parallel()

	if got := domain.AddYears(day(2028, time.February, 29), 1); !got.Equal(day(2029, time.February, 28)) {
		t.Errorf("AddYears = %s, want 2029-02-28", got.Format(time.DateOnly))
	}
	if got := domain.AddYears(day(2028, time.February, 29), 4); !got.Equal(day(2032, time.February, 29)) {
		t.Errorf("AddYears = %s, want 2032-02-29", got.Format(time.DateOnly))
	}
}

func TestNormalizeUTC(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("UTC+2", 2*60*60)
	local := time.Date(2026, time.August, 26, 12, 0, 0, 0, zone)

	normalized := domain.NormalizeUTC(local)
	if normalized.Location() != time.UTC || normalized.Hour() != 10 {
		t.Errorf("NormalizeUTC = %s, want 10:00 UTC", normalized)
	}

	if domain.NormalizeUTCPtr(nil) != nil {
		t.Error("NormalizeUTCPtr(nil) is not nil")
	}
	if got := domain.NormalizeUTCPtr(&local); got == nil || got.Hour() != 10 {
		t.Errorf("NormalizeUTCPtr = %v", got)
	}
}
