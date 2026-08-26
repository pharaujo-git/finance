// Package pgtest gives an integration test a throwaway Postgres schema built
// from the real baseline migration. Nothing outside tests should import it.
//
//	docker run --rm -d -p 5433:5432 -e POSTGRES_HOST_AUTH_METHOD=trust \
//	  -e POSTGRES_USER=finance -e POSTGRES_DB=finance --name finance-pg postgres:16
//	TEST_DATABASE_URL=postgres://finance@localhost:5433/finance?sslmode=disable \
//	  go test ./...
//
// Trust auth keeps a password out of the connection string, matching how the
// CI service container is configured.
//
// Port 5433 rather than 5432 because a Homebrew Postgres commonly owns 5432 on
// a developer machine (see db/README.md). Without the variable every test that
// calls NewPool skips, so `go test ./...` stays green with no Docker.
package pgtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseURLVariable names the connection string these tests run against. It
// is deliberately not DATABASE_URL: pointing the suite at a developer's real
// database by accident would be expensive, and this one only ever exists for
// the duration of a container.
const DatabaseURLVariable = "TEST_DATABASE_URL"

// cleanupTimeout bounds the drop of a schema after a test.
const cleanupTimeout = 30 * time.Second

// NewPool gives each test its own schema, created from db/migrations'
// baseline and dropped afterwards, so tests neither collide nor leave anything
// behind. The schema is put on the connection's search_path, which is why the
// repositories' unqualified "Users" resolves to it.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv(DatabaseURLVariable))
	if databaseURL == "" {
		t.Skipf("%s is not set; skipping the Postgres integration test", DatabaseURLVariable)
	}

	ctx := context.Background()
	schema := "gotest_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24]

	admin, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connecting to %s: %v", DatabaseURLVariable, err)
	}
	defer func() {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Logf("closing admin connection: %v", closeErr)
		}
	}()

	if _, err = admin.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %q", schema)); err != nil {
		t.Fatalf("creating schema: %v", err)
	}
	t.Cleanup(func() { dropSchema(t, databaseURL, schema) })

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parsing %s: %v", DatabaseURLVariable, err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	// The baseline is one script of many statements, which only the simple
	// protocol accepts in a single round trip.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, BaselineDDL(t)); err != nil {
		t.Fatalf("applying the baseline migration: %v", err)
	}
	return pool
}

// BaselineDDL returns everything between the dbmate markers of the baseline
// migration, which is the DDL the real database is built from.
func BaselineDDL(t *testing.T) string {
	t.Helper()

	path := filepath.Join(repositoryRoot(t), "db", "migrations", "0001_baseline.sql")
	raw, err := os.ReadFile(path) // #nosec G304 -- the path is derived from this file's own location.
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	sql := string(raw)
	if _, after, ok := strings.Cut(sql, "-- migrate:up"); ok {
		sql = after
	}
	if before, _, ok := strings.Cut(sql, "-- migrate:down"); ok {
		sql = before
	}
	return sql
}

// repositoryRoot walks up from this source file, so a test finds the migration
// whatever package it runs from.
func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the pgtest source file")
	}
	// .../backend-go/internal/pgtest/pgtest.go -> the repository root.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func dropSchema(t *testing.T, databaseURL, schema string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Logf("dropping schema %s: %v", schema, err)
		return
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, fmt.Sprintf("DROP SCHEMA %q CASCADE", schema)); err != nil {
		t.Logf("dropping schema %s: %v", schema, err)
	}
}
