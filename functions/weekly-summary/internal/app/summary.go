// Package app runs one weekly summary.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/domain"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/ports"
)

// MaxAdminPages stops a run whose admin list never ends (100 pages of up to
// 100 admins each).
const MaxAdminPages = 100

var (
	ErrStats  = errors.New("get summary statistics failed")
	ErrAdmins = errors.New("list admins failed")
	ErrSend   = errors.New("send summary email failed")
)

// Result says how a run went; it never holds email addresses.
type Result struct {
	Admins int
	Sent   int
	Failed int
}

// Runner sends the summary of the last 7 days to every admin.
type Runner struct {
	Stats  ports.Stats
	Admins ports.Admins
	Mailer ports.Mailer
	Now    func() time.Time
	Logger *slog.Logger
}

// Run fetches everything before sending, so a failed read sends no email.
// A failed send does not stop the others; the run still fails so it can be
// retried, which can email the admins who already got it again.
func (r Runner) Run(ctx context.Context) (Result, error) {
	start, end := domain.Period(r.Now())
	summary, err := r.Stats.Summary(ctx, start, end)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrStats, err)
	}
	// Report the period we asked for, whatever the response echoes.
	summary.Start, summary.End = start, end
	emails, err := r.adminEmails(ctx)
	if err != nil {
		return Result{}, err
	}
	result := Result{Admins: len(emails)}
	if len(emails) == 0 {
		r.Logger.Warn("no admins to send the weekly summary to")
		return result, nil
	}
	subject, body := domain.Subject(summary), domain.Body(summary)
	for _, email := range emails {
		if err := r.Mailer.Send(ctx, email, subject, body); err != nil {
			result.Failed++
			continue
		}
		result.Sent++
	}
	if result.Failed > 0 {
		return result, fmt.Errorf("%w for %d of %d admins", ErrSend, result.Failed, result.Admins)
	}
	return result, nil
}

// adminEmails pages through all admins and removes duplicates, ignoring case.
func (r Runner) adminEmails(ctx context.Context) ([]string, error) {
	var emails []string
	seenEmail := map[string]bool{}
	seenToken := map[string]bool{"": true}
	token := ""
	for page := 1; ; page++ {
		if page > MaxAdminPages {
			return nil, fmt.Errorf("%w: more than %d pages", ErrAdmins, MaxAdminPages)
		}
		batch, next, err := r.Admins.ListAdmins(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrAdmins, err)
		}
		for _, email := range batch {
			email = strings.TrimSpace(email)
			key := strings.ToLower(email)
			if email == "" || seenEmail[key] {
				continue
			}
			seenEmail[key] = true
			emails = append(emails, email)
		}
		if next == "" {
			return emails, nil
		}
		if seenToken[next] {
			return nil, fmt.Errorf("%w: page token repeated", ErrAdmins)
		}
		seenToken[next] = true
		token = next
	}
}
