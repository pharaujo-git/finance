package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
)

const (
	// PrincipalContextKey holds the validated application.Principal.
	PrincipalContextKey = "principal"
	// UserIDContextKey holds the caller's uuid.UUID, the value handlers want.
	UserIDContextKey = "userID"

	bearerPrefix = "Bearer "
)

// RequireAuth validates the Authorization bearer token and puts the caller's
// id in the request context. Failures render 401 problem+json.
//
// Deviation from the .NET API: its JwtBearer handler answers a missing or bad
// token with an empty 401 body plus WWW-Authenticate. Here the body is a
// problem document so every 401 the Go API emits has the same shape.
func RequireAuth(tokens application.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, bearerPrefix) {
			unauthorized(c, "Authentication is required.")
			return
		}

		raw := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
		if raw == "" {
			unauthorized(c, "Authentication is required.")
			return
		}

		principal, err := tokens.Validate(raw)
		if err != nil {
			unauthorized(c, "The access token is invalid or has expired.")
			return
		}

		c.Set(PrincipalContextKey, principal)
		c.Set(UserIDContextKey, principal.UserID)
		c.Next()
	}
}

func unauthorized(c *gin.Context, detail string) {
	c.Header("WWW-Authenticate", "Bearer")
	WriteProblem(c, http.StatusUnauthorized, "Unauthorized", detail)
}

// UserID returns the authenticated caller's id. The second result is false
// when the handler ran outside RequireAuth.
func UserID(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get(UserIDContextKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := value.(uuid.UUID)
	return id, ok
}

// CurrentPrincipal returns the full principal stored by RequireAuth.
func CurrentPrincipal(c *gin.Context) (application.Principal, bool) {
	value, exists := c.Get(PrincipalContextKey)
	if !exists {
		return application.Principal{}, false
	}
	principal, ok := value.(application.Principal)
	return principal, ok
}
