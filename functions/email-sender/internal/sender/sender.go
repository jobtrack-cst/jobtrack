// Package sender implements the JT-77 HTTP-to-SMTP function.
package sender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaxRequestBytes = 128 * 1024

// Message is the agreed request shared by the two calling functions.
type Message struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

// Delivery is the SMTP port; success means the server accepted DATA.
type Delivery interface {
	Send(context.Context, Message) error
}

type Handler struct {
	Delivery Delivery
	Logger   *slog.Logger
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		http.Error(w, "application/json required", 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBytes)
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			http.Error(w, "request too large", 413)
		} else {
			http.Error(w, "invalid request", 400)
		}
		return
	}
	if !utf8.Valid(data) {
		http.Error(w, "invalid JSON", 400)
		return
	}
	var m Message
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if !ValidAddress(m.Recipient) || !validText(m.Subject, 200, false) || !validText(m.Body, 64*1024, true) {
		http.Error(w, "invalid recipient, subject or body", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), SMTPTimeout)
	defer cancel()
	if err := h.Delivery.Send(ctx, m); err != nil {
		if h.Logger != nil {
			h.Logger.Warn("SMTP delivery failed", "recipient", m.Recipient)
		}
		http.Error(w, "email delivery unavailable", 503)
		return
	}
	if h.Logger != nil {
		h.Logger.Info("email accepted", "recipient", m.Recipient)
	}
	w.WriteHeader(http.StatusNoContent)
}

func ValidAddress(value string) bool {
	if len(value) > 254 || !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n") {
		return false
	}
	for _, c := range value {
		if c > 127 || unicode.IsControl(c) {
			return false
		}
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Name == "" && address.Address == value && strings.Contains(value, "@")
}
func validText(value string, max int, multiline bool) bool {
	if !utf8.ValidString(value) || len(value) > max || strings.TrimSpace(value) == "" {
		return false
	}
	for _, c := range value {
		if unicode.IsControl(c) && !(multiline && (c == '\r' || c == '\n' || c == '\t')) {
			return false
		}
	}
	return true
}

// SMTP uses one bounded connection per request. Mailpit requires no credentials.
type SMTP struct{ Address, From string }

func NewSMTP(address, from string) (SMTP, error) {
	host, port, err := net.SplitHostPort(address)
	n, e := strconv.Atoi(port)
	if err != nil || host == "" || e != nil || n < 1 || n > 65535 || !ValidAddress(from) {
		return SMTP{}, errors.New("invalid SMTP_ADDR or SMTP_FROM")
	}
	return SMTP{Address: address, From: from}, nil
}
func (s SMTP) Send(ctx context.Context, m Message) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Address)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(SMTPTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	client := textproto.NewConn(conn)
	defer client.Close()
	if _, _, err = client.ReadResponse(220); err != nil {
		return err
	}
	command := func(code int, format string, args ...any) error {
		if err := client.PrintfLine(format, args...); err != nil {
			return err
		}
		_, _, err := client.ReadResponse(code)
		return err
	}
	if err = command(250, "HELO email-sender"); err != nil {
		return err
	}
	if err = command(250, "MAIL FROM:<%s>", s.From); err != nil {
		return err
	}
	if err = command(250, "RCPT TO:<%s>", m.Recipient); err != nil {
		return err
	}
	if err = command(354, "DATA"); err != nil {
		return err
	}
	writer := client.DotWriter()
	body := strings.ReplaceAll(strings.ReplaceAll(m.Body, "\r\n", "\n"), "\r", "\n")
	_, err = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", s.From, m.Recipient, mime.QEncoding.Encode("utf-8", m.Subject))
	if err != nil {
		return err
	}
	// Quoted-printable bounds SMTP line lengths and carries UTF-8 over 7-bit SMTP.
	encoded := quotedprintable.NewWriter(writer)
	if _, err = io.WriteString(encoded, strings.ReplaceAll(body, "\n", "\r\n")); err != nil {
		return err
	}
	if err = encoded.Close(); err != nil {
		return err
	}
	// DATA's final 250 response is the acceptance boundary; QUIT cannot undo it.
	if err = writer.Close(); err != nil {
		return err
	}
	_, _, err = client.ReadResponse(250)
	return err
}
