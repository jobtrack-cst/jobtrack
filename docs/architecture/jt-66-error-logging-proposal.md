# JT-66 unexpected gRPC error logging — approved architecture and implementation

The user explicitly confirmed team agreement to this approach and approval for
Sangay-owned job-posting adoption on 20 November 2026. The approved wording is now
recorded in AGENTS.md Sections 9 and 15. This document retains the proposal
filename so existing references remain valid.

## Approved AGENTS.md Section 9 text

> Unexpected gRPC failures are logged once per failed RPC, at the gRPC adapter's
> error return, through `pkg/logging.GRPCError` and the service's existing JSON
> logger. Pass the original cause before status conversion, the generated full
> method constant, and the mapped status. Only `INTERNAL` emits an error entry,
> with `service`, `grpc_method`, `code`, safe `cause`, and `request_id` when a single
> canonical nonzero UUID is available in incoming `x-request-id` metadata. Other
> metadata is never copied. Do not also log that failure in the app, client,
> repository or a server interceptor. Separate downstream RPC invocations may
> each log their own failure; this is not global deduplication across services.
>
> Preserve discarded unexpected causes with `logging.InternalError` where needed;
> its diagnostic message must be a fixed code-owned literal. The adapter logs
> only those controlled labels and wrapped error types/status codes, never
> arbitrary error text, status messages/details, credentials, tokens, passwords,
> request/response bodies, private user data, or unrestricted metadata. Inject
> loggers without private attributes. The returned `INTERNAL` status uses a fixed
> sanitized message, with no original cause or status details attached. Unknown
> error text is deliberately withheld; add a controlled operation label rather
> than trusting or attempting to redact arbitrary strings.

## Approved Section 15 decision-log row

| Date | Decision | Reason |
|---|---|---|
| 20 Nov 2026 | JT-66: the team agreed to log unexpected `INTERNAL` failures once at the gRPC adapter using `pkg/logging.GRPCError`, preserving original causes until mapping and recording only controlled operation labels, wrapped types and status codes. Include service, full gRPC method, code and a validated request ID when available; return only fixed sanitized statuses. Service owners adopt the helper in their own services. Team agreement and approval for Sangay-owned job-posting adoption were explicitly confirmed by the user on 20 Nov 2026. | Logging before status conversion retains safe diagnostics that a server interceptor would otherwise lose, without exposing arbitrary error text or private request data. |

## Approval and ownership

- Team agreement and job-posting adoption are authorized by the user's explicit
  confirmation on 20 November 2026; no architecture/ownership approval remains
  pending for this scope.
- Changes cover shared logging, Phuntsho's Application, Offer, Messaging and Job
  Alert, and Sangay-owned job-posting. Other service owners retain control over
  later adoption. Resume Validator has no gRPC server/error mapping to change.
- Job-posting's employer client now retains a controlled GetCompanyProfile label,
  downstream code and domain sentinel in wrapped diagnostics. It discards the
  downstream message/details before returning to the app. Public, internal and
  health adapters use the same single logging boundary as the existing services.
  Startup passes the existing JSON logger into these handlers.

No new interceptor, contract, dependency, RPC, request-ID generator or gateway
change is proposed. Request IDs are currently optional: this change does not
establish end-to-end propagation through the gateway or outbound clients.

## Implementation and test cases

`pkg/logging/grpc.go` provides cause retention and the single logging boundary.
Application's app mapping previously discarded storage/dependency causes; it now
retains them without logging. Constructors accept the existing startup logger;
callers omitting it retain JSON output with the service name on stdout.

| Case | Test evidence |
|---|---|
| One JSON error record per unexpected failure, full method/service/code/request ID | Shared helper tests and every public RPC in the five service adapters |
| Safe wrapped cause and sanitized client status with no details/unwrap | Shared wrapper tests, Application app retention test, adapter failure tests |
| Employer verification equivalent, dependency down | Shared test uses a controlled GetCompanyProfile label wrapping `Unavailable` with a secret-bearing remote message; the actual PublishJob → app → employer-client path is additionally tested with a stubbed generated employer RPC client |
| Actual eligibility dependency error | Application adapter test injects an `Internal` Job Posting failure through its port |
| No credential, body, identity/email or arbitrary error-text leakage | Secret-bearing wrapped failures, request content/terms/location, and sensitive metadata injected in tests |
| Error renderer never invoked by diagnostic extraction | Error implementation panics if `Error()` is called |
| Expected codes produce no unexpected-error entry | Table covers OK, validation, auth, ownership, business, cancellation, deadline and availability codes |
| Health failures and invalid health state | All five health adapters |
| Untrusted request ID omitted | Arbitrary text, oversized value, nil UUID and duplicate values |

From the repository root, verify with:

