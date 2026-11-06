package backends

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAll_ListsEachServiceOnce(t *testing.T) {
	kebab := regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	names := map[string]bool{}
	envs := map[string]bool{}
	for _, b := range All() {
		require.Regexp(t, kebab, b.Name)
		require.NotNil(t, b.Register, b.Name)
		require.False(t, names[b.Name], "duplicate service %s", b.Name)
		require.False(t, envs[b.AddrEnv()], "duplicate env %s", b.AddrEnv())
		names[b.Name] = true
		envs[b.AddrEnv()] = true
	}
	require.Len(t, names, 20)
}

func TestBackend_AddrEnv(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "auth", want: "AUTH_ADDR"},
		{name: "application-status", want: "APPLICATION_STATUS_ADDR"},
		{name: "company-verification", want: "COMPANY_VERIFICATION_ADDR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Backend{Name: tt.name}.AddrEnv())
		})
	}
}

// If someone adds HTTP annotations to an *InternalService, or a new public service
// appears in gen/go without being listed here, this test fails.
func TestAll_MatchesGeneratedPublicHandlers(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "gen", "go", "jobtrack", "*", "v1", "*.pb.gw.go"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "generated gateway files not found")

	register := regexp.MustCompile(`func Register(\w+)HandlerFromEndpoint\(`)
	generated := map[string]bool{}
	for _, file := range files {
		src, err := os.ReadFile(file)
		require.NoError(t, err)
		for _, match := range register.FindAllStringSubmatch(string(src), -1) {
			require.NotContains(t, match[1], "Internal", "internal service has a gateway handler in %s", file)
			generated[match[1]] = true
		}
	}

	listed := map[string]bool{}
	for _, b := range All() {
		listed[pascal(b.Name)+"Service"] = true
	}
	require.Equal(t, generated, listed)
}

func TestAll_RegistersOnMuxWithoutInternalRoutes(t *testing.T) {
	// NewClient does not connect until a call is made, so no backend needs to run.
	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	mux := runtime.NewServeMux()
	for _, b := range All() {
		require.NoError(t, b.Register(context.Background(), mux, conn), b.Name)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantRouted bool
	}{
		{name: "public auth route is registered", method: http.MethodGet, path: "/v1/auth/me", wantRouted: true},
		{name: "internal RPC has no route", method: http.MethodPost, path: "/jobtrack.auth.v1.AuthInternalService/GetUser"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req := httptest.NewRequest(tt.method, tt.path, nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if tt.wantRouted {
				require.NotEqual(t, http.StatusNotFound, rec.Code)
				return
			}
			require.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func pascal(kebab string) string {
	parts := strings.Split(kebab, "-")
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}
