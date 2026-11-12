# JT-52 Interview service test cases

Traceability: [JT-52](https://jobtrack-cst.atlassian.net/browse/JT-52) (parent JT-12). Owner: Kinley Tobgay Lhendrup. Verification date: 12 November 2026.

## Scope

The three public `InterviewService` RPCs (`ScheduleInterview`, `GetInterview`, `ListInterviews`), built on the existing `interview.proto` contract without changes. Interviews are stored in the `interview` database.

Ownership comes from a local `application_participants` table, the trusted projection of `application.submitted` (the Offer, Messaging and Review design). In this task it is read through the `ParticipantRepository` port, mocked with gomock in unit tests, and the Sprint 2 consumer fills it. Until then no application is known at runtime, so `ScheduleInterview` and `ListInterviews` return `NOT_FOUND`. Publishing `interview.scheduled` is Sprint 2 and is marked in `app.Interviews.Schedule`.

Recording the interview outcome is not possible yet: `interview.proto` has no outcome field or RPC. It needs a contract change agreed by the team, recorded in AGENTS.md Section 16.

Rules agreed for JT-52 (AGENTS.md Section 15, approved by Phuntsho):

- `ScheduleInterview` requires role `employer` (checked before input), a UUID `application_id` and `scheduled_at`. The application must be known (`NOT_FOUND` otherwise) and the caller must be its stored employer (`PERMISSION_DENIED` otherwise). The applicant comes from the projection, never the request.
- `scheduled_at` is stored in UTC to the microsecond and must be strictly after the current time and at most 365 days ahead. An application can have several interviews, but not two at the same time (`ALREADY_EXISTS`).
- `details` is 0 to 1000 characters after trimming. Newline, carriage return and tab are allowed; other control characters and invalid UTF-8 are `INVALID_ARGUMENT`.
- `GetInterview` and `ListInterviews` are readable by the application's applicant and owning employer. Admins, other applicants and other employers get `PERMISSION_DENIED`. Lists are ordered by `scheduled_at`, then `interview_id`, with the Section 9 pagination rules.
- No internal RPCs and no `INTERNAL_TOKEN`.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT52-01 | AC1 | Owning employer schedules an interview | Equivalence partitioning | Stored with a new UUID, employer from metadata, applicant from the projection, UTC timestamps | Unit, `internal/app`; gRPC; wiring | Pass |
| JT52-02 | AC1 | Another employer schedules, reads or lists for the application | Authorization (ownership) | `PERMISSION_DENIED`; nothing stored or returned | Unit; gRPC | Pass |
| JT52-03 | AC1 | Applicant schedules; admin calls any RPC | Authorization (role) | `PERMISSION_DENIED` before input or lookup | Unit; gRPC; wiring | Pass |
| JT52-04 | AC1 | Unknown or not yet processed application; unknown interview | Equivalence partitioning | `NOT_FOUND` | Unit; gRPC; integration | Pass |
| JT52-05 | AC1 | Applicant and owning employer read and list; other applicant denied | Authorization (readers) | Interview returned to both readers; others `PERMISSION_DENIED` | Unit; gRPC; wiring | Pass |
| JT52-06 | AC1 | Several interviews for one application, listed soonest first and paged; empty list | Equivalence partitioning | Ordered by time then ID; correct `total_count` and `next_page_token`; empty list for none | Unit; gRPC; integration | Pass |
| JT52-07 | AC1 | Same application at the same moment, including another time zone; simultaneous bookings | Equivalence partitioning, concurrency | `ALREADY_EXISTS`; exactly one stored | Unit; gRPC; integration; wiring | Pass |
| JT52-08 | AC1 | Record the interview outcome | Requirement review | Not possible: no outcome field or RPC in `interview.proto` | Contract review | Not met |
| JT52-09 | AC2 | `scheduled_at` in the past, exactly now, 1 microsecond ahead | Boundary values | Past and now `INVALID_ARGUMENT`; 1 microsecond ahead accepted | Unit, `internal/domain`; gRPC; wiring | Pass |
| JT52-10 | AC2 | `scheduled_at` exactly 365 days ahead and 1 microsecond over | Boundary values | 365 days accepted; over `INVALID_ARGUMENT` | Unit, `internal/domain`; gRPC | Pass |
| JT52-11 | AC2 | `scheduled_at` missing or out of range | Equivalence partitioning | `INVALID_ARGUMENT` | Unit; gRPC | Pass |
| JT52-12 | AC7 | `details` of 0, 1000 and 1001 characters, surrounding spaces, newline and tab, other control characters, invalid UTF-8 | Boundary values | 0 and 1000 accepted (trimmed); 1001, control characters and invalid UTF-8 `INVALID_ARGUMENT` | Unit, `internal/domain`; gRPC | Pass |
| JT52-13 | AC6 | Missing or malformed identity, unknown or wrong-case role | Equivalence partitioning | `UNAUTHENTICATED` on all three RPCs | Unit, `internal/domain`; gRPC | Pass |
| JT52-14 | AC6 | Invalid `application_id` or `interview_id`, nil request, invalid page size or token | Equivalence partitioning | `INVALID_ARGUMENT` | Unit; gRPC | Pass |
| JT52-15 | AC6 | Database error, canceled and expired context | Equivalence partitioning | `INTERNAL` "internal error" without driver details; `CANCELED`; `DEADLINE_EXCEEDED` | gRPC; integration | Pass |
| JT52-16 | AC8 | Participants and interviews read through ports mocked with gomock | Equivalence partitioning | Every rule unit-tested without a database | Unit, `internal/app` | Pass |
| JT52-17 | AC7 | Migrations applied twice; database refuses long details and unknown applications | Equivalence partitioning | Both runs succeed; constraints refuse | Integration | Pass |
| JT52-18 | AC10 | `docker compose up interview`, then `GET /healthz` | Equivalence partitioning | `HTTP/1.1 200 OK` | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-52: AC1 to AC3 are the task criteria (AC3, publishing `interview.scheduled`, is Sprint 2), AC4 to AC11 the shared Sprint 1 criteria (AC6 role and ownership through `pkg/authctx`, AC7 migrations and limits, AC8 ports mocked with gomock, AC10 Docker Compose and `/healthz`).

## Verification

Commands run on branch `JT-52-interview-basic-operations`:

    go test -count=1 -race -cover ./services/interview/...
    go test -count=1 -tags=integration ./services/interview/internal/adapters/postgres/...
    docker compose -f deploy/compose/docker-compose.yml up -d --build interview

| Check | Result |
|---|---|
| Unit tests | Pass |
| Integration tests (Testcontainers, `postgres:18-alpine`) | Pass, 8 subtests |
| Package coverage (unit) | `domain` 100%, `app` 100%, `adapters/grpc` 100%, `cmd/server` 81.2% |
| Docker Compose `/healthz` | `HTTP/1.1 200 OK` |
| CI on the pull request | CI_RUN_PLACEHOLDER |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Employer schedules an interview for an application on their own job and records the outcome | Partly met: scheduling met; recording the outcome not met, needs a contract change | JT52-01 to JT52-08; AGENTS.md Section 16 |
| Interview time must be in the future (`INVALID_ARGUMENT` otherwise) | Met | JT52-09 to JT52-11 |
| Publishing `interview.scheduled` is Sprint 2 | Not in scope | Marked in `app.Interviews.Schedule` |
| Created with `scripts/new-service.sh`, hexagonal layout | Met | First commit on the branch |
| Public RPCs exactly as in the contract; no contract change | Met | No `proto/` changes in the PR |
| Role and ownership through `pkg/authctx`, Section 9 codes | Met | JT52-02, JT52-03, JT52-13 to JT52-15 |
| goose migrations on startup; free-text limits | Met | JT52-12, JT52-17 |
| Calls to other services as ports mocked with gomock | Met | JT52-16; participants port mocked |
| Table-driven unit tests for domain and app; Testcontainers repository test | Met | All JT52 cases |
| Compose service block; `GET /healthz` returns 200 | Met | JT52-18 |
| PR with the Jira key, CI green, merged, time logged | Pending | This PR |