```sh
gofmt -l pkg/logging services/application/internal services/offer/internal/adapters/grpc services/messaging/internal/adapters/grpc services/job-alert/internal/adapters/grpc services/application/cmd/server/main.go services/offer/cmd/server/main.go services/messaging/cmd/server/main.go services/job-alert/cmd/server/main.go services/job-posting/cmd/server/main.go services/job-posting/internal/adapters/grpc services/job-posting/internal/adapters/clients/employer.go
GOCACHE=/tmp/jobtrack-jt66-go-cache go test ./pkg/logging ./services/application/... ./services/offer/... ./services/messaging/... ./services/job-alert/... ./services/job-posting/...
GOCACHE=/tmp/jobtrack-jt66-go-cache go vet ./...
GOCACHE=/tmp/jobtrack-jt66-go-cache go test ./...
```

The wider suite needs permission to bind localhost test ports. Integration-tagged
DB/broker tests are unchanged: JT-66 adds no database/event behavior. CI's full
unit and integration suites remain required before merging.

## Verification observed on 20 November 2026

- `gofmt -w` applied to the new/changed job-posting Go files; `gofmt -l`
  across every changed Go file: no output.
- `git diff --check`: exit 0.
- Focused helper and five-service tests: exit 0; 34 packages passed, 10 packages
  reported no tests, zero failures.
- `go vet ./...`: exit 0, no diagnostics.
- `go test ./...`: exit 0; 159 packages passed, 75 packages reported no tests,
  zero failures.
- Tests used `GOCACHE=/tmp/jobtrack-jt66-go-cache`. Focused/full suites ran with
  approved localhost listener access for the existing server/client tests.
- Integration-tagged tests and a live Compose smoke test were not run. No
  database/event behavior changed; CI's integration suite and the remaining
  Definition of Done steps are still required before merge.

## Acceptance criteria review

| JT-66 criterion | Evidence |
|---|---|
| Team agrees and decision is recorded | User explicitly confirmed approval; AGENTS.md Section 9 and dated 20 Nov 2026 Section 15 row record it |
| INTERNAL logs its wrapped safe cause through existing JSON logging | Existing helper adopted at all job-posting public/internal/health error returns; real PublishJob employer adapter path retains the controlled operation, downstream code and sentinel |
| No tokens, passwords or bodies in logs | No request/response objects or unrestricted metadata are logged; actual employer message/details discarded; tests inject secrets into downstream errors, stored job data, request descriptions and metadata |
| Tests assert exactly one entry and sanitized client messages | `TestJobPosting_PublishEmployerFailureLogsOneSafeWrappedCause`, all-RPC/health failure tests, expected-error silence tests, shared and existing service tests pass |

The complete tracked diff and new JT-66 files were reviewed for scope, single
logging boundaries, original error retention, fixed public messages, safe fields,
startup wiring and regression coverage. No contract, dependency or unrelated
service changes were introduced. Existing JT-66 changes were preserved. No
implementation or architecture approval blocker remains within the authorized
scope. Request-ID propagation beyond available incoming metadata and adoption in
other owners' services remain outside this task.

No commit, push, PR or Jira mutation has been made. The normal manual follow-up is
a JT-66 commit, push, PR and movement of the Jira card to In Review.

## Changed files

- `pkg/logging/grpc.go`
- `pkg/logging/grpc_test.go`
- `services/application/cmd/server/main.go`
- `services/application/internal/adapters/grpc/application.go`
- `services/application/internal/adapters/grpc/health.go`
- `services/application/internal/adapters/grpc/logging_test.go`
- `services/application/internal/app/application.go`
- `services/application/internal/app/logging_test.go`
- `services/offer/cmd/server/main.go`
- `services/offer/internal/adapters/grpc/offer.go`
- `services/offer/internal/adapters/grpc/health.go`
- `services/offer/internal/adapters/grpc/logging_test.go`
- `services/messaging/cmd/server/main.go`
- `services/messaging/internal/adapters/grpc/messages.go`
- `services/messaging/internal/adapters/grpc/health.go`
- `services/messaging/internal/adapters/grpc/logging_test.go`
- `services/job-alert/cmd/server/main.go`
- `services/job-alert/internal/adapters/grpc/alerts.go`
- `services/job-alert/internal/adapters/grpc/health.go`
- `services/job-alert/internal/adapters/grpc/logging_test.go`
- `docs/architecture/jt-66-error-logging-proposal.md`
- `AGENTS.md`
- `services/job-posting/cmd/server/main.go`
- `services/job-posting/internal/adapters/clients/employer.go`
- `services/job-posting/internal/adapters/grpc/jobs.go`
- `services/job-posting/internal/adapters/grpc/health.go`
- `services/job-posting/internal/adapters/grpc/logging_test.go`
