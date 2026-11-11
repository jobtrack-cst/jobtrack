package postgres

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
)

// SearchRepository reads the job read model. The query returns only open jobs
// (closed = FALSE and deadline after now), sorted by deadline soonest first
// with job_id breaking ties. Filters combine with AND; tag_ids use OR.
type SearchRepository struct {
	pool *pgxpool.Pool
}

func NewSearchRepository(pool *pgxpool.Pool) *SearchRepository {
	return &SearchRepository{pool: pool}
}

func (r *SearchRepository) Search(ctx context.Context, f domain.Filters) ([]domain.JobSummary, int, error) {
	// Build the WHERE clause with fixed placeholders. Filters are appended in
	// a fixed order so placeholder numbers stay consistent across queries.
	where := "closed = FALSE AND deadline > now()"
	args := []any{}

	add := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if f.Query != "" {
		where += " AND lower(title) LIKE '%' || " + add(f.Query) + " || '%'"
	}
	if f.CategoryID != "" {
		where += " AND category_id = " + add(f.CategoryID) + "::uuid"
	}
	if len(f.TagIDs) > 0 {
		// At least one tag matches. && is array overlap, so the OR stays in SQL.
		where += " AND tag_ids && " + add(f.TagIDs) + "::text[]"
	}
	if f.Location != "" {
		where += " AND location = " + add(f.Location)
	}

	// Total uses the same predicate and arguments.
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// The page query reuses the filter args, then appends LIMIT and OFFSET, so
	// their placeholder numbers must follow the filter args in the same slice.
	pageArgs := append([]any{}, args...)

	pageArgs = append(pageArgs, f.PageSize)
	limitPlaceholder := "$" + strconv.Itoa(len(pageArgs))

	pageArgs = append(pageArgs, f.PageOffset)
	offsetPlaceholder := "$" + strconv.Itoa(len(pageArgs))

	sql := "SELECT job_id, employer_id, title, coalesce(category_id::text, ''), " +
		"coalesce(tag_ids, '{}'), location, deadline " +
		"FROM jobs WHERE " + where +
		" ORDER BY deadline ASC, job_id ASC LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder

	rows, err := r.pool.Query(ctx, sql, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	jobs := make([]domain.JobSummary, 0, f.PageSize)
	for rows.Next() {
		var j domain.JobSummary
		if err := rows.Scan(&j.JobID, &j.EmployerID, &j.Title, &j.CategoryID, &j.TagIDs, &j.Location, &j.Deadline); err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}
