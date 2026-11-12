package contract_test

import (
	"os"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestResumeEventFixtures(t *testing.T) {
	for _, tc := range []struct {
		name, producer string
		payload        proto.Message
	}{
		{"uploaded", "resume", &eventsv1.ResumeUploaded{}}, {"validated", "resume-validator", &eventsv1.ResumeValidated{}}, {"rejected", "resume-validator", &eventsv1.ResumeRejected{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := os.ReadFile("fixtures/resume." + tc.name + ".json")
			require.NoError(t, err)
			e, err := events.UnmarshalEnvelope(body)
			require.NoError(t, err)
			require.Equal(t, "resume."+tc.name, e.Type)
			require.Equal(t, tc.producer, e.Producer)
			require.NoError(t, e.Data.UnmarshalTo(tc.payload))
			fields := tc.payload.ProtoReflect().Descriptor().Fields()
			require.Equal(t, "11111111-1111-4111-8111-111111111111", tc.payload.ProtoReflect().Get(fields.ByName("resume_id")).String())
			require.Equal(t, "22222222-2222-4222-8222-222222222222", tc.payload.ProtoReflect().Get(fields.ByName("applicant_id")).String())
			if r, ok := tc.payload.(*eventsv1.ResumeRejected); ok {
				require.Equal(t, "Invalid PDF signature", r.Reason)
			}
			encoded, err := events.MarshalEnvelope(e)
			require.NoError(t, err)
			decoded, err := events.UnmarshalEnvelope(encoded)
			require.NoError(t, err)
			require.True(t, proto.Equal(e, decoded))
		})
	}
}
