package grpc

import (
	"context"
	"errors"
	"time"

	auditlogv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/audit_log/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

// AuditLogHandler serves the public AuditLogService through the gateway.
// There is no internal service in the contract; entries are written only by
// the Sprint 2 event consumer.
type AuditLogHandler struct {
	auditlogv1.UnimplementedAuditLogServiceServer
	auditLog *app.AuditLog
}

var _ auditlogv1.AuditLogServiceServer = (*AuditLogHandler)(nil)

func NewAuditLogHandler(auditLog *app.AuditLog) *AuditLogHandler {
	return &AuditLogHandler{auditLog: auditLog}
}

// The generated getters are nil-safe, so a nil request means no filters and
// the default page.
func (h *AuditLogHandler) ListAuditEntries(ctx context.Context, req *auditlogv1.ListAuditEntriesRequest) (*auditlogv1.ListAuditEntriesResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	// Timestamps are converted here, before the app, so the role is checked
	// first: a non-admin must not learn which timestamps are valid.
	if err := caller.RequireAdmin(); err != nil {
		return nil, toStatus(err)
	}
	start, err := optionalTime(req.GetStartTime())
	if err != nil {
		return nil, toStatus(err)
	}
	end, err := optionalTime(req.GetEndTime())
	if err != nil {
		return nil, toStatus(err)
	}
	p := req.GetPagination()
	result, err := h.auditLog.ListEntries(ctx, caller, app.ListEntriesInput{
		PageSize:  p.GetPageSize(),
		PageToken: p.GetPageToken(),
		EventType: req.GetEventType(),
		Start:     start,
		End:       end,
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &auditlogv1.ListAuditEntriesResponse{
		Entries:    entriesToProto(result.Entries),
		Pagination: &commonv1.PaginationResponse{NextPageToken: result.NextPageToken, TotalCount: result.TotalCount},
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

// optionalTime returns nil for an unset filter. CheckValid rejects seconds
// outside years 1 to 9999 and nanos outside 0 to 999999999.
func optionalTime(ts *timestamppb.Timestamp) (*time.Time, error) {
	if ts == nil {
		return nil, nil
	}
	if err := ts.CheckValid(); err != nil {
		return nil, domain.ErrInvalidTimestamp
	}
	t := ts.AsTime()
	return &t, nil
}

func entryToProto(e domain.Entry) *auditlogv1.AuditEntry {
	return &auditlogv1.AuditEntry{
		EventId:    e.EventID,
		EventType:  e.EventType,
		Producer:   e.Producer,
		OccurredAt: timestamppb.New(e.OccurredAt),
		DataJson:   e.DataJSON,
	}
}

func entriesToProto(items []domain.Entry) []*auditlogv1.AuditEntry {
	out := make([]*auditlogv1.AuditEntry, 0, len(items))
	for _, e := range items {
		out = append(out, entryToProto(e))
	}
	return out
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidEventType), errors.Is(err, domain.ErrInvalidTimestamp),
		errors.Is(err, domain.ErrInvalidTimeRange), errors.Is(err, domain.ErrInvalidPageSize),
		errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrAdminRoleRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
