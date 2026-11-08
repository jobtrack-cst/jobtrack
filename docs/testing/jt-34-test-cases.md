# JT-34 test cases and verification evidence

## Scope and evidence rules

Jira: JT-34, Implement moderation service basic operations (Sprint 1, Should). Branch: `JT-34-moderation`. Follow [AGENTS.md](../../AGENTS.md), especially Sections 4, 6, 9, 10 and 13. The case identifiers below are local to JT-34.

All results were run by Jigden Shakya on his laptop (WSL Ubuntu, Docker Desktop) on 8 Nov 2026. Times are UTC. Each result names the commit whose service code it ran against; the service code has not changed since `0bc7f3a` (only this document and AGENTS.md follow). Commands below are also reproduction instructions; run them from the repository root.

The service was created with `bash scripts/new-service.sh moderation` from the empty `.gitkeep` placeholder (commit `74f3c60`). The public contract `proto/jobtrack/moderation/v1/moderation.proto` and the generated code are unchanged. All four public RPCs in the contract are implemented: `ReportJob`, `ListReports`, `DismissReport` and `RemoveJob`. The contract has no `*InternalService`, and moderation calls no other service.

Test design follows AGENTS.md Section 10: equivalence partitioning (each caller role, valid and invalid input classes, each status filter), boundary values (each length limit tested on and either side of the limit, counted in characters; pagination sizes and tokens) and state transitions (OPEN, DISMISSED, ACTIONED, removed job, and races between two callers).

The repository integration test follows the JT-20 pattern used by the template, auth, employer and company-verification: Testcontainers with `postgres:18-alpine`, build tag `integration`, migrations applied twice.

## Decisions approved for JT-34

The contract and AGENTS.md left these open. Jigden Shakya (task owner) approved them on 8 Nov 2026; they are also recorded in the AGENTS.md decision log.

1. `ReportJob` is open to every logged-in role (applicant, employer and admin), as the contract says "any logged-in user". `ListReports`, `DismissReport` and `RemoveJob` are admin only; other roles get `PERMISSION_DENIED`. The role is checked before the input and before any stored state.
2. The `reporter_id` of a new report is always the caller's own user ID; no request field can set it.
3. "Job already removed" (contract, `ReportJob` and `RemoveJob`): moderation has no allowed synchronous call to job-posting (AGENTS.md Section 6), so it cannot check whether a job exists or is open. It keeps its own `removed_jobs` table and knows only the jobs it removed. Any well-formed job ID can be reported; reporting or removing a job moderation already removed returns `FAILED_PRECONDITION`.
4. `RemoveJob` works even when the job has no reports. It marks every OPEN report for the job as ACTIONED and stores the reason and the admin's user ID for `moderation.job_removed` in Sprint 2. DISMISSED reports stay DISMISSED.
5. At most one OPEN report per reporter and job (`ALREADY_EXISTS`), enforced by a partial unique index; after a dismissal the same user can report again. When a job is removed, `FAILED_PRECONDITION` wins over `ALREADY_EXISTS`.
6. `ReportJob` and `RemoveJob` take a per-job PostgreSQL transaction lock (`pg_advisory_xact_lock`), so a report can never be left OPEN on a removed job. `DismissReport` only updates a row that is still OPEN, so when two admins dismiss at once, one gets `FAILED_PRECONDITION`.
7. `ListReports` returns newest first (as the contract says), with `report_id` breaking ties so paging is stable. `REPORT_STATUS_UNSPECIFIED` means all statuses; an enum number outside the known values is `INVALID_ARGUMENT`, checked after the role.
8. Field limits, after trimming spaces and counted in characters: see the boundary table. NUL characters are refused, because PostgreSQL text columns cannot store them. A `job_id` or `report_id` that is not a UUID is `INVALID_ARGUMENT`; a valid `report_id` with no report is `NOT_FOUND`.

A forwarded identity whose user ID is not a UUID or whose role is unknown is treated as missing (`UNAUTHENTICATED`), because the gateway never sets such values. New reports start as `REPORT_STATUS_OPEN`.

## Execution results

