# JT-35 test cases and verification evidence

## Scope and evidence rules

Jira: JT-35, Implement audit-log service basic operations (Sprint 1, Should). Branch: `JT-35-audit-log`. Follow [AGENTS.md](../../AGENTS.md), especially Sections 4, 6, 9, 10 and 13. The case identifiers below are local to JT-35.

All results were run by Jigden Shakya on his laptop (WSL Ubuntu, Docker Desktop) on 8 Nov 2026. Times are UTC. Each result names the commit whose service code it ran against; the service code has not changed since `e7ca419` (`0f9c077` only adds the Compose block, and only this document and AGENTS.md follow). Commands below are also reproduction instructions; run them from the repository root.

The service was created with `bash scripts/new-service.sh audit-log` from the empty `.gitkeep` placeholder (commit `469754a`). The public contract `proto/jobtrack/audit_log/v1/audit_log.proto` and the generated code are unchanged. The only public RPC in the contract, `ListAuditEntries`, is implemented. The contract has no `*InternalService`, and audit-log calls no other service.

Test design follows AGENTS.md Section 10: equivalence partitioning (each caller role, valid and invalid input classes, each filter alone and combined), boundary values (each length limit tested on and either side of the limit; the `data_json` limit in bytes, including multibyte characters; pagination sizes and tokens; the time window edges) and state transitions (an event ID not yet processed, already processed, two deliveries at once, and a refused insert).

The repository integration test follows the JT-20 pattern used by the template and moderation: Testcontainers with `postgres:18-alpine`, build tag `integration`, migrations applied twice.

## Decisions approved for JT-35

The contract and AGENTS.md left these open. Jigden Shakya (task owner) approved them on 8 Nov 2026; they are also recorded in the AGENTS.md decision log.

1. Actor: `EventEnvelope` has no `actor_id` yet (AGENTS.md Section 16) and `AuditEntry` has no actor field, so there is no actor column. When the envelope gains `actor_id`, a new migration adds a nullable column.
2. Stored fields: `event_id` (UUID primary key, from the envelope), `event_type`, `producer`, `occurred_at` and `data_json`, plus an internal `recorded_at` that is never returned. `data_json` is TEXT, not JSONB, so the admin view shows the data exactly as received (JSONB would reorder keys, drop spaces and refuse `\u0000`).
3. Field limits: `event_type` 1 to 100 characters, a lowercase dot-separated routing key (for example `job_alert.matched`); `producer` 1 to 50 characters, kebab-case like service names; `data_json` a valid UTF-8 JSON object of at most 65536 bytes, counted in bytes because it can hold any text. Values are not trimmed: a routing key with spaces is wrong, not untidy.
4. The `event_type` filter uses the same format rule (`INVALID_ARGUMENT` otherwise). A well-formed type that no entry has returns an empty list, so adding an event never needs an audit-log change.
5. `ListAuditEntries` is admin only; other roles get `PERMISSION_DENIED`. The role is checked before any input, including the timestamps, which the gRPC handler converts. Entries are newest first by `occurred_at`, with `event_id` breaking ties so paging is stable. `start_time` is inclusive and `end_time` exclusive; when both are set, `start_time` must be strictly before `end_time`. A timestamp that fails protobuf validation is `INVALID_ARGUMENT`.
6. Processed event IDs: a separate `processed_events (event_id, processed_at)` table. `RecordEntry` inserts the event ID with `ON CONFLICT DO NOTHING` and then the entry, in one transaction. A duplicate returns "not recorded" with no error, so the Sprint 2 consumer can acknowledge it; if the entry insert fails, the claim rolls back too, so a redelivery can still record the event.
7. As in the template and moderation, the RabbitMQ connection is opened at startup and `RABBITMQ_URL` is required; audit-log never publishes, and the connection is kept for the Sprint 2 consumer. No `INTERNAL_TOKEN` is loaded.

A forwarded identity whose user ID is not a UUID or whose role is unknown is treated as missing (`UNAUTHENTICATED`), because the gateway never sets such values.

## Execution results

