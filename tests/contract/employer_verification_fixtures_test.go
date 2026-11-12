package contract_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
)

var fixtureUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// The employer consumer tests (JT-56) load these same files, so a change here
// must still pass both this test and the consumer tests.
func TestEmployerVerificationFixtures_DecodeWithSharedEnvelope(t *testing.T) {
	tests := []struct {
		file   string
		check  func(t *testing.T, data proto.Message)
		dataTy proto.Message
	}{
		{
			file:   "employer.verified.json",
			dataTy: &eventsv1.EmployerVerified{},
			check: func(t *testing.T, data proto.Message) {
				e := data.(*eventsv1.EmployerVerified)
				require.Regexp(t, fixtureUUID, e.GetEmployerId())
			},
		},
		{
			file:   "employer.verification_rejected.json",
			dataTy: &eventsv1.EmployerVerificationRejected{},
			check: func(t *testing.T, data proto.Message) {
				e := data.(*eventsv1.EmployerVerificationRejected)
				require.Regexp(t, fixtureUUID, e.GetEmployerId())
				require.NotEmpty(t, e.GetReason())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("fixtures", tt.file))
			require.NoError(t, err)

			envelope, err := events.UnmarshalEnvelope(raw)
			require.NoError(t, err)
			// The file name is the routing key, so consumers can bind to it.
			require.Equal(t, tt.file, envelope.GetType()+".json")
			require.Equal(t, "company-verification", envelope.GetProducer())
			require.Regexp(t, fixtureUUID, envelope.GetEventId())

			data := tt.dataTy.ProtoReflect().New().Interface()
			require.NoError(t, envelope.GetData().UnmarshalTo(data))
			tt.check(t, data)

			// Encoding and decoding again keeps every field.
			encoded, err := events.MarshalEnvelope(envelope)
			require.NoError(t, err)
			again, err := events.UnmarshalEnvelope(encoded)
			require.NoError(t, err)
			require.True(t, proto.Equal(envelope, again))
		})
	}
}

// Order matters for the employer consumer: the rejection fixture is older,
// so the "latest occurred_at wins" tests can use the pair as they are.
func TestEmployerVerificationFixtures_RejectionIsOlderThanApproval(t *testing.T) {
	load := func(name string) *eventsv1.EventEnvelope {
		raw, err := os.ReadFile(filepath.Join("fixtures", name))
		require.NoError(t, err)
		envelope, err := events.UnmarshalEnvelope(raw)
		require.NoError(t, err)
		return envelope
	}
	verified := load("employer.verified.json")
	rejected := load("employer.verification_rejected.json")
	require.True(t, rejected.GetOccurredAt().AsTime().Before(verified.GetOccurredAt().AsTime()))
}
