// Package jobs holds the background work the API host runs beside the HTTP
// server. It is the Go counterpart of FinanceTracker.Infrastructure.BackgroundJobs.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/pharaujo/finance/backend-go/internal/infrastructure/persistence"
)

// PassLockKey is the Postgres advisory lock both backends take before a
// materialization pass. It is a single arbitrary constant shared by the .NET
// worker and this one, which is what stops two hosts — or the two backends
// deployed side by side — from writing the same occurrence twice.
const PassLockKey int64 = 723441001

// Interval is how often a pass runs after the one at startup.
const Interval = 6 * time.Hour

// Materializer is the slice of RecurringService the worker drives.
type Materializer interface {
	MaterializeDue(ctx context.Context, now time.Time) (int, error)
}

// Factory builds a materializer bound to one transaction, so the repositories
// the service writes through enlist in the transaction that holds the lock.
type Factory func(db persistence.Querier) Materializer

// Beginner is the pool's ability to open a transaction.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// RecurringWorker materializes due recurring rules at startup and then every
// six hours. A failed pass is logged and dropped: the next tick retries, and
// the host must survive either way.
type RecurringWorker struct {
	db         Beginner
	newService Factory
	logger     *slog.Logger

	interval time.Duration
	now      func() time.Time
}

// NewRecurringWorker wires a worker to a pool and a service factory.
func NewRecurringWorker(db Beginner, newService Factory, logger *slog.Logger) *RecurringWorker {
	return &RecurringWorker{
		db:         db,
		newService: newService,
		logger:     logger,
		interval:   Interval,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// WithInterval returns a copy that ticks at a different period; tests use it to
// avoid waiting six hours.
func (w *RecurringWorker) WithInterval(interval time.Duration) *RecurringWorker {
	clone := *w
	clone.interval = interval
	return &clone
}

// WithClock returns a copy that reads the cutoff from now.
func (w *RecurringWorker) WithClock(now func() time.Time) *RecurringWorker {
	clone := *w
	clone.now = now
	return &clone
}

// Run passes once immediately and then on every tick, returning when the
// context is cancelled.
func (w *RecurringWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		w.runOnceLogged(ctx)

		select {
		case <-ctx.Done():
			w.logger.Info("recurring worker stopping after host shutdown")
			return
		case <-ticker.C:
		}
	}
}

// runOnceLogged swallows a failed pass so the host survives it.
func (w *RecurringWorker) runOnceLogged(ctx context.Context) {
	created, acquired, err := w.RunOnce(ctx)
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return
	case err != nil:
		w.logger.Error("recurring transaction pass failed", slog.String("error", err.Error()))
	case !acquired:
		w.logger.Info("recurring pass skipped: another instance holds the lock")
	default:
		w.logger.Info("recurring pass finished", slog.Int("created", created))
	}
}

// RunOnce takes the advisory lock and runs one pass inside the transaction that
// holds it. The second result is false when another instance had the lock, in
// which case nothing was read or written.
func (w *RecurringWorker) RunOnce(ctx context.Context) (int, bool, error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("jobs: opening the pass transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The lock is held until the transaction ends, so it covers the whole pass
	// and is released by the commit or the rollback, even if this process dies.
	var acquired bool
	if err = tx.QueryRow(ctx,
		`SELECT pg_try_advisory_xact_lock($1)`, PassLockKey).Scan(&acquired); err != nil {
		return 0, false, fmt.Errorf("jobs: taking the pass lock: %w", err)
	}
	if !acquired {
		return 0, false, nil
	}

	created, err := w.newService(tx).MaterializeDue(ctx, w.now())
	if err != nil {
		return 0, true, fmt.Errorf("jobs: materializing recurring rules: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, true, fmt.Errorf("jobs: committing the pass: %w", err)
	}
	return created, true, nil
}
