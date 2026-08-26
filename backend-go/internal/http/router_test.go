package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/fixtures"
	httpapi "github.com/pharaujo/finance/backend-go/internal/http"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// newEngine builds the real router plus one protected probe route that echoes
// the authenticated caller's id.
func newEngine(tokens *identity.TokenService) *gin.Engine {
	return httpapi.New(httpapi.Options{
		Tokens:         tokens,
		AllowedOrigins: []string{"http://localhost:5173", "https://finance.example"},
		Register: []httpapi.RegisterFunc{func(api *gin.RouterGroup) {
			api.GET("/probe", func(c *gin.Context) {
				id, ok := middleware.UserID(c)
				if !ok {
					c.String(http.StatusInternalServerError, "no user id in context")
					return
				}
				c.JSON(http.StatusOK, gin.H{"userId": id.String()})
			})
		}},
	})
}

func do(engine *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestHealthReturnsPlainOk(t *testing.T) {
	t.Parallel()

	response := do(newEngine(identity.NewTokenService(config.LocalDevelopmentSecret)), http.MethodGet, "/health", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if body := response.Body.String(); body != "ok" {
		t.Errorf("body = %q, want \"ok\"", body)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Errorf("content type = %q, want text/plain; charset=utf-8", contentType)
	}
}

func TestRootReturnsServiceDocument(t *testing.T) {
	t.Parallel()

	response := do(newEngine(identity.NewTokenService(config.LocalDevelopmentSecret)), http.MethodGet, "/", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body %q: %v", response.Body.String(), err)
	}
	want := map[string]string{
		"service": "FinanceTracker API (Go)",
		"status":  "ok",
		"docs":    "/swagger",
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("%s = %q, want %q", key, body[key], value)
		}
	}
}

func TestProtectedRouteRejectsMissingOrBadTokens(t *testing.T) {
	t.Parallel()

	engine := newEngine(identity.NewTokenService(config.LocalDevelopmentSecret))
	otherIssuer := identity.NewTokenService("some-other-secret")
	foreign, err := otherIssuer.Issue(uuid.New(), "intruder@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	cases := map[string]map[string]string{
		"no header":       nil,
		"empty bearer":    {"Authorization": "Bearer "},
		"wrong scheme":    {"Authorization": "Basic abc123"},
		"garbage token":   {"Authorization": "Bearer not-a-token"},
		"foreign signing": {"Authorization": "Bearer " + foreign},
	}

	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := do(engine, http.MethodGet, "/api/probe", headers)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body %q)", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); contentType != middleware.ProblemContentType {
				t.Errorf("content type = %q, want %q", contentType, middleware.ProblemContentType)
			}

			var problem middleware.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decoding problem %q: %v", response.Body.String(), err)
			}
			if problem.Title != "Unauthorized" {
				t.Errorf("title = %q, want Unauthorized", problem.Title)
			}
			if problem.Status != http.StatusUnauthorized {
				t.Errorf("status field = %d, want 401", problem.Status)
			}
			if problem.Detail == "" {
				t.Error("detail is empty")
			}
			if problem.Instance != "/api/probe" {
				t.Errorf("instance = %q, want /api/probe", problem.Instance)
			}
		})
	}
}

func TestProtectedRouteAcceptsGoIssuedToken(t *testing.T) {
	t.Parallel()

	tokens := identity.NewTokenService(config.LocalDevelopmentSecret)
	engine := newEngine(tokens)
	userID := uuid.New()

	token, err := tokens.Issue(userID, "go@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	response := do(engine, http.MethodGet, "/api/probe", map[string]string{"Authorization": "Bearer " + token})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", response.Code, response.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body["userId"] != userID.String() {
		t.Errorf("userId = %q, want %q", body["userId"], userID)
	}
}

// TestProtectedRouteAcceptsDotNetToken is the end-to-end parity assertion: a
// bearer token minted by the .NET API opens a Go route. The clock is pinned to
// just after the fixture was issued so the test does not rot when the token's
// seven days elapse.
func TestProtectedRouteAcceptsDotNetToken(t *testing.T) {
	t.Parallel()

	fixture := fixtures.MustLoadDotNetIdentity()
	moment := time.Unix(fixture.IssuedAtUnix+60, 0).UTC()
	tokens := identity.NewTokenService(config.LocalDevelopmentSecret).
		WithClock(func() time.Time { return moment })

	response := do(newEngine(tokens), http.MethodGet, "/api/probe",
		map[string]string{"Authorization": "Bearer " + fixture.Token})

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", response.Code, response.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body["userId"] != fixture.UserID {
		t.Errorf("userId = %q, want %q", body["userId"], fixture.UserID)
	}
}

func TestCors(t *testing.T) {
	t.Parallel()

	engine := newEngine(identity.NewTokenService(config.LocalDevelopmentSecret))

	t.Run("allowed origin is echoed", func(t *testing.T) {
		t.Parallel()
		response := do(engine, http.MethodGet, "/health",
			map[string]string{"Origin": "http://localhost:5173"})
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Errorf("allow-origin = %q, want http://localhost:5173", got)
		}
	})

	t.Run("unknown origin is not echoed", func(t *testing.T) {
		t.Parallel()
		response := do(engine, http.MethodGet, "/health",
			map[string]string{"Origin": "https://evil.example"})
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("allow-origin = %q, want empty", got)
		}
		if response.Code != http.StatusOK {
			t.Errorf("status = %d, want 200: CORS is a browser guard, not an authorisation gate", response.Code)
		}
	})

	t.Run("preflight", func(t *testing.T) {
		t.Parallel()
		response := do(engine, http.MethodOptions, "/api/probe", map[string]string{
			"Origin":                         "https://finance.example",
			"Access-Control-Request-Method":  "GET",
			"Access-Control-Request-Headers": "authorization,content-type",
		})
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", response.Code)
		}
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://finance.example" {
			t.Errorf("allow-origin = %q, want https://finance.example", got)
		}
		if got := response.Header().Get("Access-Control-Allow-Headers"); got != "authorization,content-type" {
			t.Errorf("allow-headers = %q, want the requested headers echoed", got)
		}
		if got := response.Header().Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("allow-methods is empty")
		}
	})
}
