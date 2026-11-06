package jwtauth

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef" // exactly 32 bytes
	otherSecret = "fedcba9876543210fedcba9876543210"
	testUserID  = "3f2b8c1e-7a4d-4c2b-9e1f-0a1b2c3d4e5f"
)

var testNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func newTestVerifier(t *testing.T) *Verifier {
	t.Helper()
	v, err := NewVerifier(testSecret, func() time.Time { return testNow })
	require.NoError(t, err)
	return v
}

// validClaims mirrors what auth issues: iat now, exp = iat + 1 hour.
func validClaims() *claims {
	return &claims{
		Role: "applicant",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   testUserID,
			Issuer:    Issuer,
			IssuedAt:  jwt.NewNumericDate(testNow),
			ExpiresAt: jwt.NewNumericDate(testNow.Add(time.Hour)),
		},
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, c *claims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, c).SignedString(key)
	require.NoError(t, err)
	return token
}

func TestNewVerifier_SecretLength(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "empty rejected", secret: "", wantErr: true},
		{name: "31 bytes rejected", secret: strings.Repeat("a", 31), wantErr: true},
		{name: "32 bytes accepted", secret: strings.Repeat("a", 32), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVerifier(tt.secret, nil)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrWeakSecret)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestVerifier_Verify_AcceptsValidTokens(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*claims)
		wantRole string
	}{
		{name: "applicant", mutate: func(c *claims) { c.Role = "applicant" }, wantRole: "applicant"},
		{name: "employer", mutate: func(c *claims) { c.Role = "employer" }, wantRole: "employer"},
		{name: "admin", mutate: func(c *claims) { c.Role = "admin" }, wantRole: "admin"},
		{
			name:     "expires in one second",
			mutate:   func(c *claims) { c.ExpiresAt = jwt.NewNumericDate(testNow.Add(time.Second)) },
			wantRole: "applicant",
		},
	}
	v := newTestVerifier(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validClaims()
			tt.mutate(c)
			got, err := v.Verify(sign(t, jwt.SigningMethodHS256, []byte(testSecret), c))
			require.NoError(t, err)
			require.Equal(t, Identity{UserID: testUserID, Role: tt.wantRole}, got)
		})
	}
}

func TestVerifier_Verify_RejectsInvalidTokens(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// hs256 signs valid claims after one change, so each case tests a single rule.
	hs256 := func(mutate func(*claims)) func(*testing.T) string {
		return func(t *testing.T) string {
			c := validClaims()
			mutate(c)
			return sign(t, jwt.SigningMethodHS256, []byte(testSecret), c)
		}
	}

	tests := []struct {
		name  string
		token func(*testing.T) string
	}{
		{name: "empty token", token: func(*testing.T) string { return "" }},
		{name: "not a JWT", token: func(*testing.T) string { return "not-a-jwt" }},
		{name: "bad signature", token: func(t *testing.T) string {
			return sign(t, jwt.SigningMethodHS256, []byte(otherSecret), validClaims())
		}},
		{name: "alg none", token: func(t *testing.T) string {
			return sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims())
		}},
		{name: "alg HS384", token: func(t *testing.T) string {
			return sign(t, jwt.SigningMethodHS384, []byte(testSecret), validClaims())
		}},
		{name: "alg HS512", token: func(t *testing.T) string {
			return sign(t, jwt.SigningMethodHS512, []byte(testSecret), validClaims())
		}},
		{name: "alg RS256", token: func(t *testing.T) string {
			return sign(t, jwt.SigningMethodRS256, rsaKey, validClaims())
		}},
		{name: "wrong iss", token: hs256(func(c *claims) { c.Issuer = "someone-else" })},
		{name: "missing iss", token: hs256(func(c *claims) { c.Issuer = "" })},
		{name: "expired one second ago", token: hs256(func(c *claims) {
			c.ExpiresAt = jwt.NewNumericDate(testNow.Add(-time.Second))
		})},
		{name: "missing exp", token: hs256(func(c *claims) { c.ExpiresAt = nil })},
		{name: "unknown role", token: hs256(func(c *claims) { c.Role = "superuser" })},
		{name: "role wrong case", token: hs256(func(c *claims) { c.Role = "ADMIN" })},
		{name: "missing role", token: hs256(func(c *claims) { c.Role = "" })},
		{name: "missing sub", token: hs256(func(c *claims) { c.Subject = "" })},
	}
	v := newTestVerifier(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v.Verify(tt.token(t))
			require.ErrorIs(t, err, ErrInvalidToken)
			require.Equal(t, Identity{}, got)
		})
	}
}
