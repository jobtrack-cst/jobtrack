# JT-36 test cases and verification evidence

## Scope and evidence rules

Jira: JT-36, Implement analytics service basic operations (Sprint 1, Should). Branch: `JT-36-analytics`. Follow [AGENTS.md](../../AGENTS.md), especially Sections 4, 6, 9, 10 and 13. The case identifiers below are local to JT-36.

All results were run by Jigden Shakya on his laptop (WSL Ubuntu, Docker Desktop, Go 1.27.3) on 8 Nov 2026. Times are UTC. Each result names the commit whose service code it ran against; the service code has not changed since `ea47ce8` (`8bddc3d` only adds the Compose block and the `.env.example` line, and only this document and AGENTS.md follow). Commands below are also reproduction instructions; run them from the repository root.

The service was created with `bash scripts/new-service.sh analytics` from the empty `.gitkeep` placeholder (commit `9e96775`). The public contract `proto/jobtrack/analytics/v1/analytics.proto` and the generated code are unchanged. Both RPCs in the contract are implemented: the public `AnalyticsService.GetSummary` and `AnalyticsInternalService.GetInternalSummary`, whose only allowed caller is `weekly-summary` (AGENTS.md Section 6). Analytics calls no other service.

Test design follows AGENTS.md Section 10: equivalence partitioning (each caller role, the internal caller and wrong callers, valid and invalid input classes, each of the eight counted event types), boundary values (the `new_status` length limit on and either side of the limit, in characters, including multibyte characters; the time range edges at the second, microsecond and nanosecond; the earliest and latest valid timestamps) and state transitions (an event ID not yet processed, already processed, two deliveries at once, and a refused insert).

The repository integration test follows the JT-20 pattern used by the template, moderation and audit-log: Testcontainers with `postgres:18-alpine`, build tag `integration`, migrations applied twice.

## Decisions approved for JT-36

The contract and AGENTS.md left these open. Jigden Shakya (task owner) approved them on 8 Nov 2026; they are also recorded in the AGENTS.md decision log.

1. Storage: one row per counted event in `outcome_events`: `event_id` (UUID primary key, from the envelope), `event_type`, `new_status` (only for `application.status_changed`) and `occurred_at`, plus an internal `recorded_at` that is never returned. The range is any pair of exact timestamps, so daily counters could not answer it correctly. No job, employer or applicant IDs are stored, because the summary does not return them. A CHECK allows only the eight consumed event types, and another makes `new_status` set if and only if the type is `application.status_changed`. The index is `(occurred_at) INCLUDE (event_type, new_status)`, because the summary filters only by time; this replaced the first proposal `(event_type, occurred_at)`.
2. Processed event IDs: a separate `processed_events (event_id, processed_at)` table, as in audit-log. `RecordEvent` inserts the event ID with `ON CONFLICT DO NOTHING` and then the outcome event, in one transaction. A duplicate returns "not recorded" with no error, so the Sprint 2 consumers can acknowledge it; if the event insert fails, the claim rolls back too, so a redelivery can still record the event.
3. Time range (both RPCs, one shared path): `start_time` and `end_time` are both required; a missing or invalid timestamp is `INVALID_ARGUMENT`, and so is `start_time` equal to or after `end_time` (the contract says start must be before end). Events count when `start_time <= occurred_at < end_time`. There is no maximum range. PostgreSQL stores microseconds, so both bounds are rounded up to a whole microsecond before the query; this keeps the comparison exact for stored events when a request time has extra nanoseconds.
4. Response: the summary echoes the request times exactly; types with no events count 0; `status_changes` lists only statuses with a count above 0, sorted by status name.
5. Field limits: neither RPC has a free-text request field. The only stored text is `new_status`: trimmed, 1 to 32 characters, valid UTF-8, NUL refused. It is stored as sent in the event and not checked against the lifecycle list. The column is TEXT with a length CHECK, not VARCHAR(32), because VARCHAR would silently cut trailing spaces instead of refusing the value.
6. Roles: `GetSummary` is admin only; applicant and employer get `PERMISSION_DENIED`, checked before any input, including the timestamps, which the gRPC handler converts. `GetInternalSummary` has no user role check; it is protected by `authctx.InternalUnaryServerInterceptor` (token and the `weekly-summary` allowlist). Both services are registered on the analytics gRPC server; the gateway never registers the internal one.
7. `INTERNAL_TOKEN`: analytics is the first service in Docker Compose that serves an `*InternalService`. Only the analytics block gets `INTERNAL_TOKEN: ${INTERNAL_TOKEN:?set in .env}`, not the shared `*service-env` anchor, so no other service gets the token before its owner wires JT-44; the gateway never gets it. `.env.example` has a commented `INTERNAL_TOKEN=change-me` placeholder. Startup refuses a missing or invalid token, and the token is never logged.
8. As in the template, moderation and audit-log, the RabbitMQ connection is opened at startup and `RABBITMQ_URL` is required; analytics never publishes, and the connection is kept for the Sprint 2 consumers.

