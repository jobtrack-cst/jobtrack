// Package emailsender calls the email-sender function
// (docs/deployment/email-sender-http-contract.md).
package emailsender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Timeout is the contract's suggested caller timeout: above email-sender's
// 22 s write budget plus network margin.
const Timeout = 30 * time.Second

var ErrInvalidURL = errors.New("EMAIL_SENDER_URL must be an absolute http or https URL with a host")

// Client implements ports.Mailer. URL is the full endpoint, for example
// http://email-sender:8080/send-email.
type Client struct {
	url  string
	http *http.Client
}

func New(endpoint string) (Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Client{}, ErrInvalidURL
	}
	return Client{url: u.String(), http: &http.Client{
		Timeout: Timeout,
		// A redirect would turn the POST into a different request; treat it as a failure.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

type request struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

// Send succeeds only on 204, which means SMTP accepted the email.
func (c Client) Send(ctx context.Context, recipient, subject, body string) error {
	payload, err := json.Marshal(request{Recipient: recipient, Subject: subject, Body: body})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("email-sender request failed")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("email-sender returned HTTP %d", resp.StatusCode)
	}
	return nil
}
