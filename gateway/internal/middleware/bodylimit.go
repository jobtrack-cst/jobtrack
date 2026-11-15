package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// MaxBodyBytes is the largest request body the gateway accepts (JT-54, agreed with
// Kinley as resume owner). A 2,000,000 byte resume is about 2.67 MB as base64 JSON.
const MaxBodyBytes int64 = 3 << 20 // 3 MiB = 3,145,728 bytes

// LimitBody rejects bodies larger than limit with HTTP 413 before they reach a
// service. The body is read here so chunked requests without Content-Length get
// the same 413 instead of grpc-gateway's generic decode error.
func LimitBody(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || r.Body == http.NoBody {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength > limit {
			writeBodyTooLarge(w, limit)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeBodyTooLarge(w, limit)
				return
			}
			writeUnreadableBody(w)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		next.ServeHTTP(w, r)
	})
}

// writeBodyTooLarge uses grpc-gateway's JSON error shape; code 3 is INVALID_ARGUMENT.
func writeBodyTooLarge(w http.ResponseWriter, limit int64) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_, _ = fmt.Fprintf(w, `{"code":3,"message":"request body exceeds %d bytes","details":[]}`, limit)
}

// writeUnreadableBody covers read failures such as a client disconnecting mid-upload.
func writeUnreadableBody(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"code":3,"message":"could not read request body","details":[]}`))
}
