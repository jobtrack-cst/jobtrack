package grpc

import (
	"context"
	"errors"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	reviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/review/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

// ReviewHandler serves ReviewService through the gateway.
type ReviewHandler struct {
	reviewv1.UnimplementedReviewServiceServer
	reviews *app.Reviews
}

var _ reviewv1.ReviewServiceServer = (*ReviewHandler)(nil)

func NewReviewHandler(reviews *app.Reviews) *ReviewHandler {
	return &ReviewHandler{reviews: reviews}
}

// CompleteReview is for the employer that owns the application. The
// generated getters are nil-safe, so a nil request is an empty one.
func (h *ReviewHandler) CompleteReview(ctx context.Context, req *reviewv1.CompleteReviewRequest) (*reviewv1.CompleteReviewResponse, error) {
	caller, err := employerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	review, err := h.reviews.Complete(ctx, caller, domain.ReviewInput{
		ApplicationID: req.GetApplicationId(), Score: req.GetScore(), Decision: decisionFromProto(req.GetDecision()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &reviewv1.CompleteReviewResponse{Review: reviewToProto(review)}, nil
}

// GetReview is for the owning employer only (JT-51).
func (h *ReviewHandler) GetReview(ctx context.Context, req *reviewv1.GetReviewRequest) (*reviewv1.GetReviewResponse, error) {
	caller, err := employerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	review, err := h.reviews.Get(ctx, caller, req.GetApplicationId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &reviewv1.GetReviewResponse{Review: reviewToProto(review)}, nil
}

// employerFrom trusts x-user-id and x-user-role only because the gateway sets
// them and strips any client-sent values (JT-31). The role is checked before
// any input, so other roles learn nothing about which applications exist.
func employerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	caller, err := domain.NewCaller(identity.UserID, identity.Role)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireEmployer(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

// decisionFromProto maps unknown enum values to UNSPECIFIED, which validation
// rejects.
func decisionFromProto(d reviewv1.ReviewDecision) domain.Decision {
	switch d {
	case reviewv1.ReviewDecision_REVIEW_DECISION_SHORTLISTED:
		return domain.DecisionShortlisted
	case reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED:
		return domain.DecisionRejected
	default:
		return domain.DecisionUnspecified
	}
}

func decisionToProto(d domain.Decision) reviewv1.ReviewDecision {
	switch d {
	case domain.DecisionShortlisted:
		return reviewv1.ReviewDecision_REVIEW_DECISION_SHORTLISTED
	case domain.DecisionRejected:
		return reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED
	default:
		return reviewv1.ReviewDecision_REVIEW_DECISION_UNSPECIFIED
	}
}

func reviewToProto(r domain.Review) *reviewv1.Review {
	return &reviewv1.Review{
		ReviewId:      r.ReviewID,
		ApplicationId: r.ApplicationID,
		EmployerId:    r.EmployerID,
		Score:         r.Score,
		Decision:      decisionToProto(r.Decision),
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(r.CreatedAt),
			UpdatedAt: timestamppb.New(r.UpdatedAt),
		},
	}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidApplicationID), errors.Is(err, domain.ErrInvalidScore),
		errors.Is(err, domain.ErrInvalidDecision):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrEmployerRoleRequired), errors.Is(err, domain.ErrNotOwningEmployer):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrApplicationNotFound), errors.Is(err, domain.ErrReviewNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrReviewExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