A forwarded identity whose user ID is not a UUID or whose role is unknown is treated as missing (`UNAUTHENTICATED`), because the gateway never sets such values.

## Execution results

| Check | Command | Result (8 Nov 2026) |
|---|---|---|
| Formatting | `gofmt -l services/analytics` | PASS, no output (13:46, `ea47ce8` service code) |
| Static checks | `go vet ./services/analytics/...` and `go vet -tags=integration ./services/analytics/...` | PASS, no output |
| Unit tests | `go test -count=1 -cover ./services/analytics/...` | PASS, all 6 packages with tests `ok` (14:00, `8bddc3d`) |
| Integration tests | `go test -count=1 -tags=integration ./services/analytics/...` | PASS, all 6 packages `ok` (postgres 29.1 s, rabbitmq 23.7 s, real containers; 14:00, `8bddc3d`) |
| Compose build and start | `docker compose -f deploy/compose/docker-compose.yml up -d --build analytics` | PASS, image built, analytics started after Postgres and RabbitMQ were healthy (13:53, service code of `ea47ce8`) |
| Compose health | `docker compose ... ps analytics` | PASS, `Up 26 seconds (healthy)`; ports 8080 and 9090 not published (`8080/tcp, 9090/tcp`) |
| Health endpoint | `docker compose ... exec analytics wget -qO- http://localhost:8080/healthz` | PASS, body `ok` (wget fails on any non-2xx status; the container healthcheck runs the same request) |
| Startup logs and migrations | `docker compose ... logs analytics` and `docker compose ... exec postgres psql -U postgres -d analytics -c '\dt'` | PASS, `servers starting` with `"service":"analytics"`, no errors, no token in the logs; tables `outcome_events`, `processed_events` and `goose_db_version`, owned by `analytics` |
| Through the gateway | Manual smoke checks, see JT36-S-04 | PASS, 23 of 23 (about 13:56, `8bddc3d`) |

## Unit test coverage (`8bddc3d`)

Statement coverage from `go test -count=1 -cover ./services/analytics/...`. AGENTS.md sets an 80 percent target only for `application` and `application-status`; for analytics the rule is meaningful tests for every business rule.

| Package | Coverage |
|---|---:|
| `services/analytics/internal/domain` | 98.7% |
| `services/analytics/internal/app` | 100.0% |
| `services/analytics/internal/adapters/grpc` | 100.0% |
| `services/analytics/cmd/server` | 84.7% |
| `services/analytics/internal/adapters/rabbitmq` | 83.3% (template code, unchanged) |
| `services/analytics/internal/adapters/postgres` | 35.7% in the unit run; the repository is covered by the integration tests |

## Boundary values

