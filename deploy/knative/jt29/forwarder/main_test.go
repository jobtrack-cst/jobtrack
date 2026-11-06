package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type acknowledgement struct {
	acked, rejected, requeued bool
}

func (a *acknowledgement) Ack(bool) error { a.acked = true; return nil }
func (a *acknowledgement) Nack(_ bool, requeue bool) error {
	a.rejected, a.requeued = true, requeue
	return nil
}

func TestForward_AcknowledgesOnlySuccessfulHTTPDelivery(t *testing.T) {
	data, err := anypb.New(&eventsv1.UserRegistered{UserId: "22222222-2222-4222-8222-222222222222"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := events.MarshalEnvelope(&eventsv1.EventEnvelope{
		EventId: "11111111-1111-4111-8111-111111111111", Type: "user.registered",
		OccurredAt: timestamppb.New(time.Now()), Producer: "jt29-spike", Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name                 string
		status               int
		poison, canceled     bool
		wantAck, wantRequeue bool
	}{
		{name: "200 acknowledged", status: 200, wantAck: true},
		{name: "204 acknowledged", status: 204, wantAck: true},
		{name: "500 requeued", status: 500, wantRequeue: true},
		{name: "400 requeued", status: 400, wantRequeue: true},
		{name: "redirect not acknowledged", status: 302, wantRequeue: true},
		{name: "invalid envelope rejected", poison: true},
		{name: "canceled request requeued", canceled: true, wantRequeue: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ack := &acknowledgement{}
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if ack.acked || ack.rejected {
					t.Error("message acknowledged before sink responded")
				}
				if r.Method != http.MethodPost || r.Header.Get("Ce-Id") != "11111111-1111-4111-8111-111111111111" ||
					r.Header.Get("Ce-Type") != "user.registered" || r.Header.Get("Ce-Specversion") != "1.0" ||
					r.Header.Get("Ce-Source") != "/jobtrack/jt29/rabbitmq" {
					t.Error("CloudEvent identity or method changed")
				}
				received, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(received, body) {
					t.Error("domain envelope changed in transit")
				}
				w.WriteHeader(tt.status)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.canceled {
				cancel()
			}
			payload := body
			if tt.poison {
				payload = []byte("invalid")
			}
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			err := forward(ctx, client, server.URL, payload, ack)
			if (err == nil) != tt.wantAck || ack.acked != tt.wantAck || ack.requeued != tt.wantRequeue || ack.rejected == tt.wantAck {
				t.Fatalf("unexpected result: ack=%+v err=%v", ack, err)
			}
			if tt.poison && called {
				t.Error("invalid event reached sink")
			}
		})
	}
}
