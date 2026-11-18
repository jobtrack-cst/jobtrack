# JT-90 job-posting publishes job.closed on moderation removal: test cases

Traceability: [JT-90](https://jobtrack-cst.atlassian.net/browse/JT-90) (Bug, found during JT-87). Fixed by: Jigden Shakya as job-posting backup. Service owner and reviewer: Sangay Tenzin. Verification date: 18 November 2026.

## Scope

job-posting's `moderation.job_removed` consumer (`RecordJobRemoved`, JT-68) set the job to `removed` but stored no outbox row, so `job.closed` was never published and `job-search` and `saved-jobs` were not told about removed jobs. Its `WHERE status != 'REMOVED'` compared against uppercase while statuses are stored lowercase, so it never filtered anything. The fix stores `job.closed` in job-posting's existing `outbox_events` (JT-68) in the same transaction as the status change. No `.proto`, `events.proto`, migration or new tool.

Acceptance criteria used in the matrix:

- AC1: removing a published job through moderation stores and publishes exactly one `job.closed` in the same transaction as the status change
- AC2: no `job.closed` for an unknown job, an already removed job or a redelivered `event_id`
- AC3: unit and integration tests (Testcontainers PostgreSQL and RabbitMQ, build tag `integration`); consumer tests load `tests/contract/fixtures/moderation.job_removed.json`
- AC4: verified end to end in Docker Compose: a verified employer publishes a job, the admin calls `POST /v1/moderation/jobs/<job_id>/remove`, one `job.closed` row appears in the `job_posting` outbox and is published on `jobtrack.events`
- AC5: AGENTS.md Section 16 item "Found during JT-87" removed

Rules (AGENTS.md Section 15, 18 Nov 2026):

- One transaction: claim the `event_id` in `processed_events` with `ON CONFLICT DO NOTHING`, lock the job row with `SELECT ... FOR UPDATE`, set `removed`, and store `job.closed` through the same `storeJobEvent` helper that `UpdateFields` and `UpdateStatus` use.
- `job.closed` only when the job was `published` (`domain.RemovalPublishesClose`). A draft job was never published; a closed job already sent `job.closed`, and analytics counts every `job.closed`. Approved by Sangay as job-posting owner in PR #69.
- Producer `job-posting`, `event_id` a new UUID, `occurred_at` and outbox `created_at` the removal time (`updated_at`).

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT90-01 | AC1, AC2 | `RemovalPublishesClose` for draft, published, closed, removed and unspecified | Equivalence partitioning | True only for published | Unit, `internal/domain/removal_test.go` | Pass |
| JT90-02 | AC3 | Consumer handler with the shared `moderation.job_removed` fixture | Contract | Repository called once with the fixture's `event_id` and `job_id` | Unit, `internal/adapters/rabbitmq/moderation_consumer_test.go` | Pass |
| JT90-03 | AC3 | Fixture changed: wrong producer, wrong type, wrong data type, `job_id` not a UUID | Equivalence partitioning | Error returned; repository never called | Unit, same file | Pass |
| JT90-04 | AC3 | Repository returns a database error | Error guessing | Same error returned, so the shared consumer retries it | Unit, same file | Pass |
| JT90-05 | AC1 | Published job removed | Equivalence partitioning | Status `removed`; exactly one `job.closed` for the job, producer `job-posting`, UUID `event_id`, `occurred_at` equal to `updated_at` | Integration, `internal/adapters/postgres/outbox_integration_test.go` | Pass |
| JT90-06 | AC2 | Same `event_id` delivered twice | State transition | Still one `job.closed` | Integration, same file | Pass |
| JT90-07 | AC2 | Already removed job, new `event_id` | State transition | `updated_at` unchanged; still one `job.closed` | Integration, same file | Pass |
| JT90-08 | AC2 | Unknown job, delivered twice | Equivalence partitioning | No error, no job row created, no event | Integration, same file | Pass |
| JT90-09 | AC2 | Draft job removed | Equivalence partitioning | Status `removed`; no `job.closed` | Integration, same file | Pass |
| JT90-10 | AC2 | Closed job removed | Equivalence partitioning | Status `removed`; no second `job.closed` | Integration, same file | Pass |
| JT90-11 | AC1 | Five concurrent removals of one published job with different event IDs | Concurrency | All succeed; exactly one `job.closed` | Integration, same file | Pass |
| JT90-12 | AC1, AC2, AC3 | Removal, redelivery and second removal, then the real `OutboxRelay` and publisher against real RabbitMQ | End to end (component) | Relay publishes 1 event; one message with routing key `job.closed`, producer `job-posting` and the job ID on `jobtrack.events`; no second message within 2 s; row marked published | Integration, `internal/adapters/postgres/broker_integration_test.go` | Pass |
| JT90-13 | AC1 | Existing JT-68 outbox test (`TestOutbox_SaveAndPending`) and all job-posting tests after extracting `storeJobEvent` from `write` | Regression | Unchanged results | Unit and integration | Pass |
| JT90-14 | AC4 | Docker Compose through the gateway: new employer verified by the admin, creates and publishes a job; admin removes it with reason "JT-90 end-to-end check"; then removes it again | Manual end to end | See Verification | Manual, Compose (local helper, not committed) | Pass |

## Verification

All commands ran on 18 November 2026 on Jigden's laptop (WSL Ubuntu, Docker Desktop), on branch `JT-90-job-removed-publishes-job-closed` on top of `b861368`.

    gofmt -l services/job-posting
    go vet ./services/job-posting/...
    go vet -tags=integration ./services/job-posting/...
    go test -count=1 ./services/job-posting/...
    go test -count=1 -tags=integration ./services/job-posting/...

| Check | Result |
|---|---|
| Formatting and static checks | Pass, no output |
| Unit tests | Pass, all packages `ok` |
| Integration tests | Pass, all packages `ok` (postgres package 18.3 s, real PostgreSQL 17 and RabbitMQ 4.1 containers) |
| Compose | job-posting rebuilt from the branch; database at version 3, no migration added; consumer ready on `job-posting.moderation` |
| End to end (JT90-14) | Pass. Job `d5b82017-4fdd-4a59-a4be-2e48bcc6c7fb` read as `JOB_STATUS_REMOVED` 2.1 s after the removal. Outbox: `job.published` `57df01e0-7b2b-4764-93f1-339ebf370669` and `job.closed` `b7771e5c-4b4f-40ed-b3b3-7b308ee795da` (created 10:00:36.118 UTC, published 10:00:37.119 UTC); exactly one `job.closed` message for the job received on a check queue bound to `job.closed` on `jobtrack.events`, producer `job-posting`; job-posting logged `outbox events published` with count 1. A second removal was refused by moderation (HTTP 400, code 9) and added no message or outbox row |
| CI on the pull request | Green, all checks passed on PR #69 (https://github.com/jobtrack-cst/jobtrack/pull/69) |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| AC1: exactly one `job.closed` in the same transaction | Met | JT90-05, JT90-11, JT90-12, JT90-14 |
| AC2: none for an unknown job, an already removed job or a redelivered `event_id` | Met | JT90-06 to JT90-10, JT90-12, JT90-14 |
| AC3: unit and integration tests; consumer tests load the fixture | Met | JT90-01 to JT90-13 |
| AC4: end to end in Docker Compose | Met | JT90-14 |
| AC5: Section 16 item removed | Met | AGENTS.md diff in the PR |

## Not covered

- Jobs removed before JT-90 (for example `12c406e5-d54b-4439-9bf1-df2749e5b125` from the JT-87 check) never get `job.closed`; not backfilled because the data is synthetic.
- How `job-search` and `saved-jobs` handle `job.closed` was not checked here; they are Sangay's consumers and were not running in the Compose check.
- Invalid `moderation.job_removed` events are checked to return an error; that they reach `job-posting.moderation.dead` relies on `events.Permanent` and the shared `events.Consume` tests (JT-56).
