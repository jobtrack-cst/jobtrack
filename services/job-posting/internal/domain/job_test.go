package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

const (
	tagA = "aaaaaaaa-1111-4222-8333-444455556666"
	tagB = "bbbbbbbb-1111-4222-8333-444455556666"
)

func validInput() domain.JobInput {
	return domain.JobInput{
		Title:       "Junior Go Developer",
		Description: "Build microservices.",
		CategoryID:  tagA,
		TagIDs:      []string{tagA, tagB},
		Location:    "Thimphu",
		Deadline:    now.Add(48 * time.Hour),
	}
}

func tagIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = strings.Replace(tagA, "aaaaaaaa", strings.Repeat("0", 6)+twoDigits(i), 1)
	}
	return ids
}

func twoDigits(i int) string { return string(rune('0'+i/10)) + string(rune('0'+i%10)) }

func TestNewJobFields_Valid(t *testing.T) {
	in := validInput()
	in.Title = "  Junior Go Developer  "
	in.TagIDs = []string{strings.ToUpper(tagB), tagA, tagB}
	in.Deadline = now.Add(time.Hour + 1500*time.Nanosecond).In(time.FixedZone("BTT", 6*3600))

	got, err := domain.NewJobFields(in, now)

	require.NoError(t, err)
	assert.Equal(t, "Junior Go Developer", got.Title)
	assert.Equal(t, []string{tagB, tagA}, got.TagIDs, "lowercased, duplicates removed, order kept")
	assert.Equal(t, now.Add(time.Hour+time.Microsecond), got.Deadline)
	assert.Equal(t, time.UTC, got.Deadline.Location())
}

