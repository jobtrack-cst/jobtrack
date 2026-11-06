package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/jwtauth"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef" // exactly 32 bytes
	otherSecret = "fedcba9876543210fedcba9876543210"
	testUserID  = "3f2b8c1e-7a4d-4c2b-9e1f-0a1b2c3d4e5f"
	routeMe     = "/v1/auth/me"
	routeLogin  = "/v1/auth/login"
	routeReg    = "/v1/auth/register"
)

var testNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func newVerifier(t *testing.T) *jwtauth.Verifier {
	t.Helper()
	v, err := jwtauth.NewVerifier(testSecret, func() time.Time { return testNow })
	require.NoError(t, err)
	return v
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// makeToken signs the claims auth issues, after an optional change.
func makeToken(t *testing.T, method jwt.SigningMethod, key any, mutate func(jwt.MapClaims)) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  testUserID,
		"role": "applicant",
		"iss":  jwtauth.Issuer,
		"iat":  testNow.Unix(),
		"exp":  testNow.Add(time.Hour).Unix(),
	}
	if mutate != nil {
		mutate(claims)
	}
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return token
}

func validToken(t *testing.T) string {
	return makeToken(t, jwt.SigningMethodHS256, []byte(testSecret), nil)
}

func TestAuthenticate_RejectsWithUnauthenticated(t *testing.T) {
	hs256 := func(mutate func(jwt.MapClaims)) string {
		return "Bearer " + makeToken(t, jwt.SigningMethodHS256, []byte(testSecret), mutate)
	}
	tests := []struct {
		name          string
		method        string
		path          string
		authorization string
	}{
		{name: "no Authorization header", method: http.MethodGet, path: routeMe},
		{name: "Basic scheme", method: http.MethodGet, path: routeMe, authorization: "Basic dXNlcjpwYXNz"},
		{name: "Bearer without token", method: http.MethodGet, path: routeMe, authorization: "Bearer "},
		{name: "token without scheme", method: http.MethodGet, path: routeMe, authorization: validToken(t)},
		{name: "not a JWT", method: http.MethodGet, path: routeMe, authorization: "Bearer not-a-jwt"},
		{name: "bad signature", method: http.MethodGet, path: routeMe,
			authorization: "Bearer " + makeToken(t, jwt.SigningMethodHS256, []byte(otherSecret), nil)},
		{name: "alg none", method: http.MethodGet, path: routeMe,
			authorization: "Bearer " + makeToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, nil)},
		{name: "alg HS384", method: http.MethodGet, path: routeMe,
			authorization: "Bearer " + makeToken(t, jwt.SigningMethodHS384, []byte(testSecret), nil)},
		{name: "wrong iss", method: http.MethodGet, path: routeMe,
			authorization: hs256(func(c jwt.MapClaims) { c["iss"] = "someone-else" })},
		{name: "expired", method: http.MethodGet, path: routeMe,
			authorization: hs256(func(c jwt.MapClaims) { c["exp"] = testNow.Add(-time.Second).Unix() })},
		{name: "unknown role", method: http.MethodGet, path: routeMe,
			authorization: hs256(func(c jwt.MapClaims) { c["role"] = "superuser" })},
		{name: "register with GET is not public", method: http.MethodGet, path: routeReg},
		{name: "login with trailing slash is not public", method: http.MethodPost, path: routeLogin + "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
			rec := httptest.NewRecorder()

			Authenticate(newVerifier(t), discardLogger(), next).ServeHTTP(rec, req)

			require.False(t, called, "request must not reach the backend")
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.JSONEq(t, `{"code":16,"message":"missing or invalid token","details":[]}`, rec.Body.String())
		})
	}
}

func TestAuthenticate_AllowsPublicRoutesAndValidTokens(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		authorization string
		wantIdentity  bool
	}{
		{name: "register without token", method: http.MethodPost, path: routeReg},
		{name: "login without token", method: http.MethodPost, path: routeLogin},
		{name: "login ignores a bad token", method: http.MethodPost, path: routeLogin, authorization: "Bearer not-a-jwt"},
		{name: "protected route with valid token", method: http.MethodGet, path: routeMe,
			authorization: "Bearer " + validToken(t), wantIdentity: true},
		{name: "lowercase bearer scheme", method: http.MethodGet, path: routeMe,
			authorization: "bearer " + validToken(t), wantIdentity: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			called := false
			var got jwtauth.Identity
			var gotOK bool
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				got, gotOK = IdentityFromContext(r.Context())
			})
			rec := httptest.NewRecorder()

			Authenticate(newVerifier(t), discardLogger(), next).ServeHTTP(rec, req)

			require.True(t, called)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.wantIdentity, gotOK)
			if tt.wantIdentity {
				require.Equal(t, jwtauth.Identity{UserID: testUserID, Role: "applicant"}, got)
			}
		})
	}
}

