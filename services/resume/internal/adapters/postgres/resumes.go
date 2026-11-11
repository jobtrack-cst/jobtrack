package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// resumeColumns never includes content, so metadata reads stay small.
const resumeColumns = "resume_id, applicant_id, file_name, content_type, size_bytes, validation_status, rejection_reason, created_at, updated_at"

// Resumes stores resumes in the resume database. The adapter never owns the pool.
type Resumes struct{ pool *pgxpool.Pool }

var _ ports.ResumeRepository = (*Resumes)(nil)

func NewResumes(pool *pgxpool.Pool) *Resumes { return &Resumes{pool: pool} }

func (r *Resumes) Create(ctx context.Context, f domain.ResumeFile) (domain.Resume, error) {
	if _, ok := domain.ParseValidationStatus(f.ValidationStatus.String()); !ok {
		return domain.Resume{}, errors.New("resume validation status is not valid")
	}
	created, err := scanResume(r.pool.QueryRow(ctx,
		`INSERT INTO resumes (resume_id, applicant_id, file_name, content_type, size_bytes,
		     content, validation_status, rejection_reason, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING `+resumeColumns,
		f.ID, f.ApplicantID, f.FileName, f.ContentType, f.SizeBytes,
		f.Content, f.ValidationStatus.String(), f.RejectionReason, f.CreatedAt, f.UpdatedAt))
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return domain.Resume{}, hideDriverError(ctx, "create resume failed")
	}
	return created, nil
}

func (r *Resumes) GetByID(ctx context.Context, resumeID string) (domain.Resume, error) {
	resume, err := scanResume(r.pool.QueryRow(ctx,
		`SELECT `+resumeColumns+` FROM resumes WHERE resume_id = $1`, resumeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Resume{}, domain.ErrResumeNotFound
	}
	if err != nil {
		return domain.Resume{}, hideDriverError(ctx, "get resume failed")
	}
	return resume, nil
}

func (r *Resumes) GetFile(ctx context.Context, resumeID string) (domain.ResumeFile, error) {
	var f domain.ResumeFile
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT `+resumeColumns+`, content FROM resumes WHERE resume_id = $1`, resumeID).
		Scan(append(resumeDest(&f.Resume, &status), &f.Content)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ResumeFile{}, domain.ErrResumeNotFound
	}
	if err != nil {
		return domain.ResumeFile{}, hideDriverError(ctx, "get resume file failed")
	}
	if err := finishResume(&f.Resume, status); err != nil {
		return domain.ResumeFile{}, err
	}
	return f, nil
}

// ListByApplicant runs the count and the page in one read-only snapshot, so
// total_count matches the rows returned.
func (r *Resumes) ListByApplicant(ctx context.Context, applicantID string, page domain.Page) (resumes []domain.Resume, total int, err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list resumes failed")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `SELECT count(*) FROM resumes WHERE applicant_id = $1`, applicantID).Scan(&total); err != nil {
		return nil, 0, hideDriverError(ctx, "count resumes failed")
	}
	rows, err := tx.Query(ctx,
		`SELECT `+resumeColumns+` FROM resumes WHERE applicant_id = $1
		 ORDER BY created_at DESC, resume_id DESC LIMIT $2 OFFSET $3`,
		applicantID, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list resumes failed")
	}
	defer rows.Close()
	resumes = []domain.Resume{}
	for rows.Next() {
		resume, err := scanResume(rows)
		if err != nil {
			return nil, 0, hideDriverError(ctx, "list resumes failed")
		}
		resumes = append(resumes, resume)
	}
	if rows.Err() != nil {
		return nil, 0, hideDriverError(ctx, "list resumes failed")
	}
	return resumes, total, nil
}

// resumeDest lists the scan targets for resumeColumns, in order.
func resumeDest(r *domain.Resume, status *string) []any {
	return []any{&r.ID, &r.ApplicantID, &r.FileName, &r.ContentType, &r.SizeBytes,
		status, &r.RejectionReason, &r.CreatedAt, &r.UpdatedAt}
}

func scanResume(row pgx.Row) (domain.Resume, error) {
	var r domain.Resume
	var status string
	if err := row.Scan(resumeDest(&r, &status)...); err != nil {
		return domain.Resume{}, err
	}
	if err := finishResume(&r, status); err != nil {
		return domain.Resume{}, err
	}
	return r, nil
}

// finishResume parses the stored status and returns times in UTC, as
// AGENTS.md Section 9 requires.
func finishResume(r *domain.Resume, status string) error {
	parsed, ok := domain.ParseValidationStatus(status)
	if !ok {
		return errors.New("stored resume validation status is not valid")
	}
	r.ValidationStatus = parsed
	r.CreatedAt, r.UpdatedAt = r.CreatedAt.UTC(), r.UpdatedAt.UTC()
	return nil
}

// hideDriverError keeps context cancellation visible but hides driver details.
func hideDriverError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
