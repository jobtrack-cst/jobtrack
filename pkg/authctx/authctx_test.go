package authctx_test

import (
	"context"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/metadata"
)

func TestFromContext_ExtractsOnlyCompleteIncomingIdentity(t *testing.T) {
	const userID = "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name string
		ctx  context.Context
		want authctx.Identity
		ok   bool
	}{
		{
			name: "complete identity",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", userID, "x-user-role", "applicant")),
			want: authctx.Identity{UserID: userID, Role: "applicant"}, ok: true,
		},
		{
			name: "missing user ID",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "applicant")),
		},
		{
			name: "missing role",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", userID)),
		},
		{
			name: "empty user ID",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "", "x-user-role", "applicant")),
		},
		{
			name: "empty role",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", userID, "x-user-role", "")),
		},
		{
			name: "both values empty",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "", "x-user-role", "")),
		},
		{
			name: "whitespace user ID",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "  ", "x-user-role", "applicant")),
		},
		{
			name: "whitespace role",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", userID, "x-user-role", "  ")),
		},
		{
			name: "empty value lists",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.MD{"x-user-id": nil, "x-user-role": {}}),
		},
		{
			name: "no incoming metadata", ctx: context.Background(),
		},
		{
			name: "empty metadata", ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{}),
		},
		{
			name: "unrelated metadata",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", userID, "x-user-role", "employer", "request-id", "request-123")),
			want: authctx.Identity{UserID: userID, Role: "employer"}, ok: true,
		},
		{
			name: "only unrelated metadata",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("request-id", "request-123")),
		},
		{
			name: "outgoing identity is not incoming identity",
			ctx:  metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", userID, "x-user-role", "admin")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := authctx.FromContext(tt.ctx)
			if got != tt.want || ok != tt.ok {
				t.Errorf("FromContext() = (%+v, %t), want (%+v, %t)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