| Check | Command | Result (8 Nov 2026) |
|---|---|---|
| Formatting | `gofmt -l services/moderation` | PASS, no output (service code of `d7ab514`) |
| Static checks | `go vet ./services/moderation/...` and `go vet -tags=integration ./services/moderation/...` | PASS, no output |
| Unit tests | `go test -count=1 -cover ./services/moderation/...` | PASS, all 6 packages with tests `ok` (service code of `d7ab514`) |
| Integration tests | `go test -count=1 -tags=integration ./services/moderation/...` | PASS, all 6 packages `ok` (postgres 41.0 s, rabbitmq 27.4 s, real containers; repository code of `f5936f9`, unchanged since) |
| Compose build and start | `docker compose -f deploy/compose/docker-compose.yml up -d --build moderation` | PASS, image built, moderation started after Postgres and RabbitMQ were healthy |
| Compose health | `docker compose ... ps moderation` | PASS, `Up 31 seconds (healthy)`; ports 8080 and 9090 not published (`8080/tcp, 9090/tcp`) |
| Health endpoint | `docker compose ... exec moderation wget -S -qO- http://localhost:8080/healthz` | PASS, `HTTP/1.1 200 OK`, body `ok` (10:06:32, service code of `0bc7f3a`) |
| Startup logs and migrations | `docker compose ... logs moderation` | PASS, `servers starting` with `"service":"moderation"`, no errors. Startup applies the goose migrations before the servers start (`TestRunWithDatabase_OrdersStartupAndCleansUp`), so this line also shows `00002_create_reports.sql` applied |
| Through the gateway | Manual smoke checks, see JT34-S-03 | PASS, 28 of 28 (about 10:13, `0bc7f3a`) |

## Unit test coverage (`d7ab514`)

Statement coverage from `go test -count=1 -cover ./services/moderation/...`. AGENTS.md sets an 80 percent target only for `application` and `application-status`; for moderation the rule is meaningful tests for every business rule.

| Package | Coverage |
|---|---:|
| `services/moderation/internal/domain` | 98.4% |
| `services/moderation/internal/app` | 100.0% |
| `services/moderation/internal/adapters/grpc` | 97.3% |
| `services/moderation/cmd/server` | 83.3% |
| `services/moderation/internal/adapters/rabbitmq` | 83.3% (template code, unchanged) |
| `services/moderation/internal/adapters/postgres` | 18.3% in the unit run; the repository is covered by the integration tests |

## Boundary values

All text limits count characters after trimming spaces. The database columns (`VARCHAR(n)`, which also counts characters) back up each limit.

| Input | Below or invalid | On the boundary (valid) | Above (invalid) | Test |
|---|---|---|---|---|
| `reason` (ReportJob) | empty; spaces only; contains NUL; invalid UTF-8 | 1 character; 1000 characters; 1000 multibyte characters (`é`, `ä`) | 1001 characters | `TestNewReportReason`, `TestReports_Report`, `TestReports_DatabaseLimits` |
| `reason` (RemoveJob) | empty; spaces only; contains NUL; invalid UTF-8 | 1 character; 500 characters; 500 multibyte characters | 501 characters | `TestNewRemovalReason`, `TestReports_RemoveJob`, `TestReports_DatabaseLimits` |
| `job_id`, `report_id` | empty; `job-1`; no hyphens; 35 or 37 characters; non-hex; surrounding spaces | lower- and upper-case UUID (upper case is lowercased) | | `TestNormalizeID`, `TestReports_Report`, `TestReports_Dismiss`, `TestReports_RemoveJob` |
| `page_size` | -1 | 0 (default 20); 1; 100 | 101 and 500 (capped at 100) | `TestNewPage`, `TestReports_List` |
| `page_token` | `abc`; `-1`; `+5`; `007`; ` 20` | empty; `0`; `2`; `20` | | `TestNewPage`, `TestReports_List` |
| `status` filter | enum number 4; -1; 256 (would wrap to 0 as a byte) | UNSPECIFIED, OPEN, DISMISSED, ACTIONED | | `TestParseStatusFilter`, `TestModerationHandler_ListReports` |
| Forwarded `x-user-id` and `x-user-role` | missing; user ID not a UUID; role `superuser`, `Admin` or empty | UUID with `applicant`, `employer` or `admin` | | `TestNewCaller`, `TestModerationHandler_RejectsBadIdentityOnEveryRPC` |

