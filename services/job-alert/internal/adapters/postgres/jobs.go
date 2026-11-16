package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RecordJob claims the delivery, snapshots matching subscriptions and stores notifications atomically.
func (r *Alerts) RecordJob(ctx context.Context, eventID string, job *eventsv1.JobPublished) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return databaseError(err)
	}
	defer tx.Rollback(ctx)
	claim, err := tx.Exec(ctx, "INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING", eventID)
	if err != nil {
		return databaseError(err)
	}
	if claim.RowsAffected() == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT alert_id::text,applicant_id::text FROM job_alerts
 WHERE (cardinality(category_ids)=0 OR $1=ANY(category_ids))
 AND (cardinality(tag_ids)=0 OR tag_ids && $2::text[])
 AND (location='' OR location=$3) ORDER BY alert_id`, job.CategoryId, job.TagIds, job.Location)
	if err != nil {
		return databaseError(err)
	}
	type match struct{ alert, applicant string }
	matches := []match{}
	for rows.Next() {
		var m match
		if err = rows.Scan(&m.alert, &m.applicant); err != nil {
			rows.Close()
			return databaseError(err)
		}
		matches = append(matches, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return databaseError(err)
	}
	now := time.Now().UTC()
	for _, m := range matches {
		if _, err = tx.Exec(ctx, "INSERT INTO matched_subscriptions(source_event_id,alert_id,job_id,applicant_id) VALUES($1,$2,$3,$4)", eventID, m.alert, job.JobId, m.applicant); err != nil {
			return databaseError(err)
		}
		data, err := anypb.New(&eventsv1.JobAlertMatched{JobId: job.JobId, ApplicantId: m.applicant})
		if err != nil {
			return err
		}
		envelope := &eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: "job_alert.matched", Producer: "job-alert", OccurredAt: timestamppb.New(now), Data: data}
		payload, err := events.MarshalEnvelope(envelope)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO outbox_events(event_id,routing_key,payload,created_at) VALUES($1,$2,$3,$4)", envelope.EventId, envelope.Type, string(payload), now); err != nil {
			return databaseError(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return databaseError(err)
	}
	return nil
}
