# JT-33 test cases and verification evidence

## Scope and evidence rules

Jira: JT-33, Implement company-verification service basic operations (Sprint 1, Should). Branch: `JT-33-company-verification`. Follow [AGENTS.md](../../AGENTS.md), especially Sections 4, 6, 9, 10 and 13. The case identifiers below are local to JT-33.

All results were run by Jigden Shakya on his laptop (WSL Ubuntu, Docker Desktop) against commit `9486efc` with a clean working tree (`git status --short` printed nothing). Times are UTC on 8 Nov 2026. Commands below are also reproduction instructions; run them from the repository root.

The service was created with `bash scripts/new-service.sh company-verification` from the empty `.gitkeep` placeholder (commit `03aabdf`). The public contract `proto/jobtrack/company_verification/v1/company_verification.proto` and the generated code are unchanged. All five public RPCs in the contract are implemented: `SubmitVerificationRequest`, `ListMyVerificationRequests`, `ListVerificationRequests`, `ApproveVerificationRequest` and `RejectVerificationRequest`. The contract has no `*InternalService`.

Test design follows AGENTS.md Section 10: equivalence partitioning (each caller role, valid and invalid input classes, each status filter), boundary values (each length limit tested on and either side of the limit, counted in characters; pagination sizes and tokens) and state transitions (no request, PENDING, APPROVED, REJECTED, and two admins deciding at the same moment).

The repository integration test follows the JT-20 pattern used by the template, auth and employer: Testcontainers with `postgres:18-alpine`, build tag `integration`, migrations applied twice.

## Decisions approved for JT-33

The contract and AGENTS.md left these open. Jigden Shakya (task owner) approved them on 8 Nov 2026; they are also recorded in the AGENTS.md decision log.

1. `SubmitVerificationRequest` and `ListMyVerificationRequests` are employer only; `ListVerificationRequests`, `ApproveVerificationRequest` and `RejectVerificationRequest` are admin only. Other roles get `PERMISSION_DENIED`. The role is checked before the input and before any stored state.
2. The employer ID of a new request is always the caller's own user ID; no request field can target another employer.
3. `ListMyVerificationRequests` returns newest first (as the contract says); `ListVerificationRequests` returns oldest first, so admins handle the queue in order. A status filter outside the enum values is `INVALID_ARGUMENT`; `REQUEST_STATUS_UNSPECIFIED` means all statuses.
4. A `request_id` that is not a UUID is `INVALID_ARGUMENT`; a valid UUID with no request is `NOT_FOUND`.
5. Field limits, after trimming spaces and counted in characters: see the boundary table. NUL characters are refused, because PostgreSQL text columns cannot store them.
6. At most one PENDING and one APPROVED request per employer, enforced in the domain and by partial unique indexes, so two fast submits cannot both succeed. A decision only changes a row that is still PENDING, so when two admins decide at once one gets `FAILED_PRECONDITION`.

A forwarded identity whose user ID is not a UUID or whose role is unknown is treated as missing (`UNAUTHENTICATED`), because the gateway never sets such values. New requests start as `REQUEST_STATUS_PENDING` with an empty `reviewed_by`.

## Execution results

| Check | Command | Result (`9486efc`, 8 Nov 2026) |
|---|---|---|
| Formatting | `gofmt -l services/company-verification` | PASS, no output (08:28) |
| Static checks | `go vet ./services/company-verification/...` and `go vet -tags=integration ./services/company-verification/...` | PASS, no output |
| Unit tests | `go test -count=1 -cover ./services/company-verification/...` | PASS, all 6 packages with tests `ok` |
| Integration tests | `go test -count=1 -tags=integration ./services/company-verification/...` | PASS, all 6 packages `ok` (postgres 38.5 s, rabbitmq 31.4 s, real containers) |
| Compose build and start | `docker compose -f deploy/compose/docker-compose.yml up -d --build auth company-verification gateway` | PASS, images built from cache, company-verification started after Postgres and RabbitMQ were healthy |
| Compose health | `docker compose ... ps auth company-verification gateway` | PASS, company-verification `Up 11 seconds (healthy)`; ports 8080 and 9090 not published |
| Health endpoint | `docker compose ... exec company-verification wget -S -qO- http://localhost:8080/healthz` | PASS, `HTTP/1.1 200 OK`, body `ok` (08:01:54, image built from the same service code before the Compose commit) |
| Startup logs | `docker compose ... logs company-verification` | PASS, `servers starting` with `"service":"company-verification"`, no errors |
| Migrations on startup | `docker compose ... exec postgres psql -U postgres -d company_verification -c '\dt'` | PASS, `goose_db_version` and `verification_requests`, both owned by `company_verification`; goose versions 0, 1 and 2 applied |
| Missing configuration | `docker compose ... run --rm --no-deps -e DATABASE_URL= company-verification` | PASS, exits 1 with `invalid startup configuration` and `DATABASE_URL is required`; no password in the output (08:30) |
| Through the gateway | Manual smoke checks, see JT33-S-03 | PASS, 21 of 21 (08:27:27) |

