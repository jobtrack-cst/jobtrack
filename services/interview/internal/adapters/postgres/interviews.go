package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/ports"
)

// Participants reads the application_participants projection. The adapter
// never owns the pool.
type Participants struct{ pool *pgxpool.Pool }

var _ ports.ParticipantRepository = (*Participants)(nil)

func NewParticipants(pool *pgxpool.Pool) *Participants { return &Participants{pool: pool} }

func (r *Participants) GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationParticipants, error) {
	var p domain.ApplicationParticipants
	err := r.pool.QueryRow(ctx,
		`SELECT application_id, applicant_id, employer_id FROM application_participants WHERE application_id = $1`,
		applicationID).Scan(&p.ApplicationID, &p.ApplicantID, &p.EmployerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ApplicationParticipants{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.ApplicationParticipants{}, hideDriverError(ctx, "get application participants failed")
	}
	return p, nil
}

const interviewColumns = "interview_id, application_id, employer_id, applicant_id, scheduled_at, details, created_at, updated_at"

// Interviews stores interviews. The unique (application_id, scheduled_at)
// keeps "not two at the same time" true even for simultaneous requests.
type Interviews struct{ pool *pgxpool.Pool }

var _ ports.InterviewRepository = (*Interviews)(nil)

func NewInterviews(pool *pgxpool.Pool) *Interviews { return &Interviews{pool: pool} }

func (r *Interviews) Create(ctx context.Context, iv domain.Interview) error {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO interviews (`+interviewColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (application_id, scheduled_at) DO NOTHING`,
		iv.InterviewID, iv.ApplicationID, iv.EmployerID, iv.ApplicantID, iv.ScheduledAt, iv.Details, iv.CreatedAt, iv.UpdatedAt)
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return hideDriverError(ctx, "create interview failed")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInterviewExists
	}
	return nil
}

func (r *Interviews) Get(ctx context.Context, interviewID string) (domain.Interview, error) {
	iv, err := scanInterview(r.pool.QueryRow(ctx,
		`SELECT `+interviewColumns+` FROM interviews WHERE interview_id = $1`, interviewID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Interview{}, domain.ErrInterviewNotFound
	}
	if err != nil {
		return domain.Interview{}, hideDriverError(ctx, "get interview failed")
	}
	return iv, nil
}

// ListByApplication runs the count and the page in one read-only snapshot, so
// total_count matches the rows returned.
func (r *Interviews) ListByApplication(ctx context.Context, applicationID string, page domain.Page) (interviews []domain.Interview, total int, err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list interviews failed")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `SELECT count(*) FROM interviews WHERE application_id = $1`, applicationID).Scan(&total); err != nil {
		return nil, 0, hideDriverError(ctx, "count interviews failed")
	}
	rows, err := tx.Query(ctx,
		`SELECT `+interviewColumns+` FROM interviews WHERE application_id = $1
		 ORDER BY scheduled_at, interview_id LIMIT $2 OFFSET $3`,
		applicationID, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list interviews failed")
	}
	defer rows.Close()
	interviews = []domain.Interview{}
	for rows.Next() {
		iv, err := scanInterview(rows)
		if err != nil {
			return nil, 0, hideDriverError(ctx, "list interviews failed")
		}
		interviews = append(interviews, iv)
	}
	if rows.Err() != nil {
		return nil, 0, hideDriverError(ctx, "list interviews failed")
	}
	return interviews, total, nil
}

// scanInterview returns times in UTC, as AGENTS.md Section 9 requires.
func scanInterview(row pgx.Row) (domain.Interview, error) {
	var iv domain.Interview
	if err := row.Scan(&iv.InterviewID, &iv.ApplicationID, &iv.EmployerID, &iv.ApplicantID,
		&iv.ScheduledAt, &iv.Details, &iv.CreatedAt, &iv.UpdatedAt); err != nil {
		return domain.Interview{}, err
	}
	iv.ScheduledAt, iv.CreatedAt, iv.UpdatedAt = iv.ScheduledAt.UTC(), iv.CreatedAt.UTC(), iv.UpdatedAt.UTC()
	return iv, nil
}

// hideDriverError keeps context cancellation visible but hides driver details.
func hideDriverError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
