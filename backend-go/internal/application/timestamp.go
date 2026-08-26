package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// dateLayouts are the ISO 8601 forms System.Text.Json accepts for a DateTime,
// most specific first. The frontend posts bare calendar dates ("2024-05-17")
// from its <input type="date"> fields, which is why the shortest form matters.
var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

// Timestamp is the request-DTO field type for every date the API accepts. It
// carries UtcDate.Normalize's rule that a value with no zone is a UTC wall
// clock, which Go's own time.Time JSON decoding cannot express: that one
// insists on a full RFC 3339 string with an offset.
type Timestamp struct {
	Time time.Time
}

// NewTimestamp wraps a time, normalised to UTC.
func NewTimestamp(value time.Time) Timestamp { return Timestamp{Time: value.UTC()} }

// MarshalJSON writes the value the way the .NET API writes a UTC DateTime.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.UTC())
}

// UnmarshalJSON accepts any of dateLayouts, or null for an absent value.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if string(trimmed) == "null" {
		return nil
	}

	var raw string
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return fmt.Errorf("the JSON value could not be converted to a date")
	}

	parsed, ok := ParseWireDate(raw)
	if !ok {
		return fmt.Errorf("the JSON value %q could not be converted to a date", raw)
	}
	t.Time = parsed
	return nil
}

// ParseWireDate reads one of dateLayouts, treating a value with no zone as a
// UTC wall clock. The query-string binder uses it too, so a date reaches the
// services the same way whether it arrived in a body or in a URL.
func ParseWireDate(value string) (time.Time, bool) {
	for _, layout := range dateLayouts {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// TimePtr unwraps an optional timestamp.
func TimePtr(value *Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.Time.UTC()
	return &utc
}
