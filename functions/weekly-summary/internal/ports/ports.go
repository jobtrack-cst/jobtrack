// Package ports defines what the weekly summary needs from other services.
package ports

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/ports_mock.go -package=mocks . Stats,Admins,Mailer

// Stats reads outcome counts (AnalyticsInternalService.GetInternalSummary).
type Stats interface {
	Summary(ctx context.Context, start, end time.Time) (domain.Summary, error)
}

// Admins reads admin accounts one page at a time
// (AuthInternalService.ListUsersByRole). next is empty on the last page.
type Admins interface {
	ListAdmins(ctx context.Context, pageToken string) (emails []string, next string, err error)
}

// Mailer sends one plain-text email (email-sender POST /send-email).
type Mailer interface {
	Send(ctx context.Context, recipient, subject, body string) error
}