| Input | Below or invalid | On the boundary (valid) | Above (invalid) | Test |
|---|---|---|---|---|
| `new_status` (status changes) | empty; spaces only; NUL; invalid UTF-8; empty string inserted directly (database) | 1 character; 32 characters; 32 characters after trimming; 32 two-byte characters; trimmed `UNDER_REVIEW` | 33 characters; 33 two-byte characters | `TestNewOutcomeEvent`, `TestOutcomes_DatabaseLimits` |
| `new_status` on other event types | any value, including a single space | empty (stored as NULL) | | `TestNewOutcomeEvent`, `TestOutcomes_DatabaseLimits` |
| `event_type` | empty; `user.registered`; `job.updated`; upper case; leading or trailing space | each of the eight counted types | | `TestNewOutcomeEvent`, `TestIsCountedEventType`, `TestOutcomes_DatabaseLimits` |
| `event_id` | empty; `123`; `event-1` (database) | lower- and upper-case UUID (upper case is lowercased) | | `TestNormalizeID`, `TestNewOutcomeEvent`, `TestOutcomes_DatabaseLimits` |
| `occurred_at` | zero time | any set time, stored in UTC with microseconds | | `TestNewOutcomeEvent`, `TestOutcomes_RecordEvent` |
| `start_time`, `end_time` | either or both missing; start equal to end; start after end; negative nanos; nanos of 1e9; before year 1; after year 9999 | start 1 ns before end; the Unix epoch (a set time, not "unset"); 1-microsecond window; year 1 to the last nanosecond of 9999 | | `TestNewTimeRange`, `TestAnalyticsHandlers_RejectInvalidTimes`, `TestAnalyticsHandler_UnixEpochIsASetTime`, `TestOutcomes_CountEvents` |
| Range edges in the database | start 1 ns after an event (event left out) | event exactly at start (counted); end 1 ns after an event (counted) | event exactly at end (left out) | `TestOutcomes_CountEvents`, `TestTimeRange_QueryBounds` |
| Forwarded `x-user-id` and `x-user-role` | missing either; user ID not a UUID; role `superuser`, `analyst`, `Admin` or empty | UUID with `applicant`, `employer` or `admin` | | `TestNewCaller`, `TestAnalyticsHandler_GetSummary_RejectsBadIdentity` |
| `INTERNAL_TOKEN` (startup) | empty; contains a space, a tab, a newline or a non-ASCII character | any nonempty visible ASCII without whitespace | | `TestLoadConfig`, `TestRun_RejectsInvalidInternalTokenBeforeStartup`, `TestInternalServerOptions_RejectsInvalidTokens` |

## Authorization matrix

Every cell is covered by table-driven unit tests in `internal/domain/caller_test.go`, `internal/app/summary_test.go` and `internal/adapters/grpc/analytics_test.go` (`TestAnalyticsHandler_RoleMatrix`, `TestAnalyticsHandler_RoleBeforeInput` and `TestAnalyticsInternalHandler_IgnoresUserIdentity`), over a real gRPC server with the real JT-44 interceptor in `cmd/server/register_test.go` (`TestServe_InternalInterceptorGuardsOnlyTheInternalService`), and through the gateway in JT36-S-04.

| RPC | Applicant | Employer | Admin | No identity |
|---|---|---|---|---|
| `GetSummary` | `PERMISSION_DENIED` | `PERMISSION_DENIED` | OK | `UNAUTHENTICATED` |

| RPC | `weekly-summary` with the right token | Wrong token | No token | Another caller (`notification-dispatcher`) | Through the gateway |
|---|---|---|---|---|---|
| `GetInternalSummary` | OK (any forwarded user identity is ignored) | `UNAUTHENTICATED` | `UNAUTHENTICATED` | `PERMISSION_DENIED` | not routed (HTTP 404) |

## State transitions

For the processed event ID of an incoming event (used by the Sprint 2 consumers through `RecordEvent`).

| From | Event | To or result | Test |
|---|---|---|---|
| event ID not processed | event recorded | event ID in `processed_events`, outcome event stored, `recorded` true | `TestOutcomes_RecordEvent` |
| event ID processed | same event ID again, even with another status | `recorded` false, no error; the original row is unchanged | `TestOutcomes_RecordEvent` |
| event ID not processed | two deliveries at once | exactly one recorded, neither is an error, one row stored | `TestOutcomes_RecordEvent_ConcurrentDuplicates` (10 rounds) |
| event ID not processed | event refused by a database rule | error; the event ID is not marked processed | `TestOutcomes_DatabaseLimits` |
| refused earlier | redelivery with a valid event | recorded | `TestOutcomes_DatabaseLimits` ("a refused event can be recorded on redelivery") |

## Acceptance test cases

### JT36-A-01: Database schema for outcome statistics, plus a processed event_id table

