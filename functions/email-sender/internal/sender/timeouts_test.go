package sender_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/email-sender/internal/sender"
	"github.com/stretchr/testify/require"
)

// final runs after the complete SMTP DATA has arrived, before acceptance.
func smtpServer(t *testing.T, final func(net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("SMTP server did not stop")
		}
	})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		reader := textproto.NewReader(bufio.NewReader(conn))
		_, _ = io.WriteString(conn, "220 fake SMTP\r\n")
		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}
			if line == "DATA" {
				_, _ = io.WriteString(conn, "354 send data\r\n")
				if _, err = io.Copy(io.Discard, reader.DotReader()); err != nil {
					return
				}
				final(conn)
				return
			}
			_, _ = io.WriteString(conn, "250 ok\r\n")
		}
	}()
	return listener.Addr().String()
}

func serveHTTP(t *testing.T, server *http.Server) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close(); require.ErrorIs(t, <-done, http.ErrServerClosed) })
	return listener.Addr().String()
}

const emailJSON = `{"recipient":"student@example.test","subject":"Update","body":"Synthetic test"}`

func TestServer_DelayedBodyAndSMTP_ReturnsAcceptedBeforeWriteDeadline(t *testing.T) {
	// Scale only the HTTP deadlines by ten. The real handler/SMTP adapter retain
	// their production budget; each fake SMTP operation completes well within it.
	for _, tt := range []struct {
		name    string
		write   time.Duration
		want204 bool
	}{
		{"old deadline loses response after acceptance", 1200 * time.Millisecond, false},
		{"aligned deadline preserves response", sender.WriteTimeout / 10, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			accepted := make(chan struct{})
			smtpAddress := smtpServer(t, func(conn net.Conn) {
				time.Sleep(700 * time.Millisecond)
				_, _ = io.WriteString(conn, "250 accepted\r\n")
				close(accepted)
			})
			delivery, err := sender.NewSMTP(smtpAddress, "noreply@jobtrack.test")
			require.NoError(t, err)
			headersRead := make(chan struct{})
			handler := sender.Handler{Delivery: delivery}
			server := sender.NewServer("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(headersRead)
				handler.ServeHTTP(w, r)
			}))
			server.ReadTimeout = sender.ReadTimeout / 10
			server.WriteTimeout = tt.write
			address := serveHTTP(t, server)
			conn, err := net.DialTimeout("tcp", address, time.Second)
			require.NoError(t, err)
			defer conn.Close()
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
			_, err = fmt.Fprintf(conn, "POST /send-email HTTP/1.1\r\nHost: test\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", len(emailJSON))
			require.NoError(t, err)
			// A partial body makes the server start its body read before the delay.
			_, err = io.WriteString(conn, emailJSON[:1])
			require.NoError(t, err)
			select {
			case <-headersRead:
			case <-time.After(5 * time.Second):
				t.Fatal("HTTP headers not read")
			}
			time.Sleep(700 * time.Millisecond)
			_, err = io.WriteString(conn, emailJSON[1:])
			require.NoError(t, err)
			response, err := http.ReadResponse(bufio.NewReader(conn), nil)
			select {
			case <-accepted:
			case <-time.After(5 * time.Second):
				t.Fatal("SMTP did not accept")
			}
			if tt.want204 {
				require.NoError(t, err)
				defer response.Body.Close()
				require.Equal(t, 204, response.StatusCode)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestSMTP_StallOrRejectionAfterDATA_NeverReportsSuccess(t *testing.T) {
	for _, mode := range []string{"stalled DATA response", "rejected DATA"} {
		t.Run(mode, func(t *testing.T) {
			reached := make(chan struct{})
			address := smtpServer(t, func(conn net.Conn) {
				close(reached)
				if mode == "rejected DATA" {
					_, _ = io.WriteString(conn, "550 rejected\r\n")
					return
				}
				_, _ = io.Copy(io.Discard, conn)
			})
			delivery, err := sender.NewSMTP(address, "noreply@jobtrack.test")
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			err = delivery.Send(ctx, sender.Message{Recipient: "student@example.test", Subject: "Update", Body: "Test"})
			require.Error(t, err)
			select {
			case <-reached:
			default:
				t.Fatal("did not reach DATA response")
			}
		})
	}
}

func TestServer_ShutdownDrainsInFlightDelivery(t *testing.T) {
	dataRead := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	address := smtpServer(t, func(conn net.Conn) { close(dataRead); <-release; _, _ = io.WriteString(conn, "250 accepted\r\n") })
	delivery, err := sender.NewSMTP(address, "noreply@jobtrack.test")
	require.NoError(t, err)
	server := sender.NewServer("", sender.Handler{Delivery: delivery})
	httpAddress := serveHTTP(t, server)
	result := make(chan error, 1)
	go func() {
		client := http.Client{Timeout: 5 * time.Second}
		response, err := client.Post("http://"+httpAddress+"/send-email", "application/json", strings.NewReader(emailJSON))
		if err == nil {
			defer response.Body.Close()
			if response.StatusCode != 204 {
				err = fmt.Errorf("status %d", response.StatusCode)
			}
		}
		result <- err
	}()
	select {
	case <-dataRead:
	case <-time.After(5 * time.Second):
		t.Fatal("DATA not received")
	}
	stopping := make(chan struct{})
	server.RegisterOnShutdown(func() { close(stopping) })
	shutdown := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), sender.ShutdownTimeout)
		defer cancel()
		shutdown <- server.Shutdown(ctx)
	}()
	select {
	case <-stopping:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown not started")
	}
	select {
	case err := <-shutdown:
		t.Fatalf("shutdown returned before delivery: %v", err)
	default:
	}
	release <- struct{}{}
	require.NoError(t, <-result)
	require.NoError(t, <-shutdown)
}

func TestServer_ProductionBudgetsCoverSequentialWork(t *testing.T) {
	server := sender.NewServer("", http.NotFoundHandler())
	require.Equal(t, 10*time.Second, server.ReadTimeout)
	require.Equal(t, 8*time.Second, sender.SMTPTimeout)
	require.GreaterOrEqual(t, server.WriteTimeout, server.ReadTimeout+sender.SMTPTimeout+4*time.Second)
	require.GreaterOrEqual(t, sender.ShutdownTimeout, server.ReadTimeout+sender.SMTPTimeout+4*time.Second)
}
