// Package jwtauth verifies access tokens issued by the auth service.
package jwtauth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Values from AGENTS.md Section 4. Keep them equal to
// services/auth/internal/adapters/security/jwt.go, which the gateway cannot import.
const (
	Issuer         = "jobtrack-auth"
	MinSecretBytes = 32
)

var (
	ErrWeakSecret = errors.New("JWT_SECRET must be at least 32 bytes")
	// ErrInvalidToken wraps every rejection so callers map all of them to UNAUTHENTICATED.
	ErrInvalidToken = errors.New("invalid token")
)

// Role values map to jobtrack.auth.v1.Role; anything else is rejected.
var validRoles = map[string]bool{"applicant": true, "employer": true, "admin": true}

// Identity is the verified caller that the gateway forwards as x-user-id and x-user-role.
type Identity struct {
	UserID string
	Role   string
}

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Verifier checks HS256 tokens signed with JWT_SECRET.
type Verifier struct {
	secret []byte
	parser *jwt.Parser
}

// NewVerifier takes now so tests can control expiry; nil means time.Now.
func NewVerifier(secret string, now func() time.Time) (*Verifier, error) {
	if len(secret) < MinSecretBytes {
		return nil, ErrWeakSecret
	}
	if now == nil {
		now = time.Now
	}
	return &Verifier{
		secret: []byte(secret),
		parser: jwt.NewParser(
			// Only HS256: blocks "none" and algorithm confusion attacks.
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(Issuer),
			jwt.WithExpirationRequired(),
			jwt.WithTimeFunc(now),
		),
	}, nil
}

// Verify returns the caller identity, or an error wrapping ErrInvalidToken.
func (v *Verifier) Verify(token string) (Identity, error) {
	var c claims
	_, err := v.parser.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) {
		return v.secret, nil
	})
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if c.Subject == "" {
		return Identity{}, fmt.Errorf("%w: missing sub", ErrInvalidToken)
	}
	if !validRoles[c.Role] {
		return Identity{}, fmt.Errorf("%w: unknown role", ErrInvalidToken)
	}
	return Identity{UserID: c.Subject, Role: c.Role}, nil
}
