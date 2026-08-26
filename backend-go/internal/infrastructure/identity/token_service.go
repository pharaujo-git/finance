package identity

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
)

const (
	// Issuer and Audience match JwtOptions in the .NET API.
	Issuer   = "finance-tracker"
	Audience = "finance-tracker"

	// TokenLifetime matches JwtOptions.ExpiryDays (7).
	TokenLifetime = 7 * 24 * time.Hour

	// ClockLeeway matches the JwtBearer ClockSkew of one minute.
	ClockLeeway = time.Minute

	signingAlg = "HS256"
)

// ErrInvalidToken is returned for every rejection reason; callers turn it into
// a 401 without leaking which check failed.
var ErrInvalidToken = errors.New("identity: invalid token")

// TokenService issues and validates HS256 bearer tokens interchangeable with
// the ones FinanceTracker.Infrastructure.Identity.TokenService produces.
type TokenService struct {
	secret []byte
	// now is injectable so tests can pin the clock; nil means time.Now.
	now func() time.Time
}

// NewTokenService builds a service signing with the raw bytes of secret.
func NewTokenService(secret string) *TokenService {
	return &TokenService{secret: []byte(secret)}
}

// WithClock returns a copy that reads the current time from now. Used by tests
// to validate a captured token long after it was minted.
func (s *TokenService) WithClock(now func() time.Time) *TokenService {
	clone := *s
	clone.now = now
	return &clone
}

var _ application.TokenService = (*TokenService)(nil)

func (s *TokenService) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Issue mints a token for the given user. The claim set is the same one the
// .NET handler writes: iss, aud, exp, iat, nbf, sub, email, jti.
func (s *TokenService) Issue(userID uuid.UUID, email string) (string, error) {
	now := s.clock().UTC()
	claims := jwt.MapClaims{
		"iss":   Issuer,
		"aud":   Audience,
		"exp":   now.Add(TokenLifetime).Unix(),
		"iat":   now.Unix(),
		"nbf":   now.Unix(),
		"sub":   userID.String(),
		"email": email,
		"jti":   uuid.NewString(),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("identity: signing token: %w", err)
	}
	return signed, nil
}

// Validate checks the signature, issuer, audience and lifetime, then extracts
// the principal. This is the single path the HTTP middleware uses.
func (s *TokenService) Validate(token string) (application.Principal, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{signingAlg}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(Audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(ClockLeeway),
		jwt.WithTimeFunc(s.clock),
	)

	claims := jwt.MapClaims{}
	if _, err := parser.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	}); err != nil {
		return application.Principal{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return application.Principal{}, fmt.Errorf("%w: missing subject", ErrInvalidToken)
	}
	userID, err := uuid.Parse(subject)
	if err != nil {
		return application.Principal{}, fmt.Errorf("%w: subject is not a uuid", ErrInvalidToken)
	}

	email, _ := claims["email"].(string)
	return application.Principal{UserID: userID, Email: email}, nil
}
