// Package identity implements the two pieces both backends must agree on
// byte for byte: the ASP.NET Core Identity v3 password blob and the HS256
// bearer token.
package identity

import (
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- PRF 0 exists only to verify legacy blobs.
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"

	"golang.org/x/crypto/pbkdf2"

	"github.com/pharaujo/finance/backend-go/internal/application"
)

// Layout of the version-3 blob produced by Microsoft.AspNetCore.Identity's
// PasswordHasher (PasswordHasherCompatibilityMode.IdentityV3), base64-encoded:
//
//	byte  0      format marker, always 0x01
//	bytes 1..4   PRF id, uint32 big-endian
//	bytes 5..8   iteration count, uint32 big-endian
//	bytes 9..12  salt length in bytes, uint32 big-endian
//	bytes 13..   salt, then the derived subkey (rest of the blob)
const (
	formatMarkerV3 byte = 0x01
	headerLen      int  = 13

	// prfHMACSHA1 and prfHMACSHA256 are only ever read from stored blobs;
	// matching one means the hash predates the current defaults.
	prfHMACSHA1   uint32 = 0
	prfHMACSHA256 uint32 = 1
	prfHMACSHA512 uint32 = 2

	// Defaults used when writing a new hash. Confirmed against a blob generated
	// by the .NET API on 2026-08-26: marker 0x01, PRF 2, 100000 iterations,
	// 16-byte salt, 32-byte subkey.
	defaultPRF        = prfHMACSHA512
	defaultIterations = 100_000
	defaultSaltLen    = 16
	defaultSubkeyLen  = 32
)

// PasswordHasher implements application.PasswordHasher.
type PasswordHasher struct{}

// NewPasswordHasher returns a hasher writing the current Identity v3 defaults.
func NewPasswordHasher() *PasswordHasher { return &PasswordHasher{} }

var _ application.PasswordHasher = (*PasswordHasher)(nil)

// Hash derives a new Identity v3 blob for password.
func (PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, defaultSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("identity: reading salt: %w", err)
	}

	prf, err := prfHash(defaultPRF)
	if err != nil {
		return "", err
	}
	subkey := pbkdf2.Key([]byte(password), salt, defaultIterations, defaultSubkeyLen, prf)

	blob := make([]byte, headerLen, headerLen+len(salt)+len(subkey))
	blob[0] = formatMarkerV3
	binary.BigEndian.PutUint32(blob[1:5], defaultPRF)
	binary.BigEndian.PutUint32(blob[5:9], defaultIterations)
	binary.BigEndian.PutUint32(blob[9:13], uint32(len(salt)))
	blob = append(blob, salt...)
	blob = append(blob, subkey...)

	return base64.StdEncoding.EncodeToString(blob), nil
}

// Verify checks password against a stored blob. A correct password whose blob
// uses parameters weaker than the current defaults reports
// PasswordSuccessRehashNeeded, exactly as Identity's own hasher does.
func (PasswordHasher) Verify(hash, password string) application.PasswordVerificationOutcome {
	blob, err := base64.StdEncoding.DecodeString(hash)
	if err != nil || len(blob) < headerLen+1 || blob[0] != formatMarkerV3 {
		return application.PasswordFailed
	}

	prfID := binary.BigEndian.Uint32(blob[1:5])
	iterations := binary.BigEndian.Uint32(blob[5:9])
	saltLen := binary.BigEndian.Uint32(blob[9:13])

	// Identity rejects salts under 128 bits; anything longer than the blob is
	// corrupt. Both checks also keep the slicing below in range.
	if saltLen < 16 || uint64(headerLen)+uint64(saltLen) >= uint64(len(blob)) {
		return application.PasswordFailed
	}
	if iterations == 0 || iterations > 10_000_000 {
		return application.PasswordFailed
	}

	prf, err := prfHash(prfID)
	if err != nil {
		return application.PasswordFailed
	}

	// The bounds check above keeps saltLen below len(blob), so it fits an int.
	saltEnd := headerLen + int(saltLen)
	salt := blob[headerLen:saltEnd]
	expected := blob[saltEnd:]
	if len(expected) < 16 {
		return application.PasswordFailed
	}

	actual := pbkdf2.Key([]byte(password), salt, int(iterations), len(expected), prf)
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return application.PasswordFailed
	}

	if prfID != defaultPRF || iterations < defaultIterations || len(expected) < defaultSubkeyLen {
		return application.PasswordSuccessRehashNeeded
	}
	return application.PasswordSuccess
}

// prfHash maps an Identity PRF id onto its hash constructor.
func prfHash(prf uint32) (func() hash.Hash, error) {
	switch prf {
	case prfHMACSHA1:
		return sha1.New, nil
	case prfHMACSHA256:
		return sha256.New, nil
	case prfHMACSHA512:
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("identity: unknown PRF id %d", prf)
	}
}
