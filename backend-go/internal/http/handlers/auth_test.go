package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/apptest"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	httpapi "github.com/pharaujo/finance/backend-go/internal/http"
	"github.com/pharaujo/finance/backend-go/internal/http/handlers"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
)

const testPassword = "correct horse battery"

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// api is the real router with the auth endpoints mounted on it, backed by an
// in-memory repository.
type api struct {
	engine *gin.Engine
	users  *apptest.Users
	hasher application.PasswordHasher
	tokens *identity.TokenService
}

// newAPI wires the router with the fast hashing double.
func newAPI(t *testing.T) *api {
	t.Helper()
	return newAPIWith(t, apptest.NewHasher())
}

// newAPIWith wires the router with an explicit hasher, so one test can pay for
// the real PBKDF2 implementation and the rest can skip it.
func newAPIWith(t *testing.T, hasher application.PasswordHasher) *api {
	t.Helper()

	users := apptest.NewUsers()
	tokens := identity.NewTokenService(config.LocalDevelopmentSecret)
	auth := handlers.NewAuth(application.NewAuthService(users, tokens, hasher))

	engine := httpapi.New(httpapi.Options{
		Tokens:            tokens,
		AllowedOrigins:    []string{"http://localhost:5173"},
		RegisterAnonymous: []httpapi.RegisterFunc{auth.AnonymousRoutes},
		Register:          []httpapi.RegisterFunc{auth.AuthenticatedRoutes},
	})

	return &api{engine: engine, users: users, hasher: hasher, tokens: tokens}
}

// do sends a request with an optional JSON body and bearer token.
func (a *api) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else if raw, ok := body.(string); ok {
		reader = bytes.NewReader([]byte(raw))
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	a.engine.ServeHTTP(recorder, request)
	return recorder
}

// register drives the real endpoint and returns the decoded response.
func (a *api) register(t *testing.T, email, name string) application.AuthResponse {
	t.Helper()

	response := a.do(t, http.MethodPost, "/api/auth/register", "", map[string]string{
		"email":    email,
		"password": testPassword,
		"name":     name,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("register status = %d, body %s", response.Code, response.Body)
	}

	var decoded application.AuthResponse
	decode(t, response, &decoded)
	return decoded
}

func decode(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decoding %s: %v", response.Body, err)
	}
}

// TestRegisterAndLoginRoundTripWithTheRealHasher is the wiring proof: the
// stored blob is a genuine Identity v3 hash, and the token the endpoints hand
// out opens the authenticated routes.
func TestRegisterAndLoginRoundTripWithTheRealHasher(t *testing.T) {
	t.Parallel()

	a := newAPIWith(t, identity.NewPasswordHasher())
	registered := a.register(t, "Owner@Example.COM", " Ada ")

	if registered.User.Email != "owner@example.com" || registered.User.Name != "Ada" {
		t.Errorf("user = %+v, want owner@example.com / Ada", registered.User)
	}
	stored, ok := a.users.Get(registered.User.ID)
	if !ok {
		t.Fatal("no user was stored")
	}
	if outcome := a.hasher.Verify(stored.PasswordHash, testPassword); outcome != application.PasswordSuccess {
		t.Errorf("stored hash verifies as %v, want PasswordSuccess", outcome)
	}

	login := a.do(t, http.MethodPost, "/api/auth/login", "", map[string]string{
		"email":    "owner@example.com",
		"password": testPassword,
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body %s", login.Code, login.Body)
	}
	var loggedIn application.AuthResponse
	decode(t, login, &loggedIn)

	me := a.do(t, http.MethodGet, "/api/auth/me", loggedIn.Token, nil)
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, body %s", me.Code, me.Body)
	}
	var profile application.UserDto
	decode(t, me, &profile)
	if profile.ID != registered.User.ID {
		t.Errorf("me id = %s, want %s", profile.ID, registered.User.ID)
	}
}

