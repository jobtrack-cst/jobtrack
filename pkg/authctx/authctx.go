package authctx

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"
)

// Identity contains caller metadata supplied by the API Gateway.
type Identity struct {
	UserID string
	Role   string
}

// FromContext returns the caller identity only when both incoming values are present.
// Services remain responsible for role and ownership checks.
func FromContext(ctx context.Context) (Identity, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Identity{}, false
	}
	ids, roles := md.Get("x-user-id"), md.Get("x-user-role")
	if len(ids) == 0 || len(roles) == 0 || strings.TrimSpace(ids[0]) == "" || strings.TrimSpace(roles[0]) == "" {
		return Identity{}, false
	}
	return Identity{UserID: ids[0], Role: roles[0]}, true
}