## Authorization matrix

Every cell is covered by table-driven unit tests in `internal/app/reports_test.go` and `internal/adapters/grpc/moderation_test.go` (`TestModerationHandler_RoleMatrix`, 12 cases, and `TestModerationHandler_RoleBeforeInput`).

| RPC | Applicant | Employer | Admin | No identity |
|---|---|---|---|---|
| `ReportJob` | OK, keyed by own ID | OK, keyed by own ID | OK, keyed by own ID | `UNAUTHENTICATED` |
| `ListReports` | `PERMISSION_DENIED` | `PERMISSION_DENIED` | OK, all reports | `UNAUTHENTICATED` |
| `DismissReport` | `PERMISSION_DENIED` | `PERMISSION_DENIED` | OK | `UNAUTHENTICATED` |
| `RemoveJob` | `PERMISSION_DENIED` | `PERMISSION_DENIED` | OK | `UNAUTHENTICATED` |

## State transitions

| From | Event | To or result | Test |
|---|---|---|---|
| no report by this user on the job | user reports | OPEN | `TestNewReport_StartsOpenInUTC`, `TestReports_CreateAndGet` |
| OPEN report by this user on the job | same user reports | `ALREADY_EXISTS` | `TestReports_CreateAndGet`, `TestReports_Create_ConcurrentDuplicates` (two at once, exactly one wins) |
| DISMISSED report by this user | same user reports | new OPEN report | `TestReports_CreateAndGet` |
| OPEN | admin dismisses | DISMISSED, `updated_at` set | `TestReport_Dismiss`, `TestReports_Dismiss` |
| DISMISSED or ACTIONED | admin dismisses | `FAILED_PRECONDITION`, row unchanged | `TestReport_Dismiss`, `TestReports_Dismiss` |
| OPEN | two admins dismiss at once | one succeeds, the other `FAILED_PRECONDITION` | `TestReports_Dismiss_ConcurrentAdmins` |
| job not removed (with or without reports) | admin removes | job recorded in `removed_jobs`; OPEN reports ACTIONED; DISMISSED reports unchanged; other jobs untouched | `TestReports_RemoveJob` |
| job removed | admin removes again | `FAILED_PRECONDITION` | `TestReports_RemoveJob`, `TestReports_RemoveJob_ConcurrentAdmins` (two at once, exactly one wins) |
| job removed | anyone reports it | `FAILED_PRECONDITION` | `TestReports_RemoveJob` |
| report and removal of the same job at the same moment | both run | the report is either refused or stored and then ACTIONED, never left OPEN | `TestReports_ReportRacingRemoval_NeverLeavesOpenReport` (20 rounds) |

## Acceptance test cases

### JT34-A-01: Users report a job posting with a reason

- Criterion: Users report a job posting with a reason.
- Inputs: a report by an applicant, an employer and an admin; a reason with spaces to trim; an upper-case job ID; a second report by the same user while OPEN; a report after a dismissal; an empty, spaces-only or 1001-character reason; a job ID that is not a UUID.
- Expected: the report is stored as OPEN with a new UUID, the trimmed reason, the lower-case job ID and the caller's user ID as `reporter_id`; a second OPEN report by the same user returns `ALREADY_EXISTS` (also enforced by the `reports_one_open_per_reporter` index); a new report is allowed after a dismissal; bad input returns `INVALID_ARGUMENT`.
- Tests: `internal/domain/report_test.go` (`TestNewReportReason`, `TestNewReport_StartsOpenInUTC`); `internal/app/reports_test.go` (`TestReports_Report`, `TestReports_Report_NewIDEachTime`); `internal/adapters/grpc/moderation_test.go` (`TestModerationHandler_ReportJob`); `internal/adapters/postgres/reports_integration_test.go` (`TestReports_CreateAndGet`, `TestReports_Create_ConcurrentDuplicates`).
- Command: `go test -count=1 ./services/moderation/internal/...` and `go test -count=1 -tags=integration ./services/moderation/internal/adapters/postgres/`
- Status: PASS.

