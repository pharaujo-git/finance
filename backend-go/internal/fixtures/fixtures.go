// Package fixtures embeds artefacts captured from a running .NET API so the Go
// tests can prove cross-backend identity compatibility without a network call
// or a second toolchain. Nothing outside tests should import it.
//
// The JSON was produced by running backend/FinanceTracker.Api in SQLite mode
// with neither DATABASE_URL nor JWT_SECRET set (so the dev-fallback signing key
// applies), registering a user over POST /api/auth/register, and reading the
// stored PasswordHash back out of finance.db with the sqlite3 CLI.
package fixtures

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed dotnet_identity.json
var dotnetIdentityJSON []byte

// ProblemSamples are verbatim problem+json bodies the .NET API emitted.
type ProblemSamples struct {
	Conflict     string `json:"conflict"`
	Unauthorized string `json:"unauthorized"`
}

// DotNetIdentity is one registration captured from the .NET API.
type DotNetIdentity struct {
	GeneratedAt   string         `json:"generatedAt"`
	Source        string         `json:"source"`
	Password      string         `json:"password"`
	PasswordHash  string         `json:"passwordHash"`
	Token         string         `json:"token"`
	UserID        string         `json:"userId"`
	Email         string         `json:"email"`
	Name          string         `json:"name"`
	IssuedAtUnix  int64          `json:"issuedAtUnix"`
	ExpiresAtUnix int64          `json:"expiresAtUnix"`
	ProblemJSON   ProblemSamples `json:"problemJson"`
}

// LoadDotNetIdentity decodes the embedded fixture.
func LoadDotNetIdentity() (DotNetIdentity, error) {
	var fixture DotNetIdentity
	if err := json.Unmarshal(dotnetIdentityJSON, &fixture); err != nil {
		return DotNetIdentity{}, fmt.Errorf("fixtures: decoding dotnet_identity.json: %w", err)
	}
	return fixture, nil
}

// MustLoadDotNetIdentity is LoadDotNetIdentity for tests.
func MustLoadDotNetIdentity() DotNetIdentity {
	fixture, err := LoadDotNetIdentity()
	if err != nil {
		panic(err)
	}
	return fixture
}
