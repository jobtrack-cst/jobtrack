package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const applicant = "11111111-1111-4111-8111-111111111111"
const employer = "22222222-2222-4222-8222-222222222222"
const jobID = "33333333-3333-4333-8333-333333333333"
const resumeID = "44444444-4444-4444-8444-444444444444"
const applicationID = "55555555-5555-4555-8555-555555555555"

func caller(role, id string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}
func record() domain.Application {
	return domain.Application{ID: applicationID, ApplicantID: applicant, EmployerID: employer, JobID: jobID, ResumeID: resumeID}
}
func setup(t *testing.T) (*Service, *mocks.MockRepository, *mocks.MockJobPosting, *mocks.MockResume) {
	c := gomock.NewController(t)
	r := mocks.NewMockRepository(c)
	j := mocks.NewMockJobPosting(c)
	v := mocks.NewMockResume(c)
	s := NewService(r, j, v)
	s.now = func() time.Time { return time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC) }
	return s, r, j, v
}
func checkCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("code=%v want=%v err=%v", status.Code(err), want, err)
	}
}
func TestApplication_Authorization_AllOperations(t *testing.T) {
	for _, tt := range []struct {
		name, role, id string
		read, write    codes.Code
	}{
		{"missing", "", "", codes.Unauthenticated, codes.Unauthenticated},
		{"invalid identity", "applicant", "bad", codes.Unauthenticated, codes.Unauthenticated},
		{"unknown role", "root", applicant, codes.PermissionDenied, codes.PermissionDenied},
		{"employer", "employer", employer, codes.OK, codes.PermissionDenied},
		{"admin", "admin", employer, codes.OK, codes.PermissionDenied},
		{"applicant", "applicant", applicant, codes.OK, codes.OK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, r, _, _ := setup(t)
			ctx := caller(tt.role, tt.id)
			if tt.read == codes.OK {
				r.EXPECT().Get(ctx, applicationID).Return(record(), nil)
				r.EXPECT().List(ctx, gomock.Any()).Return([]domain.Application{}, int64(0), nil)
			}
			_, e := s.Get(ctx, applicationID)
			checkCode(t, e, tt.read)
			_, _, _, e = s.List(ctx, "", 0, "")
			checkCode(t, e, tt.read)
			// Invalid input stops valid writers before dependency/repository access.
			want := tt.write
			if want == codes.OK {
				want = codes.InvalidArgument
			}
			_, e = s.Submit(ctx, "bad", resumeID)
			checkCode(t, e, want)
			_, e = s.Withdraw(ctx, "bad")
			checkCode(t, e, want)
		})
	}
}
func TestApplication_Submit_EligibilityAndPersistence(t *testing.T) {
	for _, name := range []string{"success", "closed", "deadline", "expired", "missing job", "job unavailable", "job unexpected error", "bad job response", "bad employer", "missing deadline", "missing resume", "resume unavailable", "bad resume response", "wrong resume owner", "pending resume", "duplicate", "storage failure"} {
		t.Run(name, func(t *testing.T) {
			s, r, j, v := setup(t)
			ctx := caller("applicant", applicant)
			r.EXPECT().List(ctx, ports.Filter{ApplicantID: applicant, JobID: jobID, Limit: 1}).Return(nil, int64(0), nil)
			job := domain.Job{ID: jobID, EmployerID: employer, Published: true, Deadline: s.now().Add(time.Hour)}
			resume := domain.Resume{ID: resumeID, ApplicantID: applicant, Validated: true}
			want := codes.OK
			var je, re error
			switch name {
			case "closed":
				job.Published = false
				want = codes.FailedPrecondition
			case "deadline":
				job.Deadline = s.now()
				want = codes.FailedPrecondition
			case "expired":
				job.Deadline = s.now().Add(-time.Nanosecond)
				want = codes.FailedPrecondition
			case "missing job":
				je = status.Error(codes.NotFound, "private")
				want = codes.NotFound
			case "job unavailable":
				je = status.Error(codes.Unavailable, "private")
				want = codes.Unavailable
			case "job unexpected error":
				je = errors.New("private")
				want = codes.Internal
			case "bad job response":
				job.ID = resumeID
				want = codes.Internal
			case "bad employer":
				job.EmployerID = ""
				want = codes.Internal
			case "missing deadline":
				job.Deadline = time.Time{}
				want = codes.Internal
			case "missing resume":
				re = status.Error(codes.NotFound, "private")
				want = codes.NotFound
			case "resume unavailable":
				re = context.DeadlineExceeded
				want = codes.DeadlineExceeded
			case "bad resume response":
				resume.ID = jobID
				want = codes.Internal
			case "wrong resume owner":
				resume.ApplicantID = employer
				want = codes.PermissionDenied
			case "pending resume":
				resume.Validated = false
				want = codes.FailedPrecondition
			case "duplicate":
				want = codes.AlreadyExists
			case "storage failure":
				want = codes.Internal
			}
			j.EXPECT().GetJob(ctx, jobID).Return(job, je)
			if je == nil && job.ID == jobID && domain.ValidID(job.EmployerID) && !job.Deadline.IsZero() && job.Eligible(s.now()) {
				v.EXPECT().GetResume(ctx, resumeID).Return(resume, re)
				if re == nil && resume.ID == resumeID && resume.ApplicantID == applicant && resume.Validated {
					r.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, a domain.Application) (domain.Application, error) {
						if !domain.ValidID(a.ID) || a.ApplicantID != applicant || a.EmployerID != employer || a.JobID != jobID || a.ResumeID != resumeID || a.Withdrawn || !a.CreatedAt.Equal(s.now()) || !a.UpdatedAt.Equal(a.CreatedAt) {
							t.Fatal("invalid persisted application")
						}
						switch name {
						case "duplicate":
							return a, domain.ErrDuplicate
						case "storage failure":
							return a, errors.New("private")
						}
						return a, nil
					})
				}
			}
			a, e := s.Submit(ctx, jobID, resumeID)
			checkCode(t, e, want)
			if e != nil && strings.Contains(e.Error(), "private") {
				t.Fatal("private error leaked")
			}
			if e == nil && a.ApplicantID != applicant {
				t.Fatal("owner mismatch")
			}
		})
	}
}
func TestApplication_Submit_InvalidInputsAndDeferredClients(t *testing.T) {
	for _, ids := range [][2]string{{"", resumeID}, {jobID, ""}, {jobID, "invalid"}, {strings.Repeat("x", 1000), resumeID}} {
		s, _, _, _ := setup(t)
		_, e := s.Submit(caller("applicant", applicant), ids[0], ids[1])
		checkCode(t, e, codes.InvalidArgument)
	}
	_, r, _, _ := setup(t)
	ctx := caller("applicant", applicant)
	r.EXPECT().List(ctx, ports.Filter{ApplicantID: applicant, JobID: jobID, Limit: 1}).Return(nil, int64(0), nil)
	s := NewService(r, nil, nil)
	_, e := s.Submit(ctx, jobID, resumeID)
	checkCode(t, e, codes.Unavailable)
}
func TestApplication_Get_OwnershipAndFailures(t *testing.T) {
	for _, tt := range []struct {
		role, id string
		err      error
		want     codes.Code
	}{
		{"applicant", applicant, nil, codes.OK}, {"employer", employer, nil, codes.OK}, {"admin", applicant, nil, codes.OK},
		{"applicant", employer, nil, codes.PermissionDenied}, {"employer", applicant, nil, codes.PermissionDenied},
		{"applicant", applicant, domain.ErrNotFound, codes.NotFound}, {"applicant", applicant, errors.New("private"), codes.Internal},
	} {
		t.Run(tt.role+tt.id+tt.want.String(), func(t *testing.T) {
			s, r, _, _ := setup(t)
			ctx := caller(tt.role, tt.id)
			r.EXPECT().Get(ctx, applicationID).Return(record(), tt.err)
			_, e := s.Get(ctx, applicationID)
			checkCode(t, e, tt.want)
		})
	}
	s, _, _, _ := setup(t)
	_, e := s.Get(caller("applicant", applicant), "bad")
	checkCode(t, e, codes.InvalidArgument)
}
func TestApplication_Withdraw_StateAndOwnership(t *testing.T) {
	for _, name := range []string{"success", "already withdrawn", "other owner", "missing", "race withdrawal", "storage failure"} {
		t.Run(name, func(t *testing.T) {
			s, r, _, _ := setup(t)
			ctx := caller("applicant", applicant)
			a := record()
			want := codes.OK
			var getErr, errorWrite error
			switch name {
			case "already withdrawn":
				a.Withdrawn = true
				want = codes.FailedPrecondition
			case "other owner":
				a.ApplicantID = employer
				want = codes.PermissionDenied
			case "missing":
				getErr = domain.ErrNotFound
				want = codes.NotFound
			case "race withdrawal":
				errorWrite = domain.ErrWithdrawn
				want = codes.FailedPrecondition
			case "storage failure":
				errorWrite = errors.New("private")
				want = codes.Internal
			}
			r.EXPECT().Get(ctx, applicationID).Return(a, getErr)
			if getErr == nil && a.ApplicantID == applicant && !a.Withdrawn {
				a.Withdrawn = true
				r.EXPECT().Withdraw(ctx, applicationID, applicant).Return(a, errorWrite)
			}
			got, e := s.Withdraw(ctx, applicationID)
			checkCode(t, e, want)
			if e == nil && !got.Withdrawn {
				t.Fatal("withdrawal not returned")
			}
		})
	}
}
func TestApplication_List_FiltersPaginationBoundaries(t *testing.T) {
	for _, tt := range []struct {
		role, job, token string
		size             int32
		want             codes.Code
	}{
		{"applicant", "", "", 0, codes.OK}, {"employer", jobID, "20", 100, codes.OK}, {"admin", "", "0", 1, codes.OK},
		{"applicant", "bad", "", 0, codes.InvalidArgument}, {"applicant", "", "", -1, codes.InvalidArgument}, {"applicant", "", "", 101, codes.InvalidArgument},
		{"applicant", "", "bad", 1, codes.InvalidArgument}, {"applicant", "", "-1", 1, codes.InvalidArgument}, {"applicant", "", "01", 1, codes.InvalidArgument}, {"applicant", "", strings.Repeat("1", 11), 1, codes.InvalidArgument}, {"applicant", "", "2147483548", 1, codes.InvalidArgument},
	} {
		t.Run(tt.role+tt.job+tt.token+string(rune(tt.size+200)), func(t *testing.T) {
			s, r, _, _ := setup(t)
			ctx := caller(tt.role, applicant)
			if tt.want == codes.OK {
				r.EXPECT().List(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, f ports.Filter) ([]domain.Application, int64, error) {
					if (tt.role == "applicant" && f.ApplicantID != applicant) || (tt.role == "employer" && f.EmployerID != applicant) || (tt.role == "admin" && (f.ApplicantID != "" || f.EmployerID != "")) || f.JobID != tt.job || f.Limit < 1 || f.Limit > 100 {
						t.Fatal("incorrect filter")
					}
					return []domain.Application{record()}, int64(100), nil
				})
			}
			_, _, next, e := s.List(ctx, tt.job, tt.size, tt.token)
			checkCode(t, e, tt.want)
			if e == nil && next == "" {
				t.Fatal("next page missing")
			}
		})
	}
	s, r, _, _ := setup(t)
	ctx := caller("applicant", applicant)
	r.EXPECT().List(ctx, gomock.Any()).Return(nil, int64(0), errors.New("private"))
	_, _, _, e := s.List(ctx, "", 1, "")
	checkCode(t, e, codes.Internal)
}
func TestApplication_ErrorMapping(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want codes.Code
	}{{nil, codes.OK}, {context.Canceled, codes.Canceled}, {context.DeadlineExceeded, codes.DeadlineExceeded}, {domain.ErrWithdrawn, codes.FailedPrecondition}, {domain.ErrDuplicate, codes.AlreadyExists}, {domain.ErrNotFound, codes.NotFound}, {errors.New("private"), codes.Internal}} {
		checkCode(t, mapped(tt.err), tt.want)
	}
	for _, code := range []codes.Code{codes.NotFound, codes.PermissionDenied, codes.FailedPrecondition, codes.Unavailable, codes.DeadlineExceeded, codes.Canceled} {
		checkCode(t, dependencyError(status.Error(code, "private")), code)
	}
	checkCode(t, dependencyError(context.Canceled), codes.Canceled)
}

