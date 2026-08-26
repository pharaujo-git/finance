package persistence_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	httpapi "github.com/pharaujo/finance/backend-go/internal/http"
	"github.com/pharaujo/finance/backend-go/internal/http/handlers"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/persistence"
	"github.com/pharaujo/finance/backend-go/internal/pgtest"
)

// newTestPool is pgtest.NewPool, kept as a local name so the tests below read
// the way they did before the helper moved to a package the jobs tests can
// share.
func newTestPool(t *testing.T) *pgxpool.Pool { return pgtest.NewPool(t) }

const testPassword = "correct horse battery"

func newUser(email string) *domain.User {
	return &domain.User{
		Id:           uuid.New(),
		Email:        email,
		Name:         "Owner",
		PasswordHash: "hash-" + email,
		Currency:     application.DefaultCurrency,
		// Truncated to microseconds: Postgres timestamps have no nanoseconds,
		// so a raw time.Now() would not survive a round trip intact.
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestUserRepositoryRoundTrip(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewUserRepository(pool)
	ctx := context.Background()

	user := newUser("owner@example.com")
	if err := repo.Add(ctx, user); err != nil {
		t.Fatalf("Add: %v", err)
	}

	byEmail, err := repo.FindByEmail(ctx, "owner@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if byEmail.Id != user.Id || byEmail.Name != "Owner" || byEmail.Currency != "USD" {
		t.Errorf("user = %+v, want the inserted row", byEmail)
	}
	if !byEmail.CreatedAt.Equal(user.CreatedAt) {
		t.Errorf("createdAt = %s, want %s", byEmail.CreatedAt, user.CreatedAt)
	}

	byID, err := repo.FindByID(ctx, user.Id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if byID.Email != user.Email {
		t.Errorf("email = %q, want %q", byID.Email, user.Email)
	}

	if err := repo.UpdatePasswordHash(ctx, user.Id, "rehashed"); err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}
	if err := repo.UpdateProfile(ctx, user.Id, "Grace", "EUR"); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	updated, err := repo.FindByID(ctx, user.Id)
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}
	if updated.PasswordHash != "rehashed" || updated.Name != "Grace" || updated.Currency != "EUR" {
		t.Errorf("updated = %+v, want rehashed / Grace / EUR", updated)
	}
}

func TestUserRepositoryReportsMissingRows(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewUserRepository(pool)
	ctx := context.Background()

	if _, err := repo.FindByEmail(ctx, "nobody@example.com"); !errors.Is(err, application.ErrUserNotFound) {
		t.Errorf("FindByEmail error = %v, want ErrUserNotFound", err)
	}
	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, application.ErrUserNotFound) {
		t.Errorf("FindByID error = %v, want ErrUserNotFound", err)
	}
	if err := repo.UpdateProfile(ctx, uuid.New(), "Ghost", "EUR"); !errors.Is(err, application.ErrUserNotFound) {
		t.Errorf("UpdateProfile error = %v, want ErrUserNotFound", err)
	}
	if err := repo.UpdatePasswordHash(ctx, uuid.New(), "hash"); !errors.Is(err, application.ErrUserNotFound) {
		t.Errorf("UpdatePasswordHash error = %v, want ErrUserNotFound", err)
	}
}

// The unique index is the real arbiter of email uniqueness; the service turns
// this error into the same 409 the pre-insert check produces.
func TestUserRepositoryMapsTheUniqueIndexToErrEmailTaken(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t)
	repo := persistence.NewUserRepository(pool)
	ctx := context.Background()

	if err := repo.Add(ctx, newUser("dup@example.com")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := repo.Add(ctx, newUser("dup@example.com")); !errors.Is(err, application.ErrEmailTaken) {
		t.Fatalf("second Add error = %v, want ErrEmailTaken", err)
	}
}

// TestAuthFlowOverHTTPAgainstPostgres drives the real router, the real service,
// the real Identity hasher and the real SQL repository against a real database.
func TestAuthFlowOverHTTPAgainstPostgres(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	pool := newTestPool(t)
	tokens := identity.NewTokenService(config.LocalDevelopmentSecret)
	auth := handlers.NewAuth(application.NewAuthService(
		persistence.NewUserRepository(pool),
		tokens,
		identity.NewPasswordHasher(),
	))
	engine := httpapi.New(httpapi.Options{
		Tokens:            tokens,
		AllowedOrigins:    []string{"http://localhost:5173"},
		RegisterAnonymous: []httpapi.RegisterFunc{auth.AnonymousRoutes},
		Register:          []httpapi.RegisterFunc{auth.AuthenticatedRoutes},
	})

	send := func(method, path, token string, body any) *httptest.ResponseRecorder {
		t.Helper()

		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding body: %v", err)
		}
		request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	register := send(http.MethodPost, "/api/auth/register", "", map[string]string{
		"email":    " Flow@Example.COM ",
		"password": testPassword,
		"name":     " Flow ",
	})
	if register.Code != http.StatusOK {
		t.Fatalf("register status = %d, body %s", register.Code, register.Body)
	}
	var registered application.AuthResponse
	if err := json.Unmarshal(register.Body.Bytes(), &registered); err != nil {
		t.Fatalf("decoding register body: %v", err)
	}
	if registered.User.Email != "flow@example.com" || registered.User.Name != "Flow" {
		t.Errorf("user = %+v, want flow@example.com / Flow", registered.User)
	}

	duplicate := send(http.MethodPost, "/api/auth/register", "", map[string]string{
		"email":    "flow@example.com",
		"password": testPassword,
		"name":     "Copy",
	})
	if duplicate.Code != http.StatusConflict {
		t.Errorf("duplicate status = %d, want 409 (body %s)", duplicate.Code, duplicate.Body)
	}

	login := send(http.MethodPost, "/api/auth/login", "", map[string]string{
		"email":    "flow@example.com",
		"password": testPassword,
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body %s", login.Code, login.Body)
	}
	var loggedIn application.AuthResponse
	if err := json.Unmarshal(login.Body.Bytes(), &loggedIn); err != nil {
		t.Fatalf("decoding login body: %v", err)
	}

	me := send(http.MethodGet, "/api/auth/me", loggedIn.Token, nil)
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, body %s", me.Code, me.Body)
	}
	var profile application.UserDto
	if err := json.Unmarshal(me.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decoding me body: %v", err)
	}
	if profile.ID != registered.User.ID {
		t.Errorf("me id = %s, want %s", profile.ID, registered.User.ID)
	}

	update := send(http.MethodPut, "/api/auth/me", loggedIn.Token, map[string]string{
		"name":     " Grace ",
		"currency": "eur",
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, body %s", update.Code, update.Body)
	}
	var updated application.UserDto
	if err := json.Unmarshal(update.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decoding update body: %v", err)
	}
	if updated.Name != "Grace" || updated.Currency != "EUR" {
		t.Errorf("updated = %+v, want Grace / EUR", updated)
	}

	stored, err := persistence.NewUserRepository(pool).FindByID(context.Background(), registered.User.ID)
	if err != nil {
		t.Fatalf("reading the row back: %v", err)
	}
	if stored.Name != "Grace" || stored.Currency != "EUR" {
		t.Errorf("stored row = %+v, want Grace / EUR", stored)
	}
}
