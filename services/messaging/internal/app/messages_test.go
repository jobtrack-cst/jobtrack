package app_test

import (
	"context"
	"errors"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
	"time"
)

const applicationID = "aaaaaaaa-1111-4111-8111-111111111111"
const applicantID = "bbbbbbbb-2222-4222-8222-222222222222"
const employerID = "cccccccc-3333-4333-8333-333333333333"

func TestMessages_Authorization(t *testing.T) {
	for _, operation := range []string{"send", "list"} {
		for _, tc := range []struct {
			name, id, role, application string
			lookup                      bool
			lookupErr, want             error
		}{{"applicant", applicantID, "applicant", applicationID, true, nil, nil}, {"employer", employerID, "employer", applicationID, true, nil, nil}, {"unrelated", applicationID, "applicant", applicationID, true, nil, domain.ErrPermissionDenied}, {"wrong role", applicantID, "employer", applicationID, true, nil, domain.ErrPermissionDenied}, {"admin", applicantID, "admin", applicationID, false, nil, domain.ErrPermissionDenied}, {"invalid caller", "bad", "applicant", applicationID, false, nil, domain.ErrUnauthenticated}, {"unknown role", applicantID, "other", applicationID, false, nil, domain.ErrUnauthenticated}, {"invalid application", applicantID, "applicant", "bad", false, nil, domain.ErrInvalidApplicationID}, {"missing", applicantID, "applicant", applicationID, true, domain.ErrParticipantsNotFound, domain.ErrParticipantsNotFound}, {"database failure", applicantID, "applicant", applicationID, true, errors.New("failed"), errors.New("failed")}} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				repo := mocks.NewMockMessageRepository(ctrl)
				participants := mocks.NewMockParticipantRepository(ctrl)
				clock := mocks.NewMockClock(ctrl)
				p := domain.ApplicationParticipants{ApplicationID: applicationID, ApplicantID: applicantID, EmployerID: employerID}
				if tc.lookup {
					participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(p, tc.lookupErr)
				}
				if tc.want == nil {
					if operation == "send" {
						now := time.Now()
						clock.EXPECT().Now().Return(now)
						repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m domain.Message) (domain.Message, error) {
							require.Equal(t, "hello", m.Content)
							require.Equal(t, tc.id, m.SenderID)
							recipient := employerID
							if tc.role == "employer" {
								recipient = applicantID
							}
							require.Equal(t, recipient, m.RecipientID)
							require.Equal(t, time.UTC, m.CreatedAt.Location())
							require.NotEmpty(t, m.ID)
							return m, nil
						})
					} else {
						repo.EXPECT().List(gomock.Any(), applicationID, domain.Page{Size: 20}).Return([]domain.Message{}, int64(0), nil)
					}
				}
				s := app.NewMessages(repo, participants, clock)
				var err error
				if operation == "send" {
					_, err = s.Send(context.Background(), domain.Caller{UserID: tc.id, Role: tc.role}, tc.application, " hello ")
				} else {
					_, _, _, err = s.List(context.Background(), domain.Caller{UserID: tc.id, Role: tc.role}, tc.application, 0, "")
				}
				if tc.want == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, tc.want.Error())
				}
			})
		}
	}
}
func TestMessages_ValidationAndRepositoryErrors(t *testing.T) {
	for _, tc := range []string{"content", "pagination", "create error", "list error"} {
		t.Run(tc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockMessageRepository(ctrl)
			p := mocks.NewMockParticipantRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			p.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationParticipants{ApplicationID: applicationID, ApplicantID: applicantID, EmployerID: employerID}, nil)
			s := app.NewMessages(repo, p, clock)
			caller := domain.Caller{UserID: applicantID, Role: "applicant"}
			var err error
			switch tc {
			case "content":
				_, err = s.Send(context.Background(), caller, applicationID, "")
				require.ErrorIs(t, err, domain.ErrInvalidContent)
			case "pagination":
				_, _, _, err = s.List(context.Background(), caller, applicationID, -1, "")
				require.ErrorIs(t, err, domain.ErrInvalidPagination)
			case "create error":
				clock.EXPECT().Now().Return(time.Now())
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Message{}, errors.New("storage"))
				_, err = s.Send(context.Background(), caller, applicationID, "hello")
				require.EqualError(t, err, "storage")
			case "list error":
				repo.EXPECT().List(gomock.Any(), applicationID, domain.Page{Size: 20}).Return(nil, int64(0), errors.New("storage"))
				_, _, _, err = s.List(context.Background(), caller, applicationID, 0, "")
				require.EqualError(t, err, "storage")
			}
		})
	}
}