- Criterion: Database schema for outcome statistics, plus a processed event_id table for idempotent consumers.
- Inputs: migration `00002_create_outcome_events.sql` applied twice; a new `job.published` and a new status change; the same event ID again with another status; two deliveries of one event at once; events that break each database rule; a refused event followed by a valid redelivery.
- Expected: `outcome_events` and `processed_events` exist and migration 2 is recorded once; new events are stored in UTC, with NULL status for types other than status changes; a duplicate is skipped without error and the original kept; concurrent duplicates store exactly one row; the database refuses an unknown or empty type, a status on the wrong type, a missing or empty status on a status change and a status over 32 characters; a refused event leaves nothing marked processed.
- Tests: `internal/domain/outcome_test.go` (`TestNewOutcomeEvent`, `TestIsCountedEventType`); `internal/adapters/postgres/outcomes_test.go` (`TestNullableStatus`); `internal/adapters/postgres/outcomes_integration_test.go` (`TestOutcomes_MigrationsRecorded`, `TestOutcomes_RecordEvent`, `TestOutcomes_RecordEvent_ConcurrentDuplicates`, `TestOutcomes_DatabaseLimits`).
- Command: `go test -count=1 ./services/analytics/internal/domain/` and `go test -count=1 -tags=integration ./services/analytics/internal/adapters/postgres/`
- Status: PASS.

### JT36-A-02: Admin-only summary RPC

- Criterion: Admin-only summary RPC.
- Inputs: a range over seeded events of every counted type and two statuses; a range with no events; the start exactly on an event and the end exactly on an event; a start 1 ns after an event; a 1-microsecond window; the widest valid range; every invalid time case as admin; nil requests.
- Expected: one counter per event type, 0 for types with no events; `status_changes` only above 0, sorted by status; start inclusive and end exclusive, also at the nanosecond edge; the request times echoed exactly; each invalid time `INVALID_ARGUMENT` with no repository call.
- Tests: `internal/domain/timerange_test.go` (`TestNewTimeRange`, `TestTimeRange_QueryBounds`); `internal/domain/summary_test.go` (`TestBuildSummary`); `internal/app/summary_test.go` (`TestAnalytics_RejectsInvalidInput`, `TestAnalytics_BothSummariesCountTheSameRange`, `TestAnalytics_EmptyRangeGivesZeros`); `internal/adapters/grpc/analytics_test.go` (`TestAnalyticsHandlers_RejectInvalidTimes`, `TestAnalyticsHandlers_MapAllFields`, `TestAnalyticsHandlers_EmptyRange`, `TestAnalyticsHandler_UnixEpochIsASetTime`); `internal/adapters/postgres/outcomes_integration_test.go` (`TestOutcomes_CountEvents`, 8 subtests).
- Command: `go test -count=1 ./services/analytics/internal/...` and `go test -count=1 -tags=integration ./services/analytics/internal/adapters/postgres/`
- Status: PASS.

### JT36-A-03: AnalyticsInternalService.GetInternalSummary

