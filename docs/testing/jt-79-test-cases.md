# JT-79 Review events test cases

Traceability: [JT-79](https://jobtrack-cst.atlassian.net/browse/JT-79) (parent JT-12). Owner: Kinley Tobgay Lhendrup. Verification date: 15 November 2026.

## Scope

Review consumes `application.submitted` to fill its `application_participants` projection, and publishes `review.completed` when `CompleteReview` succeeds. It follows the JT-74 Offer design and uses the existing contracts without changes. The public `ReviewService` RPCs and their JT-51 rules are unchanged.

Rules agreed for JT-79 (AGENTS.md Section 15, approved by Phuntsho):

- Queue `review.applications` through `events.Consume`, bound only to `application.submitted` (JT-61 retry and dead-letter policy).
- The producer must be `application` and all five IDs UUIDs (stored lowercase); any other event goes to the dead queue at once.
- The `event_id` claim in `processed_events` and the participants are written in one transaction. A repeated `event_id` changes nothing; the same participants under a new `event_id` are accepted; different participants for a stored application go to the dead queue and keep no claim. Database errors are retried.
- `review.completed` carries `application_id` and `decision` `SHORTLISTED` or `REJECTED`, producer `review`, `occurred_at` = the review's `created_at` in UTC.
- `CompleteReview` stores the event in `outbox_events` in the review's transaction. One relay publishes oldest first, waits up to 10 s for each confirmation, marks after confirmation and keeps published rows; a reconnecting publisher wraps `pkg/events.Publisher`.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT79-01 | AC1 | Shared `application.submitted` fixture handled | Equivalence partitioning | Participants recorded with the event's `event_id` | Unit, `adapters/rabbitmq`; integration | Pass |
| JT79-02 | AC1 | Same event delivered 3 times, and 5 times at once; twice through a real broker | Equivalence partitioning, concurrency | One `processed_events` row, one participants row | Integration, `adapters/postgres` (PostgreSQL and RabbitMQ) | Pass |
| JT79-03 | AC1 | Same participants under a new `event_id` | Equivalence partitioning | Accepted, nothing changed | Integration | Pass |
| JT79-04 | AC1 | Different applicant or employer for a stored application | Equivalence partitioning | Dead queue at once (`review.applications.dead` has the message); no claim kept; stored participants unchanged | Unit; integration (real broker) | Pass |
| JT79-05 | AC1 | Wrong producer or type, `event_id` not a UUID, missing `occurred_at`, missing data, data of another event, nil envelope | Equivalence partitioning | Dead queue at once; nothing recorded | Unit, `adapters/rabbitmq` | Pass |
| JT79-06 | AC1 | Each of the five IDs missing or not a UUID; uppercase IDs | Equivalence partitioning | Invalid IDs dead-lettered; uppercase stored lowercase | Unit | Pass |
| JT79-07 | AC1 | Database error; canceled context | Equivalence partitioning | Retried (not permanent); driver details hidden | Unit; integration | Pass |
| JT79-08 | AC1 | Consumer configuration | Equivalence partitioning | Queue `review.applications`, binding `application.submitted` only | Unit; Docker Compose (`list_bindings`) | Pass |
| JT79-09 | AC2 | Review completed with each decision | Equivalence partitioning | Exactly one outbox row: UUID `event_id`, type `review.completed`, producer `review`, `occurred_at` = `created_at` (UTC), decision as stored | Unit, `internal/app`; integration | Pass |
| JT79-10 | AC2 | Score 1 SHORTLISTED and score 5 REJECTED; unspecified decision | Equivalence partitioning | Decision is independent of score; unspecified refused | Unit | Pass |
| JT79-11 | AC2 | Second review; five simultaneous reviews | Equivalence partitioning, concurrency | `ALREADY_EXISTS` adds no event; exactly one event | Integration | Pass |
| JT79-12 | AC2 | Outbox insert fails | Fault injection | Review rolled back (`NOT_FOUND`); "create review failed" without driver details | Integration | Pass |
| JT79-13 | AC2 | Relay publishes several events; publish fails; mark fails after a confirmed publish | State transition | Oldest first; stops at first failure; next run resends the same `event_id`; nothing published twice once marked | Unit, `internal/app`; integration | Pass |
| JT79-14 | AC2 | Each publish has a deadline | Boundary values | At most `PublishTimeout` (10 s) | Unit | Pass |
| JT79-15 | AC2 | Outbox limit 0, limit 1, invalid stored payload; mark twice | Boundary values | 0 refused; 1 returns one; invalid payload refused; second mark is not an error | Integration | Pass |
| JT79-16 | AC2 | Built event matches `review.completed.json`; fixture round-trips through `pkg/events` | Contract | Equal; decision is SHORTLISTED or REJECTED | Unit, `internal/app`; contract, `tests/contract` | Pass |
| JT79-17 | AC2 | Completed review published on a real broker | Equivalence partitioning | Delivered with routing key `review.completed` and decision `SHORTLISTED` | Integration (PostgreSQL and RabbitMQ) | Pass |
| JT79-18 | AC2 | Broker connection lost during a publish | State transition | Connection dropped and reopened on the next publish | Unit, `cmd/server` | Pass |
| JT79-19 | AC2 | Server wiring: relay publishes through the opened connection; consumer and relay stop on shutdown; consumer stopping early is logged | State transition | One connection opened and closed after shutdown; workers stopped before return | Unit, `cmd/server` | Pass |
| JT79-20 | AC4 | Real flow through the gateway: verified employer, published job, validated resume, application, review | End to end | All 20 steps `ok`; applicant 403; second review 409; stored review is the first | Docker Compose (manual) | Pass |
| JT79-21 | AC4 | Database and broker state after JT79-20 | End to end | Participants stored; one published `review.completed`; 0 unpublished; consumer ready; 0 dead messages | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-79: AC1 to AC4 are the task criteria, AC5 and AC6 the shared Sprint 3 criteria (AC5 AGENTS.md Sections 6, 9 and 10 with no new tools or contract changes, AC6 Docker Compose, PR, CI, merge and time logged).

## Verification

Commands run on branch `JT-79-review-events`:

    go test -count=1 -race -cover ./services/review/... ./tests/contract/...
    go test -count=1 -tags=integration ./services/review/...
    python3 /tmp/jt79_e2e.py   (local script, not committed; synthetic accounts, no credentials printed)

| Check | Result |
|---|---|
| Unit and contract tests | Pass |
| Integration tests (Testcontainers `postgres:18-alpine` and `rabbitmq:4.1-alpine`) | Pass, including `TestReviewEvents_RealBrokerAndPostgres` |
| Package coverage (unit) | `domain` 100%, `app` 98.2%, `adapters/grpc` 98.4%, `adapters/rabbitmq` 96.2%, `cmd/server` 86.0% |
| Docker Compose end to end | Pass: application `c5ba671e-12a1-497c-b827-d0decf7f64ca`; `review.applications` 1 consumer, 0 ready; `review.applications.dead` 0; log "outbox events published" |
| CI on the pull request | Green, [CI run](https://github.com/jobtrack-cst/jobtrack/actions/runs/43632892754) |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Consumes `application.submitted` from `review.applications` using `events.Consume` and stores the participants; idempotent on `event_id` | Met | JT79-01 to JT79-08 |
| `CompleteReview` stores `review.completed` in an outbox in the same transaction and publishes it through `pkg/events`; fixture added | Met | JT79-09 to JT79-19 |
| Unit tests; integration tests with Testcontainers PostgreSQL and RabbitMQ | Met | All JT79 cases |
| Verified end to end in Docker Compose | Met | JT79-20, JT79-21 |
| Follows AGENTS.md Sections 6, 9 and 10; no new tools or contract changes | Met | No `proto/` or `go.mod` changes; queue name, envelope, idempotency and outbox per Section 9 |
| Runs in Docker Compose; PR with the Jira key, CI green, merged, time logged | Pending | This PR |
