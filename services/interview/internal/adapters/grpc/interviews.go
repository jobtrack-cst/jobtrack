package grpc

import (
	"context"
	"errors"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	interviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/interview/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
)

// InterviewHandler serves InterviewService through the gateway.
type InterviewHandler struct {
	interviewv1.UnimplementedInterviewServiceServer
	interviews *app.Interviews
}

var _ interviewv1.InterviewServiceServer = (*InterviewHandler)(nil)

func NewInterviewHandler(interviews *app.Interviews) *InterviewHandler {
	return &InterviewHandler{interviews: interviews}
}

// ScheduleInterview is for the employer that owns the application. The
// generated getters are nil-safe, so a nil request is an empty one.
func (h *InterviewHandler) ScheduleInterview(ctx context.Context, req *interviewv1.ScheduleInterviewRequest) (*interviewv1.ScheduleInterviewResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if err := caller.RequireEmployer(); err != nil {
		return nil, toStatus(err)
	}
	iv, err := h.interviews.Schedule(ctx, caller, domain.ScheduleInput{
		ApplicationID: req.GetApplicationId(), ScheduledAt: timeFromProto(req.GetScheduledAt()), Details: req.GetDetails(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &interviewv1.ScheduleInterviewResponse{Interview: interviewToProto(iv)}, nil
}

// GetInterview is for the interview's applicant or owning employer (JT-52).
func (h *InterviewHandler) GetInterview(ctx context.Context, req *interviewv1.GetInterviewRequest) (*interviewv1.GetInterviewResponse, error) {
	caller, err := readerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	iv, err := h.interviews.Get(ctx, caller, req.GetInterviewId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &interviewv1.GetInterviewResponse{Interview: interviewToProto(iv)}, nil
}

// ListInterviews lists one application's interviews, soonest first.
func (h *InterviewHandler) ListInterviews(ctx context.Context, req *interviewv1.ListInterviewsRequest) (*interviewv1.ListInterviewsResponse, error) {
	caller, err := readerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	page, err := h.interviews.List(ctx, caller, req.GetApplicationId(),
		req.GetPagination().GetPageSize(), req.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*interviewv1.Interview, 0, len(page.Interviews))
	for _, iv := range page.Interviews {
		out = append(out, interviewToProto(iv))
	}
	return &interviewv1.ListInterviewsResponse{
		Interviews: out,
		Pagination: &commonv1.PaginationResponse{NextPageToken: page.NextPageToken, TotalCount: int64(page.TotalCount)},
	}, nil
}

// callerFrom trusts x-user-id and x-user-role only because the gateway sets
// them and strips any client-sent values (JT-31).
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

// readerFrom checks the role before any input, so admins learn nothing.
func readerFrom(ctx context.Context) (domain.Caller, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireParticipantRole(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

// timeFromProto treats a missing or out-of-range timestamp as missing, which
// validation rejects.
func timeFromProto(ts *timestamppb.Timestamp) time.Time {
	if ts == nil || ts.CheckValid() != nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func interviewToProto(iv domain.Interview) *interviewv1.Interview {
	return &interviewv1.Interview{
		InterviewId:   iv.InterviewID,
		ApplicationId: iv.ApplicationID,
		EmployerId:    iv.EmployerID,
		ApplicantId:   iv.ApplicantID,
		ScheduledAt:   timestamppb.New(iv.ScheduledAt),
		Details:       iv.Details,
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(iv.CreatedAt),
			UpdatedAt: timestamppb.New(iv.UpdatedAt),
		},
	}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidApplicationID), errors.Is(err, domain.ErrInvalidInterviewID),
		errors.Is(err, domain.ErrScheduledAtMissing), errors.Is(err, domain.ErrScheduledAtNotFuture),
		errors.Is(err, domain.ErrScheduledAtTooFar), errors.Is(err, domain.ErrInvalidDetails),
		errors.Is(err, domain.ErrInvalidPageSize), errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrEmployerRoleRequired), errors.Is(err, domain.ErrRoleNotAllowed),
		errors.Is(err, domain.ErrNotOwningEmployer), errors.Is(err, domain.ErrNotParticipant):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrApplicationNotFound), errors.Is(err, domain.ErrInterviewNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInterviewExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