| Check | Command | Result (8 Nov 2026) |
|---|---|---|
| Formatting | `gofmt -l services/audit-log` | PASS, no output (12:20, `0f9c077`) |
| Static checks | `go vet ./services/audit-log/...` and `go vet -tags=integration ./services/audit-log/...` | PASS, no output |
| Unit tests | `go test -count=1 -cover ./services/audit-log/...` | PASS, all 6 packages with tests `ok` |
| Integration tests | `go test -count=1 -p 2 -tags=integration ./services/audit-log/...` | PASS, all 6 packages `ok` (postgres 21.6 s, rabbitmq 17.6 s, real containers) |
| Compose build and start | `docker compose -f deploy/compose/docker-compose.yml up -d --build audit-log` | PASS, image built, audit-log started after Postgres and RabbitMQ were healthy (12:11, service code of `e7ca419`) |
| Compose health | `docker compose ... ps audit-log` | PASS, `Up 9 minutes (healthy)`; ports 8080 and 9090 not published (`8080/tcp, 9090/tcp`) |
| Health endpoint | `docker compose ... exec audit-log wget -S -qO- http://localhost:8080/healthz` | PASS, `HTTP/1.1 200 OK`, body `ok` (12:21, `0f9c077`) |
| Startup logs and migrations | `docker compose ... logs audit-log` and `docker compose ... exec postgres psql -U postgres -d audit_log -c '\dt'` | PASS, `servers starting` with `"service":"audit-log"`, no errors; tables `audit_entries`, `processed_events` and `goose_db_version`, owned by `audit_log` |
| Through the gateway | Manual smoke checks, see JT35-S-03 | PASS, 18 of 18 (about 12:16, `0f9c077`) |

## Unit test coverage (`0f9c077`)

Statement coverage from `go test -count=1 -cover ./services/audit-log/...`. AGENTS.md sets an 80 percent target only for `application` and `application-status`; for audit-log the rule is meaningful tests for every business rule.

| Package | Coverage |
|---|---:|
| `services/audit-log/internal/domain` | 98.6% |
| `services/audit-log/internal/app` | 100.0% |
| `services/audit-log/internal/adapters/grpc` | 100.0% |
| `services/audit-log/cmd/server` | 82.5% |
| `services/audit-log/internal/adapters/rabbitmq` | 83.3% (template code, unchanged) |
| `services/audit-log/internal/adapters/postgres` | 41.3% in the unit run; the repository is covered by the integration tests |

## Boundary values

| Input | Below or invalid | On the boundary (valid) | Above (invalid) | Test |
|---|---|---|---|---|
| `event_type` (entry and list filter) | empty; one part (`job`); upper case; leading or trailing space; empty part (`job..published`); trailing dot; hyphen | `job.published`; `job_alert.matched`; three parts; 100 characters | 101 characters | `TestNewEntry`, `TestNewListFilter`, `TestAuditLogHandler_ListAuditEntries`, `TestEntries_DatabaseLimits` |
| `producer` | empty; upper case; underscore; trailing hyphen | `auth`; `job-posting`; 50 characters | 51 characters | `TestNewEntry`, `TestEntries_DatabaseLimits` |
| `data_json` | empty; spaces only; array; `null`; string; broken JSON; two objects; invalid UTF-8; one byte (database) | `{}`; object with surrounding spaces; escaped `\u0000`; 65536 bytes; 65536 bytes of 2-byte characters | 65537 bytes; 65538 bytes of 2-byte characters | `TestNewEntry`, `TestEntries_DatabaseLimits` |
| `event_id` | empty; `123`; `event-1` (database) | lower- and upper-case UUID (upper case is lowercased) | | `TestNormalizeID`, `TestNewEntry`, `TestEntries_DatabaseLimits` |
| `occurred_at` | zero time | any set time, stored in UTC with microseconds | | `TestNewEntry`, `TestEntries_RecordEntry` |
| `start_time`, `end_time` | start equal to end; start after end; negative nanos; after year 9999 | start only; end only; the Unix epoch (a set filter, not "unset"); start 1 ns or 1 microsecond before end | | `TestNewListFilter`, `TestAuditLogHandler_ListAuditEntries`, `TestEntries_ListEntries` |
| `page_size` | -1 | 0 (default 20); 1; 2; 100 | 101 and 500 (capped at 100) | `TestNewPage`, `TestAuditLog_ListEntries_PassesFilterAndPage`, `TestAuditLogHandler_ListAuditEntries` |
| `page_token` | `abc`; `-1`; `+5`; `007`; ` 5` | empty; `0`; `2`; `20`; `100` | | `TestNewPage`, `TestAuditLog_ListEntries_RejectsInvalidInput` |
| Forwarded `x-user-id` and `x-user-role` | missing either; user ID not a UUID; role `superuser`, `auditor`, `Admin` or empty | UUID with `applicant`, `employer` or `admin` | | `TestNewCaller`, `TestAuditLogHandler_RejectsBadIdentity` |

