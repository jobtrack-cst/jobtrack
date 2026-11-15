package middleware_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/middleware"
)

// recorder is the next handler: it remembers whether it was called and what body it got.
type recorder struct {
	called bool
	body   []byte
}

func (h *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.body, _ = io.ReadAll(r.Body)
	w.WriteHeader(http.StatusOK)
}

// failingReader fails on any read, to prove a body was rejected without being read.
type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

func assertTooLarge(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var got struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Details []json.RawMessage `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, 3, got.Code)
	assert.Equal(t, "request body exceeds 3145728 bytes", got.Message)
	assert.NotNil(t, got.Details)
	assert.Empty(t, got.Details)
}

func TestLimitBody_MaxBodyBytesIs3MiB(t *testing.T) {
	assert.Equal(t, int64(3145728), middleware.MaxBodyBytes)
}

func TestLimitBody_RejectsBodiesAboveLimit(t *testing.T) {
	limit := middleware.MaxBodyBytes
	tests := []struct {
		name     string
		size     int64
		chunked  bool // no Content-Length, as with Transfer-Encoding: chunked
		wantPass bool
	}{
		{name: "empty body", size: 0, wantPass: true},
		{name: "one byte", size: 1, wantPass: true},
		{name: "below limit", size: limit - 1, wantPass: true},
		{name: "at limit", size: limit, wantPass: true},
		{name: "above limit", size: limit + 1, wantPass: false},
		{name: "far above limit", size: 2 * limit, wantPass: false},
		{name: "chunked below limit", size: limit - 1, chunked: true, wantPass: true},
		{name: "chunked at limit", size: limit, chunked: true, wantPass: true},
		{name: "chunked above limit", size: limit + 1, chunked: true, wantPass: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := bytes.Repeat([]byte("a"), int(tc.size))
			req := httptest.NewRequest(http.MethodPost, "/v1/resumes", bytes.NewReader(payload))
			if tc.chunked {
				req.ContentLength = -1
			}
			next := &recorder{}
			rec := httptest.NewRecorder()

			middleware.LimitBody(limit, next).ServeHTTP(rec, req)

			if tc.wantPass {
				assert.Equal(t, http.StatusOK, rec.Code)
				require.True(t, next.called, "request within the limit must reach the next handler")
				assert.Equal(t, payload, next.body, "the next handler must get the whole body")
				return
			}
			assert.False(t, next.called, "oversize request must not reach any service")
			assertTooLarge(t, rec)
		})
	}
}

func TestLimitBody_RejectsDeclaredOversizeWithoutReading(t *testing.T) {
	body := failingReader{err: errors.New("body must not be read")}
	req := httptest.NewRequest(http.MethodPost, "/v1/resumes", body)
	req.ContentLength = middleware.MaxBodyBytes + 1
	next := &recorder{}
	rec := httptest.NewRecorder()

	middleware.LimitBody(middleware.MaxBodyBytes, next).ServeHTTP(rec, req)

	assert.False(t, next.called)
	assertTooLarge(t, rec)
}

func TestLimitBody_PassesRequestWithoutBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/resumes", nil)
	next := &recorder{}
	rec := httptest.NewRecorder()

	middleware.LimitBody(middleware.MaxBodyBytes, next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, next.called)
}

func TestLimitBody_ReturnsBadRequestWhenBodyCannotBeRead(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/resumes", failingReader{err: io.ErrUnexpectedEOF})
	req.ContentLength = -1
	next := &recorder{}
	rec := httptest.NewRecorder()

	middleware.LimitBody(middleware.MaxBodyBytes, next).ServeHTTP(rec, req)

	assert.False(t, next.called)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"code":3,"message":"could not read request body","details":[]}`, rec.Body.String())
}

// The largest valid UploadResume request (AGENTS.md Section 15, JT-48): 2,000,000
// bytes of content as base64 and a 255 character file name, in protojson field names.
func TestLimitBody_AcceptsMaximumResumeUpload(t *testing.T) {
	content := append([]byte("%PDF-"), bytes.Repeat([]byte("x"), 2_000_000-5)...)
	encoded := base64.StdEncoding.EncodeToString(content)
	tests := []struct {
		name     string
		fileName string // already JSON-encoded
	}{
		{name: "plain 255 character name", fileName: strings.Repeat("a", 251) + ".pdf"},
		// Worst case: every character is an emoji escaped as a \uXXXX surrogate pair.
		{name: "fully escaped 255 character name", fileName: strings.Repeat(`😀`, 255)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"fileName":"` + tc.fileName + `","contentType":"application/pdf","content":"` + encoded + `"}`
			require.True(t, json.Valid([]byte(body)))
			require.Less(t, int64(len(body)), middleware.MaxBodyBytes)

			req := httptest.NewRequest(http.MethodPost, "/v1/resumes", strings.NewReader(body))
			next := &recorder{}
			rec := httptest.NewRecorder()

			middleware.LimitBody(middleware.MaxBodyBytes, next).ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			require.True(t, next.called)
			assert.Equal(t, body, string(next.body))
		})
	}
}