func TestApplication_Submit_DuplicateAfterJobBecomesIneligible(t *testing.T) {
	for _, name := range []string{"closed", "expired"} {
		t.Run(name, func(t *testing.T) {
			s, r, j, v := setup(t)
			ctx := caller("applicant", applicant)
			job := domain.Job{ID: jobID, EmployerID: employer, Published: true, Deadline: s.now().Add(time.Hour)}
			created := false
			// The first submit creates the row; subsequent submits must find it.
			r.EXPECT().List(ctx, ports.Filter{ApplicantID: applicant, JobID: jobID, Limit: 1}).DoAndReturn(func(context.Context, ports.Filter) ([]domain.Application, int64, error) {
				if created {
					return []domain.Application{record()}, 1, nil
				}
				return nil, 0, nil
			}).Times(2)
			j.EXPECT().GetJob(ctx, jobID).DoAndReturn(func(context.Context, string) (domain.Job, error) { return job, nil }).Times(1)
			v.EXPECT().GetResume(ctx, resumeID).Return(domain.Resume{ID: resumeID, ApplicantID: applicant, Validated: true}, nil)
			r.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, a domain.Application) (domain.Application, error) {
				created = true
				return a, nil
			})
			_, err := s.Submit(ctx, jobID, resumeID)
			checkCode(t, err, codes.OK)
			if name == "closed" {
				job.Published = false
			} else {
				job.Deadline = s.now().Add(-time.Nanosecond)
			}
			_, err = s.Submit(ctx, jobID, resumeID)
			checkCode(t, err, codes.AlreadyExists)
		})
	}
}

