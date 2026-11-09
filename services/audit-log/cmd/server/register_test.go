package main

import (
	"context"
	"testing"
	"time"

	auditlogv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/audit_log/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

// stubEntries is a fixed repository for wiring tests; the real one is
// covered by the Testcontainers integration test.
type stubEntries struct{}

func (stubEntries) RecordEntry(context.Context, domain.Entry) (bool, error) { return false, nil }

func (stubEntries) ListEntries(context.Context, domain.ListFilter, domain.Page) ([]domain.Entry, int64, error) {
	return []domain.Entry{{
		EventID: "33333333-3333-4333-8333-333333333333", EventType: "job.published", Producer: "job-posting",
		OccurredAt: time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC), DataJSON: `{}`,
	}}, 1, nil
}

// recordingRegistrar keeps what registerAuditLog registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterAuditLog_RegistersOnlyThePublicService(t *testing.T) {
	var reg recordingRegistrar
	registerAuditLog(stubEntries{})(&reg)
	if len(reg.descs) != 1 || reg.descs[0].ServiceName != "jobtrack.audit_log.v1.AuditLogService" {
		t.Fatalf("registered %v, want only jobtrack.audit_log.v1.AuditLogService", reg.descs)
	}
	server, ok := reg.impls[0].(auditlogv1.AuditLogServiceServer)
	if !ok {
		t.Fatalf("registered %T, not an AuditLogServiceServer", reg.impls[0])
	}

	ctxFor := func(role string) context.Context {
		return metadata.NewIncomingContext(context.Background(),
			metadata.Pairs("x-user-id", "22222222-2222-4222-8222-222222222222", "x-user-role", role))
	}
	resp, err := server.ListAuditEntries(ctxFor("admin"), &auditlogv1.ListAuditEntriesRequest{})
	if err != nil || len(resp.GetEntries()) != 1 || resp.GetPagination().GetTotalCount() != 1 {
		t.Fatalf("admin list: %v, %v", resp, err)
	}
	if _, err := server.ListAuditEntries(ctxFor("employer"), &auditlogv1.ListAuditEntriesRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("employer list: %v, want PermissionDenied", err)
	}
}
