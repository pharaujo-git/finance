package identity_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/fixtures"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
)

// atIssue pins the clock a minute after the fixture token was minted, so the
// test never depends on when it runs.
func atIssue(fixture fixtures.DotNetIdentity) func() time.Time {
	moment := time.Unix(fixture.IssuedAtUnix+60, 0).UTC()
	return func() time.Time { return moment }
}

func claimsOf(t *testing.T, token string) jwt.MapClaims {
	t.Helper()

	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		t.Fatalf("parsing token payload: %v", err)
	}
	return claims
}

func headerOf(t *testing.T, token string) map[string]any {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}
	raw, err := jwt.NewParser().DecodeSegment(parts[0])
	if err != nil {
		t.Fatalf("decoding header: %v", err)
	}

	var header map[string]any
	if err := json.Unmarshal(raw, &header); err != nil {
		t.Fatalf("unmarshalling header: %v", err)
	}
	return header
}

func keysOf(claims jwt.MapClaims) []string {
	keys := make([]string, 0, len(claims))
	for key := range claims {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestValidateDotNetToken is the parity check: a token the .NET API issued,
// signed with the shared dev-fallback secret, must pass the exact validation
// path the Go middleware uses.
func TestValidateDotNetToken(t *testing.T) {
	t.Parallel()

	fixture := fixtures.MustLoadDotNetIdentity()
	tokens := identity.NewTokenService(config.LocalDevelopmentSecret).WithClock(atIssue(fixture))

	principal, err := tokens.Validate(fixture.Token)
	if err != nil {
		t.Fatalf("Validate(.NET token): %v", err)
	}
	if principal.UserID.String() != fixture.UserID {
		t.Errorf("sub = %q, want %q", principal.UserID, fixture.UserID)
	}
	if principal.Email != fixture.Email {
		t.Errorf("email = %q, want %q", principal.Email, fixture.Email)
	}

	claims := claimsOf(t, fixture.Token)
	if got := claims["iss"]; got != identity.Issuer {
		t.Errorf("iss = %v, want %q", got, identity.Issuer)
	}
	if got := claims["aud"]; got != identity.Audience {
		t.Errorf("aud = %v, want %q", got, identity.Audience)
	}
	if got := headerOf(t, fixture.Token)["alg"]; got != "HS256" {
		t.Errorf("alg = %v, want HS256", got)
	}
	if lifetime := fixture.ExpiresAtUnix - fixture.IssuedAtUnix; lifetime != int64(identity.TokenLifetime.Seconds()) {
		t.Errorf("lifetime = %ds, want %ds", lifetime, int64(identity.TokenLifetime.Seconds()))
	}
}

func TestValidateDotNetTokenRejections(t *testing.T) {
	t.Parallel()

	fixture := fixtures.MustLoadDotNetIdentity()

	t.Run("expired", func(t *testing.T) {
		t.Parallel()
		expired := time.Unix(fixture.ExpiresAtUnix+5*60, 0).UTC()
		tokens := identity.NewTokenService(config.LocalDevelopmentSecret).
			WithClock(func() time.Time { return expired })
		if _, err := tokens.Validate(fixture.Token); err == nil {
			t.Fatal("expired token accepted")
		}
	})

	t.Run("before nbf", func(t *testing.T) {
		t.Parallel()
		early := time.Unix(fixture.IssuedAtUnix-10*60, 0).UTC()
		tokens := identity.NewTokenService(config.LocalDevelopmentSecret).
			WithClock(func() time.Time { return early })
		if _, err := tokens.Validate(fixture.Token); err == nil {
			t.Fatal("not-yet-valid token accepted")
		}
	})

	t.Run("within leeway", func(t *testing.T) {
		t.Parallel()
		// 30s past expiry is inside the one-minute skew both backends allow.
		grace := time.Unix(fixture.ExpiresAtUnix+30, 0).UTC()
		tokens := identity.NewTokenService(config.LocalDevelopmentSecret).
			WithClock(func() time.Time { return grace })
		if _, err := tokens.Validate(fixture.Token); err != nil {
			t.Fatalf("token 30s past expiry rejected despite %v leeway: %v", identity.ClockLeeway, err)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		t.Parallel()
		tokens := identity.NewTokenService("a-different-secret").WithClock(atIssue(fixture))
		if _, err := tokens.Validate(fixture.Token); err == nil {
			t.Fatal("token accepted under the wrong signing key")
		}
	})

	t.Run("tampered signature", func(t *testing.T) {
		t.Parallel()
		parts := strings.Split(fixture.Token, ".")
		tampered := parts[0] + "." + parts[1] + "." + strings.Repeat("A", len(parts[2]))
		tokens := identity.NewTokenService(config.LocalDevelopmentSecret).WithClock(atIssue(fixture))
		if _, err := tokens.Validate(tampered); err == nil {
			t.Fatal("token with a rewritten signature accepted")
		}
	})

	t.Run("alg none", func(t *testing.T) {
		t.Parallel()
		unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"iss": identity.Issuer,
			"aud": identity.Audience,
			"sub": fixture.UserID,
			"exp": fixture.ExpiresAtUnix,
			"nbf": fixture.IssuedAtUnix,
			"iat": fixture.IssuedAtUnix,
		}).SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("building alg=none token: %v", err)
		}
		tokens := identity.NewTokenService(config.LocalDevelopmentSecret).WithClock(atIssue(fixture))
		if _, err := tokens.Validate(unsigned); err == nil {
			t.Fatal("alg=none token accepted")
		}
	})

	t.Run("garbage", func(t *testing.T) {
		t.Parallel()
		tokens := identity.NewTokenService(config.LocalDevelopmentSecret)
		for _, raw := range []string{"", "not-a-token", "a.b.c"} {
			if _, err := tokens.Validate(raw); err == nil {
				t.Fatalf("Validate(%q) accepted", raw)
			}
		}
	})
}

