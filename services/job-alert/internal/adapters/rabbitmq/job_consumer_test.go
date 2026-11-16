package rabbitmq_test

import (
	"context"
	"encoding/json"
	"errors"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/rabbitmq"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"os"
	"testing"
)

type recorder struct {
	calls int
	job   *eventsv1.JobPublished
	err   error
}

func (r *recorder) RecordJob(_ context.Context, _ string, job *eventsv1.JobPublished) error {
	r.job = job
	r.calls++
	return r.err
}
func TestJobHandler_ValidationAndRetry(t *testing.T) {
	raw, err := os.ReadFile("../../../../../tests/contract/fixtures/job.published.json")
	require.NoError(t, err)
	original, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	for _, name := range []string{"valid", "nil", "producer", "type", "payload", "id", "timestamp", "job id", "deadline", "database"} {
		t.Run(name, func(t *testing.T) {
			e := proto.Clone(original).(*eventsv1.EventEnvelope)
			r := &recorder{}
			switch name {
			case "nil":
				e = nil
			case "producer":
				e.Producer = "offer"
			case "type":
				e.Type = "job.closed"
			case "payload":
				e.Data, _ = anypb.New(&eventsv1.JobAlertMatched{})
			case "id":
				e.EventId = "bad"
			case "timestamp":
				e.OccurredAt = nil
			case "job id", "deadline":
				var j eventsv1.JobPublished
				require.NoError(t, e.Data.UnmarshalTo(&j))
				if name == "job id" {
					j.JobId = "bad"
				} else {
					j.Deadline = nil
				}
				e.Data, _ = anypb.New(&j)
			case "database":
				r.err = errors.New("database unavailable")
			}
			err := rabbitmq.JobHandler(r)(context.Background(), e)
			if name == "valid" {
				require.NoError(t, err)
				require.Equal(t, 1, r.calls)
			} else if name == "database" {
				require.ErrorIs(t, err, r.err)
			} else {
				require.Error(t, err)
				require.ErrorIs(t, err, events.ErrPermanent)
				require.Zero(t, r.calls)
			}
		})
	}
}

// A missing proto3 category_id decodes to the permitted empty value.
func TestJobHandler_CategorylessJobsValidateRemainingIDs(t *testing.T) {
	raw, err := os.ReadFile("../../../../../tests/contract/fixtures/job.published.json")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(raw, &document))
	delete(document["data"].(map[string]any), "category_id")
	raw, err = json.Marshal(document)
	require.NoError(t, err)
	original, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		change func(*eventsv1.JobPublished)
		valid  bool
	}{
		{"missing category", func(*eventsv1.JobPublished) {}, true},
		{"malformed category", func(j *eventsv1.JobPublished) { j.CategoryId = "bad" }, false},
		{"whitespace category", func(j *eventsv1.JobPublished) { j.CategoryId = " " }, false},
		{"missing job", func(j *eventsv1.JobPublished) { j.JobId = "" }, false},
		{"malformed job", func(j *eventsv1.JobPublished) { j.JobId = "bad" }, false},
		{"missing employer", func(j *eventsv1.JobPublished) { j.EmployerId = "" }, false},
		{"malformed employer", func(j *eventsv1.JobPublished) { j.EmployerId = "bad" }, false},
		{"empty tag", func(j *eventsv1.JobPublished) { j.TagIds = append(j.TagIds, "") }, false},
		{"malformed tag", func(j *eventsv1.JobPublished) { j.TagIds = append(j.TagIds, "bad") }, false},
		{"no tags", func(j *eventsv1.JobPublished) { j.TagIds = nil }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := proto.Clone(original).(*eventsv1.EventEnvelope)
			var job eventsv1.JobPublished
			require.NoError(t, e.Data.UnmarshalTo(&job))
			tc.change(&job)
			e.Data, err = anypb.New(&job)
			require.NoError(t, err)
			r := &recorder{}
			err := rabbitmq.JobHandler(r)(context.Background(), e)
			if tc.valid {
				require.NoError(t, err)
				require.Equal(t, 1, r.calls)
				require.Empty(t, r.job.CategoryId)
				require.True(t, proto.Equal(&job, r.job))
			} else {
				require.ErrorIs(t, err, events.ErrPermanent)
				require.Zero(t, r.calls)
			}
		})
	}
}
