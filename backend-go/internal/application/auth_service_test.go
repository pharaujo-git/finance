package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/apptest"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
)

const testPassword = "correct horse battery"

// harness is one service wired to doubles, plus the doubles themselves.
type harness struct {
	service *application.AuthService
	users   *apptest.Users
	hasher  *apptest.Hasher
	tokens  *identity.TokenService
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	users := apptest.NewUsers()
	hasher := apptest.NewHasher()
	tokens := identity.NewTokenService(config.LocalDevelopmentSecret)

	return &harness{
		service: application.NewAuthService(users, tokens, hasher),
		users:   users,
		hasher:  hasher,
		tokens:  tokens,
	}
}

// seed stores a user whose password is testPassword, hashed by the double.
func (h *harness) seed(t *testing.T, email string) domain.User {
	t.Helper()

	hash, err := h.hasher.Hash(testPassword)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	user := domain.User{
		Id:           uuid.New(),
		Email:        email,
		Name:         "Owner",
		PasswordHash: hash,
		Currency:     application.DefaultCurrency,
		CreatedAt:    time.Now().UTC(),
	}
	h.users.Seed(user)
	return user
}

// requireAppError asserts the error is an AppError of the given kind carrying
// exactly the given message.
func requireAppError(t *testing.T, err error, kind domain.ErrorKind, message string) {
	t.Helper()

	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v (%T), want *domain.AppError", err, err)
	}
	if appErr.Kind != kind {
		t.Errorf("kind = %v, want %v", appErr.Kind, kind)
	}
	if appErr.Message != message {
		t.Errorf("message = %q, want %q", appErr.Message, message)
	}
}

// requireFieldErrors asserts the error is a ValidationError whose dictionary is
// exactly want.
func requireFieldErrors(t *testing.T, err error, want map[string][]string) {
	t.Helper()

	var validationErr *domain.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v (%T), want *domain.ValidationError", err, err)
	}
	if len(validationErr.Errors) != len(want) {
		t.Fatalf("fields = %v, want %v", validationErr.Errors, want)
	}
	for field, messages := range want {
		got := validationErr.Errors[field]
		if len(got) != len(messages) {
			t.Fatalf("%s = %q, want %q", field, got, messages)
		}
		for i, message := range messages {
			if got[i] != message {
				t.Errorf("%s[%d] = %q, want %q", field, i, got[i], message)
			}
		}
	}
}

