// Package config reads the process environment into a validated Config.
// Variable names and fallbacks are identical to the .NET API's, so a single
// deployment environment can drive either backend.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

const (
	// DatabaseURLVariable is the Postgres connection string, required at runtime.
	DatabaseURLVariable = "DATABASE_URL"
	// JWTSecretVariable holds the HS256 signing key shared with the .NET API.
	JWTSecretVariable = "JWT_SECRET"
	// PortVariable is the TCP port to listen on.
	PortVariable = "PORT"
	// AllowedOriginsVariable is a comma-separated CORS origin list.
	AllowedOriginsVariable = "ALLOWED_ORIGINS"

	// LocalDevelopmentSecret is the placeholder used only when JWT_SECRET is
	// absent. It must stay byte-identical to JwtOptions.LocalDevelopmentSecret
	// in the .NET API, otherwise locally issued tokens stop crossing backends.
	LocalDevelopmentSecret = "finance-tracker-local-development-signing-key-please-override"

	// DefaultAllowedOrigins matches CorsOrigins.Default.
	DefaultAllowedOrigins = "http://localhost:5173"

	// DefaultPort keeps the Go API off the .NET API's port during local parity runs.
	DefaultPort = 8081
)

// ErrDatabaseURLMissing is returned by Load when DATABASE_URL is unset or blank.
var ErrDatabaseURLMissing = errors.New(
	"config: " + DatabaseURLVariable + " is required (postgres:// connection string)")

// Config is the fully resolved runtime configuration.
type Config struct {
	DatabaseURL    string
	JWTSecret      string
	Port           int
	AllowedOrigins []string
}

// Load reads the environment. DATABASE_URL is required; everything else falls
// back to the same defaults the .NET API uses.
func Load() (Config, error) {
	databaseURL := strings.TrimSpace(os.Getenv(DatabaseURLVariable))
	if databaseURL == "" {
		return Config{}, ErrDatabaseURLMissing
	}

	secret := strings.TrimSpace(os.Getenv(JWTSecretVariable))
	if secret == "" {
		secret = LocalDevelopmentSecret
	}

	port := DefaultPort
	if raw := strings.TrimSpace(os.Getenv(PortVariable)); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 65535 {
			return Config{}, errors.New("config: " + PortVariable + " must be a TCP port number, got " + raw)
		}
		port = parsed
	}

	return Config{
		DatabaseURL:    databaseURL,
		JWTSecret:      secret,
		Port:           port,
		AllowedOrigins: ParseOrigins(os.Getenv(AllowedOriginsVariable)),
	}, nil
}

// ParseOrigins splits the comma-separated origin list, trimming blanks and
// falling back to the default when nothing usable is configured.
func ParseOrigins(raw string) []string {
	value := raw
	if strings.TrimSpace(value) == "" {
		value = DefaultAllowedOrigins
	}

	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return []string{DefaultAllowedOrigins}
	}
	return origins
}
