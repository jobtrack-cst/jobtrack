package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/jwtauth"
	"github.com/jobtrack-cst/jobtrack/gateway/internal/middleware"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
)

const bodyTooLargeJSON = `{"code":3,"message":"request body exceeds 3145728 bytes","details":[]}`

// bodyLimitToken signs a token in the AGENTS.md Section 4 claims format.
func bodyLimitToken(t *testing.T, role string) string {
	t.Helper()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  testUserID,
		"role": role,
		"iss":  "jobtrack-auth",
		"iat":  now.Unix(),
		"exp":  now.Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return signed
}

// resumeUploadBody builds an UploadResume request in protojson field names.
func resumeUploadBody(contentSize int) string {
	content := append([]byte("%PDF-"), bytes.Repeat([]byte("x"), contentSize-5)...)
	return `{"fileName":"` + strings.Repeat("a", 251) + `.pdf","contentType":"application/pdf","content":"` +
		base64.StdEncoding.EncodeToString(content) + `"}`
}

func TestNewHandler_BodyLimit(t *testing.T) {
	verifier, err := jwtauth.NewVerifier(testSecret, nil)
	require.NoError(t, err)
	limit := int(middleware.MaxBodyBytes)
	tests := []struct {
		name        string
		path        string
		size        int
		withToken   bool
		wantCode    int
		wantBody    string
		wantAPICall bool
	}{
		{name: "below limit with token reaches the API", path: "/v1/resumes", size: limit - 1, withToken: true, wantCode: http.StatusOK, wantAPICall: true},
		{name: "at limit with token reaches the API", path: "/v1/resumes", size: limit, withToken: true, wantCode: http.StatusOK, wantAPICall: true},
		{name: "above limit with token is rejected", path: "/v1/resumes", size: limit + 1, withToken: true, wantCode: http.StatusRequestEntityTooLarge, wantBody: bodyTooLargeJSON},
		{name: "above limit without token gets 401 first", path: "/v1/resumes", size: limit + 1, wantCode: http.StatusUnauthorized},
		{name: "above limit on public login is rejected", path: "/v1/auth/login", size: limit + 1, wantCode: http.StatusRequestEntityTooLarge, wantBody: bodyTooLargeJSON},
		{name: "at limit on public login reaches the API", path: "/v1/auth/login", size: limit, wantCode: http.StatusOK, wantAPICall: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			var received int
			api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				body, _ := io.ReadAll(r.Body)
				received = len(body)
				w.WriteHeader(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(strings.Repeat("a", tt.size)))
			if tt.withToken {
				req.Header.Set("Authorization", "Bearer "+bodyLimitToken(t, "applicant"))
			}
			rec := httptest.NewRecorder()

			newHandler(api, verifier, discardLogger()).ServeHTTP(rec, req)

			require.Equal(t, tt.wantCode, rec.Code)
			require.Equal(t, tt.wantAPICall, called)
			if tt.wantAPICall {
				require.Equal(t, tt.size, received, "the API must get the whole body")
			}
			if tt.wantBody != "" {
				require.JSONEq(t, tt.wantBody, rec.Body.String())
			}
		})
	}
}

// fakeResume records the UploadResume requests that reach the resume service.
type fakeResume struct {
	resumev1.UnimplementedResumeServiceServer
	received chan *resumev1.UploadResumeRequest
}

func (f *fakeResume) UploadResume(_ context.Context, req *resumev1.UploadResumeRequest) (*resumev1.UploadResumeResponse, error) {
	f.received <- req
	return &resumev1.UploadResumeResponse{Resume: &resumev1.Resume{}}, nil
}

// newResumeGateway runs newHandler in front of the generated grpc-gateway handler and
// a fake resume gRPC server, so the request is decoded exactly as in production.
func newResumeGateway(t *testing.T) (http.Handler, *fakeResume) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	fake := &fakeResume{received: make(chan *resumev1.UploadResumeRequest, 1)}
	resumev1.RegisterResumeServiceServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	mux := middleware.NewServeMux()
	require.NoError(t, resumev1.RegisterResumeServiceHandler(context.Background(), mux, conn))
	verifier, err := jwtauth.NewVerifier(testSecret, nil)
	require.NoError(t, err)
	return newHandler(mux, verifier, discardLogger()), fake
}

func TestGateway_ResumeUploadBodyLimit(t *testing.T) {
	handler, fake := newResumeGateway(t)
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/resumes", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+bodyLimitToken(t, "applicant"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	t.Run("2,000,000 byte resume reaches resume service", func(t *testing.T) {
		rec := post(resumeUploadBody(2_000_000))

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		select {
		case got := <-fake.received:
			require.Len(t, got.GetContent(), 2_000_000)
			require.Equal(t, "application/pdf", got.GetContentType())
			require.Len(t, got.GetFileName(), 255)
		case <-time.After(5 * time.Second):
			t.Fatal("upload did not reach the resume service")
		}
	})

	t.Run("body above the limit never reaches resume service", func(t *testing.T) {
		// 2,400,000 bytes is 3,200,000 bytes as base64, above 3 MiB.
		rec := post(resumeUploadBody(2_400_000))

		require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		require.JSONEq(t, bodyTooLargeJSON, rec.Body.String())
		select {
		case <-fake.received:
			t.Fatal("oversize upload reached the resume service")
		default:
		}
	})
}
