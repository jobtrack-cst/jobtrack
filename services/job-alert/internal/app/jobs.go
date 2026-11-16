package app

import (
	"context"
	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type Jobs struct {
	repo  ports.JobRepository
	clock ports.Clock
}

func NewJobs(repo ports.JobRepository, clock ports.Clock) *Jobs {
	return &Jobs{repo: repo, clock: clock}
}

// RecordJob owns the matching snapshot and notification policy inside one transaction.
func (s *Jobs) RecordJob(ctx context.Context, eventID string, data *eventsv1.JobPublished) error {
	job := domain.PublishedJob{ID: data.JobId, CategoryID: data.CategoryId, TagIDs: data.TagIds, Location: data.Location}
	return s.repo.WithinTransaction(ctx, func(tx ports.JobTransaction) error {
		claimed, err := tx.Claim(ctx, eventID)
		if err != nil || !claimed {
			return err
		}
		candidates, err := tx.Candidates(ctx, job)
		if err != nil {
			return err
		}
		matches := domain.PlanMatches(job, candidates)
		if err = tx.SaveMatches(ctx, eventID, job.ID, matches); err != nil {
			return err
		}
		now := time.Now().UTC()
		if s.clock != nil {
			now = s.clock.Now().UTC()
		}
		for _, applicant := range domain.MatchApplicants(matches) {
			packed, err := anypb.New(&eventsv1.JobAlertMatched{JobId: job.ID, ApplicantId: applicant})
			if err != nil {
				return err
			}
			e := &eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: "job_alert.matched", Producer: "job-alert", OccurredAt: timestamppb.New(now), Data: packed}
			if err = tx.AddEvent(ctx, e); err != nil {
				return err
			}
		}
		return nil
	})
}
