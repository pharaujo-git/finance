package jobs_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/jobs"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/persistence"
	"github.com/pharaujo/finance/backend-go/internal/pgtest"
)

// cutoff is the instant every pass in this file runs at.
var cutoff = time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newWorker builds a worker whose service is bound to whatever transaction the
// pass opens, which is what makes the whole pass atomic.
func newWorker(pool *pgxpool.Pool) *jobs.RecurringWorker {
	return jobs.NewRecurringWorker(pool, func(db persistence.Querier) jobs.Materializer {
		return application.NewRecurringService(
			persistence.NewRecurringRepository(db),
			persistence.NewTransactionRepository(db),
			persistence.NewAccountRepository(db),
			application.NewCategoryService(persistence.NewCategoryRepository(db)),
		)
	}, quietLogger()).WithClock(func() time.Time { return cutoff })
}

// seedDailyRule inserts a user, an account and a rule that is due for a known
// number of occurrences at the cutoff.
func seedDailyRule(t *testing.T, pool *pgxpool.Pool, occurrences int) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	userID := uuid.New()

	if err := persistence.NewUserRepository(pool).Add(ctx, &domain.User{
		Id: userID, Email: uuid.NewString() + "@example.com", Name: "Owner",
		PasswordHash: "hash", Currency: "USD", CreatedAt: cutoff,
	}); err != nil {
		t.Fatalf("seeding a user: %v", err)
	}

	account := &domain.Account{
		Id: uuid.New(), UserId: userID, Name: "Checking", Type: domain.AccountChecking,
		InitialBalance: decimal.RequireFromString("0.00"), Currency: "USD", CreatedAt: cutoff,
	}
	if err := persistence.NewAccountRepository(pool).Add(ctx, account); err != nil {
		t.Fatalf("seeding an account: %v", err)
	}

	start := cutoff.AddDate(0, 0, -(occurrences - 1)).Truncate(24 * time.Hour)
	if err := persistence.NewRecurringRepository(pool).Add(ctx, &domain.RecurringRule{
		Id: uuid.New(), UserId: userID, AccountId: account.Id,
		Type: domain.TransactionExpense, Amount: decimal.RequireFromString("10.00"),
		Description: "Daily", Frequency: domain.FrequencyDaily,
		StartDate: start, NextRunDate: start, IsActive: true,
	}); err != nil {
		t.Fatalf("seeding a rule: %v", err)
	}
	return userID
}

func countTransactions(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) int {
	t.Helper()

	var total int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM "Transactions" WHERE "UserId" = $1`, userID).Scan(&total); err != nil {
		t.Fatalf("counting transactions: %v", err)
	}
	return total
}

// The tests below deliberately do not call t.Parallel: a Postgres advisory lock
// is scoped to the database, not to a schema, so two passes running at once
// would contend for the very lock under test.
func TestRunOnceMaterializesAndCommits(t *testing.T) {
	pool := pgtest.NewPool(t)
	userID := seedDailyRule(t, pool, 3)

	created, acquired, err := newWorker(pool).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !acquired {
		t.Fatal("the first pass did not take the lock")
	}
	if created != 3 {
		t.Errorf("created = %d, want 3", created)
	}
	if got := countTransactions(t, pool, userID); got != 3 {
		t.Errorf("stored transactions = %d, want 3", got)
	}

	// Nothing is due any more, so a second pass writes nothing.
	again, _, err := newWorker(pool).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if again != 0 {
		t.Errorf("second pass created %d, want 0", again)
	}
}

// A pass that cannot take the advisory lock skips silently rather than waiting
// or duplicating work.
func TestRunOnceSkipsWhenTheLockIsHeld(t *testing.T) {
	pool := pgtest.NewPool(t)
	userID := seedDailyRule(t, pool, 2)
	ctx := context.Background()

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("opening the holding transaction: %v", err)
	}
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, jobs.PassLockKey); err != nil {
		t.Fatalf("taking the lock: %v", err)
	}

	created, acquired, err := newWorker(pool).RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if acquired {
		t.Error("the pass took a lock another session holds")
	}
	if created != 0 || countTransactions(t, pool, userID) != 0 {
		t.Errorf("a skipped pass wrote %d transactions", countTransactions(t, pool, userID))
	}

	// Releasing the lock lets the next pass through.
	if err := holder.Rollback(ctx); err != nil {
		t.Fatalf("releasing the lock: %v", err)
	}

	created, acquired, err = newWorker(pool).RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !acquired || created != 2 {
		t.Errorf("pass after the release = %d created, acquired %v; want 2 and true", created, acquired)
	}
}

// Two hosts passing at the same moment must not double-write: the lock makes
// one of them skip, and the total is the same as a single pass.
func TestConcurrentPassesDoNotDuplicate(t *testing.T) {
	pool := pgtest.NewPool(t)
	const occurrences = 5
	userID := seedDailyRule(t, pool, occurrences)

	var (
		wait      sync.WaitGroup
		mutex     sync.Mutex
		total     int
		acquiring int
		failures  []error
	)

	start := make(chan struct{})
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start

			created, acquired, err := newWorker(pool).RunOnce(context.Background())

			mutex.Lock()
			defer mutex.Unlock()
			total += created
			if acquired {
				acquiring++
			}
			if err != nil {
				failures = append(failures, err)
			}
		}()
	}

	close(start)
	wait.Wait()

	for _, err := range failures {
		t.Errorf("a concurrent pass failed: %v", err)
	}
	if total != occurrences {
		t.Errorf("created = %d across both passes, want %d", total, occurrences)
	}
	if got := countTransactions(t, pool, userID); got != occurrences {
		t.Errorf("stored transactions = %d, want %d with no duplicates", got, occurrences)
	}
	if acquiring == 0 {
		t.Error("neither pass took the lock")
	}
}

// Run passes immediately, keeps ticking, and returns when its context ends.
func TestRunStopsWithItsContext(t *testing.T) {
	pool := pgtest.NewPool(t)
	userID := seedDailyRule(t, pool, 1)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		newWorker(pool).WithInterval(10 * time.Millisecond).Run(ctx)
	}()

	deadline := time.After(5 * time.Second)
	for countTransactions(t, pool, userID) == 0 {
		select {
		case <-deadline:
			t.Fatal("the worker never ran its first pass")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}