## Unit test coverage (`9486efc`)

Statement coverage from `go test -count=1 -cover ./services/company-verification/...`. AGENTS.md sets an 80 percent target only for `application` and `application-status`; for company-verification the rule is meaningful tests for every business rule.

| Package | Coverage |
|---|---:|
| `services/company-verification/internal/domain` | 100.0% |
| `services/company-verification/internal/app` | 100.0% |
| `services/company-verification/internal/adapters/grpc` | 98.9% |
| `services/company-verification/cmd/server` | 83.3% |
| `services/company-verification/internal/adapters/rabbitmq` | 83.3% (template code, unchanged) |
| `services/company-verification/internal/adapters/postgres` | 22.7% in the unit run; the repository is covered by the integration tests |

## Boundary values

All text limits count characters after trimming spaces. The database columns (`VARCHAR(n)`, which also counts characters) back up each limit.

| Input | Below or invalid | On the boundary (valid) | Above (invalid) | Test |
|---|---|---|---|---|
| `registration_number` | empty; spaces only; contains NUL | 1 character; 50 multibyte characters (`ä`) | 51 characters | `TestNewSubmission`, `TestRequests_DatabaseLimits` |
| `notes` | contains NUL | empty; 1000 characters | 1001 characters | `TestNewSubmission`, `TestRequests_DatabaseLimits` |
| `reason` (reject) | empty; spaces only; contains NUL | 1 character; 500 multibyte characters | 501 characters | `TestNewRejectionReason`, `TestRequests_DatabaseLimits` |
| `request_id` | empty; `req-1`; braces; no hyphens; 35 or 37 characters | lower- and upper-case UUID (upper case is lowercased) | | `TestNormalizeID`, `TestRequests_Approve`, `TestRequests_Reject` |
| `page_size` | -1 | 0 (default 20); 1; 100 | 101 and 500 (capped at 100) | `TestNewPage`, `TestRequests_List` |
| `page_token` | `abc`; `-5`; `+5`; `05` | empty; `0`; `20` | | `TestNewPage`, `TestRequests_ListMine` |
| `status` filter | enum number 7; -1; domain value 99 | UNSPECIFIED, PENDING, APPROVED, REJECTED | | `TestRequestStatus_ValidFilter`, `TestVerificationHandler_ListVerificationRequests` |
| Forwarded `x-user-id` and `x-user-role` | missing; user ID not a UUID; role `superuser` or empty | UUID with `applicant`, `employer` or `admin` | | `TestNewCaller`, `TestVerificationHandler_RejectsBadIdentityOnEveryRPC` |

## Authorization matrix

Every cell is covered by table-driven unit tests in `internal/app/requests_test.go` and `internal/adapters/grpc/verification_test.go` (`TestVerificationHandler_RoleChecks`).

| RPC | Employer | Admin | Applicant | No identity |
|---|---|---|---|---|
| `SubmitVerificationRequest` | OK, keyed by own ID | `PERMISSION_DENIED` | `PERMISSION_DENIED` | `UNAUTHENTICATED` |
| `ListMyVerificationRequests` | OK, own requests only | `PERMISSION_DENIED` | `PERMISSION_DENIED` | `UNAUTHENTICATED` |
| `ListVerificationRequests` | `PERMISSION_DENIED` | OK, all employers | `PERMISSION_DENIED` | `UNAUTHENTICATED` |
| `ApproveVerificationRequest` | `PERMISSION_DENIED` | OK | `PERMISSION_DENIED` | `UNAUTHENTICATED` |
| `RejectVerificationRequest` | `PERMISSION_DENIED` | OK | `PERMISSION_DENIED` | `UNAUTHENTICATED` |

## State transitions

