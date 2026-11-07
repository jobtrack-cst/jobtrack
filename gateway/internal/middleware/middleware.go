// Package middleware authenticates gateway requests and controls which metadata reaches services.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/jwtauth"
)

// Metadata keys read by pkg/authctx.
const (
	metadataUserID   = "x-user-id"
	metadataUserRole = "x-user-role"
)

// publicRoutes skip the JWT check (AGENTS.md Section 4). Matching is exact, so any
// other method or path variant fails closed and needs a token.
var publicRoutes = map[string]bool{
	"POST /v1/auth/register": true,
	"POST /v1/auth/login":    true,
}

// Clients must never set identity or internal-call metadata (AGENTS.md Sections 4 and 6).
var reservedPrefixes = []string{"x-user-", "x-internal-", "x-caller-"}

// TokenVerifier is implemented by jwtauth.Verifier.
type TokenVerifier interface {
	Verify(token string) (jwtauth.Identity, error)
}

type identityKey struct{}

// IdentityFromContext returns the identity that Authenticate verified for this request.
func IdentityFromContext(ctx context.Context) (jwtauth.Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(jwtauth.Identity)
	return id, ok
}

// Authenticate removes client-sent reserved headers, then requires a valid bearer
// token on every route except register and login.
func Authenticate(verifier TokenVerifier, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stripReservedHeaders(r.Header)
		if publicRoutes[r.Method+" "+r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeUnauthenticated(w)
			return
		}
		id, err := verifier.Verify(token)
		if err != nil {
			// Log why for debugging, but never the token itself.
			logger.Info("request rejected", "reason", err.Error(), "method", r.Method, "path", r.URL.Path)
			writeUnauthenticated(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, id)))
	})
}

// IncomingHeaderMatcher is a second layer: even if a reserved header got past
// Authenticate, grpc-gateway never forwards it as metadata.
func IncomingHeaderMatcher(key string) (string, bool) {
	if isReserved(key) {
		return "", false
	}
	return runtime.DefaultHeaderMatcher(key)
}

// IdentityMetadata sets x-user-id and x-user-role from the verified token only.
func IdentityMetadata(_ context.Context, r *http.Request) metadata.MD {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		return nil
	}
	return metadata.Pairs(metadataUserID, id.UserID, metadataUserRole, id.Role)
}

// NewServeMux returns the grpc-gateway mux with both metadata rules applied.
func NewServeMux() *runtime.ServeMux {
	return runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(IncomingHeaderMatcher),
		runtime.WithMetadata(IdentityMetadata),
		// Authenticate decides on the original method. grpc-gateway must not change it
		// afterwards (X-HTTP-Method-Override, or POST form falling back to GET), or a
		// request let through as POST login could reach a different route.
		runtime.WithDisableHTTPMethodOverride(),
		runtime.WithDisablePathLengthFallback(),
	)
}

func isReserved(key string) bool {
	name := strings.TrimPrefix(strings.ToLower(key), strings.ToLower(runtime.MetadataHeaderPrefix))
	for _, prefix := range reservedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func stripReservedHeaders(h http.Header) {
	for key := range h {
		if isReserved(key) {
			delete(h, key)
		}
	}
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// writeUnauthenticated uses grpc-gateway's JSON error shape; code 16 is UNAUTHENTICATED.
func writeUnauthenticated(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="jobtrack"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"code":16,"message":"missing or invalid token","details":[]}`))
}
