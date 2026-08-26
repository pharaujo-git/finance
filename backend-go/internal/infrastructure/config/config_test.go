package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv(config.DatabaseURLVariable, "")

	if _, err := config.Load(); !errors.Is(err, config.ErrDatabaseURLMissing) {
		t.Fatalf("err = %v, want ErrDatabaseURLMissing", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv(config.DatabaseURLVariable, "postgres://finance:postgres@localhost:5432/postgres?sslmode=disable")
	t.Setenv(config.JWTSecretVariable, "")
	t.Setenv(config.PortVariable, "")
	t.Setenv(config.AllowedOriginsVariable, "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// This fallback must stay byte-identical to JwtOptions.LocalDevelopmentSecret
	// in backend/FinanceTracker.Infrastructure/Identity/JwtOptions.cs.
	if cfg.JWTSecret != "finance-tracker-local-development-signing-key-please-override" {
		t.Errorf("JWTSecret = %q, want the .NET dev fallback", cfg.JWTSecret)
	}
	if cfg.Port != 8081 {
		t.Errorf("Port = %d, want 8081", cfg.Port)
	}
	if strings.Join(cfg.AllowedOrigins, ",") != "http://localhost:5173" {
		t.Errorf("AllowedOrigins = %v, want [http://localhost:5173]", cfg.AllowedOrigins)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv(config.DatabaseURLVariable, "postgres://user:pass@db.neon.tech/finance?sslmode=require")
	t.Setenv(config.JWTSecretVariable, "  production-secret  ")
	t.Setenv(config.PortVariable, "10000")
	t.Setenv(config.AllowedOriginsVariable, " https://a.example , https://b.example ,")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.JWTSecret != "production-secret" {
		t.Errorf("JWTSecret = %q, want the trimmed override", cfg.JWTSecret)
	}
	if cfg.Port != 10000 {
		t.Errorf("Port = %d, want 10000", cfg.Port)
	}
	if want := "https://a.example,https://b.example"; strings.Join(cfg.AllowedOrigins, ",") != want {
		t.Errorf("AllowedOrigins = %v, want %q", cfg.AllowedOrigins, want)
	}
}

func TestLoadRejectsBadPort(t *testing.T) {
	t.Setenv(config.DatabaseURLVariable, "postgres://localhost/finance")

	for _, port := range []string{"not-a-number", "0", "70000", "-1"} {
		t.Run(port, func(t *testing.T) {
			t.Setenv(config.PortVariable, port)
			if _, err := config.Load(); err == nil {
				t.Fatalf("PORT=%q accepted", port)
			}
		})
	}
}