## Authorization matrix

Every cell is covered by table-driven unit tests in `internal/domain/caller_test.go`, `internal/app/entries_test.go` and `internal/adapters/grpc/audit_log_test.go` (`TestAuditLogHandler_RoleMatrix` and `TestAuditLogHandler_RoleBeforeInput`), and through the gateway in JT35-S-03.

| RPC | Applicant | Employer | Admin | No identity |
|---|---|---|---|---|
| `ListAuditEntries` | `PERMISSION_DENIED` | `PERMISSION_DENIED` | OK, all entries | `UNAUTHENTICATED` |

## State transitions

For the processed event ID of an incoming event (used by the Sprint 2 consumer through `RecordEntry`).

| From | Event | To or result | Test |
|---|---|---|---|
| event ID not processed | entry recorded | event ID in `processed_events`, entry stored, `recorded` true | `TestEntries_RecordEntry` |
| event ID processed | same event ID again, even with other data | `recorded` false, no error; the original entry is unchanged | `TestEntries_RecordEntry` |
| event ID not processed | two deliveries at once | exactly one recorded, neither is an error, one entry stored | `TestEntries_RecordEntry_ConcurrentDuplicates` (10 rounds) |
| event ID not processed | entry refused by a database limit | error; the event ID is not marked processed | `TestEntries_DatabaseLimits` |
| refused earlier | redelivery with a valid entry | recorded | `TestEntries_DatabaseLimits` ("a refused event can be recorded on redelivery") |

## Acceptance test cases

### JT35-A-01: Database schema for audit entries, plus a processed event_id table

- Criterion: Database schema for audit entries, plus a processed event_id table for idempotent consumers.
- Inputs: migration `00002_create_audit_entries.sql` applied twice; a new entry; the same event ID again with other data; two deliveries of one event at once; entries that break each database limit; a refused entry followed by a valid redelivery; data with odd spacing, key order and `\u0000`.
- Expected: `audit_entries` and `processed_events` exist and migration 2 is recorded once; a new entry is stored and returned in UTC; a duplicate is skipped without error and the original kept; concurrent duplicates store exactly one entry; the database refuses values over each limit; a refused entry leaves nothing marked processed; `data_json` comes back byte for byte.
- Tests: `internal/domain/entry_test.go` (`TestNewEntry`); `internal/adapters/postgres/entries_integration_test.go` (`TestEntries_MigrationsRecorded`, `TestEntries_RecordEntry`, `TestEntries_RecordEntry_ConcurrentDuplicates`, `TestEntries_DatabaseLimits`).
- Command: `go test -count=1 ./services/audit-log/internal/domain/` and `go test -count=1 -tags=integration ./services/audit-log/internal/adapters/postgres/`
- Status: PASS.

### JT35-A-02: Admin-only list RPC using the shared pagination messages