// forwardedMetadata sends a request through Authenticate and grpc-gateway's real
// metadata rules, and returns what a backend service would receive.
func forwardedMetadata(t *testing.T, req *http.Request) (metadata.MD, int) {
	t.Helper()
	mux := NewServeMux()
	var md metadata.MD
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx, err := runtime.AnnotateContext(r.Context(), mux, r, "/jobtrack.auth.v1.AuthService/GetCurrentUser")
		require.NoError(t, err)
		md, _ = metadata.FromOutgoingContext(ctx)
	})
	rec := httptest.NewRecorder()
	Authenticate(newVerifier(t), discardLogger(), next).ServeHTTP(rec, req)
	return md, rec.Code
}

func TestGateway_HeaderSpoofing(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		withToken    bool
		headers      map[string]string // set as given, without canonicalizing the key
		wantIdentity bool
		wantAbsent   []string
	}{
		{
			name: "Grpc-Metadata user headers replaced by token values", method: http.MethodGet, path: routeMe, withToken: true,
			headers:      map[string]string{"Grpc-Metadata-X-User-Id": "attacker", "Grpc-Metadata-X-User-Role": "admin"},
			wantIdentity: true,
		},
		{
			name: "plain X-User headers ignored", method: http.MethodGet, path: routeMe, withToken: true,
			headers:      map[string]string{"X-User-Id": "attacker", "X-User-Role": "admin"},
			wantIdentity: true,
		},
		{
			name: "lowercase grpc-metadata key removed", method: http.MethodGet, path: routeMe, withToken: true,
			headers:      map[string]string{"grpc-metadata-x-user-role": "admin"},
			wantIdentity: true,
		},
		{
			name: "internal token dropped", method: http.MethodGet, path: routeMe, withToken: true,
			headers:      map[string]string{"Grpc-Metadata-X-Internal-Token": "stolen"},
			wantIdentity: true, wantAbsent: []string{"x-internal-token"},
		},
		{
			name: "caller service dropped", method: http.MethodGet, path: routeMe, withToken: true,
			headers:      map[string]string{"Grpc-Metadata-X-Caller-Service": "resume-validator"},
			wantIdentity: true, wantAbsent: []string{"x-caller-service"},
		},
		{
			name: "public route never gets an identity", method: http.MethodPost, path: routeReg,
			headers:    map[string]string{"Grpc-Metadata-X-User-Id": "attacker", "Grpc-Metadata-X-User-Role": "admin"},
			wantAbsent: []string{"x-user-id", "x-user-role"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.withToken {
				req.Header.Set("Authorization", "Bearer "+validToken(t))
			}
			for key, value := range tt.headers {
				req.Header[key] = []string{value}
			}

			md, code := forwardedMetadata(t, req)

			require.Equal(t, http.StatusOK, code)
			if tt.wantIdentity {
				require.Equal(t, []string{testUserID}, md.Get("x-user-id"))
				require.Equal(t, []string{"applicant"}, md.Get("x-user-role"))
			}
			for _, key := range tt.wantAbsent {
				require.Empty(t, md.Get(key), key)
			}
		})
	}
}

func TestIncomingHeaderMatcher_DropsReservedKeys(t *testing.T) {
	keys := []string{
		"Grpc-Metadata-X-User-Id",
		"Grpc-Metadata-X-User-Role",
		"Grpc-Metadata-X-Internal-Token",
		"Grpc-Metadata-X-Caller-Service",
		"grpc-metadata-x-user-role",
		"X-User-Role",
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			gotKey, gotOK := IncomingHeaderMatcher(key)
			require.False(t, gotOK)
			require.Empty(t, gotKey)
		})
	}
}

// Non-reserved keys must behave exactly like grpc-gateway's default, so the gateway
// only removes what AGENTS.md says and nothing else.
func TestIncomingHeaderMatcher_OtherKeysUseDefault(t *testing.T) {
	keys := []string{"Authorization", "Content-Type", "Grpc-Metadata-X-Request-Id", "X-Request-Id"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			gotKey, gotOK := IncomingHeaderMatcher(key)
			wantKey, wantOK := runtime.DefaultHeaderMatcher(key)
			require.Equal(t, wantOK, gotOK)
			require.Equal(t, wantKey, gotKey)
		})
	}
}
