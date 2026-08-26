// Package persistence owns the Postgres connection pool shared by the
// repositories added in later phases.
package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool settings sized for a small PaaS instance sharing a Neon database with
// the .NET API.
const (
	maxConns          = 10
	minConns          = 0
	maxConnLifetime   = time.Hour
	maxConnIdleTime   = 5 * time.Minute
	healthCheckPeriod = time.Minute
	connectTimeout    = 10 * time.Second
)

// NewPool parses a postgres:// URL and opens a pool. Query parameters in the
// URL (sslmode, application_name, ...) are honoured by pgx's own parser, so a
// Neon connection string works unchanged.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("persistence: parsing %s: %w", "DATABASE_URL", err)
	}

	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime
	cfg.HealthCheckPeriod = healthCheckPeriod
	cfg.ConnConfig.ConnectTimeout = connectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("persistence: creating pool: %w", err)
	}
	return pool, nil
}

// Ping opens one connection so a misconfigured database fails at startup
// rather than on the first request.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("persistence: pinging database: %w", err)
	}
	return nil
}