- Criterion: Admin-only list RPC using the shared pagination messages.
- Inputs: a list with no filters; with an `event_type` filter (used and unused); with start only, end only, both, and a 1-microsecond window; with page sizes 0, 1, 2, 101 and 500 and page tokens; an offset past the end; entries with the same `occurred_at`; invalid filters and pagination as admin.
- Expected: entries newest first with `event_id` breaking ties; filters combine with AND, start inclusive and end exclusive; `pagination.total_count` is always filled and matches the filter; `next_page_token` is the next offset and empty on the last page; page size 0 means 20 and sizes above 100 are capped; each invalid input returns `INVALID_ARGUMENT`; every response has an `entries` list and a `pagination` message.
- Tests: `internal/domain/page_test.go` (`TestNewPage`, `TestPage_NextToken`); `internal/domain/entry_test.go` (`TestNewListFilter`); `internal/app/entries_test.go` (`TestAuditLog_ListEntries_RejectsInvalidInput`, `TestAuditLog_ListEntries_PassesFilterAndPage`); `internal/adapters/grpc/audit_log_test.go` (`TestAuditLogHandler_ListAuditEntries`, `TestAuditLogHandler_MapsEntryFields`); `internal/adapters/postgres/entries_test.go` (`TestEntryWhere`); `internal/adapters/postgres/entries_integration_test.go` (`TestEntries_ListEntries`, 12 subtests).
- Command: `go test -count=1 ./services/audit-log/internal/...` and `go test -count=1 -tags=integration ./services/audit-log/internal/adapters/postgres/`
- Status: PASS.

### JT35-A-03: Event consumer for all events is Sprint 2

- Criterion: Event consumer for all events is Sprint 2.
- Expected: no events are consumed or published in Sprint 1. The schema, the processed event ID table and `RecordEntry` in the repository are ready for the consumer; the RabbitMQ connection is opened at startup and kept for it.
- Status: Not applicable in Sprint 1, by design.

### JT35-A-04: Role and identity checks use pkg/authctx with the Section 9 codes

- Criterion: Role and ownership checks use `pkg/authctx` and return the gRPC codes from AGENTS.md Section 9.
- Inputs: no metadata, missing user ID, missing role, user ID not a UUID, unknown role, upper-case role; applicant and employer callers with valid and with invalid input (nil request, bad type, bad timestamps, start equal to end, bad page); each domain error; a database failure carrying driver text; canceled and expired contexts.
- Expected: identity problems return `UNAUTHENTICATED`; non-admins get `PERMISSION_DENIED` before any input check or repository call; bad input `INVALID_ARGUMENT`; canceled and expired contexts `CANCELED` and `DEADLINE_EXCEEDED`; anything else `INTERNAL` with the fixed message `internal error` and no driver detail.
- Tests: `internal/domain/caller_test.go` (`TestParseRole`, `TestNewCaller`, `TestCaller_RequireAdmin`); `internal/app/entries_test.go` (`TestAuditLog_ListEntries_RoleIsCheckedBeforeInput`, `TestAuditLog_ListEntries_WrapsRepositoryError`); `internal/adapters/grpc/audit_log_test.go` (`TestAuditLogHandler_RejectsBadIdentity`, `TestAuditLogHandler_RoleMatrix`, `TestAuditLogHandler_RoleBeforeInput`, `TestAuditLogHandler_HidesUnexpectedErrors`); `cmd/server/register_test.go` (`TestRegisterAuditLog_RegistersOnlyThePublicService`).
- Command: `go test -count=1 ./services/audit-log/...`
- Status: PASS.

### JT35-S-01: Migrations, repository and database constraints

- Criterion: goose migrations run on startup; maximum lengths for free-text fields are set; repository integration test with Testcontainers.
- Prerequisites: Docker; `docker pull postgres:17-alpine && docker pull postgres:18-alpine && docker pull rabbitmq:4.1-alpine` (17 is used by the template readiness test).
- Expected: migrations apply and can run twice; startup runs them before the servers start; the cases in JT35-A-01 and JT35-A-02 hold against a real PostgreSQL.
- Tests: `internal/adapters/postgres/entries_integration_test.go` (the 5 `TestEntries_` tests); `internal/adapters/postgres/readiness_integration_test.go` (template); `cmd/server/main_test.go` (`TestRunWithDatabase_OrdersStartupAndCleansUp`, which also checks the entry repository is wired).
- Command: `go test -count=1 -tags=integration ./services/audit-log/internal/adapters/postgres/`
- Status: PASS (all 7 integration tests in the package, 13.8 s).

