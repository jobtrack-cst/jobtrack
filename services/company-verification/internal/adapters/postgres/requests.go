package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports"
)

const (
	uniqueViolation  = "23505"
	onePendingIndex  = "verification_requests_one_pending"
	oneApprovedIndex = "verification_requests_one_approved"
)

const requestColumns = "id, employer_id, registration_number, notes, status, rejection_reason, reviewed_by, created_at, updated_at"

// Requests stores verification requests in the company_verification
// database. The adapter never owns the pool.
type Requests struct{ pool *pgxpool.Pool }

var _ ports.RequestRepository = (*Requests)(nil)

func NewRequests(pool *pgxpool.Pool) *Requests { return &Requests{pool: pool} }

func (r *Requests) Create(ctx context.Context, v domain.VerificationRequest) (domain.VerificationRequest, error) {
	status, ok := statusText(v.Status)
	if !ok {
		return domain.VerificationRequest{}, errors.New("request status is not valid")
	}
	created, err := scanRequest(r.pool.QueryRow(ctx,
		`INSERT INTO verification_requests (id, employer_id, registration_number, notes, status,
		     rejection_reason, reviewed_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+requestColumns,
		v.ID, v.EmployerID, v.RegistrationNumber, v.Notes, status,
		v.RejectionReason, nullable(v.ReviewedBy), v.CreatedAt, v.UpdatedAt))
	if mapped := uniqueError(err); mapped != nil {
		return domain.VerificationRequest{}, mapped
	}
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return domain.VerificationRequest{}, safe(ctx, "create verification request failed")
	}
	return created, nil
}

func (r *Requests) StatusesByEmployer(ctx context.Context, employerID string) ([]domain.RequestStatus, error) {
	rows, err := r.pool.Query(ctx, `SELECT status FROM verification_requests WHERE employer_id = $1`, employerID)
	if err != nil {
		return nil, safe(ctx, "list request statuses failed")
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, safe(ctx, "list request statuses failed")
	}
	statuses := make([]domain.RequestStatus, 0, len(names))
	for _, name := range names {
		s, ok := parseStatus(name)
		if !ok {
			return nil, errors.New("stored request status is not valid")
		}
		statuses = append(statuses, s)
	}
	return statuses, nil
}

func (r *Requests) GetByID(ctx context.Context, id string) (domain.VerificationRequest, error) {
	v, err := scanRequest(r.pool.QueryRow(ctx,
		`SELECT `+requestColumns+` FROM verification_requests WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VerificationRequest{}, domain.ErrRequestNotFound
	}
	if err != nil {
		return domain.VerificationRequest{}, safe(ctx, "get verification request failed")
	}
	return v, nil
}

func (r *Requests) ListByEmployer(ctx context.Context, employerID string, page domain.Page) ([]domain.VerificationRequest, int64, error) {
	return r.list(ctx,
		`WHERE employer_id = $1`, `ORDER BY created_at DESC, id DESC`,
		page, employerID)
}

func (r *Requests) List(ctx context.Context, status domain.RequestStatus, page domain.Page) ([]domain.VerificationRequest, int64, error) {
	if status == domain.StatusUnspecified {
		return r.list(ctx, ``, `ORDER BY created_at, id`, page)
	}
	name, ok := statusText(status)
	if !ok {
		return nil, 0, errors.New("status filter is not valid")
	}
	return r.list(ctx, `WHERE status = $1`, `ORDER BY created_at, id`, page, name)
}

// list runs the count and the page in one read-only snapshot, so total_count
// matches the rows returned. where and order are fixed strings from this file.
func (r *Requests) list(ctx context.Context, where, order string, page domain.Page, args ...any) ([]domain.VerificationRequest, int64, error) {
	var items []domain.VerificationRequest
	var total int64
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM verification_requests `+where, args...).Scan(&total); err != nil {
			return err
		}
		n := len(args)
		rows, err := tx.Query(ctx,
			`SELECT `+requestColumns+` FROM verification_requests `+where+` `+order+
				` LIMIT $`+strconv.Itoa(n+1)+` OFFSET $`+strconv.Itoa(n+2),
			append(args, page.Limit, page.Offset)...)
		if err != nil {
			return err
		}
		items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.VerificationRequest, error) {
			return scanRequest(row)
		})
		return err
	})
	if err != nil {
		return nil, 0, safe(ctx, "list verification requests failed")
	}
	return items, total, nil
}

// Decide only changes a row that is still PENDING, so two admins deciding at
// once cannot both succeed.
func (r *Requests) Decide(ctx context.Context, v domain.VerificationRequest) (domain.VerificationRequest, error) {
	status, ok := statusText(v.Status)
	if !ok || v.Status == domain.StatusPending {
		return domain.VerificationRequest{}, errors.New("decision status is not valid")
	}
	decided, err := scanRequest(r.pool.QueryRow(ctx,
		`UPDATE verification_requests
		 SET status = $2, rejection_reason = $3, reviewed_by = $4, updated_at = $5
		 WHERE id = $1 AND status = 'pending'
		 RETURNING `+requestColumns,
		v.ID, status, v.RejectionReason, nullable(v.ReviewedBy), v.UpdatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VerificationRequest{}, r.whyNotDecided(ctx, v.ID)
	}
	if mapped := uniqueError(err); mapped != nil {
		return domain.VerificationRequest{}, mapped
	}
	if err != nil {
		return domain.VerificationRequest{}, safe(ctx, "decide verification request failed")
	}
	return decided, nil
}

// whyNotDecided tells a missing row apart from one already decided.
func (r *Requests) whyNotDecided(ctx context.Context, id string) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM verification_requests WHERE id = $1)`, id).Scan(&exists); err != nil {
		return safe(ctx, "decide verification request failed")
	}
	if !exists {
		return domain.ErrRequestNotFound
	}
	return domain.ErrRequestNotPending
}

func uniqueError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return nil
	}
	switch pgErr.ConstraintName {
	case onePendingIndex:
		return domain.ErrPendingRequestExists
	case oneApprovedIndex:
		return domain.ErrAlreadyVerified
	default:
		return nil
	}
}

func scanRequest(row pgx.Row) (domain.VerificationRequest, error) {
	var v domain.VerificationRequest
	var status string
	var reviewedBy *string
	if err := row.Scan(&v.ID, &v.EmployerID, &v.RegistrationNumber, &v.Notes, &status,
		&v.RejectionReason, &reviewedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return domain.VerificationRequest{}, err
	}
	parsed, ok := parseStatus(status)
	if !ok {
		return domain.VerificationRequest{}, errors.New("stored request status is not valid")
	}
	v.Status = parsed
	if reviewedBy != nil {
		v.ReviewedBy = *reviewedBy
	}
	// Store and return UTC, as AGENTS.md Section 9 requires.
	v.CreatedAt, v.UpdatedAt = v.CreatedAt.UTC(), v.UpdatedAt.UTC()
	return v, nil
}

// nullable stores an empty reviewer as NULL, which the CHECK constraint expects.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var statusNames = map[domain.RequestStatus]string{
	domain.StatusPending:  "pending",
	domain.StatusApproved: "approved",
	domain.StatusRejected: "rejected",
}

func statusText(s domain.RequestStatus) (string, bool) {
	name, ok := statusNames[s]
	return name, ok
}

func parseStatus(name string) (domain.RequestStatus, bool) {
	for s, n := range statusNames {
		if n == name {
			return s, true
		}
	}
	return domain.StatusUnspecified, false
}

// safe keeps context cancellation visible but hides driver details.
func safe(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
