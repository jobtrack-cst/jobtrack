package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits in characters, after trimming spaces. Noted in the JT-34 PR
// and used for boundary value tests.
const (
	MaxReportReasonLength  = 1000
	MaxRemovalReasonLength = 500
)

// ReportStatus values match jobtrack.moderation.v1.ReportStatus.
type ReportStatus uint8

const (
	StatusUnspecified ReportStatus = iota
	StatusOpen
	StatusDismissed
	StatusActioned
)

// ParseStatusFilter converts the proto enum number for ListReports.
// StatusUnspecified means all statuses. Checking the int32 range first stops
// a large number from wrapping into a valid uint8 value.
func ParseStatusFilter(n int32) (ReportStatus, error) {
	if n < 0 || n > int32(StatusActioned) {
		return StatusUnspecified, ErrInvalidStatusFilter
	}
	return ReportStatus(n), nil
}

// Report is one user's report on a job posting.
type Report struct {
	ID         string
	JobID      string
	ReporterID string
	Reason     string
	Status     ReportStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewReportReason validates the reporter's reason. Empty is INVALID_ARGUMENT
// as the proto says.
func NewReportReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if !textWithin(reason, 1, MaxReportReasonLength) {
		return "", ErrInvalidReportReason
	}
	return reason, nil
}

// NewRemovalReason validates the admin's takedown reason, which Sprint 2
// sends in moderation.job_removed.
func NewRemovalReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if !textWithin(reason, 1, MaxRemovalReasonLength) {
		return "", ErrInvalidRemovalReason
	}
	return reason, nil
}

// textWithin counts characters, not bytes. NUL is refused because Postgres
// text columns cannot store it, which would surface as INTERNAL.
func textWithin(s string, minLen, maxLen int) bool {
	if strings.ContainsRune(s, 0) || !utf8.ValidString(s) {
		return false
	}
	n := utf8.RuneCountInString(s)
	return n >= minLen && n <= maxLen
}

// NewReport starts every report as OPEN. The repository decides whether the
// job is already removed and whether the reporter already has an open report,
// because only it can check both atomically.
func NewReport(id, jobID, reporterID, reason string, now time.Time) Report {
	now = now.UTC()
	return Report{
		ID:         id,
		JobID:      jobID,
		ReporterID: reporterID,
		Reason:     reason,
		Status:     StatusOpen,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Dismiss closes an OPEN report with no action. Any other status is
// FAILED_PRECONDITION.
func (r *Report) Dismiss(now time.Time) error {
	if r.Status != StatusOpen {
		return ErrReportNotOpen
	}
	r.Status = StatusDismissed
	r.UpdatedAt = now.UTC()
	return nil
}

// Removal records an admin takedown. Moderation keeps its own record because
// it has no allowed call to job-posting (AGENTS.md Section 6), so it only
// knows the jobs it removed itself.
type Removal struct {
	JobID     string
	Reason    string
	RemovedBy string
	RemovedAt time.Time
}

// NewRemoval takes a reason already checked by NewRemovalReason.
func NewRemoval(jobID, adminID, reason string, now time.Time) Removal {
	return Removal{
		JobID:     jobID,
		Reason:    reason,
		RemovedBy: adminID,
		RemovedAt: now.UTC(),
	}
}