| From | Event | To or result | Test |
|---|---|---|---|
| no request, or only REJECTED | employer submits | PENDING | `TestCheckCanSubmit`, `TestRequests_Submit` |
| PENDING exists | employer submits | `ALREADY_EXISTS` | `TestCheckCanSubmit`, `TestRequests_CreateGetAndStatuses` |
| APPROVED exists | employer submits | `FAILED_PRECONDITION` | `TestCheckCanSubmit`, `TestRequests_Submit` |
| PENDING | admin approves | APPROVED, `reviewed_by` set, no reason | `TestVerificationRequest_Decisions`, `TestRequests_Decide` |
| PENDING | admin rejects with reason | REJECTED, `reviewed_by` and `rejection_reason` set | `TestVerificationRequest_Decisions`, `TestRequests_Decide` |
| APPROVED or REJECTED | admin approves or rejects | `FAILED_PRECONDITION`, row unchanged | `TestVerificationRequest_Decisions`, `TestRequests_Decide` |
| PENDING | two admins decide at once | one succeeds, the other `FAILED_PRECONDITION` | `TestRequests_Decide_ConcurrentAdmins` (run 30 times without a failure) |

## Acceptance test cases

### JT33-A-01: Employer submits a verification request

- Criterion: Employer submits a verification request.
- Inputs: valid submit with spaces to trim; a second submit while PENDING; a submit after a rejection; a submit after an approval; an empty registration number; a submit by an admin or applicant.
- Expected: the request is stored as PENDING, keyed by the caller's user ID, with a new UUID and trimmed fields; a second PENDING request returns `ALREADY_EXISTS` (also enforced by the `verification_requests_one_pending` index); a new request is allowed after a rejection; after an approval `FAILED_PRECONDITION`; bad input `INVALID_ARGUMENT`; other roles `PERMISSION_DENIED`. The employer sees their own requests newest first.
- Tests: `internal/domain/request_test.go` (`TestNewSubmission`, `TestCheckCanSubmit`, `TestNewVerificationRequest_StartsPendingInUTC`); `internal/app/requests_test.go` (`TestRequests_Submit`, `TestRequests_ListMine`); `internal/adapters/grpc/verification_test.go` (`TestVerificationHandler_SubmitVerificationRequest`, `TestVerificationHandler_ListMyVerificationRequests`); `internal/adapters/postgres/requests_integration_test.go` (`TestRequests_CreateGetAndStatuses`, `TestRequests_Lists`).
- Command: `go test -count=1 ./services/company-verification/internal/...` and `go test -count=1 -tags=integration ./services/company-verification/internal/adapters/postgres/`
- Status: PASS.

### JT33-A-02: Admin lists pending requests and approves or rejects with a reason

- Criterion: Admin lists pending requests and approves or rejects with a reason.
- Inputs: list with each status filter and with pagination; approve a PENDING request; reject a PENDING request with a reason; reject with an empty or spaces-only reason; an ID that is not a UUID; an unknown ID.
- Expected: the list returns only the chosen status (or all for UNSPECIFIED), oldest first, with `total_count` and `next_page_token`; approve sets APPROVED and `reviewed_by` to the admin's user ID; reject also stores the trimmed reason; an empty reason returns `INVALID_ARGUMENT`; bad IDs `INVALID_ARGUMENT`; unknown IDs `NOT_FOUND`.
- Tests: `internal/domain/request_test.go` (`TestNewRejectionReason`, `TestVerificationRequest_Decisions`); `internal/app/requests_test.go` (`TestRequests_List`, `TestRequests_Approve`, `TestRequests_Reject`); `internal/adapters/grpc/verification_test.go` (`TestVerificationHandler_ListVerificationRequests`, `TestVerificationHandler_ApproveVerificationRequest`, `TestVerificationHandler_RejectVerificationRequest`); `internal/adapters/postgres/requests_integration_test.go` (`TestRequests_Decide`, `TestRequests_Lists`).
- Command: `go test -count=1 ./services/company-verification/internal/...` and `go test -count=1 -tags=integration ./services/company-verification/internal/adapters/postgres/`
- Status: PASS.

### JT33-A-03: Only admins can decide; deciding an already decided request returns FAILED_PRECONDITION