func TestNewJobFields_Boundaries(t *testing.T) {
	tests := []struct {
		name    string
		edit    func(*domain.JobInput)
		wantErr error
	}{
		{"title 1 char", func(in *domain.JobInput) { in.Title = "A" }, nil},
		{"title 150 chars", func(in *domain.JobInput) { in.Title = strings.Repeat("a", 150) }, nil},
		{"title 150 multibyte chars", func(in *domain.JobInput) { in.Title = strings.Repeat("ཀ", 150) }, nil},
		{"title 151 chars", func(in *domain.JobInput) { in.Title = strings.Repeat("a", 151) }, domain.ErrInvalidTitle},
		{"title empty", func(in *domain.JobInput) { in.Title = "" }, domain.ErrInvalidTitle},
		{"title whitespace only", func(in *domain.JobInput) { in.Title = " \t\n " }, domain.ErrInvalidTitle},
		{"title with NUL", func(in *domain.JobInput) { in.Title = "Dev\x00" }, domain.ErrInvalidTitle},
		{"title invalid utf8", func(in *domain.JobInput) { in.Title = "Dev\xff" }, domain.ErrInvalidTitle},
		{"description empty", func(in *domain.JobInput) { in.Description = "" }, nil},
		{"description 5000 chars", func(in *domain.JobInput) { in.Description = strings.Repeat("d", 5000) }, nil},
		{"description 5001 chars", func(in *domain.JobInput) { in.Description = strings.Repeat("d", 5001) }, domain.ErrInvalidDescription},
		{"description with NUL", func(in *domain.JobInput) { in.Description = "a\x00b" }, domain.ErrInvalidDescription},
		{"category empty", func(in *domain.JobInput) { in.CategoryID = "" }, nil},
		{"category not uuid", func(in *domain.JobInput) { in.CategoryID = "it" }, domain.ErrInvalidCategoryID},
		{"no tags", func(in *domain.JobInput) { in.TagIDs = nil }, nil},
		{"20 tags", func(in *domain.JobInput) { in.TagIDs = tagIDs(20) }, nil},
		{"21 tags", func(in *domain.JobInput) { in.TagIDs = tagIDs(21) }, domain.ErrInvalidTagIDs},
		{"tag not uuid", func(in *domain.JobInput) { in.TagIDs = []string{tagA, "go"} }, domain.ErrInvalidTagIDs},
		{"location empty", func(in *domain.JobInput) { in.Location = "" }, nil},
		{"location 100 chars", func(in *domain.JobInput) { in.Location = strings.Repeat("l", 100) }, nil},
		{"location 101 chars", func(in *domain.JobInput) { in.Location = strings.Repeat("l", 101) }, domain.ErrInvalidLocation},
		{"deadline missing", func(in *domain.JobInput) { in.Deadline = time.Time{} }, domain.ErrInvalidDeadline},
		{"deadline in past", func(in *domain.JobInput) { in.Deadline = now.Add(-time.Second) }, domain.ErrInvalidDeadline},
		{"deadline equal to now", func(in *domain.JobInput) { in.Deadline = now }, domain.ErrInvalidDeadline},
		{"deadline under 1 microsecond after now", func(in *domain.JobInput) { in.Deadline = now.Add(500 * time.Nanosecond) }, domain.ErrInvalidDeadline},
		{"deadline 1 microsecond after now", func(in *domain.JobInput) { in.Deadline = now.Add(time.Microsecond) }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.edit(&in)
			_, err := domain.NewJobFields(in, now)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func jobWith(status domain.Status, deadline time.Time) domain.Job {
	return domain.Job{
		ID: tagB, EmployerID: employerID, Status: status,
		JobFields: domain.JobFields{Title: "Dev", Deadline: deadline},
	}
}

var allStatuses = []domain.Status{domain.StatusDraft, domain.StatusPublished, domain.StatusClosed, domain.StatusRemoved}

func TestJob_StatusRules(t *testing.T) {
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	tests := []struct {
		name    string
		status  domain.Status
		check   func(domain.Job) error
		wantErr error
	}{
		{"edit draft", domain.StatusDraft, domain.Job.CheckEditable, nil},
		{"edit published", domain.StatusPublished, domain.Job.CheckEditable, nil},
		{"edit closed", domain.StatusClosed, domain.Job.CheckEditable, domain.ErrJobNotEditable},
		{"edit removed", domain.StatusRemoved, domain.Job.CheckEditable, domain.ErrJobNotEditable},
		{"close draft", domain.StatusDraft, domain.Job.CheckClosable, domain.ErrJobNotPublished},
		{"close published", domain.StatusPublished, domain.Job.CheckClosable, nil},
		{"close closed", domain.StatusClosed, domain.Job.CheckClosable, domain.ErrJobNotPublished},
		{"close removed", domain.StatusRemoved, domain.Job.CheckClosable, domain.ErrJobNotPublished},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.check(jobWith(tt.status, future))
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}

	publishTests := []struct {
		name     string
		status   domain.Status
		deadline time.Time
		wantErr  error
	}{
		{"publish draft future deadline", domain.StatusDraft, future, nil},
		{"publish draft deadline passed", domain.StatusDraft, past, domain.ErrDeadlinePassed},
		{"publish draft deadline equal to now", domain.StatusDraft, now, domain.ErrDeadlinePassed},
		{"publish published", domain.StatusPublished, future, domain.ErrJobNotDraft},
		{"publish closed", domain.StatusClosed, future, domain.ErrJobNotDraft},
		{"publish removed", domain.StatusRemoved, future, domain.ErrJobNotDraft},
	}
	for _, tt := range publishTests {
		t.Run(tt.name, func(t *testing.T) {
			err := jobWith(tt.status, tt.deadline).CheckPublishable(now)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestJob_CheckOwner(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		wantErr error
	}{
		{"owning employer", domain.Caller{UserID: employerID, Role: domain.RoleEmployer}, nil},
		{"other employer", domain.Caller{UserID: otherID, Role: domain.RoleEmployer}, domain.ErrNotJobOwner},
		{"admin", domain.Caller{UserID: otherID, Role: domain.RoleAdmin}, domain.ErrNotJobOwner},
		{"applicant with owner id", domain.Caller{UserID: employerID, Role: domain.RoleApplicant}, domain.ErrNotJobOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jobWith(domain.StatusDraft, now).CheckOwner(tt.caller)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestJob_VisibleTo(t *testing.T) {
	callers := map[string]domain.Caller{
		"owner":          {UserID: employerID, Role: domain.RoleEmployer},
		"other employer": {UserID: otherID, Role: domain.RoleEmployer},
		"applicant":      {UserID: otherID, Role: domain.RoleApplicant},
		"admin":          {UserID: otherID, Role: domain.RoleAdmin},
		// Same user ID as the owner but not the employer role.
		"applicant with owner id": {UserID: employerID, Role: domain.RoleApplicant},
	}
	want := map[domain.Status]map[string]bool{
		domain.StatusDraft:     {"owner": true, "admin": true},
		domain.StatusRemoved:   {"owner": true, "admin": true},
		domain.StatusPublished: {"owner": true, "other employer": true, "applicant": true, "admin": true, "applicant with owner id": true},
		domain.StatusClosed:    {"owner": true, "other employer": true, "applicant": true, "admin": true, "applicant with owner id": true},
	}
	for _, status := range allStatuses {
		for name, caller := range callers {
			t.Run(status.String()+"/"+name, func(t *testing.T) {
				assert.Equal(t, want[status][name], jobWith(status, now).VisibleTo(caller))
			})
		}
	}
}

func TestJob_IsOpen(t *testing.T) {
	tests := []struct {
		name     string
		status   domain.Status
		deadline time.Time
		want     bool
	}{
		{"published future deadline", domain.StatusPublished, now.Add(time.Microsecond), true},
		{"published deadline equal to now", domain.StatusPublished, now, false},
		{"published deadline passed", domain.StatusPublished, now.Add(-time.Hour), false},
		{"draft", domain.StatusDraft, now.Add(time.Hour), false},
		{"closed", domain.StatusClosed, now.Add(time.Hour), false},
		{"removed", domain.StatusRemoved, now.Add(time.Hour), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, jobWith(tt.status, tt.deadline).IsOpen(now))
		})
	}
}

func TestParseStatus(t *testing.T) {
	for _, s := range allStatuses {
		got, ok := domain.ParseStatus(s.String())
		assert.True(t, ok, s.String())
		assert.Equal(t, s, got)
	}
	for _, bad := range []string{"", "DRAFT", "open"} {
		_, ok := domain.ParseStatus(bad)
		assert.False(t, ok, bad)
	}
}