func TestRegisterNormalisesAndIssuesAToken(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	moment := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	service := h.service.
		WithClock(func() time.Time { return moment }).
		WithIDs(func() uuid.UUID { return id })

	response, err := service.Register(context.Background(), application.RegisterRequest{
		Email:    " Owner@Example.COM ",
		Password: testPassword,
		Name:     " Ada ",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if response.User.Email != "owner@example.com" {
		t.Errorf("email = %q, want owner@example.com", response.User.Email)
	}
	if response.User.Name != "Ada" {
		t.Errorf("name = %q, want Ada", response.User.Name)
	}
	if response.User.Currency != application.DefaultCurrency {
		t.Errorf("currency = %q, want %q", response.User.Currency, application.DefaultCurrency)
	}
	if response.User.ID != id {
		t.Errorf("id = %s, want %s", response.User.ID, id)
	}

	principal, err := h.tokens.Validate(response.Token)
	if err != nil {
		t.Fatalf("the issued token does not validate: %v", err)
	}
	if principal.UserID != id || principal.Email != "owner@example.com" {
		t.Errorf("principal = %+v, want %s / owner@example.com", principal, id)
	}

	stored, ok := h.users.Get(id)
	if !ok {
		t.Fatal("no user was stored")
	}
	if !stored.CreatedAt.Equal(moment) || stored.CreatedAt.Location() != time.UTC {
		t.Errorf("createdAt = %s, want %s (UTC)", stored.CreatedAt, moment)
	}
	if stored.Email != "owner@example.com" {
		t.Errorf("stored email = %q, want owner@example.com", stored.Email)
	}
	if h.hasher.Verify(stored.PasswordHash, testPassword) != application.PasswordSuccess {
		t.Errorf("stored hash %q does not verify the password", stored.PasswordHash)
	}
}

func TestRegisterRejectsADuplicateEmail(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.seed(t, "dup@example.com")

	_, err := h.service.Register(context.Background(), application.RegisterRequest{
		Email:    "DUP@example.com",
		Password: "another password",
		Name:     "Copy",
	})

	requireAppError(t, err, domain.KindConflict, "An account with that email already exists.")
	if h.users.Count() != 1 {
		t.Errorf("stored users = %d, want 1", h.users.Count())
	}
}

// A second registration can slip past the select-then-insert check; the unique
// index is what settles it, and the caller must not be able to tell.
func TestRegisterReportsTheUniqueIndexRaceAsTheSameConflict(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.users.AddFailsWith = application.ErrEmailTaken

	_, err := h.service.Register(context.Background(), application.RegisterRequest{
		Email:    "racer@example.com",
		Password: testPassword,
		Name:     "Racer",
	})

	requireAppError(t, err, domain.KindConflict, "An account with that email already exists.")
}

func TestRegisterPropagatesRepositoryFailures(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	boom := errors.New("connection refused")
	h.users.FailWith = boom

	_, err := h.service.Register(context.Background(), application.RegisterRequest{
		Email:    "owner@example.com",
		Password: testPassword,
		Name:     "Ada",
	})

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		t.Errorf("a repository failure must not become a caller error: %v", appErr)
	}
}

func TestRegisterValidation(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		request application.RegisterRequest
		want    map[string][]string
	}{
		"everything missing": {
			request: application.RegisterRequest{},
			want: map[string][]string{
				"Email": {
					"The Email field is required.",
					"The Email field is not a valid e-mail address.",
				},
				"Password": {
					"The Password field is required.",
					"The field Password must be a string or array type with a minimum length of '8'.",
				},
				"Name": {"The Name field is required."},
			},
		},
		"invalid email and short password": {
			request: application.RegisterRequest{Email: "not-an-email", Password: "short7x", Name: "Ada"},
			want: map[string][]string{
				"Email":    {"The Email field is not a valid e-mail address."},
				"Password": {"The field Password must be a string or array type with a minimum length of '8'."},
			},
		},
		"password of exactly eight is accepted": {
			request: application.RegisterRequest{Email: "a@b.com", Password: "12345678", Name: "Ada"},
			want:    nil,
		},
		"name of 201 characters": {
			request: application.RegisterRequest{
				Email:    "a@b.com",
				Password: testPassword,
				Name:     strings.Repeat("a", 201),
			},
			want: map[string][]string{
				"Name": {"The field Name must be a string or array type with a maximum length of '200'."},
			},
		},
		"email of 257 characters": {
			request: application.RegisterRequest{
				Email:    strings.Repeat("a", 245) + "@example.com",
				Password: testPassword,
				Name:     "Ada",
			},
			want: map[string][]string{
				"Email": {"The field Email must be a string or array type with a maximum length of '256'."},
			},
		},
		"password of 129 characters": {
			request: application.RegisterRequest{
				Email:    "a@b.com",
				Password: strings.Repeat("p", 129),
				Name:     "Ada",
			},
			want: map[string][]string{
				"Password": {"The field Password must be a string or array type with a maximum length of '128'."},
			},
		},
		"whitespace only name": {
			request: application.RegisterRequest{Email: "a@b.com", Password: testPassword, Name: "   "},
			want:    map[string][]string{"Name": {"The Name field is required."}},
		},
		"two at signs": {
			request: application.RegisterRequest{Email: "a@@b.com", Password: testPassword, Name: "Ada"},
			want:    map[string][]string{"Email": {"The Email field is not a valid e-mail address."}},
		},
		"trailing at sign": {
			request: application.RegisterRequest{Email: "ada@", Password: testPassword, Name: "Ada"},
			want:    map[string][]string{"Email": {"The Email field is not a valid e-mail address."}},
		},
		"padded email still validates": {
			request: application.RegisterRequest{Email: " Ada@Example.com ", Password: testPassword, Name: "Ada"},
			want:    nil,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			_, err := h.service.Register(context.Background(), testCase.request)

			if testCase.want == nil {
				if err != nil {
					t.Fatalf("Register: %v", err)
				}
				return
			}
			requireFieldErrors(t, err, testCase.want)
			if h.users.Count() != 0 {
				t.Errorf("stored users = %d, want 0", h.users.Count())
			}
		})
	}
}

func TestLoginSucceedsAndIssuesAToken(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	user := h.seed(t, "login@example.com")

	response, err := h.service.Login(context.Background(), application.LoginRequest{
		Email:    " Login@Example.com ",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if response.User.ID != user.Id {
		t.Errorf("id = %s, want %s", response.User.ID, user.Id)
	}
	if _, err := h.tokens.Validate(response.Token); err != nil {
		t.Errorf("the issued token does not validate: %v", err)
	}
}

// The two ways to fail must be indistinguishable, or the endpoint becomes an
// account-enumeration oracle.
func TestLoginFailsIdenticallyForUnknownEmailAndWrongPassword(t *testing.T) {
	t.Parallel()

	cases := map[string]application.LoginRequest{
		"unknown email":  {Email: "missing@example.com", Password: testPassword},
		"wrong password": {Email: "login@example.com", Password: "wrong password"},
	}

	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			h.seed(t, "login@example.com")

			_, err := h.service.Login(context.Background(), request)

			requireAppError(t, err, domain.KindUnauthorized, "Invalid email or password.")
		})
	}
}