// The frontend reads response.user.id / .email / .name / .currency, so the
// exact keys matter.
func TestRegisterBodyUsesCamelCaseKeys(t *testing.T) {
	t.Parallel()

	a := newAPI(t)
	response := a.do(t, http.MethodPost, "/api/auth/register", "", map[string]string{
		"email":    "keys@example.com",
		"password": testPassword,
		"name":     "Keys",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", response.Code, response.Body)
	}

	var body map[string]json.RawMessage
	decode(t, response, &body)
	for _, key := range []string{"token", "user"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing %q in %s", key, response.Body)
		}
	}

	var user map[string]any
	if err := json.Unmarshal(body["user"], &user); err != nil {
		t.Fatalf("decoding user: %v", err)
	}
	want := map[string]any{"email": "keys@example.com", "name": "Keys", "currency": "USD"}
	for key, value := range want {
		if user[key] != value {
			t.Errorf("user[%q] = %v, want %v", key, user[key], value)
		}
	}
	if _, ok := user["id"].(string); !ok {
		t.Errorf("user.id = %v, want a string", user["id"])
	}
	if len(user) != 4 {
		t.Errorf("user has %d keys (%v), want exactly id/email/name/currency", len(user), user)
	}
}

func TestRegisterConflictIsAProblemDocument(t *testing.T) {
	t.Parallel()

	a := newAPI(t)
	a.register(t, "dup@example.com", "First")

	response := a.do(t, http.MethodPost, "/api/auth/register", "", map[string]string{
		"email":    "DUP@example.com",
		"password": testPassword,
		"name":     "Second",
	})

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %s)", response.Code, response.Body)
	}
	if got := response.Header().Get("Content-Type"); got != middleware.ProblemContentType {
		t.Errorf("content type = %q, want %q", got, middleware.ProblemContentType)
	}

	var problem middleware.Problem
	decode(t, response, &problem)
	if problem.Title != "Conflict" || problem.Status != http.StatusConflict {
		t.Errorf("problem = %+v, want Conflict / 409", problem)
	}
	if problem.Detail != "An account with that email already exists." {
		t.Errorf("detail = %q", problem.Detail)
	}
	if problem.Instance != "/api/auth/register" {
		t.Errorf("instance = %q", problem.Instance)
	}
}

func TestLoginFailuresAreUniform401Problems(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"unknown email":  {"email": "missing@example.com", "password": testPassword},
		"wrong password": {"email": "known@example.com", "password": "wrong password"},
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			a := newAPI(t)
			a.register(t, "known@example.com", "Known")

			response := a.do(t, http.MethodPost, "/api/auth/login", "", body)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body %s)", response.Code, response.Body)
			}
			var problem middleware.Problem
			decode(t, response, &problem)
			if problem.Detail != "Invalid email or password." {
				t.Errorf("detail = %q, want the uniform message", problem.Detail)
			}
			if problem.Title != "Unauthorized" {
				t.Errorf("title = %q, want Unauthorized", problem.Title)
			}
		})
	}
}