### JT35-S-02: Docker Compose startup and health

- Criterion: Service block in `deploy/compose/docker-compose.yml`; `docker compose up` starts it and `GET /healthz` returns 200.
- Prerequisites: `deploy/compose/.env` from `.env.example`.
- Expected: the image builds; audit-log waits for healthy Postgres and RabbitMQ, applies migrations and becomes healthy; `/healthz` returns 200; ports 9090 and 8080 are not published to the host; the database password is passed as `PGPASSWORD`, not inside the URL. The `audit_log` database and user already existed in `deploy/compose/postgres-init`, and the gateway already had `AUDIT_LOG_ADDR`.
- References: `deploy/compose/docker-compose.yml` (audit-log block), `services/audit-log/Dockerfile`, `cmd/server/main_test.go` (template health and startup order tests).
- Command: the Compose commands in the execution results table.
- Status: PASS on `0f9c077`.

### JT35-S-03: Public RPC through the API gateway (manual)

- Purpose: extra evidence that the route works end to end with real JWTs from auth. Not required by the acceptance criteria.
- Setup: `docker compose -f deploy/compose/docker-compose.yml up -d --build auth gateway audit-log`; a new synthetic applicant and employer registered and logged in through `POST /v1/auth/register` and `POST /v1/auth/login` on `http://localhost:8000`; the seeded admin from `deploy/compose/.env`. Because the consumer is Sprint 2, two synthetic entries (`job.published` at 04:00 and `offer.made` at 05:00) were inserted with `psql` and deleted at the end.
- Result on `0f9c077` (8 Nov 2026, about 12:16 UTC), run with a local script that is not committed:

| # | Request | Caller | Expected | Result |
|---|---|---|---|---|
| 1 | `GET /v1/audit/entries` without a token | none | 401 (`UNAUTHENTICATED`) | PASS |
| 2 | `GET /v1/audit/entries` | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 3 | `GET /v1/audit/entries` | employer | 403 | PASS |
| 4 | `GET /v1/audit/entries?event_type=BAD&pagination.page_size=-1` | employer | 403, role before input | PASS |
| 5 | `GET /v1/audit/entries` | admin | 200, `offer.made` before `job.published`; `dataJson` exactly as stored | PASS (3 checks) |
| 6 | `?event_type=offer.made` | admin | 200, only `offer.made` | PASS (2 checks) |
| 7 | `?start_time=...T03:30:00Z&end_time=...T04:30:00Z` | admin | 200, only the 04:00 entry | PASS (2 checks) |
| 8 | `?pagination.page_size=1` | admin | 200, one entry, next page token `1` | PASS (2 checks) |
| 9 | `?event_type=Job.Published` | admin | 400 (`INVALID_ARGUMENT`) | PASS |
| 10 | `start_time` equal to `end_time` | admin | 400 | PASS |
| 11 | `start_time` after `end_time` | admin | 400 | PASS |
| 12 | `?pagination.page_size=-1` | admin | 400 | PASS |
| 13 | `?pagination.page_token=abc` | admin | 400 | PASS |

The script printed 18 PASS lines for these 13 requests (some requests check several fields), after registering and logging in the three accounts and before removing the two synthetic entries.

## Not covered or pending

- The event consumer for all events, its durable queue and its handling of events that fail validation: Sprint 2, not part of JT-35.
- Recording who acted: needs `actor_id` in `EventEnvelope`, which AGENTS.md Section 16 defers. Audit-log stores no actor until then (decision 1).
- Retention or clean-up of old audit entries: not designed; not in the acceptance criteria.
- Real client adapters: audit-log calls no other services, so there are no client ports in Sprint 1.
- Internal-token protection: not relevant, because the contract has no `*InternalService`.
- Race detector on Jigden's laptop: not run (cgo disabled). The concurrency tests above run without it.
- Whole repository after merging `main`, and GitHub Actions CI on the final commit: checked before merging the PR.
