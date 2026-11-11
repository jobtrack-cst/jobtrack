package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/ports/mocks"
	"go.uber.org/mock/gomock"
)

var errBoom = errors.New("boom")

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

const validUUID = "11111111-1111-4111-8111-111111111111"

func TestSearch_Run(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	openJob := domain.JobSummary{JobID: validUUID, Deadline: now.Add(time.Hour)}
	closedJob := domain.JobSummary{JobID: validUUID, Deadline: now.Add(-time.Hour)}

	tests := []struct {
		name      string
		in        app.RawInput
		setupMock func(*mocks.MockSearchRepository)
		wantJobs  int
		wantTotal int
		wantErr   error
	}{
		{
			name: "happy path returns open jobs",
			in:   app.RawInput{PageSize: 20},
			setupMock: func(m *mocks.MockSearchRepository) {
				m.EXPECT().Search(gomock.Any(), gomock.Any()).
					Return([]domain.JobSummary{openJob}, 1, nil)
			},
			wantJobs:  1,
			wantTotal: 1,
		},
		{
			name: "defensive filter drops non-open jobs",
			in:   app.RawInput{PageSize: 20},
			setupMock: func(m *mocks.MockSearchRepository) {
				m.EXPECT().Search(gomock.Any(), gomock.Any()).
					Return([]domain.JobSummary{openJob, closedJob}, 2, nil)
			},
			wantJobs:  1,
			wantTotal: 2,
		},
		{
			name:      "invalid filters never reach the repo",
			in:        app.RawInput{CategoryID: "not-a-uuid"},
			setupMock: func(m *mocks.MockSearchRepository) {},
			wantErr:   domain.ErrInvalidCategoryID,
		},
		{
			name: "repo error propagates",
			in:   app.RawInput{PageSize: 20},
			setupMock: func(m *mocks.MockSearchRepository) {
				m.EXPECT().Search(gomock.Any(), gomock.Any()).
					Return(nil, 0, errBoom)
			},
			wantErr: errBoom,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockSearchRepository(ctrl)
			tt.setupMock(repo)
			svc := app.NewSearch(repo, fixedClock{t: now})
			got, err := svc.Run(context.Background(), tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if len(got.Jobs) != tt.wantJobs || got.Total != tt.wantTotal {
				t.Fatalf("got jobs=%d total=%d, want jobs=%d total=%d",
					len(got.Jobs), got.Total, tt.wantJobs, tt.wantTotal)
			}
		})
	}
}