// The shape here was captured from the running .NET API; the frontend's
// readError() falls back to `title`, which is why it is reproduced exactly.
func TestValidationProblemMatchesTheDotNetShape(t *testing.T) {
	t.Parallel()

	a := newAPI(t)
	response := a.do(t, http.MethodPost, "/api/auth/register", "", map[string]string{
		"email":    "not-an-email",
		"password": "short",
		"name":     "",
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", response.Code, response.Body)
	}
	if got := response.Header().Get("Content-Type"); got != middleware.ValidationContentType {
		t.Errorf("content type = %q, want %q", got, middleware.ValidationContentType)
	}

	var problem middleware.ValidationProblem
	decode(t, response, &problem)

	if problem.Title != domain.ValidationProblemTitle {
		t.Errorf("title = %q, want %q", problem.Title, domain.ValidationProblemTitle)
	}
	if problem.Type != domain.ValidationProblemType {
		t.Errorf("type = %q, want %q", problem.Type, domain.ValidationProblemType)
	}
	if problem.Status != http.StatusBadRequest {
		t.Errorf("status field = %d, want 400", problem.Status)
	}

	want := map[string][]string{
		"Name": {"The Name field is required."},
		"Email": {
			"The Email field is not a valid e-mail address.",
		},
		"Password": {
			"The field Password must be a string or array type with a minimum length of '8'.",
		},
	}
	for field, messages := range want {
		got := problem.Errors[field]
		if strings.Join(got, "|") != strings.Join(messages, "|") {
			t.Errorf("errors[%q] = %q, want %q", field, got, messages)
		}
	}
	if len(problem.Errors) != len(want) {
		t.Errorf("errors = %v, want exactly %v", problem.Errors, want)
	}
}

func TestUnparsableBodyIsAValidationProblem(t *testing.T) {
	t.Parallel()

	a := newAPI(t)
	response := a.do(t, http.MethodPost, "/api/auth/login", "", "{")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", response.Code, response.Body)
	}
	var problem middleware.ValidationProblem
	decode(t, response, &problem)
	if len(problem.Errors[domain.JSONBodyField]) == 0 {
		t.Errorf("errors = %v, want a %q entry", problem.Errors, domain.JSONBodyField)
	}
}

func TestProfileEndpointsRequireABearerToken(t *testing.T) {
	t.Parallel()

	a := newAPI(t)

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		response := a.do(t, method, "/api/auth/me", "", map[string]string{"name": "X", "currency": "EUR"})
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401 (body %s)", method, response.Code, response.Body)
		}
	}
}

func TestUpdateProfileReturnsTheUpdatedUser(t *testing.T) {
	t.Parallel()

	a := newAPI(t)
	registered := a.register(t, "me@example.com", "Owner")

	response := a.do(t, http.MethodPut, "/api/auth/me", registered.Token, map[string]string{
		"name":     " Grace ",
		"currency": "eur",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", response.Code, response.Body)
	}

	var user application.UserDto
	decode(t, response, &user)
	if user.Name != "Grace" || user.Currency != "EUR" {
		t.Errorf("user = %+v, want Grace / EUR", user)
	}

	stored, _ := a.users.Get(registered.User.ID)
	if stored.Name != "Grace" || stored.Currency != "EUR" {
		t.Errorf("stored = %+v, want Grace / EUR", stored)
	}
}

func TestUpdateProfileValidationIsA400(t *testing.T) {
	t.Parallel()

	a := newAPI(t)
	registered := a.register(t, "me@example.com", "Owner")

	response := a.do(t, http.MethodPut, "/api/auth/me", registered.Token, map[string]string{
		"name":     "",
		"currency": "toolongcurrency",
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", response.Code, response.Body)
	}
	var problem middleware.ValidationProblem
	decode(t, response, &problem)
	want := map[string][]string{
		"Name":     {"The Name field is required."},
		"Currency": {"The field Currency must be a string or array type with a maximum length of '8'."},
	}
	for field, messages := range want {
		if strings.Join(problem.Errors[field], "|") != strings.Join(messages, "|") {
			t.Errorf("errors[%q] = %q, want %q", field, problem.Errors[field], messages)
		}
	}
}

// A token that outlives its row (the account was deleted) is a 404, matching
// AuthService.LoadAsync.
func TestProfileForADeletedUserIs404(t *testing.T) {
	t.Parallel()

	a := newAPI(t)
	token, err := a.tokens.Issue(uuid.New(), "ghost@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	response := a.do(t, http.MethodGet, "/api/auth/me", token, nil)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", response.Code, response.Body)
	}
	var problem middleware.Problem
	decode(t, response, &problem)
	if problem.Detail != "User was not found." {
		t.Errorf("detail = %q, want \"User was not found.\"", problem.Detail)
	}
	if problem.Title != "Not Found" {
		t.Errorf("title = %q, want Not Found", problem.Title)
	}
}
