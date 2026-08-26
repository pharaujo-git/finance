package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// invalidValueMessage is ASP.NET Core's AttemptedValueIsInvalidAccessor, the
// message its model binder records when a query value will not convert.
const invalidValueMessage = "The value '%s' is not valid for %s."

// missingCallerMessage is the 401 a handler renders when it somehow ran outside
// the auth middleware.
const missingCallerMessage = "The access token does not identify a user."

// caller returns the authenticated user's id, rendering a 401 when there is
// none.
func caller(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := middleware.UserID(c)
	if !ok {
		middleware.WriteAppError(c, domain.Unauthorized(missingCallerMessage))
		return uuid.Nil, false
	}
	return userID, true
}

// pathID parses the {id} route parameter. A value that is not a uuid renders a
// bare 404: the .NET routes constrain the segment with {id:guid}, so such a
// request matches no route at all and the framework answers 404 with no body.
func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return uuid.Nil, false
	}
	return id, true
}

// queryReader binds query-string values the way MVC's model binder does,
// collecting conversion failures so one response can report all of them.
type queryReader struct {
	c    *gin.Context
	errs *domain.ValidationError
}

func newQueryReader(c *gin.Context) *queryReader {
	return &queryReader{c: c, errs: domain.NewValidationError()}
}

// text returns a query value, or nil when the caller omitted the key. An empty
// value is not the same as an absent one, which is what makes `?month=` a
// validation failure rather than "this month".
func (r *queryReader) text(key string) *string {
	value, ok := r.c.GetQuery(key)
	if !ok {
		return nil
	}
	return &value
}

// number reads an optional int.
func (r *queryReader) number(key, field string) *int {
	raw := r.text(key)
	if raw == nil {
		return nil
	}

	parsed, err := strconv.Atoi(*raw)
	if err != nil {
		r.invalid(*raw, field)
		return nil
	}
	return &parsed
}

// numberOr reads an int with a default, as `[FromQuery] int months = 12` does.
func (r *queryReader) numberOr(key, field string, fallback int) int {
	if parsed := r.number(key, field); parsed != nil {
		return *parsed
	}
	return fallback
}

// identifier reads an optional uuid.
func (r *queryReader) identifier(key, field string) *uuid.UUID {
	raw := r.text(key)
	if raw == nil {
		return nil
	}

	parsed, err := uuid.Parse(*raw)
	if err != nil {
		r.invalid(*raw, field)
		return nil
	}
	return &parsed
}

// moment reads an optional date or timestamp.
func (r *queryReader) moment(key, field string) *time.Time {
	raw := r.text(key)
	if raw == nil {
		return nil
	}

	parsed, ok := application.ParseWireDate(*raw)
	if !ok {
		r.invalid(*raw, field)
		return nil
	}
	return &parsed
}

// transactionType reads an optional TransactionType, which the binder accepts
// as a member name in any casing or as an ordinal.
func (r *queryReader) transactionType(key, field string) *domain.TransactionType {
	raw := r.text(key)
	if raw == nil {
		return nil
	}

	parsed, ok := domain.ParseTransactionType(*raw)
	if !ok {
		r.invalid(*raw, field)
		return nil
	}
	return &parsed
}

func (r *queryReader) invalid(value, field string) {
	r.errs.Add(field, fmt.Sprintf(invalidValueMessage, value, field))
}

// ok reports whether every value converted, rendering the validation problem
// when one did not.
func (r *queryReader) ok() bool {
	if r.errs.Empty() {
		return true
	}
	middleware.WriteAppError(r.c, r.errs)
	return false
}