- Criterion: Only admins can decide; deciding an already decided request returns `FAILED_PRECONDITION`.
- Inputs: approve and reject as employer and applicant; approve or reject an APPROVED or REJECTED request; two admins deciding the same PENDING request at the same moment.
- Expected: non-admins get `PERMISSION_DENIED` before the request is loaded; any decision on a non-PENDING request returns `FAILED_PRECONDITION` and leaves the row unchanged; with two concurrent admins exactly one wins, because the `UPDATE` only matches rows that are still `pending`.
- Tests: `internal/domain/caller_test.go` (`TestCaller_RoleChecks`); `internal/domain/request_test.go` (`TestVerificationRequest_Decisions`); `internal/app/requests_test.go` (`TestRequests_Approve`, `TestRequests_Reject`, including "another admin decided first"); `internal/adapters/grpc/verification_test.go` (`TestVerificationHandler_RoleChecks`); `internal/adapters/postgres/requests_integration_test.go` (`TestRequests_Decide`, `TestRequests_Decide_ConcurrentAdmins`).
- Command: `go test -count=1 ./services/company-verification/internal/...` and `go test -count=1 -tags=integration ./services/company-verification/internal/adapters/postgres/`
- Status: PASS.

### JT33-A-04: Role and identity checks use pkg/authctx with the Section 9 codes

- Criterion: Role and ownership checks use `pkg/authctx` and return the gRPC codes from AGENTS.md Section 9.
- Inputs: on every RPC: no metadata, missing role, user ID not a UUID, unknown role; each domain error; a database failure carrying driver text.
- Expected: identity problems return `UNAUTHENTICATED` on all five RPCs; wrong role `PERMISSION_DENIED`; bad input `INVALID_ARGUMENT`; missing request `NOT_FOUND`; duplicate pending request `ALREADY_EXISTS`; business rule violations `FAILED_PRECONDITION`; canceled and expired contexts `CANCELED` and `DEADLINE_EXCEEDED`; anything else `INTERNAL` with the fixed message `internal error` and no driver detail.
- Tests: `internal/domain/caller_test.go` (`TestNewCaller`, `TestParseRole_RejectsUnknownValues`); `internal/adapters/grpc/verification_test.go` (`TestVerificationHandler_RejectsBadIdentityOnEveryRPC`, `TestVerificationHandler_HidesUnexpectedErrors` and the per-RPC tables); `cmd/server/verification_setup_test.go` (`TestServe_AnswersVerificationRPCsWithForwardedIdentity`, over a real gRPC connection).
- Command: `go test -count=1 ./services/company-verification/...`
- Status: PASS.

### JT33-A-05: Events are Sprint 2

- Criterion: Publishing `employer.verified` and `employer.verification_rejected` is Sprint 2.
- Expected: no events are published in Sprint 1. The template RabbitMQ publisher connection is opened at startup and kept for Sprint 2; `main.go` notes where publishing will be added.
- Status: Not applicable in Sprint 1, by design.

### JT33-S-01: Migrations, repository and database constraints

- Criterion: goose migrations run on startup; maximum lengths for free-text fields are set; repository integration test with Testcontainers.
- Prerequisites: Docker; `docker pull postgres:17-alpine && docker pull postgres:18-alpine && docker pull rabbitmq:4.1-alpine` (17 is used by the template readiness test).
- Expected: migrations apply and can run twice; timestamps are stored and returned in UTC with microsecond precision; at most one PENDING and one APPROVED request per employer; unknown IDs map to `ErrRequestNotFound`; already decided rows map to `ErrRequestNotPending`; lists return the right order, page and `total_count`, including an offset past the end; the database rejects values over each limit, an empty registration number and an `employer_id` that is not a UUID.
- Tests: `internal/adapters/postgres/requests_integration_test.go` (`TestRequests_CreateGetAndStatuses`, `TestRequests_Decide`, `TestRequests_Decide_ConcurrentAdmins`, `TestRequests_Lists` with 9 subtests, `TestRequests_DatabaseLimits` with 7 subtests); `internal/adapters/postgres/requests_test.go` (`TestStatusMapping_RoundTrips`, `TestStatusMapping_RejectsUnknownValues`, `TestUniqueError_MapsOnlyKnownIndexes`, `TestNullable`); `internal/adapters/postgres/readiness_integration_test.go` (template).
- Command: `go test -count=1 -tags=integration ./services/company-verification/internal/adapters/postgres/`
- Status: PASS.

### JT33-S-02: Docker Compose startup and health