### JT34-A-02: Admin lists reports and takes down a job

- Criterion: Admin lists reports and takes down a job.
- Inputs: list with each status filter and with pagination; take down a job with OPEN and DISMISSED reports; take down a job with no reports; a takedown with an empty, spaces-only or 501-character reason; an admin dismissing an OPEN report.
- Expected: the list returns only the chosen status (or all for UNSPECIFIED), newest first with `report_id` breaking ties, with `total_count` and `next_page_token`; the takedown returns the lower-case `job_id`, marks the job's OPEN reports ACTIONED and leaves DISMISSED reports alone; a job with no reports can be removed; a bad reason returns `INVALID_ARGUMENT`; a dismissal sets DISMISSED.
- Tests: `internal/domain/report_test.go` (`TestParseStatusFilter`, `TestNewRemovalReason`, `TestReport_Dismiss`, `TestNewRemoval_RecordsAdminInUTC`); `internal/app/reports_test.go` (`TestReports_List`, `TestReports_Dismiss`, `TestReports_RemoveJob`); `internal/adapters/grpc/moderation_test.go` (`TestModerationHandler_ListReports`, `TestModerationHandler_DismissReport`, `TestModerationHandler_RemoveJob`); `internal/adapters/postgres/reports_integration_test.go` (`TestReports_List`, `TestReports_Dismiss`, `TestReports_RemoveJob`).
- Command: `go test -count=1 ./services/moderation/internal/...` and `go test -count=1 -tags=integration ./services/moderation/internal/adapters/postgres/`
- Status: PASS.

### JT34-A-03: Only admins can take down; a second takedown of the same job returns FAILED_PRECONDITION

- Criterion: Only admins can take down; a second takedown of the same job returns `FAILED_PRECONDITION`.
- Inputs: `RemoveJob`, `ListReports` and `DismissReport` as applicant and employer, including with invalid input; a second `RemoveJob` on the same job; two admins removing the same job at the same moment; reporting a removed job; dismissing an already closed report; two admins dismissing the same report at once.
- Expected: non-admins get `PERMISSION_DENIED` before any input check or stored state; a second takedown returns `FAILED_PRECONDITION` (the `removed_jobs` primary key with `ON CONFLICT DO NOTHING`, under the per-job lock); with two concurrent admins exactly one wins; reporting a removed job and dismissing a closed report return `FAILED_PRECONDITION`.
- Tests: `internal/domain/caller_test.go` (`TestCaller_RequireAdmin`); `internal/app/reports_test.go` (`TestReports_List`, `TestReports_Dismiss`, `TestReports_RemoveJob`, including "another admin closed it first" and "second takedown of the same job"); `internal/adapters/grpc/moderation_test.go` (`TestModerationHandler_RoleMatrix`, `TestModerationHandler_RoleBeforeInput`, `TestModerationHandler_RemoveJob`); `internal/adapters/postgres/reports_integration_test.go` (`TestReports_RemoveJob`, `TestReports_RemoveJob_ConcurrentAdmins`, `TestReports_Dismiss_ConcurrentAdmins`, `TestReports_ReportRacingRemoval_NeverLeavesOpenReport`).
- Command: `go test -count=1 ./services/moderation/internal/...` and `go test -count=1 -tags=integration ./services/moderation/internal/adapters/postgres/`
- Status: PASS.

### JT34-A-04: Role and identity checks use pkg/authctx with the Section 9 codes

