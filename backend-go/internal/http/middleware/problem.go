// Package middleware holds the gin middleware and the problem+json renderer
// shared by every handler.
package middleware

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// ProblemContentType is the media type RFC 9457 defines, and the one the .NET
// API writes.
const ProblemContentType = "application/problem+json"

// Problem is the payload ASP.NET Core's ProblemDetails serialises to. Field
// order and names were captured from the running .NET API:
//
//	{"title":"Conflict","status":409,"detail":"...","instance":"/api/auth/register"}
//
// `type` is absent because the .NET handler never sets it. The frontend's
// readError() prefers `message` then `title`; neither backend emits `message`,
// so the human-readable text lands in `detail` with `title` holding the status
// phrase, and that asymmetry is reproduced here deliberately.
type Problem struct {
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
}

// statusTitles mirrors ApiExceptionHandler.Responses.
var statusTitles = map[domain.ErrorKind]struct {
	Status int
	Title  string
}{
	domain.KindValidation:   {http.StatusBadRequest, "Bad Request"},
	domain.KindUnauthorized: {http.StatusUnauthorized, "Unauthorized"},
	domain.KindNotFound:     {http.StatusNotFound, "Not Found"},
	domain.KindConflict:     {http.StatusConflict, "Conflict"},
}

// WriteProblem renders a problem document and stops the handler chain.
func WriteProblem(c *gin.Context, status int, title, detail string) {
	body, err := json.Marshal(Problem{
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: c.Request.URL.Path,
	})
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Abort()
	c.Data(status, ProblemContentType, body)
}

// ValidationProblem is the body ASP.NET Core's automatic model validation
// writes for a 400, captured verbatim from the running .NET API on 2026-08-26:
//
//	{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1",
//	 "title":"One or more validation errors occurred.","status":400,
//	 "errors":{"Email":["The Email field is required."]},"traceId":"00-...-00"}
//
// `traceId` is omitted here (the Go API has no Activity to name) and so is
// `instance`, which MVC does not set on these — unlike the exception-handler
// problems above, which do carry it.
type ValidationProblem struct {
	Type   string              `json:"type"`
	Title  string              `json:"title"`
	Status int                 `json:"status"`
	Errors map[string][]string `json:"errors"`
}

// ValidationContentType is what MVC content-negotiates for validation 400s:
// plain JSON, not problem+json. Reproduced so a client that switches on the
// media type sees no difference between the backends.
const ValidationContentType = "application/json; charset=utf-8"

// WriteValidationProblem renders the field-error dictionary and stops the
// handler chain.
func WriteValidationProblem(c *gin.Context, fieldErrors map[string][]string) {
	body, err := json.Marshal(ValidationProblem{
		Type:   domain.ValidationProblemType,
		Title:  domain.ValidationProblemTitle,
		Status: http.StatusBadRequest,
		Errors: fieldErrors,
	})
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Abort()
	c.Data(http.StatusBadRequest, ValidationContentType, body)
}

// WriteAppError maps a domain error onto its status and title. An unmapped
// kind falls back to 400 / "Error", matching the .NET handler's default.
func WriteAppError(c *gin.Context, err error) {
	var validationErr *domain.ValidationError
	if errors.As(err, &validationErr) {
		WriteValidationProblem(c, validationErr.Errors)
		return
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		WriteProblem(c, http.StatusInternalServerError, "Internal Server Error",
			"An unexpected error occurred.")
		return
	}

	mapped, ok := statusTitles[appErr.Kind]
	if !ok {
		mapped.Status, mapped.Title = http.StatusBadRequest, "Error"
	}
	WriteProblem(c, mapped.Status, mapped.Title, appErr.Message)
}
