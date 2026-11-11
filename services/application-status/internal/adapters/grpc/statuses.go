package grpc

import (
	"context"
	"errors"

	applicationstatusv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application_status/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

// ApplicationStatusHandler serves the read-only ApplicationStatusService
// through the gateway. Transitions come from events (Sprint 2), not RPCs.
type ApplicationStatusHandler struct {
	applicationstatusv1.UnimplementedApplicationStatusServiceServer
	statuses *app.Statuses
}

var _ applicationstatusv1.ApplicationStatusServiceServer = (*ApplicationStatusHandler)(nil)

func NewApplicationStatusHandler(statuses *app.Statuses) *ApplicationStatusHandler {
	return &ApplicationStatusHandler{statuses: statuses}
}

// GetApplicationStatus is for the application's applicant or employer only.
// The generated getters are nil-safe, so a nil request is an empty one.
func (h *ApplicationStatusHandler) GetApplicationStatus(ctx context.Context, req *applicationstatusv1.GetApplicationStatusRequest) (*applicationstatusv1.GetApplicationStatusResponse, error) {
	caller, err := participantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	s, err := h.statuses.GetStatus(ctx, caller, req.GetApplicationId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicationstatusv1.GetApplicationStatusResponse{StatusRecord: &applicationstatusv1.ApplicationStatusRecord{
		ApplicationId: s.ApplicationID,
		Status:        statusToProto(s.Status),
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(s.CreatedAt),
			UpdatedAt: timestamppb.New(s.UpdatedAt),
		},
	}}, nil
}

// ListApplicationStatusHistory lists every recorded transition, oldest first.
// The first entry has previous_status UNSPECIFIED and new_status SUBMITTED.
func (h *ApplicationStatusHandler) ListApplicationStatusHistory(ctx context.Context, req *applicationstatusv1.ListApplicationStatusHistoryRequest) (*applicationstatusv1.ListApplicationStatusHistoryResponse, error) {
	caller, err := participantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	page, err := h.statuses.ListHistory(ctx, caller, req.GetApplicationId(),
		req.GetPagination().GetPageSize(), req.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	entries := make([]*applicationstatusv1.StatusHistoryEntry, 0, len(page.Entries))
	for _, e := range page.Entries {
		entries = append(entries, &applicationstatusv1.StatusHistoryEntry{
			PreviousStatus: statusToProto(e.PreviousStatus),
			NewStatus:      statusToProto(e.NewStatus),
			OccurredAt:     timestamppb.New(e.OccurredAt),
		})
	}
	return &applicationstatusv1.ListApplicationStatusHistoryResponse{
		Entries:    entries,
		Pagination: &commonv1.PaginationResponse{NextPageToken: page.NextPageToken, TotalCount: int64(page.TotalCount)},
	}, nil
}

// participantFrom trusts x-user-id and x-user-role only because the gateway
// sets them and strips any client-sent values (JT-31). The role is checked
// before any input, so an admin learns nothing about which IDs exist.
func participantFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	caller, err := domain.NewCaller(identity.UserID, identity.Role)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireParticipantRole(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

var protoStatus = map[domain.Status]applicationstatusv1.ApplicationStatus{
	domain.StatusSubmitted:   applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_SUBMITTED,
	domain.StatusUnderReview: applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_UNDER_REVIEW,
	domain.StatusShortlisted: applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_SHORTLISTED,
	domain.StatusInterview:   applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_INTERVIEW,
	domain.StatusOffered:     applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_OFFERED,
	domain.StatusAccepted:    applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED,
	domain.StatusDeclined:    applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_DECLINED,
	domain.StatusRejected:    applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_REJECTED,
	domain.StatusWithdrawn:   applicationstatusv1.ApplicationStatus_APPLICATION_STATUS_WITHDRAWN,
}

// statusToProto maps UNSPECIFIED (the first entry's previous status) to the
// zero enum value.
func statusToProto(s domain.Status) applicationstatusv1.ApplicationStatus {
	return protoStatus[s]
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidApplicationID), errors.Is(err, domain.ErrInvalidParticipantID),
		errors.Is(err, domain.ErrInvalidTime), errors.Is(err, domain.ErrInvalidPageSize),
		errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrRoleNotAllowed), errors.Is(err, domain.ErrNotParticipant):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrApplicationNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrApplicationExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrStatusChanged):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// ToStatus exposes the mapping so tests can check codes that no RPC returns
// yet, such as FAILED_PRECONDITION for a disallowed transition.
func ToStatus(err error) error { return toStatus(err) }