- Criterion: Role and ownership checks use `pkg/authctx` and return the gRPC codes from AGENTS.md Section 9.
- Inputs: on every RPC: no metadata, missing role, user ID not a UUID, unknown role, upper-case role; each domain error; a database failure carrying driver text.
- Expected: identity problems return `UNAUTHENTICATED` on all four RPCs; wrong role `PERMISSION_DENIED`; bad input `INVALID_ARGUMENT`; missing report `NOT_FOUND`; duplicate OPEN report `ALREADY_EXISTS`; business rule violations `FAILED_PRECONDITION`; canceled and expired contexts `CANCELED` and `DEADLINE_EXCEEDED`; anything else `INTERNAL` with the fixed message `internal error` and no driver detail.
- Tests: `internal/domain/caller_test.go` (`TestNewCaller`, `TestParseRole`); `internal/adapters/grpc/moderation_test.go` (`TestModerationHandler_RejectsBadIdentityOnEveryRPC`, `TestModerationHandler_HidesUnexpectedErrors` and the per-RPC tables); `cmd/server/moderation_setup_test.go` (`TestServe_AnswersModerationRPCsWithForwardedIdentity`, over a real gRPC connection; `TestRegisterModeration_RegistersOnlyThePublicService`).
- Command: `go test -count=1 ./services/moderation/...`
- Status: PASS.

### JT34-A-05: Events are Sprint 2

- Criterion: Publishing `moderation.job_removed` is Sprint 2.
- Expected: no events are published in Sprint 1. The template RabbitMQ publisher connection is opened at startup and kept for Sprint 2; `main.go` notes where publishing will be added. `removed_jobs` already stores the reason and the admin's user ID that the event will carry.
- Status: Not applicable in Sprint 1, by design.

### JT34-S-01: Migrations, repository and database constraints

- Criterion: goose migrations run on startup; maximum lengths for free-text fields are set; repository integration test with Testcontainers.
- Prerequisites: Docker; `docker pull postgres:17-alpine && docker pull postgres:18-alpine && docker pull rabbitmq:4.1-alpine` (17 is used by the template readiness test).
- Expected: migrations apply and can run twice; timestamps are stored and returned in UTC with microsecond precision; one OPEN report per reporter and job; unknown IDs map to `ErrReportNotFound`; closed rows map to `ErrReportNotOpen`; removed jobs map to `ErrJobAlreadyRemoved`; lists return the right order, page and `total_count`, including an offset past the end; the database rejects reasons over each limit, empty reasons and IDs that are not UUIDs.
- Tests: `internal/adapters/postgres/reports_integration_test.go` (`TestReports_CreateAndGet`, `TestReports_Create_ConcurrentDuplicates`, `TestReports_Dismiss`, `TestReports_Dismiss_ConcurrentAdmins`, `TestReports_RemoveJob`, `TestReports_RemoveJob_ConcurrentAdmins`, `TestReports_ReportRacingRemoval_NeverLeavesOpenReport`, `TestReports_List` with 8 subtests, `TestReports_DatabaseLimits` with 8 subtests); `internal/adapters/postgres/reports_test.go` (`TestStatusMapping_RoundTrips`, `TestStatusMapping_RejectsUnknownValues`, `TestUniqueError_MapsOnlyTheOpenReportIndex`); `internal/adapters/postgres/readiness_integration_test.go` (template).
- Command: `go test -count=1 -tags=integration ./services/moderation/internal/adapters/postgres/`
- Status: PASS (all 9 `TestReports_` integration tests, 28.6 s).

### JT34-S-02: Docker Compose startup and health

- Criterion: Service block in `deploy/compose/docker-compose.yml`; `docker compose up` starts it and `GET /healthz` returns 200.
- Prerequisites: `deploy/compose/.env` from `.env.example`.
- Expected: the image builds; moderation waits for healthy Postgres and RabbitMQ, applies migrations and becomes healthy; `/healthz` returns 200; ports 9090 and 8080 are not published to the host; the database password is passed as `PGPASSWORD`, not inside the URL. The `moderation` database and user already existed in `deploy/compose/postgres-init`, and the gateway already had `MODERATION_ADDR`.
- References: `deploy/compose/docker-compose.yml` (moderation block), `services/moderation/Dockerfile`, `cmd/server/main_test.go` (template health and startup order tests).
- Command: the Compose commands in the execution results table.
- Status: PASS on `0bc7f3a`.

### JT34-S-03: Public RPCs through the API gateway (manual)

- Purpose: extra evidence that the four routes work end to end with real JWTs from auth. Not required by the acceptance criteria.
- Setup: `docker compose -f deploy/compose/docker-compose.yml up -d --build auth gateway moderation`; a new synthetic applicant and employer registered and logged in through `POST /v1/auth/register` and `POST /v1/auth/login` on `http://localhost:8000`; the seeded admin from `deploy/compose/.env`; random job IDs.
- Result on `0bc7f3a` (8 Nov 2026, about 10:13 UTC), run with a local script that is not committed:

