package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports"
)

// Claim values from AGENTS.md Section 4. The gateway checks the same values.
const (
	Issuer        = "jobtrack-auth"
	TokenLifetime = time.Hour
	// HS256 keys shorter than the 32-byte hash output are easier to brute force.
	MinSecretBytes = 32
)

var ErrWeakJWTSecret = errors.New("JWT_SECRET must be at least 32 bytes")

// Claims is exported so tests (and later the gateway) see the exact claim set.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// JWT signs access tokens with HS256 and JWT_SECRET.
type JWT struct{ secret []byte }

var _ ports.TokenIssuer = (*JWT)(nil)

func NewJWT(secret string) (*JWT, error) {
	if len(secret) < MinSecretBytes {
		return nil, ErrWeakJWTSecret
	}
	return &JWT{secret: []byte(secret)}, nil
}

func (j *JWT) Issue(userID string, role domain.Role, issuedAt time.Time) (string, time.Time, error) {
	if userID == "" || !role.Valid() {
		return "", time.Time{}, errors.New("token needs a user ID and a valid role")
	}
	// JWT times are whole Unix seconds, so exp is exactly iat + 3600.
	iat := issuedAt.UTC().Truncate(time.Second)
	exp := iat.Add(TokenLifetime)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Role: role.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    Issuer,
			IssuedAt:  jwt.NewNumericDate(iat),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, errors.New("sign token failed")
	}
	return signed, exp, nil
}
