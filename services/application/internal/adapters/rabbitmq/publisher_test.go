package rabbitmq

import (
	"context"
	"errors"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testEnvelope(t *testing.T) *eventsv1.EventEnvelope {
	t.Helper()
	data, err := anypb.New(&eventsv1.ApplicationSubmitted{ApplicationId: "44444444-4444-4444-8444-444444444444", JobId: "55555555-5555-4555-8555-555555555555", ResumeId: "22222222-2222-4222-8222-222222222222", ApplicantId: "33333333-3333-4333-8333-333333333333", EmployerId: "66666666-6666-4666-8666-666666666666"})
	if err != nil {
		t.Fatal(err)
	}
	return &eventsv1.EventEnvelope{EventId: "11111111-1111-4111-8111-111111111111", Type: "application.submitted", Producer: "application", OccurredAt: &timestamppb.Timestamp{Seconds: 1793685600}, Data: data}
}

type stubPublisher struct {
	publish func(context.Context, *eventsv1.EventEnvelope) error
	close   func() error
}

func (p stubPublisher) Publish(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
	return p.publish(ctx, envelope)
}
func (p stubPublisher) Close() error { return p.close() }

func TestPublisher_DelegatesEnvelopeContextAndErrors(t *testing.T) {
	failure := errors.New("publish failed")
	for _, outcome := range []error{nil, failure, context.Canceled} {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if outcome == context.Canceled {
			cancel()
		}
		envelope := testEnvelope(t)
		calls := 0
		p := &Publisher{backend: stubPublisher{publish: func(got context.Context, event *eventsv1.EventEnvelope) error {
			calls++
			if got != ctx || event != envelope {
				t.Error("context or envelope changed")
			}
			return outcome
		}}}
		if err := p.Publish(ctx, envelope); !errors.Is(err, outcome) || calls != 1 {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
	}
}

func TestPublisher_CloseDelegatesResourceCleanup(t *testing.T) {
	failure := errors.New("close failed")
	calls := 0
	p := &Publisher{backend: stubPublisher{close: func() error { calls++; return failure }}}
	if err := p.Close(); !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestNewPublisher_RejectsBlankURL(t *testing.T) {
	for _, url := range []string{"", " \t"} {
		if p, err := NewPublisher(url); err == nil || p != nil {
			t.Fatal("blank URL accepted")
		}
	}
}
