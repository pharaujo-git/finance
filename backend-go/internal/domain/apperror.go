package domain

import "fmt"

// ErrorKind is the transport-neutral classification of a caller-caused failure.
// It is the Go twin of FinanceTracker.Application.Common.ErrorKind; the HTTP
// layer is the only place that turns a kind into a status code.
type ErrorKind int

const (
	// KindValidation renders as 400 Bad Request.
	KindValidation ErrorKind = iota
	// KindUnauthorized renders as 401 Unauthorized.
	KindUnauthorized
	// KindNotFound renders as 404 Not Found.
	KindNotFound
	// KindConflict renders as 409 Conflict.
	KindConflict
)

// String names the kind for logs; it is not part of any wire format.
func (k ErrorKind) String() string {
	switch k {
	case KindValidation:
		return "Validation"
	case KindUnauthorized:
		return "Unauthorized"
	case KindNotFound:
		return "NotFound"
	case KindConflict:
		return "Conflict"
	default:
		return "Unknown"
	}
}

// AppError is a failure the caller caused, carrying enough context for a
// transport to render it. It is the Go twin of AppException.
type AppError struct {
	Kind    ErrorKind
	Message string
}

func (e *AppError) Error() string { return e.Message }

// NewAppError builds an error of an explicit kind.
func NewAppError(kind ErrorKind, message string) *AppError {
	return &AppError{Kind: kind, Message: message}
}

// NotFound matches AppException.NotFound, including the trailing period.
func NotFound(what string) *AppError {
	return NewAppError(KindNotFound, fmt.Sprintf("%s was not found.", what))
}

// BadRequest matches AppException.BadRequest.
func BadRequest(message string) *AppError {
	return NewAppError(KindValidation, message)
}

// Conflict matches AppException.Conflict.
func Conflict(message string) *AppError {
	return NewAppError(KindConflict, message)
}

// Unauthorized matches AppException.Unauthorized.
func Unauthorized(message string) *AppError {
	return NewAppError(KindUnauthorized, message)
}
