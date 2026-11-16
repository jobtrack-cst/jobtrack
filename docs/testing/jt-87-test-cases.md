# JT-87 Moderation job_removed outbox test cases

Traceability: [JT-87](https://jobtrack-cst.atlassian.net/browse/JT-87). Owner: Jigden Shakya. Verification date: 16 November 2026.

## Scope

Publishing `moderation.job_removed` from `moderation` through a transactional outbox when an admin removes a job (`RemoveJob`), following the company-verification design (JT-56), and the shared fixture for the event. The consumer is job-posting's `job-posting.moderation` queue from JT-68, which is unchanged and only verified end to end here. No `.proto` contract or `events.proto` change.

Acceptance criteria used in the matrix:

- AC1: `RemoveJob` stores `moderation.job_removed` (job ID and reason; the admin as stored by JT-34) in an outbox in the same transaction and publishes it through `pkg/events`, same pattern as JT-56
- AC2: `moderation.job_removed` fixture added to `tests/contract/fixtures`
- AC3: unit tests; integration tests with Testcontainers PostgreSQL and RabbitMQ
- AC4: verified end to end in Docker Compose with JT-68: an admin removes a job and job-posting marks it REMOVED
- S1: follows AGENTS.md Sections 6, 9 and 10; no new tools or contract changes (shared Sprint 3 criterion)
- S2: runs in Docker Compose (shared Sprint 3 criterion)

Rules agreed for JT-87 (AGENTS.md Section 15, 16 Nov 2026):

- `RemoveJob` stores the takedown, the ACTIONED reports and the event in one transaction. A second takedown, an invalid event or any database error stores nothing.
- The event carries `job_id` and the trimmed `reason`, the two fields `ModerationJobRemoved` has in `events.proto`. The removing admin stays in `removed_jobs.removed_by` (JT-34); adding it to the event would be a contract change, which the shared criteria exclude. Producer `moderation` (job-posting accepts the event only from this producer), `event_id` a new UUID, `occurred_at` the removal time.
- One relay publishes unpublished events oldest first every second (at once after a full batch of 50), waits up to 10 s for each broker confirmation, stops at the first failure and marks a row only after confirmation. A reconnecting publisher wraps `pkg/events.Publisher`.
- Takedowns made before JT-87 have no outbox row and are not backfilled; the data is synthetic.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT87-01 | AC2 | Fixture decodes with `events.UnmarshalEnvelope`: routing key, producer, UUID `event_id` and `job_id`, reason of 1 to 500 characters, round trip | Equivalence partitioning | Decodes and round trips unchanged | Contract, `tests/contract` | Pass |
| JT87-02 | AC1 | `JobRemovedEvent` for a complete removal and for one without job ID, reason or removal time | Equivalence partitioning | `moderation.job_removed`, producer `moderation`, job ID and reason in the data, `occurred_at` the removal time, accepted by `events.MarshalEnvelope`; incomplete removals refused | Unit, `moderation/internal/app` | Pass |
| JT87-03 | AC1 | `RemoveJob` passes the event to the repository; padded reason; two removals | Equivalence partitioning | Event carries the trimmed reason; a new UUID for each removal | Unit, `internal/app` (gomock) | Pass |
| JT87-04 | AC1 | Existing `RemoveJob` cases with the new port: applicant and employer denied, role checked first, non-UUID job ID, reason of 0, 500 and 501 characters, second takedown, repository error | Authorization, boundary values | Same errors and gRPC codes as JT-34; no repository call before the checks pass | Unit, `internal/app` and `internal/adapters/grpc` | Pass |
| JT87-05 | AC1 | Removal stores its event; the relay reads it back; marked twice | Equivalence partitioning | One row `moderation.job_removed`, `created_at` the removal time; `Pending` returns exactly the stored envelope; the first `published_at` is kept | Integration, `moderation/internal/adapters/postgres` | Pass |
| JT87-06 | AC1 | Removal that does not commit: second takedown, nil event, duplicate event ID, routing key other than `moderation.job_removed` | Equivalence partitioning | No new outbox row; with a duplicate event ID the job is not in `removed_jobs`; with a nil event the reports stay OPEN and the job can still be removed | Integration | Pass |
| JT87-07 | AC1 | Two admins remove the same job at once | Concurrency | Exactly one takedown and one outbox event; the other gets `ErrJobAlreadyRemoved` | Integration | Pass |
| JT87-08 | AC1 | Outbox `Pending` order and limit | Boundary values | Oldest first by removal time; limit 2 respected; limit 0 refused | Integration | Pass |
| JT87-09 | AC1 | Relay `PublishPending`: two events, none, read error, publish failure, mark failure, second publish failure; deadline on each publish | State transition | Oldest first, each marked after the broker confirms; stops at the first failure; earlier events stay published; every publish has a 10 s deadline | Unit, `internal/app` (gomock) | Pass |
| JT87-10 | AC1 | Relay loop: failure then success, full batch, servers stop, context ends | State transition | Failure logged and retried; full batch read again at once; loop stops with the servers | Unit, `moderation/cmd/server` | Pass |
| JT87-11 | AC1 | Reconnecting publisher: failed publish, reconnect, failed reconnect, close | State transition | Failed connection closed; next publish opens a new one; a failed reconnect returns a fixed message without credentials and is tried again; publish after close fails | Unit, `cmd/server` | Pass |
| JT87-12 | AC1, S2 | Startup wiring: outbox repository and publisher passed on; existing startup order and cleanup | State transition | Outbox and publisher wired; database, RabbitMQ, servers opened and closed in order; no publish at startup | Unit, `cmd/server` | Pass |
| JT87-13 | AC1, AC3 | Two removals through the real use case, relay against real RabbitMQ, relay run again | End to end (component) | Both envelopes delivered once, oldest first, routing key `moderation.job_removed`, producer `moderation`, body equal to the stored payload; nothing sent on the second run | Integration, Testcontainers PostgreSQL and RabbitMQ | Pass |
| JT87-14 | AC3 | JT-34 repository tests with the event added: actioned reports, report racing a removal (20 rounds), removal reason of 500 and 501 characters and empty | Regression | Unchanged JT-34 results | Integration | Pass |
| JT87-15 | AC4, S2 | Docker Compose through the gateway: new employer verified by the admin, creates and publishes a job; admin removes it with reason "JT-87 end-to-end check" | Manual end to end | Job read as `JOB_STATUS_REMOVED` 1.0 s after the removal; one outbox row with `published_at` set; moderation logs `outbox events published` with count 1; job-posting row `removed`; `job-posting.moderation` 0 messages and 1 consumer, `job-posting.moderation.dead` 0 messages | Manual, Compose (local helper, not committed) | Pass |
| JT87-16 | S1 | Changed files reviewed: no file under `proto/` or `gen/`, no new module in `go.mod` | Review | No contract or tool changes | Review | Pass |
| JT87-17 | AC1 | RabbitMQ restarted in Compose, then another removal | Manual end to end | Event published after the reconnect | Manual, Compose | Not run: optional; the reconnect is covered by JT87-11 |

## Verification

All commands ran on 16 November 2026 on Jigden's laptop (WSL Ubuntu, Docker Desktop), on branch `JT-87-moderation-job-removed-outbox` on top of `c2d497f`, with the mocks regenerated by `go generate ./services/moderation/internal/ports/...`.

    gofmt -l services/moderation tests/contract
    go vet ./services/moderation/... ./tests/contract/...
    go vet -tags=integration ./services/moderation/...
    go test -count=1 -cover ./services/moderation/... ./tests/contract/...
    go test -count=1 -tags=integration ./services/moderation/...

| Check | Result |
|---|---|
| Formatting and static checks | Pass, no output |
| Unit tests | Pass, all packages `ok` |
| Integration tests | Pass, all packages `ok` (postgres 60.1 s, rabbitmq 23.6 s, real containers) |
| Coverage (unit run) | `internal/app` 97.4%, `internal/domain` 98.4%, `internal/adapters/grpc` 97.3%, `cmd/server` 85.9%, `internal/adapters/rabbitmq` 83.3%; `internal/adapters/postgres` is covered by the integration tests |
| Compose | `00003_create_outbox_events.sql` applied, database at version 3; moderation and job-posting healthy |
| End to end (JT87-15) | Pass, job `12c406e5-d54b-4439-9bf1-df2749e5b125`, event `59405c85-dfa5-4044-81bd-fcd94e237856` created at 14:51:15.004 UTC and published at 14:51:15.918 UTC; job-posting `updated_at` 14:51:15.920 UTC |
| CI on the pull request | To be added after the PR run |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| AC1: outbox in the same transaction, published through `pkg/events` | Met. The event carries job ID and reason; the admin stays in `removed_jobs` because the contract has no admin field | JT87-02 to JT87-13 |
| AC2: fixture added | Met | JT87-01 |
| AC3: unit and integration tests | Met | JT87-02 to JT87-14 |
| AC4: end to end with JT-68 | Met | JT87-15 |
| S1: no new tools or contract changes | Met | JT87-16 |
| S2: runs in Docker Compose | Met | JT87-12, JT87-15 |

## Not covered

- Takedowns made before JT-87 (for example job `cfe7f8aa-a419-4349-a2cc-955c562167de` from the JT-88 check) have no outbox row and stay PUBLISHED in job-posting.
- Outbox rows are kept after publishing; there is no cleanup (AGENTS.md Section 16).
- job-posting does not publish `job.closed` for a removed job (found during JT-87, AGENTS.md Section 16); out of scope for this task.