- Criterion: Service block in `deploy/compose/docker-compose.yml`; `docker compose up` starts it and `GET /healthz` returns 200.
- Prerequisites: `deploy/compose/.env` from `.env.example`.
- Expected: the image builds; company-verification waits for healthy Postgres and RabbitMQ, applies migrations and becomes healthy; `/healthz` returns 200; ports 9090 and 8080 are not published to the host; the database password is passed as `PGPASSWORD`, not inside the URL; a missing `DATABASE_URL` stops startup with exit code 1.
- References: `deploy/compose/docker-compose.yml` (company-verification block), `services/company-verification/Dockerfile`, `cmd/server/main_test.go` (template health and startup order tests).
- Command: the Compose commands in the execution results table.
- Status: PASS on `9486efc`.

### JT33-S-03: Public RPCs through the API gateway (manual)

- Purpose: extra evidence that the five routes work end to end with real JWTs from auth. Not required by the acceptance criteria.
- Setup: `docker compose -f deploy/compose/docker-compose.yml up -d --build auth company-verification gateway`; a new synthetic employer and applicant registered and logged in through `POST /v1/auth/register` and `POST /v1/auth/login` on `http://localhost:8000`; the seeded admin from `deploy/compose/.env`.
- Result on `9486efc` (8 Nov 2026 08:27:27 UTC), run with a local script that is not committed:

| # | Request | Caller | Expected | Result |
|---|---|---|---|---|
| 1 | `POST /v1/verification/requests` with spaces around the number | employer | 200, PENDING, own `employerId`, trimmed number | PASS |
| 2 | `POST /v1/verification/requests` again | employer | 409 (`ALREADY_EXISTS`) | PASS |
| 3 | `POST /v1/verification/requests` with a spaces-only number | employer | 400 (`INVALID_ARGUMENT`) | PASS |
| 4 | `POST /v1/verification/requests` | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 5 | `GET /v1/verification/my-requests` | employer | 200, 1 request, `totalCount` 1 | PASS |
| 6 | `GET /v1/verification/requests` | employer | 403 (`PERMISSION_DENIED`) | PASS |
| 7 | `GET /v1/verification/requests?status=REQUEST_STATUS_PENDING` | admin | 200, only PENDING, includes the new request | PASS |
| 8 | `GET /v1/verification/requests?pagination.pageSize=-1` | admin | 400 (`INVALID_ARGUMENT`) | PASS |
| 9 | `POST .../{request_id}/reject` with a spaces-only reason | admin | 400 (`INVALID_ARGUMENT`) | PASS |
| 10 | `POST .../{request_id}/reject` | employer | 403 (`PERMISSION_DENIED`) | PASS |
| 11 | `POST .../{request_id}/reject` with a reason | admin | 200, REJECTED, reason and `reviewedBy` set | PASS |
| 12 | `POST .../{request_id}/approve` on the rejected request | admin | 400 (`FAILED_PRECONDITION`) | PASS |
| 13 | `POST /v1/verification/requests` after the rejection | employer | 200, PENDING, new `requestId` | PASS |
| 14 | `POST .../{request_id}/approve` | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 15 | `POST .../{request_id}/approve` | admin | 200, APPROVED, `reviewedBy` set, no reason | PASS |
| 16 | `POST .../{request_id}/reject` on the approved request | admin | 400 (`FAILED_PRECONDITION`) | PASS |
| 17 | `POST /v1/verification/requests` after the approval | employer | 400 (`FAILED_PRECONDITION`) | PASS |
| 18 | `POST /v1/verification/requests/not-a-uuid/approve` | admin | 400 (`INVALID_ARGUMENT`) | PASS |
| 19 | `POST /v1/verification/requests/00000000-0000-4000-8000-000000000000/approve` | admin | 404 (`NOT_FOUND`) | PASS |
| 20 | `GET /v1/verification/my-requests` | employer | 200, 2 requests, newest first | PASS |
| 21 | `GET /v1/verification/my-requests` without a token | none | 401 (`UNAUTHENTICATED`) | PASS |

The gateway maps `FAILED_PRECONDITION` and `INVALID_ARGUMENT` both to HTTP 400; the script told them apart by the gRPC `code` in the response body (9 and 3).

## Not covered or pending

- Publishing `employer.verified` and `employer.verification_rejected`: Sprint 2, not part of JT-33. Until then the employer profile is not updated when a request is decided.
- Real client adapters and events: Sprint 2. Company-verification calls no other services, so there are no client ports in Sprint 1.
- Internal-token protection: not relevant, because the contract has no `*InternalService`.
- Race tests on Jigden's laptop: not run (cgo disabled).
- Whole repository after merging `main`, and GitHub Actions CI on the final commit: checked before merging the PR.
