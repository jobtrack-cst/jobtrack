# JT-44: Internal-call security test cases

Date: 7 Nov 2026. Scope: shared unary server authentication/authorization and configured internal client metadata. All tokens in tests are synthetic. No database, event, gateway, deployment or actual service implementation changes.

## Acceptance criteria and tests

| ID | Partition or boundary | Expected result | Automated evidence |
|---|---|---|---|
| JT44-01 | Missing, empty, wrong, same-length wrong, whitespace or control-character token | UNAUTHENTICATED; handler never called | `TestInternalInterceptor_SecurityBoundaries` |
| JT44-02 | Duplicate identical or conflicting token headers | UNAUTHENTICATED; handler never called | `TestInternalInterceptor_SecurityBoundaries` |
| JT44-03 | Missing, empty, disallowed, padded, differently cased, duplicate or conflicting caller | PERMISSION_DENIED after token validation | `TestInternalInterceptor_SecurityBoundaries` |
| JT44-04 | All eight approved full RPC names crossed with all five approved callers plus gateway, frontend and unknown caller | Only the exact Section 6 caller succeeds, including separate Auth GetUser/ListUsersByRole permissions | `TestInternalInterceptor_ExactRPCAllowlist` |
| JT44-05 | Unknown internal RPC or service with valid token | PERMISSION_DENIED | `TestInternalInterceptor_SecurityBoundaries` |
| JT44-06 | Public service and health RPC with absent or invalid internal metadata | Original handler/context/request/response preserved | `TestInternalInterceptor_SecurityBoundaries` |
| JT44-07 | Invalid server/client startup token or unapproved caller configuration | Constructor returns a generic error without secret values | `TestInternalInterceptor_InvalidConfiguration`, `TestInternalClient_Configuration` |
| JT44-08 | Incoming spoofed headers and duplicated/appended outgoing credentials across four attempts | One configured token/caller value; original context metadata unchanged; trace metadata preserved | `TestInternalClient_ConfiguredMetadataWinsAcrossRetries` |
| JT44-09 | Public call through configured internal client | Internal token and caller removed; no configured secret sent | `TestInternalClient_ConfiguredMetadataWinsAcrossRetries` |
| JT44-10 | No existing outgoing metadata; downstream business denial | Configured headers added; denial propagated without retry | `TestInternalClient_WithoutExistingMetadata` |
| JT44-11 | Generated Auth client/server over in-memory gRPC transport | GetUser/ListUsersByRole approved calls succeed, wrong RPC caller denied, wrong/missing token rejected, spoofed outgoing credentials replaced | `TestInternalInterceptors_GeneratedContractTransport` |
| JT44-12 | Existing public identity forwarding, timeout, cancellation, retry and circuit behavior | Existing regression suite passes unchanged | Existing `pkg/grpcclient/client_test.go` |

The transport regression uses `bufconn` from the existing gRPC dependency. No mocks or new dependencies are needed. All current internal contracts are unary; no database/broker integration tests are warranted by this change.

## Reproduce verification

Run from the repository root:

```sh
gofmt -l pkg/authctx pkg/grpcclient
git diff --check
go vet ./...
go test -coverprofile=/tmp/jt44-coverage.out ./...
go test -race ./pkg/authctx ./pkg/grpcclient
go build ./...
go tool cover -func=/tmp/jt44-coverage.out | rg 'pkg/(authctx|grpcclient)/internal.go'
buf lint
buf breaking --against '.git#ref=origin/main'
buf generate
git diff --exit-code -- gen
```

Executed results: formatting and diff checks, vet, repository unit tests with coverage, focused race tests, build, Buf lint/breaking/regeneration all passed. Go and Buf required sandbox escalation for cache/network access. Generated files remain unchanged.

Coverage: `pkg/authctx` 100.0%; `pkg/grpcclient` 93.0%. All newly added production functions have 100.0% statement coverage. Existing uncovered resilience branches account for the remaining client-package gap. The full repository total includes generated code and is not a meaningful measure of these helpers.

## Local CI image builds

Each command completed successfully (exit 0); images were built locally, never pushed:

```sh
docker build -f services/service-template/Dockerfile -t jobtrack-jt44-service-template:review .
docker build -f services/application/Dockerfile -t jobtrack-jt44-application:review .
docker build -f services/auth/Dockerfile -t jobtrack-jt44-auth:review .
docker build -f gateway/Dockerfile -t jobtrack-jt44-gateway:review .
```

Docker Desktop socket access required sandbox escalation. No running Compose services, volumes or credentials were changed.

## Adoption and review

AGENTS.md Section 6 contains startup wiring examples. Owners must install the server interceptor on internal servers and use the configured client interceptor on approved internal callers. No credentials are read automatically from the environment by either package. Public-only callers keep the existing client constructor; gateway and frontend must never receive INTERNAL_TOKEN. The shared-token impersonation risk without mTLS remains as accepted in AGENTS.md.

PR, hosted CI, merge and Jira time/status changes are intentionally pending user review; this task is not Done under the project Definition of Done.