func TestLoginRehashesAndPersistsAStaleHash(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.hasher.Prefix = "legacy"
	user := h.seed(t, "stale@example.com")
	h.hasher.Prefix = "current"
	h.hasher.Outcome = application.PasswordSuccessRehashNeeded

	if _, err := h.service.Login(context.Background(), application.LoginRequest{
		Email:    "stale@example.com",
		Password: testPassword,
	}); err != nil {
		t.Fatalf("Login: %v", err)
	}

	stored, ok := h.users.Get(user.Id)
	if !ok {
		t.Fatal("the user disappeared")
	}
	if stored.PasswordHash == user.PasswordHash {
		t.Errorf("stored hash is still %q; the rehash was not persisted", stored.PasswordHash)
	}
	if want := "current:" + testPassword; stored.PasswordHash != want {
		t.Errorf("stored hash = %q, want %q", stored.PasswordHash, want)
	}
}

func TestLoginLeavesACurrentHashAlone(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	user := h.seed(t, "fresh@example.com")
	hashesBefore := h.hasher.Hashes()

	if _, err := h.service.Login(context.Background(), application.LoginRequest{
		Email:    "fresh@example.com",
		Password: testPassword,
	}); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if h.hasher.Hashes() != hashesBefore {
		t.Errorf("Hash was called %d extra times", h.hasher.Hashes()-hashesBefore)
	}
	stored, _ := h.users.Get(user.Id)
	if stored.PasswordHash != user.PasswordHash {
		t.Errorf("stored hash changed to %q", stored.PasswordHash)
	}
}

// Sign-in carries no [EmailAddress] rule, so a malformed address is a
// credentials failure rather than a validation error.
func TestLoginValidation(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.service.Login(context.Background(), application.LoginRequest{})
	requireFieldErrors(t, err, map[string][]string{
		"Email":    {"The Email field is required."},
		"Password": {"The Password field is required."},
	})

	_, err = h.service.Login(context.Background(), application.LoginRequest{
		Email:    "not-an-email",
		Password: "whatever it is",
	})
	requireAppError(t, err, domain.KindUnauthorized, "Invalid email or password.")
}

func TestProfileReadsTheCallersRow(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	user := h.seed(t, "me@example.com")

	profile, err := h.service.Profile(context.Background(), user.Id)
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}

	if profile.ID != user.Id || profile.Email != "me@example.com" || profile.Name != "Owner" {
		t.Errorf("profile = %+v, want the seeded user", profile)
	}
}

func TestProfileForAVanishedUserIsNotFound(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.service.Profile(context.Background(), uuid.New())

	requireAppError(t, err, domain.KindNotFound, "User was not found.")
}

func TestUpdateProfileTrimsTheNameAndUpperCasesTheCurrency(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	user := h.seed(t, "me@example.com")

	updated, err := h.service.UpdateProfile(context.Background(), user.Id, application.UpdateProfileRequest{
		Name:     " Grace ",
		Currency: " eur ",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	if updated.Name != "Grace" || updated.Currency != "EUR" {
		t.Errorf("response = %+v, want Grace / EUR", updated)
	}

	stored, _ := h.users.Get(user.Id)
	if stored.Name != "Grace" || stored.Currency != "EUR" {
		t.Errorf("stored = %+v, want Grace / EUR", stored)
	}
	if stored.Email != user.Email {
		t.Errorf("email changed to %q", stored.Email)
	}
}

func TestUpdateProfileForAVanishedUserIsNotFound(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.service.UpdateProfile(context.Background(), uuid.New(), application.UpdateProfileRequest{
		Name:     "Grace",
		Currency: "EUR",
	})

	requireAppError(t, err, domain.KindNotFound, "User was not found.")
}

func TestUpdateProfileValidation(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		request application.UpdateProfileRequest
		want    map[string][]string
	}{
		"both missing": {
			request: application.UpdateProfileRequest{},
			want: map[string][]string{
				"Name":     {"The Name field is required."},
				"Currency": {"The Currency field is required."},
			},
		},
		"currency of nine characters": {
			request: application.UpdateProfileRequest{Name: "Grace", Currency: "TOOLONGXX"},
			want: map[string][]string{
				"Currency": {"The field Currency must be a string or array type with a maximum length of '8'."},
			},
		},
		"name of 201 characters": {
			request: application.UpdateProfileRequest{Name: strings.Repeat("a", 201), Currency: "EUR"},
			want: map[string][]string{
				"Name": {"The field Name must be a string or array type with a maximum length of '200'."},
			},
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			user := h.seed(t, "me@example.com")

			_, err := h.service.UpdateProfile(context.Background(), user.Id, testCase.request)

			requireFieldErrors(t, err, testCase.want)
			stored, _ := h.users.Get(user.Id)
			if stored.Name != "Owner" {
				t.Errorf("stored name = %q; a rejected request must not be written", stored.Name)
			}
		})
	}
}
