package security_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/adapters/security"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const (
	secret = "synthetic-test-secret-32-bytes!!"
	userID = "3f2b8c1e-1111-4111-8111-111111111111"
)

func newBcrypt(t *testing.T) *security.Bcrypt {
	t.Helper()
	b, err := security.NewBcrypt(bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBcrypt_HashAndCompare(t *testing.T) {
	b := newBcrypt(t)
	hash, err := b.Hash("password123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "password123" || !strings.HasPrefix(hash, "$2a$") {
		t.Fatalf("expected a bcrypt hash, got %q", hash)
	}
	second, _ := b.Hash("password123")
	if second == hash {
		t.Fatal("expected a different salt for each hash")
	}
	for _, tt := range []struct {
		name     string
		hash     string
		password string
		want     error
		wantAny  bool
	}{
		{name: "correct password", hash: hash, password: "password123"},
		{name: "wrong password", hash: hash, password: "password124", want: domain.ErrInvalidCredentials},
		{name: "empty hash runs the dummy check", hash: "", password: "password123", want: domain.ErrInvalidCredentials},
		{name: "malformed stored hash is not bad credentials", hash: "not-a-hash", password: "password123", wantAny: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := b.Compare(tt.hash, tt.password)
			if tt.wantAny {
				if err == nil || errors.Is(err, domain.ErrInvalidCredentials) {
					t.Fatalf("err=%v, want a non-credential error", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("err=%v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewBcrypt_RejectsCostOutOfRange(t *testing.T) {
	for _, cost := range []int{bcrypt.MinCost - 1, bcrypt.MaxCost + 1} {
		if _, err := security.NewBcrypt(cost); err == nil {
			t.Errorf("cost %d accepted", cost)
		}
	}
}

func TestNewJWT_SecretLength(t *testing.T) {
	for _, tt := range []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"empty", "", true},
		{"31 bytes", strings.Repeat("s", 31), true},
		{"32 bytes", strings.Repeat("s", 32), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := security.NewJWT(tt.secret)
			if tt.wantErr != errors.Is(err, security.ErrWeakJWTSecret) || (!tt.wantErr && err != nil) {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestJWT_Issue_ClaimsMatchAgentsSection4(t *testing.T) {
	issuer, err := security.NewJWT(secret)
	if err != nil {
		t.Fatal(err)
	}
	issuedAt := time.Date(2026, 11, 6, 12, 0, 0, 900_000_000, time.FixedZone("BTT", 6*3600))
	token, expiresAt, err := issuer.Issue(userID, domain.RoleEmployer, issuedAt)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
	var header, payload map[string]any
	decodePart(t, parts[0], &header)
	decodePart(t, parts[1], &payload)

	if header["alg"] != "HS256" {
		t.Errorf("alg=%v, want HS256", header["alg"])
	}
	wantIat := float64(issuedAt.Unix())
	want := map[string]any{
		"sub": userID, "role": "employer", "iss": "jobtrack-auth",
		"iat": wantIat, "exp": wantIat + 3600,
	}
	if len(payload) != len(want) {
		t.Errorf("claims=%v, want exactly %v", payload, want)
	}
	for k, v := range want {
		if payload[k] != v {
			t.Errorf("claim %s=%v, want %v", k, payload[k], v)
		}
	}
	if !expiresAt.Equal(time.Unix(int64(wantIat)+3600, 0)) || expiresAt.Location() != time.UTC {
		t.Errorf("expiresAt=%v, want iat+1h in UTC", expiresAt)
	}

	// The gateway will verify like this: HS256 only, correct issuer, same secret.
	verify := func(key string) error {
		_, err := jwt.ParseWithClaims(token, &security.Claims{}, func(*jwt.Token) (any, error) { return []byte(key), nil },
			jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("jobtrack-auth"),
			jwt.WithTimeFunc(func() time.Time { return issuedAt.Add(time.Minute) }))
		return err
	}
	if err := verify(secret); err != nil {
		t.Errorf("valid token rejected: %v", err)
	}
	if err := verify(strings.Repeat("x", 32)); err == nil {
		t.Error("token verified with the wrong secret")
	}
}

func TestJWT_Issue_RejectsBadInput(t *testing.T) {
	issuer, _ := security.NewJWT(secret)
	for _, tt := range []struct {
		name string
		id   string
		role domain.Role
	}{
		{"empty user ID", "", domain.RoleApplicant},
		{"unspecified role", userID, domain.RoleUnspecified},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := issuer.Issue(tt.id, tt.role, time.Now()); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func decodePart(t *testing.T, part string, into *map[string]any) {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatal(err)
	}
}