// TestGoTokenRoundTrip covers Go-token -> Go-validate.
func TestGoTokenRoundTrip(t *testing.T) {
	t.Parallel()

	tokens := identity.NewTokenService(config.LocalDevelopmentSecret)
	userID := uuid.New()

	token, err := tokens.Issue(userID, "round@trip.test")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	principal, err := tokens.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if principal.UserID != userID {
		t.Errorf("sub = %v, want %v", principal.UserID, userID)
	}
	if principal.Email != "round@trip.test" {
		t.Errorf("email = %q, want round@trip.test", principal.Email)
	}

	claims := claimsOf(t, token)
	if got, ok := claims["sub"].(string); !ok || got != strings.ToLower(got) {
		t.Errorf("sub = %v, want a lowercase uuid string", claims["sub"])
	}
	if _, err := uuid.Parse(claims["jti"].(string)); err != nil {
		t.Errorf("jti is not a uuid: %v", claims["jti"])
	}

	// Two tokens for the same user differ: jti is fresh each time.
	second, err := tokens.Issue(userID, "round@trip.test")
	if err != nil {
		t.Fatalf("Issue (second): %v", err)
	}
	if claimsOf(t, second)["jti"] == claims["jti"] {
		t.Error("jti repeated across two tokens")
	}
}

// TestGoTokenMatchesDotNetStructure compares a Go-issued token with the
// captured .NET one: same header algorithm, same claim names, same iss/aud,
// same lifetime.
func TestGoTokenMatchesDotNetStructure(t *testing.T) {
	t.Parallel()

	fixture := fixtures.MustLoadDotNetIdentity()
	issuedAt := time.Unix(fixture.IssuedAtUnix, 0).UTC()
	tokens := identity.NewTokenService(config.LocalDevelopmentSecret).
		WithClock(func() time.Time { return issuedAt })

	userID := uuid.MustParse(fixture.UserID)
	goToken, err := tokens.Issue(userID, fixture.Email)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	netClaims := claimsOf(t, fixture.Token)
	goClaims := claimsOf(t, goToken)

	netKeys, goKeys := keysOf(netClaims), keysOf(goClaims)
	if strings.Join(netKeys, ",") != strings.Join(goKeys, ",") {
		t.Fatalf("claim names differ: go %v, .NET %v", goKeys, netKeys)
	}

	for _, key := range []string{"iss", "aud", "sub", "email", "exp", "iat", "nbf"} {
		netValue, goValue := netClaims[key], goClaims[key]
		if netNum, ok := netValue.(float64); ok {
			goNum, ok := goValue.(float64)
			if !ok || netNum != goNum {
				t.Errorf("claim %q: go %v, .NET %v", key, goValue, netValue)
			}
			continue
		}
		if netValue != goValue {
			t.Errorf("claim %q: go %v, .NET %v", key, goValue, netValue)
		}
	}

	if netAlg, goAlg := headerOf(t, fixture.Token)["alg"], headerOf(t, goToken)["alg"]; netAlg != goAlg {
		t.Errorf("alg: go %v, .NET %v", goAlg, netAlg)
	}

	// The .NET-issued token and the Go-issued token validate under the same key.
	if _, err := tokens.Validate(goToken); err != nil {
		t.Fatalf("Validate(go token): %v", err)
	}
	if _, err := tokens.Validate(fixture.Token); err != nil {
		t.Fatalf("Validate(.NET token): %v", err)
	}
}
