// Package apptest holds in-memory doubles for the application ports so a test
// can exercise a service, or the whole router, without a database. Nothing
// outside tests should import it.
package apptest

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Users is an in-memory application.UserRepository. Every method hands out a
// copy of the stored row, so a caller that mutates what it read cannot change
// the "database" without going back through the repository — which is what
// makes assertions about persistence meaningful.
type Users struct {
	mu   sync.Mutex
	rows map[uuid.UUID]domain.User

	// FailWith, when set, is returned by every method. It stands in for a
	// database that is down.
	FailWith error
	// AddFailsWith, when set, is returned by Add instead of inserting. Set it to
	// application.ErrEmailTaken to simulate losing the unique-index race.
	AddFailsWith error
}

// NewUsers returns an empty repository.
func NewUsers() *Users {
	return &Users{rows: make(map[uuid.UUID]domain.User)}
}

var _ application.UserRepository = (*Users)(nil)

// Seed inserts a user directly, bypassing the port.
func (u *Users) Seed(user domain.User) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.rows[user.Id] = user
}

// Get returns a stored row by id, as the test sees it after the service ran.
func (u *Users) Get(id uuid.UUID) (domain.User, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	user, ok := u.rows[id]
	return user, ok
}

// Count reports how many users are stored.
func (u *Users) Count() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.rows)
}

// FindByEmail matches on the exact stored address, as the SQL repository does:
// normalisation is the service's job.
func (u *Users) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.FailWith != nil {
		return nil, u.FailWith
	}

	for _, user := range u.rows {
		if user.Email == email {
			found := user
			return &found, nil
		}
	}
	return nil, application.ErrUserNotFound
}

// FindByID looks a user up by primary key.
func (u *Users) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.FailWith != nil {
		return nil, u.FailWith
	}

	user, ok := u.rows[id]
	if !ok {
		return nil, application.ErrUserNotFound
	}
	return &user, nil
}

// Add inserts, enforcing the unique email index in memory.
func (u *Users) Add(_ context.Context, user *domain.User) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.FailWith != nil {
		return u.FailWith
	}
	if u.AddFailsWith != nil {
		return u.AddFailsWith
	}

	for _, existing := range u.rows {
		if existing.Email == user.Email {
			return application.ErrEmailTaken
		}
	}
	u.rows[user.Id] = *user
	return nil
}

// UpdatePasswordHash replaces a stored hash.
func (u *Users) UpdatePasswordHash(_ context.Context, id uuid.UUID, hash string) error {
	return u.update(id, func(user *domain.User) { user.PasswordHash = hash })
}

// UpdateProfile writes the editable profile fields.
func (u *Users) UpdateProfile(_ context.Context, id uuid.UUID, name, currency string) error {
	return u.update(id, func(user *domain.User) {
		user.Name = name
		user.Currency = currency
	})
}

func (u *Users) update(id uuid.UUID, apply func(*domain.User)) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.FailWith != nil {
		return u.FailWith
	}

	user, ok := u.rows[id]
	if !ok {
		return application.ErrUserNotFound
	}
	apply(&user)
	u.rows[id] = user
	return nil
}

// Hasher is a password hasher that skips the key derivation: a hash is the
// string "<prefix>:<password>", so tests stay fast and can force any outcome.
type Hasher struct {
	// Prefix distinguishes hashes written before and after a rehash. It must not
	// contain a colon.
	Prefix string
	// Outcome is what Verify reports for a matching password.
	Outcome application.PasswordVerificationOutcome
	// FailWith, when set, is returned by Hash.
	FailWith error

	mu     sync.Mutex
	hashes int
}

// NewHasher returns a hasher that verifies cleanly.
func NewHasher() *Hasher {
	return &Hasher{Prefix: "hashed", Outcome: application.PasswordSuccess}
}

var _ application.PasswordHasher = (*Hasher)(nil)

// Hash returns "<prefix>:<password>".
func (h *Hasher) Hash(password string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.FailWith != nil {
		return "", h.FailWith
	}
	h.hashes++
	return h.Prefix + ":" + password, nil
}

// Hashes reports how many times Hash was called.
func (h *Hasher) Hashes() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hashes
}

// Verify reports Outcome when the blob carries this password (under any
// prefix) and PasswordFailed otherwise.
func (h *Hasher) Verify(hash, password string) application.PasswordVerificationOutcome {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, stored, ok := strings.Cut(hash, ":")
	if !ok || stored != password {
		return application.PasswordFailed
	}
	return h.Outcome
}
