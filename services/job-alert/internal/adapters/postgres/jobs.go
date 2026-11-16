package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
)

type Jobs struct{ pool *pgxpool.Pool }

var _ ports.JobRepository = (*Jobs)(nil)

func NewJobs(pool *pgxpool.Pool) *Jobs { return &Jobs{pool: pool} }

func (r *Jobs) WithinTransaction(ctx context.Context, apply func(ports.JobTransaction) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return databaseError(err)
	}
	defer tx.Rollback(ctx)
	if err = apply(jobTransaction{tx: tx}); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return databaseError(err)
	}
	return nil
}

type jobTransaction struct{ tx pgx.Tx }

func (t jobTransaction) Claim(ctx context.Context, id string) (bool, error) {
	result, err := t.tx.Exec(ctx, "INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING", id)
	if err != nil {
		return false, databaseError(err)
	}
	return result.RowsAffected() > 0, nil
}
func (t jobTransaction) Candidates(ctx context.Context, job domain.PublishedJob) ([]domain.Alert, error) {
	// Narrow in SQL to avoid loading every subscription. Domain rechecks the rules.
	rows, err := t.tx.Query(ctx, `SELECT alert_id::text,applicant_id::text,category_ids,tag_ids,location FROM job_alerts
 WHERE (cardinality(category_ids)=0 OR $1=ANY(category_ids))
 AND (cardinality(tag_ids)=0 OR tag_ids && $2::text[])
 AND (location='' OR location=$3) ORDER BY alert_id`, job.CategoryID, job.TagIDs, job.Location)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	candidates := []domain.Alert{}
	for rows.Next() {
		var a domain.Alert
		if err = rows.Scan(&a.ID, &a.ApplicantID, &a.CategoryIDs, &a.TagIDs, &a.Location); err != nil {
			return nil, databaseError(err)
		}
		candidates = append(candidates, a)
	}
	if err = rows.Err(); err != nil {
		return nil, databaseError(err)
	}
	return candidates, nil
}
func (t jobTransaction) SaveMatches(ctx context.Context, eventID, jobID string, matches []domain.Match) error {
	for _, m := range matches {
		if _, err := t.tx.Exec(ctx, "INSERT INTO matched_subscriptions(source_event_id,alert_id,job_id,applicant_id) VALUES($1,$2,$3,$4)", eventID, m.AlertID, jobID, m.ApplicantID); err != nil {
			return databaseError(err)
		}
	}
	return nil
}
func (t jobTransaction) AddEvent(ctx context.Context, e *eventsv1.EventEnvelope) error {
	payload, err := events.MarshalEnvelope(e)
	if err != nil {
		return err
	}
	if _, err = t.tx.Exec(ctx, "INSERT INTO outbox_events(event_id,routing_key,payload,created_at) VALUES($1,$2,$3,$4)", e.EventId, e.Type, string(payload), e.OccurredAt.AsTime()); err != nil {
		return databaseError(err)
	}
	return nil
}
