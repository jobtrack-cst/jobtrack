# JT-80 Interview events test cases

Traceability: [JT-80](https://jobtrack-cst.atlassian.net/browse/JT-80) (parent JT-12). Owner: Kinley Tobgay Lhendrup. Verification date: 15 November 2026.

## Scope

Interview consumes `application.submitted` to fill its `application_participants` projection, and publishes `interview.scheduled` when `ScheduleInterview` succeeds. It copies the JT-79 Review design and uses the existing contracts without changes. The public `InterviewService` RPCs and their JT-52 rules are unchanged; recording interview outcomes is still the open question in AGENTS.md Section 16. The microsecond defect found here (JT80-22) also existed in review's `CompleteReview` (JT-79); with the team's agreement it is fixed in this PR too (JT80-23).

Rules agreed for JT-80 (AGENTS.md Section 15, approved by Phuntsho):

- Queue `interview.applications` through `events.Consume`, bound only to `application.submitted` (JT-61 retry and dead-letter policy).
- The producer must be `application` and all five IDs UUIDs (stored lowercase); any other event goes to the dead queue at once.
- The `event_id` claim in `processed_events` and the participants are written in one transaction. A repeated `event_id` changes nothing; the same participants under a new `event_id` are accepted; different participants go to the dead queue and keep no claim. Database errors are retried.
- One `interview.scheduled` per scheduled interview: `interview_id`, `application_id`, `scheduled_at` in UTC, producer `interview`, `occurred_at` = the interview's `created_at`. `details` are not in the event.
- `ScheduleInterview` stores the event in `outbox_events` in the interview's transaction, so `ALREADY_EXISTS` adds no event. One relay publishes oldest first, waits up to 10 s for each confirmation, marks after confirmation and keeps published rows; a reconnecting publisher wraps `pkg/events.Publisher`.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT80-01 | AC1 | Shared `application.submitted` fixture handled | Equivalence partitioning | Participants recorded with the event's `event_id` | Unit, `adapters/rabbitmq`; integration | Pass |
| JT80-02 | AC1 | Same event delivered 3 times, and 5 times at once; twice through a real broker | Equivalence partitioning, concurrency | One `processed_events` row, one participants row | Integration, `adapters/postgres` (PostgreSQL and RabbitMQ) | Pass |
| JT80-03 | AC1 | Same participants under a new `event_id` | Equivalence partitioning | Accepted, nothing changed | Integration | Pass |
| JT80-04 | AC1 | Different applicant or employer for a stored application | Equivalence partitioning | Dead queue at once (`interview.applications.dead` has the message); no claim kept; stored participants unchanged | Unit; integration (real broker) | Pass |
| JT80-05 | AC1 | Wrong producer or type, `event_id` not a UUID, missing `occurred_at`, missing data, data of another event, nil envelope | Equivalence partitioning | Dead queue at once; nothing recorded | Unit, `adapters/rabbitmq` | Pass |
| JT80-06 | AC1 | Each of the five IDs missing or not a UUID; uppercase IDs | Equivalence partitioning | Invalid IDs dead-lettered; uppercase stored lowercase | Unit | Pass |
| JT80-07 | AC1 | Database error; canceled context | Equivalence partitioning | Retried (not permanent); driver details hidden | Unit; integration | Pass |
| JT80-08 | AC1 | Consumer configuration | Equivalence partitioning | Queue `interview.applications`, binding `application.submitted` only | Unit; Docker Compose (`list_bindings`) | Pass |
| JT80-09 | AC2 | Two interviews for one application | Equivalence partitioning | Two outbox rows, each with UUID `event_id`, type `interview.scheduled`, producer `interview`, its own `interview_id` and `scheduled_at`, `occurred_at` = `created_at` (UTC) | Unit, `internal/app`; integration | Pass |
| JT80-10 | AC2 | Interview without `scheduled_at` or `created_at` | Equivalence partitioning | Refused before any write; no event | Unit; integration | Pass |
| JT80-11 | AC2 | Same application and time; five simultaneous bookings of one time | Equivalence partitioning, concurrency | `ALREADY_EXISTS` adds no event; exactly one event, for the first interview | Integration | Pass |
| JT80-12 | AC2 | Outbox insert fails | Fault injection | Interview rolled back (`NOT_FOUND`); "create interview failed" without driver details | Integration | Pass |
| JT80-13 | AC2 | Relay publishes several events; publish fails; mark fails after a confirmed publish | State transition | Oldest first; stops at first failure; next run resends the same `event_id`; nothing published twice once marked | Unit, `internal/app`; integration | Pass |
| JT80-14 | AC2 | Each publish has a deadline | Boundary values | At most `PublishTimeout` (10 s) | Unit | Pass |
| JT80-15 | AC2 | Outbox limit 0, limit 1, invalid stored payload; mark twice | Boundary values | 0 refused; 1 returns one; invalid payload refused; second mark is not an error | Integration | Pass |
| JT80-16 | AC2 | Built event matches `interview.scheduled.json`; fixture round-trips through `pkg/events` | Contract | Equal; `scheduled_at` after `occurred_at` | Unit, `internal/app`; contract, `tests/contract` | Pass |
| JT80-17 | AC2 | Scheduled interview published on a real broker | Equivalence partitioning | Delivered with routing key `interview.scheduled` and the stored `interview_id`, `application_id`, `scheduled_at` | Integration (PostgreSQL and RabbitMQ) | Pass |
| JT80-18 | AC2 | Broker connection lost during a publish | State transition | Connection dropped and reopened on the next publish | Unit, `cmd/server` | Pass |
| JT80-19 | AC2 | Server wiring: relay publishes through the opened connection; consumer and relay stop on shutdown; consumer stopping early is logged | State transition | One connection opened and closed after shutdown; workers stopped before return | Unit, `cmd/server` | Pass |
| JT80-20 | AC4 | Real flow through the gateway: verified employer, published job, validated resume, application, two interviews | End to end | All 24 steps `ok`; applicant 403; past time 400; same time 409; `applicant_id` from the consumed event; list soonest first | Docker Compose (manual) | Pass |
| JT80-21 | AC4 | Database and broker state after JT80-20 | End to end | Participants stored; two published `interview.scheduled` matching the interviews (`scheduled_at`, `occurred_at`, `event_id`); 0 unpublished; consumer ready; 0 dead messages | Docker Compose (manual) | Pass |
| JT80-22 | AC2 | Clock time with nanoseconds (999 ns) | Boundary values | `created_at` and `updated_at` kept to the microsecond, so the stored, returned and published times are equal | Unit, `internal/app` | Pass |
| JT80-23 | AC2 (JT-79 fix) | Review: clock time with nanoseconds (999 ns); review flow repeated in Docker Compose | Boundary values; end to end | `created_at` and `updated_at` kept to the microsecond; `review.completed` `occurred_at` equals the stored `created_at` | Unit, `services/review/internal/app`; Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-80: AC1 to AC4 are the task criteria, AC5 and AC6 the shared Sprint 3 criteria (AC5 AGENTS.md Sections 6, 9 and 10 with no new tools or contract changes, AC6 Docker Compose, PR, CI, merge and time logged).

## Verification

Commands run on branch `JT-80-interview-events`:

    go test -count=1 -race -cover ./services/interview/... ./tests/contract/...
    go test -count=1 -tags=integration ./services/interview/...
    python3 /tmp/jt80_e2e.py   (local script, not committed; synthetic accounts, no credentials printed)

| Check | Result |
|---|---|
| Unit and contract tests | Pass |
| Integration tests (Testcontainers `postgres:18-alpine` and `rabbitmq:4.1-alpine`) | Pass, including `TestInterviewEvents_RealBrokerAndPostgres` |
| Package coverage (unit) | `domain` 100%, `app` 98.7%, `adapters/grpc` 100%, `adapters/rabbitmq` 96.2%, `cmd/server` 87.4% |
| Docker Compose end to end | Pass: application `500e4045-8f7e-449b-ab7a-cf8ce065c2cb`; `interview.applications` 1 consumer, 0 ready; `interview.applications.dead` 0; log "outbox events published" count 2 |
| Defect found and fixed | The first Compose run (application `1d9f52d4-a249-4a6f-be5b-3f2af84efd04`) showed one event whose `occurred_at` differed from the stored `created_at` below one microsecond: the clock has nanoseconds and PostgreSQL keeps microseconds. `Schedule` now keeps `created_at` and `updated_at` to the microsecond (JT80-22); the run was repeated after the fix |
| CI on the pull request | Green, [CI run](https://github.com/jobtrack-cst/jobtrack/actions/runs/43725121934) |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Consumes `application.submitted` from `interview.applications` using `events.Consume` and stores the participants; idempotent on `event_id` | Met | JT80-01 to JT80-08 |
| `ScheduleInterview` stores `interview.scheduled` in an outbox in the same transaction and publishes it through `pkg/events`; fixture added | Met | JT80-09 to JT80-19, JT80-22 |
| Unit tests; integration tests with Testcontainers PostgreSQL and RabbitMQ | Met | All JT80 cases |
| Verified end to end in Docker Compose | Met | JT80-20, JT80-21 |
| Follows AGENTS.md Sections 6, 9 and 10; no new tools or contract changes | Met | No `proto/` or `go.mod` changes; queue name, envelope, idempotency and outbox per Section 9 |
| Runs in Docker Compose; PR with the Jira key, CI green, merged, time logged | Pending | This PR |
