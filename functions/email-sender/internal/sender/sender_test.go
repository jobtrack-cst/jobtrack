package sender_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/email-sender/internal/sender"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/stretchr/testify/require"
)

type fakeDelivery struct {
	messages []sender.Message
	err      error
}

func (f *fakeDelivery) Send(_ context.Context, m sender.Message) error {
	f.messages = append(f.messages, m)
	return f.err
}

func TestHandler_ValidatesBeforeDelivery(t *testing.T) {
	valid := sender.Message{Recipient: "student@example.test", Subject: "Application update", Body: "Hello\nYour application was received."}
	tests := []struct {
		name, method, raw, content string
		change                     func(*sender.Message)
		smtpError                  bool
		status                     int
	}{
		{name: "valid delivery", status: 204},
		{name: "malformed JSON", raw: "{", status: 400},
		{name: "null", raw: "null", status: 400},
		{name: "missing fields", raw: "{}", status: 400},
		{name: "unknown field", raw: `{"recipient":"a@example.test","subject":"s","body":"b","extra":true}`, status: 400},
		{name: "trailing JSON", raw: `{} {}`, status: 400},
		{name: "invalid recipient", change: func(m *sender.Message) { m.Recipient = "bad" }, status: 400},
		{name: "multiple recipients", change: func(m *sender.Message) { m.Recipient = "a@example.test,b@example.test" }, status: 400},
		{name: "recipient injection", change: func(m *sender.Message) { m.Recipient = "a@example.test\r\nBcc: other@example.test" }, status: 400},
		{name: "subject injection", change: func(m *sender.Message) { m.Subject = "hello\nBcc: other@example.test" }, status: 400},
		{name: "empty subject", change: func(m *sender.Message) { m.Subject = " " }, status: 400},
		{name: "empty body", change: func(m *sender.Message) { m.Body = "\n" }, status: 400},
		{name: "body NUL", change: func(m *sender.Message) { m.Body = "hi\x00" }, status: 400},
		{name: "subject boundary", change: func(m *sender.Message) { m.Subject = strings.Repeat("a", 200) }, status: 204},
		{name: "long subject", change: func(m *sender.Message) { m.Subject = strings.Repeat("a", 201) }, status: 400},
		{name: "body boundary", change: func(m *sender.Message) { m.Body = strings.Repeat("a", 65536) }, status: 204},
		{name: "long body", change: func(m *sender.Message) { m.Body = strings.Repeat("a", 65537) }, status: 400},
		{name: "request too large", raw: strings.Repeat(" ", sender.MaxRequestBytes+1), status: 413},
		{name: "SMTP failure", smtpError: true, status: 503},
		{name: "GET", method: "GET", status: 405},
		{name: "PUT", method: "PUT", status: 405},
		{name: "wrong content type", content: "text/plain", status: 415},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid
			if tt.change != nil {
				tt.change(&m)
			}
			raw := tt.raw
			if raw == "" {
				data, err := json.Marshal(m)
				require.NoError(t, err)
				raw = string(data)
			}
			method := tt.method
			if method == "" {
				method = "POST"
			}
			content := tt.content
			if content == "" {
				content = "application/json"
			}
			delivery := &fakeDelivery{}
			if tt.smtpError {
				delivery.err = errors.New("secret credentials and other@example.test")
			}
			var logs bytes.Buffer
			h := sender.Handler{Delivery: delivery, Logger: logging.New("email-sender", &logs)}
			request := httptest.NewRequest(method, "/send-email", strings.NewReader(raw))
			request.Header.Set("Content-Type", content)
			response := httptest.NewRecorder()
			h.ServeHTTP(response, request)
			require.Equal(t, tt.status, response.Code)
			if tt.status == 204 || tt.smtpError {
				require.Equal(t, []sender.Message{m}, delivery.messages)
			} else {
				require.Empty(t, delivery.messages)
			}
			require.NotContains(t, logs.String(), "other@example.test")
			require.NotContains(t, logs.String(), "secret credentials")
			require.NotContains(t, response.Body.String(), "secret")
			if tt.status == 405 {
				require.Equal(t, "POST", response.Header().Get("Allow"))
			}
		})
	}
}

func TestSMTP_DeliversPlainTextAndHandlesRejection(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "accepted"
		if reject {
			name = "rejected"
		}
		t.Run(name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			received := make(chan string, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					received <- ""
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				reader := textproto.NewReader(bufio.NewReader(conn))
				_, _ = io.WriteString(conn, "220 fake SMTP\r\n")
				for {
					line, err := reader.ReadLine()
					if err != nil {
						received <- ""
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
						_, _ = io.WriteString(conn, "250 fake\r\n")
					case strings.HasPrefix(line, "MAIL FROM:"):
						_, _ = io.WriteString(conn, "250 ok\r\n")
					case strings.HasPrefix(line, "RCPT TO:"):
						if reject {
							_, _ = io.WriteString(conn, "550 sensitive server rejection\r\n")
							received <- ""
							return
						}
						_, _ = io.WriteString(conn, "250 ok\r\n")
					case line == "DATA":
						_, _ = io.WriteString(conn, "354 send data\r\n")
						data, _ := io.ReadAll(reader.DotReader())
						_, _ = io.WriteString(conn, "250 accepted\r\n")
						received <- string(data)
						return
					default:
						received <- ""
						return
					}
				}
			}()
			delivery, err := sender.NewSMTP(listener.Addr().String(), "noreply@jobtrack.test")
			require.NoError(t, err)
			err = delivery.Send(context.Background(), sender.Message{Recipient: "student@example.test", Subject: "Update", Body: "Hello\n.dot line"})
			if reject {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			data := <-received
			if !reject {
				require.Contains(t, data, "To: student@example.test\n")
				require.Contains(t, data, "Content-Type: text/plain; charset=UTF-8\n")
				require.Contains(t, data, "Hello\n.dot line")
				require.Contains(t, data, "From: noreply@jobtrack.test\n")
			}
		})
	}
}
func TestSMTP_RejectsInvalidConfiguration(t *testing.T) {
	for _, tt := range []struct{ address, from string }{{"", "a@example.test"}, {"mailpit:0", "a@example.test"}, {"mailpit:65536", "a@example.test"}, {"mailpit:1025", "bad"}, {"mailpit:1025", "a@example.test\nBcc: b@example.test"}} {
		t.Run(tt.address+tt.from, func(t *testing.T) { _, err := sender.NewSMTP(tt.address, tt.from); require.Error(t, err) })
	}
}
