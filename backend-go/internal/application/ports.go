// Package application holds the use-case layer: the ports the outer layers
// implement and (from later phases on) the services that orchestrate them.
package application

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// PasswordVerificationOutcome is the result of checking a password against a
// stored hash. It mirrors FinanceTracker's PasswordVerificationOutcome so both
// backends can agree on when a stored hash is stale.
type PasswordVerificationOutcome int

const (
	// PasswordFailed means the password did not match, or the blob was unusable.
	PasswordFailed PasswordVerificationOutcome = iota
	// PasswordSuccess means the password matched a hash with current parameters.
	PasswordSuccess
	// PasswordSuccessRehashNeeded means the password matched, but the stored hash
	// uses weaker parameters than the current defaults and should be replaced.
	PasswordSuccessRehashNeeded
)

// PasswordHasher produces and checks ASP.NET Core Identity v3 password blobs.
// The .NET port takes the User as its first argument (Identity's hasher ignores
// it), so it is omitted here.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) PasswordVerificationOutcome
}

// Principal is the identity carried by a validated bearer token.
type Principal struct {
	UserID uuid.UUID
	Email  string
}

// TokenService issues and validates the bearer tokens both backends accept.
type TokenService interface {
	Issue(userID uuid.UUID, email string) (string, error)
	Validate(token string) (Principal, error)
}

// Sentinels every UserRepository implementation reports; the services turn them
// into the AppErrors the transport renders.
var (
	// ErrUserNotFound means no row matched the lookup.
	ErrUserNotFound = errors.New("application: user not found")
	// ErrEmailTaken means an insert lost the race for an email address. The
	// unique index IX_Users_Email is what detects it.
	ErrEmailTaken = errors.New("application: email already registered")
)

// UserRepository is the persistence port for the "Users" table. Lookups report
// ErrUserNotFound rather than a nil user so a missing row cannot be mistaken
// for a successful read; the update methods report it too when the id is gone.
type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Add(ctx context.Context, user *domain.User) error
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error
	UpdateProfile(ctx context.Context, id uuid.UUID, name, currency string) error
}
