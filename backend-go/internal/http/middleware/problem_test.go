package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/domain"
	"github.com/pharaujo/finance/backend-go/internal/fixtures"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func render(path string, err error) *httptest.ResponseRecorder {
	engine := gin.New()
	engine.GET(path, func(c *gin.Context) { middleware.WriteAppError(c, err) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestWriteAppErrorMapsKindsToStatuses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    *domain.AppError
		status int
		title  string
	}{
		{"validation", domain.BadRequest("Month must be in YYYY-MM format."), 400, "Bad Request"},
		{"unauthorized", domain.Unauthorized("Invalid email or password."), 401, "Unauthorized"},
		{"not found", domain.NotFound("Account"), 404, "Not Found"},
		{"conflict", domain.Conflict("An account with that email already exists."), 409, "Conflict"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			response := render("/api/thing", testCase.err)

			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d", response.Code, testCase.status)
			}
			if got := response.Header().Get("Content-Type"); got != middleware.ProblemContentType {
				t.Errorf("content type = %q, want %q", got, middleware.ProblemContentType)
			}

			var problem middleware.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decoding %q: %v", response.Body.String(), err)
			}
			if problem.Title != testCase.title {
				t.Errorf("title = %q, want %q", problem.Title, testCase.title)
			}
			if problem.Status != testCase.status {
				t.Errorf("status field = %d, want %d", problem.Status, testCase.status)
			}
			if problem.Detail != testCase.err.Message {
				t.Errorf("detail = %q, want %q", problem.Detail, testCase.err.Message)
			}
			if problem.Instance != "/api/thing" {
				t.Errorf("instance = %q, want /api/thing", problem.Instance)
			}
		})
	}
}

// TestProblemMatchesDotNetWireFormat compares a rendered document, byte for
// byte, with bodies captured from the running .NET API. Field order matters
// here only as evidence that nothing extra (a `type` member, a null) crept in.
func TestProblemMatchesDotNetWireFormat(t *testing.T) {
	t.Parallel()

	fixture := fixtures.MustLoadDotNetIdentity()

	cases := []struct {
		name string
		path string
		err  *domain.AppError
		want string
	}{
		{
			name: "conflict",
			path: "/api/auth/register",
			err:  domain.Conflict("An account with that email already exists."),
			want: fixture.ProblemJSON.Conflict,
		},
		{
			name: "unauthorized",
			path: "/api/auth/login",
			err:  domain.Unauthorized("Invalid email or password."),
			want: fixture.ProblemJSON.Unauthorized,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := render(testCase.path, testCase.err).Body.String()
			if got != testCase.want {
				t.Errorf("problem body\n go:   %s\n .NET: %s", got, testCase.want)
			}
		})
	}
}

func TestWriteAppErrorFallsBackForUnknownErrors(t *testing.T) {
	t.Parallel()

	response := render("/api/thing", errPlain{})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}

	var problem middleware.Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding %q: %v", response.Body.String(), err)
	}
	if problem.Title != "Internal Server Error" {
		t.Errorf("title = %q, want Internal Server Error", problem.Title)
	}
}

type errPlain struct{}

func (errPlain) Error() string { return "something the caller should never see" }