| # | Request | Caller | Expected | Result |
|---|---|---|---|---|
| 1 | `GET /healthz` (gateway) | none | 200 | PASS |
| 2 | `POST /v1/auth/register`, then login, for applicant, employer and the seeded admin | none | 200, three JWTs | PASS |
| 3 | `POST /v1/jobs/{job_id}/reports` without a token | none | 401 (`UNAUTHENTICATED`) | PASS |
| 4 | `POST /v1/jobs/{job_id}/reports` with spaces around the reason | applicant | 200, OPEN, trimmed reason, same `jobId` | PASS |
| 5 | `POST /v1/jobs/{job_id}/reports` again | applicant | 409 (`ALREADY_EXISTS`) | PASS |
| 6 | `POST /v1/jobs/{job_id}/reports` | employer | 200, OPEN | PASS |
| 7 | `POST /v1/jobs/{other_job}/reports` | admin | 200, OPEN | PASS |
| 8 | `POST /v1/jobs/{job_id}/reports` with a spaces-only reason | employer | 400 (`INVALID_ARGUMENT`) | PASS |
| 9 | `POST /v1/jobs/not-a-uuid/reports` | applicant | 400 (`INVALID_ARGUMENT`) | PASS |
| 10 | `GET /v1/moderation/reports` | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 11 | `GET /v1/moderation/reports` | employer | 403 (`PERMISSION_DENIED`) | PASS |
| 12 | `POST /v1/moderation/reports/{report_id}/dismiss` | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 13 | `POST /v1/moderation/jobs/{job_id}/remove` | employer | 403 (`PERMISSION_DENIED`) | PASS |
| 14 | `GET /v1/moderation/reports?status=REPORT_STATUS_OPEN` | admin | 200, includes both new reports on the job | PASS |
| 15 | `POST /v1/moderation/reports/{employer_report}/dismiss` | admin | 200, DISMISSED | PASS |
| 16 | the same dismissal again | admin | 400 (`FAILED_PRECONDITION`) | PASS |
| 17 | `POST /v1/moderation/reports/{unknown UUID}/dismiss` | admin | 404 (`NOT_FOUND`) | PASS |
| 18 | `POST /v1/moderation/jobs/{job_id}/remove` with an empty reason | admin | 400 (`INVALID_ARGUMENT`) | PASS |
| 19 | `POST /v1/moderation/jobs/{job_id}/remove` with a reason | admin | 200, same `jobId` | PASS |
| 20 | the same takedown again | admin | 400 (`FAILED_PRECONDITION`) | PASS |
| 21 | `GET /v1/moderation/reports?status=REPORT_STATUS_ACTIONED` | admin | 200, the applicant's report is ACTIONED; the dismissed report is not listed | PASS |
| 22 | `POST /v1/jobs/{job_id}/reports` on the removed job | employer | 400 (`FAILED_PRECONDITION`) | PASS |
| 23 | `POST /v1/moderation/jobs/{other_job}/remove` | admin | 200 | PASS |

The script printed 28 PASS lines for these 23 requests (some requests check several fields). The gateway maps `FAILED_PRECONDITION` and `INVALID_ARGUMENT` both to HTTP 400; the script checked HTTP codes only, and the unit and handler tests tell the two gRPC codes apart.

## Not covered or pending

- Publishing `moderation.job_removed`: Sprint 2, not part of JT-34. Until then job-posting does not close a removed job.
- Whether a reported job exists or is open in job-posting: not checked, because Section 6 allows no such call (decision 3). Recorded as an open question in AGENTS.md Section 16.
- Real client adapters and events: Sprint 2. Moderation calls no other services, so there are no client ports in Sprint 1.
- Internal-token protection: not relevant, because the contract has no `*InternalService`.
- Race detector on Jigden's laptop: not run (cgo disabled). The concurrency tests above run without it.
- Whole repository after merging `main`, and GitHub Actions CI on the final commit: checked before merging the PR.