func TestApplication_Submit_DuplicateLookupFailure(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		code codes.Code
	}{
		{"storage", errors.New("private storage failure"), codes.Internal},
		{"canceled", context.Canceled, codes.Canceled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, r, _, _ := setup(t)
			ctx := caller("applicant", applicant)
			r.EXPECT().List(ctx, ports.Filter{ApplicantID: applicant, JobID: jobID, Limit: 1}).Return(nil, int64(0), tt.err)
			_, err := s.Submit(ctx, jobID, resumeID)
			checkCode(t, err, tt.code)
			if strings.Contains(err.Error(), "private") {
				t.Fatal("storage details leaked")
			}
		})
	}
}

func TestApplication_Submit_ExistingApplicationSkipsEligibility(t *testing.T) {
	for _, withdrawn := range []bool{false, true} {
		t.Run(map[bool]string{false: "submitted", true: "withdrawn"}[withdrawn], func(t *testing.T) {
			_, r, _, _ := setup(t)
			ctx := caller("applicant", applicant)
			a := record()
			a.Withdrawn = withdrawn
			r.EXPECT().List(ctx, ports.Filter{ApplicantID: applicant, JobID: jobID, Limit: 1}).Return([]domain.Application{a}, int64(1), nil)
			s := NewService(r, nil, nil)
			_, err := s.Submit(ctx, jobID, resumeID)
			checkCode(t, err, codes.AlreadyExists)
		})
	}
}
