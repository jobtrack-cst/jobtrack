package grpc

import (
	"context"
	"errors"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	companyverificationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/company_verification/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

// VerificationHandler serves the public CompanyVerificationService through
// the gateway. There is no internal service in the contract.
type VerificationHandler struct {
	companyverificationv1.UnimplementedCompanyVerificationServiceServer
	requests *app.Requests
}

var _ companyverificationv1.CompanyVerificationServiceServer = (*VerificationHandler)(nil)

func NewVerificationHandler(requests *app.Requests) *VerificationHandler {
	return &VerificationHandler{requests: requests}
}

func (h *VerificationHandler) SubmitVerificationRequest(ctx context.Context, req *companyverificationv1.SubmitVerificationRequestRequest) (*companyverificationv1.SubmitVerificationRequestResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	r, err := h.requests.Submit(ctx, caller, req.GetRegistrationNumber(), req.GetNotes())
	if err != nil {
		return nil, toStatus(err)
	}
	return &companyverificationv1.SubmitVerificationRequestResponse{Request: requestToProto(r)}, nil
}

func (h *VerificationHandler) ListMyVerificationRequests(ctx context.Context, req *companyverificationv1.ListMyVerificationRequestsRequest) (*companyverificationv1.ListMyVerificationRequestsResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	// A missing pagination message means the defaults, so nil is fine here.
	p := req.GetPagination()
	result, err := h.requests.ListMine(ctx, caller, p.GetPageSize(), p.GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &companyverificationv1.ListMyVerificationRequestsResponse{
		Requests:   requestsToProto(result.Requests),
		Pagination: paginationToProto(result),
	}, nil
}

func (h *VerificationHandler) ListVerificationRequests(ctx context.Context, req *companyverificationv1.ListVerificationRequestsRequest) (*companyverificationv1.ListVerificationRequestsResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	p := req.GetPagination()
	result, err := h.requests.List(ctx, caller, statusFromProto(req.GetStatus()), p.GetPageSize(), p.GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &companyverificationv1.ListVerificationRequestsResponse{
		Requests:   requestsToProto(result.Requests),
		Pagination: paginationToProto(result),
	}, nil
}

func (h *VerificationHandler) ApproveVerificationRequest(ctx context.Context, req *companyverificationv1.ApproveVerificationRequestRequest) (*companyverificationv1.ApproveVerificationRequestResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	r, err := h.requests.Approve(ctx, caller, req.GetRequestId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &companyverificationv1.ApproveVerificationRequestResponse{Request: requestToProto(r)}, nil
}

func (h *VerificationHandler) RejectVerificationRequest(ctx context.Context, req *companyverificationv1.RejectVerificationRequestRequest) (*companyverificationv1.RejectVerificationRequestResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	r, err := h.requests.Reject(ctx, caller, req.GetRequestId(), req.GetReason())
	if err != nil {
		return nil, toStatus(err)
	}
	return &companyverificationv1.RejectVerificationRequestResponse{Request: requestToProto(r)}, nil
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

// invalidStatus is outside every known value, so ValidFilter rejects it.
const invalidStatus domain.RequestStatus = 255

// statusFromProto keeps unknown enum numbers unknown instead of guessing.
func statusFromProto(s companyverificationv1.RequestStatus) domain.RequestStatus {
	switch s {
	case companyverificationv1.RequestStatus_REQUEST_STATUS_UNSPECIFIED:
		return domain.StatusUnspecified
	case companyverificationv1.RequestStatus_REQUEST_STATUS_PENDING:
		return domain.StatusPending
	case companyverificationv1.RequestStatus_REQUEST_STATUS_APPROVED:
		return domain.StatusApproved
	case companyverificationv1.RequestStatus_REQUEST_STATUS_REJECTED:
		return domain.StatusRejected
	default:
		return invalidStatus
	}
}

func statusToProto(s domain.RequestStatus) companyverificationv1.RequestStatus {
	switch s {
	case domain.StatusPending:
		return companyverificationv1.RequestStatus_REQUEST_STATUS_PENDING
	case domain.StatusApproved:
		return companyverificationv1.RequestStatus_REQUEST_STATUS_APPROVED
	case domain.StatusRejected:
		return companyverificationv1.RequestStatus_REQUEST_STATUS_REJECTED
	default:
		return companyverificationv1.RequestStatus_REQUEST_STATUS_UNSPECIFIED
	}
}

func requestToProto(r domain.VerificationRequest) *companyverificationv1.VerificationRequest {
	return &companyverificationv1.VerificationRequest{
		RequestId:          r.ID,
		EmployerId:         r.EmployerID,
		RegistrationNumber: r.RegistrationNumber,
		Notes:              r.Notes,
		Status:             statusToProto(r.Status),
		RejectionReason:    r.RejectionReason,
		ReviewedBy:         r.ReviewedBy,
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(r.CreatedAt),
			UpdatedAt: timestamppb.New(r.UpdatedAt),
		},
	}
}

func requestsToProto(items []domain.VerificationRequest) []*companyverificationv1.VerificationRequest {
	out := make([]*companyverificationv1.VerificationRequest, 0, len(items))
	for _, r := range items {
		out = append(out, requestToProto(r))
	}
	return out
}

func paginationToProto(result app.ListResult) *commonv1.PaginationResponse {
	return &commonv1.PaginationResponse{NextPageToken: result.NextPageToken, TotalCount: result.TotalCount}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidRequestID), errors.Is(err, domain.ErrInvalidRegistrationNumber),
		errors.Is(err, domain.ErrInvalidNotes), errors.Is(err, domain.ErrInvalidRejectionReason),
		errors.Is(err, domain.ErrInvalidStatusFilter), errors.Is(err, domain.ErrInvalidPageSize),
		errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrEmployerRoleRequired), errors.Is(err, domain.ErrAdminRoleRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrRequestNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrPendingRequestExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrAlreadyVerified), errors.Is(err, domain.ErrRequestNotPending):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
