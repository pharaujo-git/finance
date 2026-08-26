package identity_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"testing"

	"golang.org/x/crypto/pbkdf2"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/fixtures"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
)

// blobHeader is the parsed Identity v3 preamble.
type blobHeader struct {
	Marker     byte
	PRF        uint32
	Iterations uint32
	SaltLen    uint32
	SubkeyLen  int
}

func parseHeader(t *testing.T, encoded string) blobHeader {
	t.Helper()

	blob, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("hash is not base64: %v", err)
	}
	if len(blob) < 13 {
		t.Fatalf("hash too short: %d bytes", len(blob))
	}

	header := blobHeader{
		Marker:     blob[0],
		PRF:        binary.BigEndian.Uint32(blob[1:5]),
		Iterations: binary.BigEndian.Uint32(blob[5:9]),
		SaltLen:    binary.BigEndian.Uint32(blob[9:13]),
	}
	header.SubkeyLen = len(blob) - 13 - int(header.SaltLen)
	return header
}

// TestDotNetFixtureHeader documents the parameters the .NET API actually wrote.
// Observed on 2026-08-26 from .NET 10 / Microsoft.AspNetCore.Identity:
// marker 0x01, PRF 2 (HMACSHA512), 100000 iterations, 16-byte salt, 32-byte
// subkey — identical to this package's defaults, so Verify must report
// Success rather than SuccessRehashNeeded.
func TestDotNetFixtureHeader(t *testing.T) {
	t.Parallel()

	header := parseHeader(t, fixtures.MustLoadDotNetIdentity().PasswordHash)

	if header.Marker != 0x01 {
		t.Errorf("marker = %#x, want 0x01", header.Marker)
	}
	if header.PRF != 2 {
		t.Errorf("prf = %d, want 2 (HMACSHA512)", header.PRF)
	}
	if header.Iterations != 100_000 {
		t.Errorf("iterations = %d, want 100000", header.Iterations)
	}
	if header.SaltLen != 16 {
		t.Errorf("salt length = %d, want 16", header.SaltLen)
	}
	if header.SubkeyLen != 32 {
		t.Errorf("subkey length = %d, want 32", header.SubkeyLen)
	}
}

// TestVerifyDotNetHash is the parity check: a hash the .NET API stored must be
// accepted by the Go hasher for the same password.
func TestVerifyDotNetHash(t *testing.T) {
	t.Parallel()

	fixture := fixtures.MustLoadDotNetIdentity()
	hasher := identity.NewPasswordHasher()

	if got := hasher.Verify(fixture.PasswordHash, fixture.Password); got != application.PasswordSuccess {
		t.Fatalf("Verify(.NET hash, correct password) = %v, want PasswordSuccess", got)
	}
	if got := hasher.Verify(fixture.PasswordHash, fixture.Password+"x"); got != application.PasswordFailed {
		t.Fatalf("Verify(.NET hash, wrong password) = %v, want PasswordFailed", got)
	}
	if got := hasher.Verify(fixture.PasswordHash, ""); got != application.PasswordFailed {
		t.Fatalf("Verify(.NET hash, empty password) = %v, want PasswordFailed", got)
	}
}

// TestGoHashRoundTrip covers Go-hash -> Go-verify, and asserts the blob the Go
// hasher writes carries the same header the .NET hasher writes, so a hash
// created by either backend is indistinguishable to the other.
func TestGoHashRoundTrip(t *testing.T) {
	t.Parallel()

	hasher := identity.NewPasswordHasher()
	const password = "another-P@ssw0rd"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if got := hasher.Verify(hash, password); got != application.PasswordSuccess {
		t.Fatalf("Verify(go hash, correct password) = %v, want PasswordSuccess", got)
	}
	if got := hasher.Verify(hash, "not-the-password"); got != application.PasswordFailed {
		t.Fatalf("Verify(go hash, wrong password) = %v, want PasswordFailed", got)
	}

	goHeader := parseHeader(t, hash)
	netHeader := parseHeader(t, fixtures.MustLoadDotNetIdentity().PasswordHash)
	if goHeader != netHeader {
		t.Fatalf("go header %+v != .NET header %+v", goHeader, netHeader)
	}

	// Two hashes of the same password must differ: the salt is random.
	second, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash (second): %v", err)
	}
	if second == hash {
		t.Fatal("two hashes of the same password are identical; salt is not random")
	}
}

// TestVerifyLegacyParametersNeedRehash builds a blob with pre-.NET-Core-3
// parameters (PRF HMACSHA256, 10000 iterations) and expects the outcome that
// tells the caller to re-hash.
func TestVerifyLegacyParametersNeedRehash(t *testing.T) {
	t.Parallel()

	const (
		password   = "legacy-P@ssw0rd"
		iterations = 10_000
	)
	salt := []byte("0123456789abcdef")
	subkey := pbkdf2.Key([]byte(password), salt, iterations, 32, sha256.New)

	blob := make([]byte, 13, 13+len(salt)+len(subkey))
	blob[0] = 0x01
	binary.BigEndian.PutUint32(blob[1:5], 1) // HMACSHA256
	binary.BigEndian.PutUint32(blob[5:9], iterations)
	binary.BigEndian.PutUint32(blob[9:13], uint32(len(salt)))
	blob = append(blob, salt...)
	blob = append(blob, subkey...)
	encoded := base64.StdEncoding.EncodeToString(blob)

	hasher := identity.NewPasswordHasher()
	if got := hasher.Verify(encoded, password); got != application.PasswordSuccessRehashNeeded {
		t.Fatalf("Verify(legacy hash, correct password) = %v, want PasswordSuccessRehashNeeded", got)
	}
	if got := hasher.Verify(encoded, "wrong"); got != application.PasswordFailed {
		t.Fatalf("Verify(legacy hash, wrong password) = %v, want PasswordFailed", got)
	}
}

func TestVerifyRejectsMalformedBlobs(t *testing.T) {
	t.Parallel()

	fixture := fixtures.MustLoadDotNetIdentity()
	raw, err := base64.StdEncoding.DecodeString(fixture.PasswordHash)
	if err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}

	withMarker := func(marker byte) string {
		clone := append([]byte(nil), raw...)
		clone[0] = marker
		return base64.StdEncoding.EncodeToString(clone)
	}
	tamperedSubkey := func() string {
		clone := append([]byte(nil), raw...)
		clone[len(clone)-1] ^= 0xFF
		return base64.StdEncoding.EncodeToString(clone)
	}

	cases := map[string]string{
		"empty":            "",
		"not base64":       "this is not base64!!",
		"v2 marker":        withMarker(0x00),
		"unknown marker":   withMarker(0x09),
		"truncated":        base64.StdEncoding.EncodeToString(raw[:10]),
		"header only":      base64.StdEncoding.EncodeToString(raw[:13]),
		"salt only":        base64.StdEncoding.EncodeToString(raw[:29]),
		"flipped subkey":   tamperedSubkey(),
		"single zero byte": base64.StdEncoding.EncodeToString([]byte{0x01}),
	}

	hasher := identity.NewPasswordHasher()
	for name, hash := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := hasher.Verify(hash, fixture.Password); got != application.PasswordFailed {
				t.Fatalf("Verify(%q) = %v, want PasswordFailed", name, got)
			}
		})
	}
}
