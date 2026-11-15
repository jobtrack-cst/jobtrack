package emailsender_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/adapters/emailsender"
)

func TestNew_ValidatesTheEndpoint(t *testing.T) {
	for _, tt := range []struct {
		endpoint string
		ok       bool
	}{
		{"http://email-sender:8080/send-email", true},
		{"https://email-sender.default.svc.cluster.local/send-email", true},
		{"", false},
		{"email-sender:8080/send-email", false},
		{"ftp://email-sender/send-email", false},
		{"http:///send-email", false},
		{"http://bad host/send-email", false},
	} {
		t.Run(tt.endpoint, func(t *testing.T) {
			_, err := emailsender.New(tt.endpoint)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, emailsender.ErrInvalidURL)
			}
		})
	}
}

func TestClient_Send_PostsTheContractJSON(t *testing.T) {
	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/send-email", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := emailsender.New(server.URL + "/send-email")
	require.NoError(t, err)
	require.NoError(t, client.Send(context.Background(), "admin@cst.edu.bt", "Weekly summary", "Line 1\nLine 2"))
	assert.Equal(t, map[string]string{"recipient": "admin@cst.edu.bt", "subject": "Weekly summary", "body": "Line 1\nLine 2"}, got)
}

func TestClient_Send_FailsOnAnythingButNoContent(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusFound, http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if code == http.StatusFound {
					w.Header().Set("Location", "/elsewhere")
				}
				w.WriteHeader(code)
				_, _ = w.Write([]byte("email delivery unavailable"))
			}))
			defer server.Close()
			client, err := emailsender.New(server.URL + "/send-email")
			require.NoError(t, err)
			err = client.Send(context.Background(), "admin@cst.edu.bt", "s", "b")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "HTTP")
		})
	}
}

func TestClient_Send_UnreachableOrCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := server.URL + "/send-email"
	server.Close()
	client, err := emailsender.New(endpoint)
	require.NoError(t, err)
	assert.EqualError(t, client.Send(context.Background(), "admin@cst.edu.bt", "s", "b"), "email-sender request failed")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Error(t, client.Send(ctx, "admin@cst.edu.bt", "s", "b"))
}
