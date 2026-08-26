package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Strings the frontend surfaces verbatim (it renders the problem document's
// `detail`), so they are byte-identical to AuthService's in the .NET API.
const (
	// DefaultCurrency is the currency a new account starts with; it mirrors the
	// initialiser on FinanceTracker.Domain.User.Currency.
	DefaultCurrency = "USD"

	duplicateEmailMessage     = "An account with that email already exists."
	invalidCredentialsMessage = "Invalid email or password."

	// userEntityName feeds domain.NotFound, producing "User was not found.".
	userEntityName = "User"
)

// AuthService is registration, sign-in and profile maintenance. It is the Go
// twin of FinanceTracker.Application.Services.AuthService and holds every rule
// the endpoints enforce; the HTTP layer only binds and renders.
type AuthService struct {
	users  UserRepository
	tokens TokenService
	hasher PasswordHasher

	// now and newID are injectable so tests can assert on the stored row.
	now   func() time.Time
	newID func() uuid.UUID
}

// NewAuthService wires the service to its ports.
func NewAuthService(users UserRepository, tokens TokenService, hasher PasswordHasher) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
		hasher: hasher,
		now:    func() time.Time { return time.Now().UTC() },
		newID:  uuid.New,
	}
}

// WithClock returns a copy that stamps CreatedAt from now.
func (s *AuthService) WithClock(now func() time.Time) *AuthService {
	clone := *s
	clone.now = now
	return &clone
}

// WithIDs returns a copy that takes new user ids from newID.
func (s *AuthService) WithIDs(newID func() uuid.UUID) *AuthService {
	clone := *s
	clone.newID = newID
	return &clone
}

// Register creates an account and signs the caller straight in. The email is
// trimmed and lowercased before both the uniqueness check and storage, so
// "  Owner@Example.COM " and "owner@example.com" are the same account.
func (s *AuthService) Register(ctx context.Context, request RegisterRequest) (AuthResponse, error) {
	if err := request.Validate(); err != nil {
		return AuthResponse{}, err
	}

	email := normalizeEmail(request.Email)
	switch _, err := s.users.FindByEmail(ctx, email); {
	case err == nil:
		return AuthResponse{}, domain.Conflict(duplicateEmailMessage)
	case !errors.Is(err, ErrUserNotFound):
		return AuthResponse{}, fmt.Errorf("application: looking up user by email: %w", err)
	}

	hash, err := s.hasher.Hash(request.Password)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("application: hashing password: %w", err)
	}

	user := &domain.User{
		Id:           s.newID(),
		Email:        email,
		Name:         strings.TrimSpace(request.Name),
		PasswordHash: hash,
		Currency:     DefaultCurrency,
		CreatedAt:    s.now(),
	}

	if err := s.users.Add(ctx, user); err != nil {
		// The select above is racy on its own; the unique index is what actually
		// decides, and a loser reports the same conflict as the early check.
		if errors.Is(err, ErrEmailTaken) {
			return AuthResponse{}, domain.Conflict(duplicateEmailMessage)
		}
		return AuthResponse{}, fmt.Errorf("application: inserting user: %w", err)
	}

	return s.authResponse(user)
}

// Login verifies credentials and issues a token. An unknown address and a wrong
// password fail identically, so the response cannot be used to enumerate
// accounts. A password that still verifies against outdated hash parameters is
// re-hashed and persisted before the response goes out.
func (s *AuthService) Login(ctx context.Context, request LoginRequest) (AuthResponse, error) {
	if err := request.Validate(); err != nil {
		return AuthResponse{}, err
	}

	user, err := s.users.FindByEmail(ctx, normalizeEmail(request.Email))
	switch {
	case errors.Is(err, ErrUserNotFound):
		return AuthResponse{}, domain.Unauthorized(invalidCredentialsMessage)
	case err != nil:
		return AuthResponse{}, fmt.Errorf("application: looking up user by email: %w", err)
	}

	switch s.hasher.Verify(user.PasswordHash, request.Password) {
	case PasswordFailed:
		return AuthResponse{}, domain.Unauthorized(invalidCredentialsMessage)

	case PasswordSuccessRehashNeeded:
		hash, hashErr := s.hasher.Hash(request.Password)
		if hashErr != nil {
			return AuthResponse{}, fmt.Errorf("application: rehashing password: %w", hashErr)
		}
		if updateErr := s.users.UpdatePasswordHash(ctx, user.Id, hash); updateErr != nil {
			return AuthResponse{}, fmt.Errorf("application: persisting rehashed password: %w", updateErr)
		}
		user.PasswordHash = hash

	case PasswordSuccess:
	}

	return s.authResponse(user)
}

// Profile returns the caller's own row. A token whose subject no longer exists
// gets 404 "User was not found.", matching AuthService.LoadAsync.
func (s *AuthService) Profile(ctx context.Context, userID uuid.UUID) (UserDto, error) {
	user, err := s.load(ctx, userID)
	if err != nil {
		return UserDto{}, err
	}
	return NewUserDto(user), nil
}

// UpdateProfile renames the caller and sets their reporting currency. The name
// is trimmed and the currency upper-cased, as in the .NET service.
func (s *AuthService) UpdateProfile(
	ctx context.Context,
	userID uuid.UUID,
	request UpdateProfileRequest,
) (UserDto, error) {
	if err := request.Validate(); err != nil {
		return UserDto{}, err
	}

	user, err := s.load(ctx, userID)
	if err != nil {
		return UserDto{}, err
	}

	user.Name = strings.TrimSpace(request.Name)
	user.Currency = strings.ToUpper(strings.TrimSpace(request.Currency))

	if err := s.users.UpdateProfile(ctx, user.Id, user.Name, user.Currency); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return UserDto{}, domain.NotFound(userEntityName)
		}
		return UserDto{}, fmt.Errorf("application: updating profile: %w", err)
	}

	return NewUserDto(user), nil
}

// load fetches a user by id, turning a missing row into a 404.
func (s *AuthService) load(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	switch {
	case errors.Is(err, ErrUserNotFound):
		return nil, domain.NotFound(userEntityName)
	case err != nil:
		return nil, fmt.Errorf("application: looking up user by id: %w", err)
	}
	return user, nil
}

// authResponse mints the token that accompanies a user on register and login.
func (s *AuthService) authResponse(user *domain.User) (AuthResponse, error) {
	token, err := s.tokens.Issue(user.Id, user.Email)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("application: issuing token: %w", err)
	}
	return AuthResponse{Token: token, User: NewUserDto(user)}, nil
}

// normalizeEmail is AuthService.Normalize: trim, then lowercase.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