- Criterion: `AnalyticsInternalService.GetInternalSummary`, called only by weekly-summary (AGENTS.md Section 6), protected by the JT-44 internal server interceptor with `INTERNAL_TOKEN`; the gateway never registers it.
- Inputs: `weekly-summary` with the right token; a wrong token; no token; `notification-dispatcher` with the right token; the public `GetSummary` on the same server as admin and employer; no forwarded identity, an applicant identity and a malformed identity on the internal handler; invalid times; missing or invalid `INTERNAL_TOKEN` at startup.
- Expected: only `weekly-summary` with the right token gets the summary, with the same counts as `GetSummary`; a wrong or missing token `UNAUTHENTICATED`; another caller `PERMISSION_DENIED`; the public RPC is unchanged by the interceptor; user identity is ignored on the internal RPC; invalid times `INVALID_ARGUMENT`; startup stops on a bad token before opening the database, and no error contains the token; both services are registered on the analytics server, and the gateway has no route to the internal one.
- Tests: `internal/app/summary_test.go` (`TestAnalytics_BothSummariesCountTheSameRange`); `internal/adapters/grpc/analytics_test.go` (`TestAnalyticsInternalHandler_IgnoresUserIdentity`, `TestAnalyticsHandlers_RejectInvalidTimes`, `TestAnalyticsHandlers_MapAllFields`); `cmd/server/main_test.go` (`TestLoadConfig`, `TestRun_RejectsInvalidInternalTokenBeforeStartup`); `cmd/server/register_test.go` (`TestRegisterAnalytics_RegistersPublicAndInternalServices`, `TestInternalServerOptions_RejectsInvalidTokens`, `TestServe_InternalInterceptorGuardsOnlyTheInternalService`, 6 subtests); gateway checks in JT36-S-04 (#11 and #12).
- Command: `go test -count=1 ./services/analytics/...`
- Status: PASS.

### JT36-A-04: Event consumers are Sprint 2

- Criterion: Event consumers are Sprint 2.
- Expected: no events are consumed or published in Sprint 1. The schema, the processed event ID table, `domain.NewOutcomeEvent` and `RecordEvent` in the repository are ready for the consumers; the RabbitMQ connection is opened at startup and kept for them.
- Status: Not applicable in Sprint 1, by design.

### JT36-A-05: Role and identity checks use pkg/authctx with the Section 9 codes

- Criterion: Role and ownership checks use `pkg/authctx` and return the gRPC codes from AGENTS.md Section 9.
- Inputs: no metadata, missing user ID, missing role, user ID not a UUID, unknown role, upper-case role; applicant and employer callers with a valid range, with every invalid time case and with a nil request; each domain error; a database failure carrying driver text; canceled and expired contexts, on both RPCs.
- Expected: identity problems return `UNAUTHENTICATED`; non-admins get `PERMISSION_DENIED` before any input check or repository call; bad input `INVALID_ARGUMENT`; canceled and expired contexts `CANCELED` and `DEADLINE_EXCEEDED`; anything else `INTERNAL` with the fixed message `internal error` and no driver detail.
- Tests: `internal/domain/caller_test.go` (`TestParseRole`, `TestNormalizeID`, `TestNewCaller`, `TestCaller_RequireAdmin`); `internal/app/summary_test.go` (`TestAnalytics_GetSummary_RoleIsCheckedBeforeInput`, `TestAnalytics_WrapsRepositoryError`); `internal/adapters/grpc/analytics_test.go` (`TestAnalyticsHandler_GetSummary_RejectsBadIdentity`, `TestAnalyticsHandler_RoleMatrix`, `TestAnalyticsHandler_RoleBeforeInput`, `TestAnalyticsHandlers_HideUnexpectedErrors`); `internal/adapters/postgres/outcomes_test.go` (`TestSafe_HidesDetailsButKeepsCancellation`).
- Command: `go test -count=1 ./services/analytics/...`
- Status: PASS.

### JT36-S-01: Migrations, repository and database constraints

- Criterion: goose migrations run on startup; maximum lengths for free-text fields are set; repository integration test with Testcontainers.
- Prerequisites: Docker; `docker pull postgres:17-alpine && docker pull postgres:18-alpine && docker pull rabbitmq:4.1-alpine` (17 is used by the template readiness test).
- Expected: migrations apply and can run twice; startup runs them before the servers start; the cases in JT36-A-01 and JT36-A-02 hold against a real PostgreSQL.
- Tests: `internal/adapters/postgres/outcomes_integration_test.go` (the 5 `TestOutcomes_` tests); `internal/adapters/postgres/readiness_integration_test.go` (template); `cmd/server/main_test.go` (`TestRunWithDatabase_OrdersStartupAndCleansUp`, which also checks the outcome repository is wired).
- Command: `go test -count=1 -tags=integration ./services/analytics/internal/adapters/postgres/`
- Status: PASS (the 5 `TestOutcomes_` tests and both readiness tests, 18.4 s on `7e509e5`; the full package run in the execution results took 29.1 s).

### JT36-S-02: Docker Compose startup and health

- Criterion: Service block in `deploy/compose/docker-compose.yml`; `docker compose up` starts it and `GET /healthz` returns 200.
- Prerequisites: `deploy/compose/.env` from `.env.example`, with `INTERNAL_TOKEN` set (for example `openssl rand -hex 32`).
- Expected: the image builds; analytics waits for healthy Postgres and RabbitMQ, applies migrations and becomes healthy; `/healthz` returns 200; ports 9090 and 8080 are not published to the host; the database password is passed as `PGPASSWORD`, not inside the URL; `INTERNAL_TOKEN` is set only in the analytics block. The `analytics` database and user already existed in `deploy/compose/postgres-init`, and the gateway already had `ANALYTICS_ADDR`.
- References: `deploy/compose/docker-compose.yml` (analytics block), `deploy/compose/.env.example`, `services/analytics/Dockerfile`, `cmd/server/main_test.go` (template health and startup order tests).
- Command: the Compose commands in the execution results table.
- Status: PASS on `8bddc3d`.

### JT36-S-03: Startup with the internal token

- Criterion: AGENTS.md Section 6, internal-call wiring: load `INTERNAL_TOKEN` only when the service serves or calls internal RPCs, refuse invalid configuration, and never log the token.
- Expected: `loadConfig` requires a nonempty visible ASCII token; `run` builds the interceptor before opening the database; errors name `INTERNAL_TOKEN` but never contain its value; the startup log has no token.
- Tests: `cmd/server/main_test.go` (`TestLoadConfig`, 4 token cases; `TestRun_RejectsInvalidInternalTokenBeforeStartup`); `cmd/server/register_test.go` (`TestInternalServerOptions_RejectsInvalidTokens`); Compose logs in the execution results.
- Status: PASS.

### JT36-S-04: Public RPC through the API gateway (manual)

- Purpose: extra evidence that the route works end to end with real JWTs from auth, and that the gateway does not expose the internal RPC. Not required by the acceptance criteria.
- Setup: `docker compose -f deploy/compose/docker-compose.yml up -d --build auth gateway analytics`; a new synthetic applicant registered and logged in through `POST /v1/auth/register` and `POST /v1/auth/login` on `http://localhost:8000`; the seeded admin from `deploy/compose/.env`. Because the consumers are Sprint 2, six synthetic events in January 2030 were inserted with `psql` and deleted at the end: `job.published` at 00:00 and 05:00 on 1 Jan, `offer.made` at 06:00, status changes `SHORTLISTED` at 07:00 and `OFFERED` at 08:00, and `SHORTLISTED` at 00:00 on 2 Jan.
- Result on `8bddc3d` (8 Nov 2026, about 13:56 UTC), run with a local script that is not committed:

| # | Request | Caller | Expected | Result |
|---|---|---|---|---|
| 1 | `GET /v1/analytics/summary` without a token | none | 401 (`UNAUTHENTICATED`) | PASS |
| 2 | `GET /v1/analytics/summary` for 1 Jan 2030 | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 3 | start after end | applicant | 403, role before input | PASS |
| 4 | `?start_time=2030-01-01T00:00:00Z&end_time=2030-01-02T00:00:00Z` | admin | 200; `jobsPublished` 2 (start inclusive), `offersMade` 1, `applicationsSubmitted` 0, `statusChanges` `OFFERED=1,SHORTLISTED=1` (sorted, end exclusive), `startTime` echoed | PASS (6 checks) |
| 5 | start 1 ns after the 05:00 event, end 06:00 | admin | 200, `jobsPublished` 0 | PASS |
| 6 | a day in 2031 with no events | admin | 200, no status changes | PASS (2 checks) |
| 7 | `end_time` missing | admin | 400 (`INVALID_ARGUMENT`) | PASS |
| 8 | no times | admin | 400 | PASS |
| 9 | `start_time` equal to `end_time` | admin | 400 | PASS |
| 10 | `start_time` after `end_time` | admin | 400 | PASS |
| 11 | `POST /jobtrack.analytics.v1.AnalyticsInternalService/GetInternalSummary` | admin | 404, the gateway has no route | PASS |
| 12 | `GET /v1/analytics/internal-summary` | admin | 404 | PASS |

The script printed 23 PASS lines: 4 for the setup (admin login, applicant registration and login, six events seeded), 18 for these 12 requests (some requests check several fields) and 1 confirming the synthetic events were removed.

## Not covered or pending

- The event consumers for the eight event types, their durable queue and their handling of events that fail validation: Sprint 2, not part of JT-36.
- A call from the real `weekly-summary` function: Sprint 3. The internal RPC was tested over a real gRPC server with the shared interceptors, not from the function.
- NetworkPolicy for port 9090 (AGENTS.md Section 6, layer 3): pending in `deploy/k8s`, not part of JT-36. In Docker Compose the port is not published.
- Retention or clean-up of old outcome events: not designed; not in the acceptance criteria.
- Real client adapters: analytics calls no other services, so there are no client ports in Sprint 1.
- Race detector on Jigden's laptop: not run (cgo disabled). The concurrency test above runs without it.
- Whole repository after merging `main`, and GitHub Actions CI on the final commit: checked before merging the PR.
