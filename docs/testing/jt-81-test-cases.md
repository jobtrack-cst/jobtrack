# JT-81 Application-status hiring events test cases

Traceability: [JT-81](https://jobtrack-cst.atlassian.net/browse/JT-81) (parent JT-11). Owner: Kinley Tobgay Lhendrup. Verification date: 15 November 2026.

## Scope

Application-status consumes `review.completed`, `interview.scheduled`, `offer.made`, `offer.accepted` and `offer.declined` and moves each application through the Section 6 lifecycle. It builds on JT-60: the same `processed_events`, history, outbox, relay and `application.status_changed`, and a second queue next to `application-status.applications`. No contract or Compose changes.

Rules agreed for JT-81 (AGENTS.md Section 15, approved by Phuntsho):

- Queue `application-status.hiring` through `events.Consume`, bound to the five events. Each type must come from its producer (`review`, `interview`, `offer`), with a UUID `event_id`, valid `occurred_at`, matching payload, UUID IDs (stored lowercase) and decision `SHORTLISTED` or `REJECTED`.
- `review.completed` SHORTLISTED from SUBMITTED is two steps, SUBMITTED to UNDER_REVIEW to SHORTLISTED, in one transaction (this is how UNDER_REVIEW is reached). REJECTED from SUBMITTED is one step.
- `interview.scheduled` when already INTERVIEW claims the `event_id` and changes nothing.
- Any other transition Section 6 does not allow is not applied and goes to the dead queue at once. An offer event for a different applicant is a conflict and goes to the dead queue.
- Every step stores a history row and an `application.status_changed` outbox row with the status, in one transaction with the `event_id` claim; any failure releases the claim. An unknown application or a concurrent change is retried.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT81-01 | AC1 | Each of the five shared fixtures read by the consumer | Contract | Expected domain event (kind, IDs, time) reaches the repository | Unit, `adapters/rabbitmq` | Pass |
| JT81-02 | AC1 | Consumer configuration | Equivalence partitioning | Queue `application-status.hiring`, exactly the five bindings; both consumers started by `runWorkers` | Unit, `adapters/rabbitmq`, `cmd/server`; Docker Compose (`list_bindings`) | Pass |
| JT81-03 | AC1 | Same `event_id` delivered 3 times, 8 times at once, and twice through a real broker | Equivalence partitioning, concurrency | Applied once: no extra history or outbox row | Integration, `adapters/postgres` (PostgreSQL and RabbitMQ) | Pass |
| JT81-04 | AC1 | Wrong producer for each type, unknown type, application event on this queue, bad or dashless `event_id`, missing, zero or invalid `occurred_at`, missing data, data of another event, unknown or lowercase decision, non-UUID application, interview, offer or applicant ID, missing `scheduled_at`, nil envelope | Equivalence partitioning | Dead queue at once; nothing reaches the repository | Unit, `adapters/rabbitmq` | Pass |
| JT81-05 | AC1 | Uppercase IDs; REJECTED decision | Equivalence partitioning | Stored lowercase; REJECTED kind | Unit | Pass |
| JT81-06 | AC2 | Every hiring event against every one of the 9 statuses | State transition (full table) | Allowed pairs give the expected steps; all others `ErrInvalidTransition` | Unit, `internal/domain` | Pass |
| JT81-07 | AC2 | SHORTLISTED from SUBMITTED; SHORTLISTED from UNDER_REVIEW; REJECTED from SUBMITTED | State transition | Two steps via UNDER_REVIEW; one step; one step | Unit; integration | Pass |
| JT81-08 | AC2 | Second `interview.scheduled` while INTERVIEW | State transition | `event_id` claimed; no history, no event, `updated_at` unchanged | Unit; integration; Docker Compose | Pass |
| JT81-09 | AC2 | Interview before shortlisting; offer before interview; any event after a final state | State transition | Not applied; no claim kept; same `event_id` applies later once valid | Integration | Pass |
| JT81-10 | AC2 | Skipped step on a real broker and in Compose (`offer.made` from SUBMITTED, `offer.declined` after ACCEPTED) | State transition | Event in `application-status.hiring.dead`; status unchanged; later events for the same application still apply | Integration (real broker); Docker Compose | Pass |
| JT81-11 | AC2 | Offer event with another `applicant_id` | Authorization (participant) | Conflict, dead queue; no claim; status unchanged | Unit; integration | Pass |
| JT81-12 | AC2 | Unknown application; concurrent change; database error | Equivalence partitioning | Retried (not dead-lettered); no claim kept | Unit; integration | Pass |
| JT81-13 | AC2 | `offer.accepted` and `offer.declined` at the same time | Concurrency | Exactly one applies; the other is an invalid transition | Integration | Pass |
| JT81-14 | AC3 | Full path to ACCEPTED (and to DECLINED) | State transition | History and `application.status_changed` for every step: UNSPECIFIED>SUBMITTED, SUBMITTED>UNDER_REVIEW, UNDER_REVIEW>SHORTLISTED, SHORTLISTED>INTERVIEW, INTERVIEW>OFFERED, OFFERED>ACCEPTED | Unit; integration; real broker; Docker Compose | Pass |
| JT81-15 | AC3 | Outbox insert fails on the second step of SHORTLISTED | Fault injection | Both steps, the status and the claim roll back; "apply hiring event failed" without driver details; retry succeeds | Integration | Pass |
| JT81-16 | AC3 | Relay publishes the six events on a real broker | Equivalence partitioning | Delivered in processing order with producer `application-status` | Integration (PostgreSQL and RabbitMQ) | Pass |
| JT81-17 | AC4 | `runWorkers` with an unreachable broker | State transition | Relay publishes; consumers keep retrying; all workers stop on shutdown | Unit, `cmd/server` | Pass |
| JT81-18 | AC5 | Real flow through the gateway: application, review, interview, second interview, offer, accept; and a second application with an offer from SUBMITTED, then REJECTED | End to end | Status read after each step; history SUBMITTED to ACCEPTED; skipped step not applied; REJECTED still applies | Docker Compose (manual) | Pass |
| JT81-19 | AC5 | Database and broker state after JT81-18 | End to end | 6 history rows and 6 published events in order; 0 unpublished; both queues 1 consumer, 0 ready; one message in `application-status.hiring.dead` | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-81: AC1 to AC5 are the task criteria, AC6 and AC7 the shared Sprint 3 criteria (AC6 AGENTS.md Sections 6, 9 and 10 with no new tools or contract changes, AC7 Docker Compose, PR, CI, merge and time logged).

## Verification

Commands run on branch `JT-81-hiring-events`:

    go test -count=1 -race -cover ./services/application-status/...
    go test -count=1 -tags=integration ./services/application-status/...
    go test -count=1 -tags=integration -coverprofile=/tmp/jt81.out ./services/application-status/...
    python3 /tmp/jt81_e2e.py   (local script, not committed; synthetic accounts, no credentials printed)

| Check | Result |
|---|---|
| Unit tests | Pass |
| Integration tests (Testcontainers PostgreSQL and `rabbitmq:4.1-alpine`) | Pass, including `TestHiringEvents_RealBrokerAndPostgres` |
| Package coverage (unit) | `domain` 100%, `app` 100%, `adapters/grpc` 100%, `adapters/rabbitmq` 97.9%, `cmd/server` 84.8% |
| Service coverage, unit plus integration, excluding generated mocks (AC4 target 80%) | NN.N% of statements |
| Docker Compose end to end | Pass: applications `e2f9afad-6e2f-4c6a-bd4b-253ba02468e1` (ACCEPTED) and `a94eb4d2-d08d-477c-93df-74ff4ffb1b5f` (skipped step, then REJECTED); log "event rejected by handler; dead-lettered" for the skipped `offer.made` |
| CI on the pull request | CI_RUN_PLACEHOLDER |

Known limitation (AGENTS.md Section 16): history is listed by each producer's `occurred_at`. The shared fixtures are dated out of order (`interview.scheduled.json` after `offer.made.json`), so the real-broker test reads history in write order. `application.status_changed` events always follow processing order.

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Consumes `review.completed`, `interview.scheduled`, `offer.made`, `offer.accepted` and `offer.declined`; idempotent on `event_id` | Met | JT81-01 to JT81-05 |
| Each event drives the Section 6 transition; an invalid transition is not applied and does not block the queue; how UNDER_REVIEW is reached is agreed with Phuntsho and recorded | Met | JT81-06 to JT81-13; AGENTS.md Section 15 |
| Every transition stores history and publishes `application.status_changed` | Met | JT81-14 to JT81-16 |
| Coverage at least 80 percent; integration tests with Testcontainers; consumer tests load the fixtures | Met | JT81-01, JT81-03, JT81-16, Verification table |
| Verified end to end in Docker Compose | Met | JT81-18, JT81-19 |
| Follows AGENTS.md Sections 6, 9 and 10; no new tools or contract changes | Met | No `proto/`, `go.mod` or Compose changes |
| Runs in Docker Compose; PR with the Jira key, CI green, merged, time logged | Pending | This PR |
