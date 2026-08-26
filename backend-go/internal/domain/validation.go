package domain

import (
	"sort"
	"strings"
)

// The constants below are the ones ASP.NET Core's automatic model validation
// puts in the body of a 400. They were captured verbatim from the running .NET
// API on 2026-08-26 (POST /api/auth/register with an invalid payload):
//
//	{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1",
//	 "title":"One or more validation errors occurred.",
//	 "status":400,
//	 "errors":{"Email":["The Email field is not a valid e-mail address."]},
//	 "traceId":"00-...-00"}
const (
	// ValidationProblemTitle is the title of every validation 400. The frontend's
	// readError() falls back to `title`, so this string is what a user sees.
	ValidationProblemTitle = "One or more validation errors occurred."

	// ValidationProblemType is the `type` member MVC writes on those 400s.
	ValidationProblemType = "https://tools.ietf.org/html/rfc9110#section-15.5.1"

	// JSONBodyField is the key MVC uses for errors raised by the JSON reader
	// rather than by a property rule.
	JSONBodyField = "$"
)

// ValidationError collects per-field failures. Keys are the PascalCase names of
// the .NET request DTO properties ("Email", "Password", ...) because those are
// the ModelState keys the .NET API emits: no JSON naming policy is applied to
// validation metadata there, only to serialisation.
//
// It is deliberately not an AppError: the HTTP layer renders it with a body
// shape of its own (a `errors` dictionary instead of a `detail` string).
type ValidationError struct {
	// Errors maps a field name to every message that failed for it, in the
	// order the rules ran.
	Errors map[string][]string
}

// NewValidationError returns an empty collector.
func NewValidationError() *ValidationError {
	return &ValidationError{Errors: make(map[string][]string, 4)}
}

// Add appends one message for field.
func (e *ValidationError) Add(field, message string) {
	if e.Errors == nil {
		e.Errors = make(map[string][]string, 4)
	}
	e.Errors[field] = append(e.Errors[field], message)
}

// Empty reports whether nothing failed.
func (e *ValidationError) Empty() bool { return e == nil || len(e.Errors) == 0 }

// OrNil returns e as an error, or a nil error when nothing failed. Callers use
// it as the last statement of a Validate method so they never hand back a
// non-nil interface wrapping an empty collector.
func (e *ValidationError) OrNil() error {
	if e.Empty() {
		return nil
	}
	return e
}

// Error renders the failures for logs. The wire format is produced by the HTTP
// layer, not here.
func (e *ValidationError) Error() string {
	if e.Empty() {
		return ValidationProblemTitle
	}

	fields := make([]string, 0, len(e.Errors))
	for field := range e.Errors {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	var b strings.Builder
	b.WriteString(ValidationProblemTitle)
	for _, field := range fields {
		b.WriteString(" ")
		b.WriteString(field)
		b.WriteString(": ")
		b.WriteString(strings.Join(e.Errors[field], " "))
	}
	return b.String()
}
