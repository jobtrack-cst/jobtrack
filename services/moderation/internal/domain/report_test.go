package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

const (
	reportID = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	jobID    = "7b2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e"
	adminID  = "8c3d4e5f-6a7b-4c8d-ae9f-1a2b3c4d5e6f"
)

// A non-UTC time shows that the domain stores timestamps in UTC.
var thimphu = time.FixedZone("BTT", 6*60*60)

func TestParseStatusFilter(t *testing.T) {
	tests := []struct {
		name    string
		input   int32
		want    domain.ReportStatus
		wantErr error
	}{
		{"unspecified means all", 0, domain.StatusUnspecified, nil},
		{"open", 1, domain.StatusOpen, nil},
		{"dismissed", 2, domain.StatusDismissed, nil},
		{"actioned", 3, domain.StatusActioned, nil},
		{"just above the last value", 4, domain.StatusUnspecified, domain.ErrInvalidStatusFilter},
		{"negative", -1, domain.StatusUnspecified, domain.ErrInvalidStatusFilter},
		{"wraps to zero as uint8", 256, domain.StatusUnspecified, domain.ErrInvalidStatusFilter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ParseStatusFilter(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// reasonCases covers the boundary values shared by both reason fields.
func reasonCases(maxLen int, invalid error) []struct {
	name    string
	input   string
	want    string
	wantErr error
} {
	return []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"empty", "", "", invalid},
		{"only spaces", "   \t\n", "", invalid},
		{"one character", "x", "x", nil},
		{"trimmed", "  spam posting \n", "spam posting", nil},
		{"exactly the maximum", strings.Repeat("a", maxLen), strings.Repeat("a", maxLen), nil},
		{"one over the maximum", strings.Repeat("a", maxLen+1), "", invalid},
		{"maximum after trimming", " " + strings.Repeat("a", maxLen) + " ", strings.Repeat("a", maxLen), nil},
		{"multi-byte characters count once", strings.Repeat("é", maxLen), strings.Repeat("é", maxLen), nil},
		{"multi-byte one over the maximum", strings.Repeat("é", maxLen+1), "", invalid},
		{"NUL is refused", "bad\x00reason", "", invalid},
		{"invalid UTF-8 is refused", "bad\xffreason", "", invalid},
	}
}

func TestNewReportReason(t *testing.T) {
	for _, tt := range reasonCases(domain.MaxReportReasonLength, domain.ErrInvalidReportReason) {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewReportReason(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewRemovalReason(t *testing.T) {
	for _, tt := range reasonCases(domain.MaxRemovalReasonLength, domain.ErrInvalidRemovalReason) {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewRemovalReason(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFieldLimits(t *testing.T) {
	assert.Equal(t, 1000, domain.MaxReportReasonLength)
	assert.Equal(t, 500, domain.MaxRemovalReasonLength)
}

func TestNewReport_StartsOpenInUTC(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 30, 0, 0, thimphu)

	r := domain.NewReport(reportID, jobID, userID, "spam", now)

	assert.Equal(t, domain.Report{
		ID:         reportID,
		JobID:      jobID,
		ReporterID: userID,
		Reason:     "spam",
		Status:     domain.StatusOpen,
		CreatedAt:  now.UTC(),
		UpdatedAt:  now.UTC(),
	}, r)
	assert.Equal(t, time.UTC, r.CreatedAt.Location())
}

func TestReport_Dismiss(t *testing.T) {
	created := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	later := time.Date(2026, 11, 8, 16, 0, 0, 0, thimphu)

	tests := []struct {
		name        string
		from        domain.ReportStatus
		wantErr     error
		wantStatus  domain.ReportStatus
		wantUpdated time.Time
	}{
		{"open becomes dismissed", domain.StatusOpen, nil, domain.StatusDismissed, later.UTC()},
		{"dismissed cannot be dismissed again", domain.StatusDismissed, domain.ErrReportNotOpen, domain.StatusDismissed, created},
		{"actioned cannot be dismissed", domain.StatusActioned, domain.ErrReportNotOpen, domain.StatusActioned, created},
		{"unspecified cannot be dismissed", domain.StatusUnspecified, domain.ErrReportNotOpen, domain.StatusUnspecified, created},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := domain.Report{ID: reportID, Status: tt.from, CreatedAt: created, UpdatedAt: created}

			err := r.Dismiss(later)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantStatus, r.Status)
			assert.Equal(t, tt.wantUpdated, r.UpdatedAt)
			assert.Equal(t, created, r.CreatedAt)
		})
	}
}

func TestNewRemoval_RecordsAdminInUTC(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 30, 0, 0, thimphu)

	got := domain.NewRemoval(jobID, adminID, "fake company", now)

	assert.Equal(t, domain.Removal{
		JobID:     jobID,
		Reason:    "fake company",
		RemovedBy: adminID,
		RemovedAt: now.UTC(),
	}, got)
}
