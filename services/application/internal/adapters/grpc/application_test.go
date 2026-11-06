package grpc

import (
	"context"
	applicationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func TestApplicationHandler_PublicRPCs(t *testing.T) {
	const owner = "11111111-1111-4111-8111-111111111111"
	const id = "22222222-2222-4222-8222-222222222222"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", owner, "x-user-role", "applicant"))
	for _, name := range []string{"submit", "withdraw", "get", "list"} {
		t.Run(name, func(t *testing.T) {
			c := gomock.NewController(t)
			r := mocks.NewMockRepository(c)
			j := mocks.NewMockJobPosting(c)
			v := mocks.NewMockResume(c)
			h := NewApplicationHandler(app.NewService(r, j, v))
			a := domain.Application{ID: id, ApplicantID: owner, EmployerID: id, JobID: id, ResumeID: id, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			var result *applicationv1.Application
			var err error
			switch name {
			case "submit":
				j.EXPECT().GetJob(ctx, id).Return(domain.Job{ID: id, EmployerID: id, Published: true, Deadline: time.Now().Add(time.Hour)}, nil)
				v.EXPECT().GetResume(ctx, id).Return(domain.Resume{ID: id, ApplicantID: owner, Validated: true}, nil)
				r.EXPECT().Create(ctx, gomock.Any()).Return(a, nil)
				response, e := h.SubmitApplication(ctx, &applicationv1.SubmitApplicationRequest{JobId: id, ResumeId: id})
				err = e
				result = response.GetApplication()
			case "withdraw":
				r.EXPECT().Get(ctx, id).Return(a, nil)
				a.Withdrawn = true
				r.EXPECT().Withdraw(ctx, id, owner).Return(a, nil)
				response, e := h.WithdrawApplication(ctx, &applicationv1.WithdrawApplicationRequest{ApplicationId: id})
				err = e
				result = response.GetApplication()
			case "get":
				r.EXPECT().Get(ctx, id).Return(a, nil)
				response, e := h.GetApplication(ctx, &applicationv1.GetApplicationRequest{ApplicationId: id})
				err = e
				result = response.GetApplication()
			case "list":
				r.EXPECT().List(ctx, gomock.Any()).Return([]domain.Application{a}, int64(2), nil)
				response, e := h.ListApplications(ctx, &applicationv1.ListApplicationsRequest{Pagination: &commonv1.PaginationRequest{PageSize: 1}, JobId: id})
				err = e
				if response != nil {
					result = response.Applications[0]
					if response.Pagination.TotalCount != 2 || response.Pagination.NextPageToken != "1" {
						t.Fatal("pagination conversion")
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.ApplicationId != a.ID || result.ApplicantId != owner || result.EmployerId != a.EmployerID || result.JobId != a.JobID || result.ResumeId != a.ResumeID || result.Withdrawn != a.Withdrawn || !result.Timestamps.CreatedAt.AsTime().Equal(a.CreatedAt) || !result.Timestamps.UpdatedAt.AsTime().Equal(a.UpdatedAt) {
				t.Fatal("conversion mismatch")
			}
		})
	}
}
func TestApplicationHandler_NilRequestsAndUseCaseErrors(t *testing.T) {
	h := NewApplicationHandler(app.NewService(nil, nil, nil))
	ctx := context.Background()
	for _, nilRequest := range []bool{true, false} {
		var submit *applicationv1.SubmitApplicationRequest
		var withdraw *applicationv1.WithdrawApplicationRequest
		var get *applicationv1.GetApplicationRequest
		var list *applicationv1.ListApplicationsRequest
		want := codes.InvalidArgument
		if !nilRequest {
			submit = &applicationv1.SubmitApplicationRequest{}
			withdraw = &applicationv1.WithdrawApplicationRequest{}
			get = &applicationv1.GetApplicationRequest{}
			list = &applicationv1.ListApplicationsRequest{}
			want = codes.Unauthenticated
		}
		_, a := h.SubmitApplication(ctx, submit)
		_, b := h.WithdrawApplication(ctx, withdraw)
		_, c := h.GetApplication(ctx, get)
		_, d := h.ListApplications(ctx, list)
		for _, e := range []error{a, b, c, d} {
			if status.Code(e) != want {
				t.Fatalf("code %v want %v", status.Code(e), want)
			}
		}
	}
}
