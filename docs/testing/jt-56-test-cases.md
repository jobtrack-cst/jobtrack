# JT-56 Employer verification events test cases

Traceability: [JT-56](https://jobtrack-cst.atlassian.net/browse/JT-56). Owner: Jigden Shakya. Verification date: 12 November 2026.

## Scope

Publishing `employer.verified` and `employer.verification_rejected` from `company-verification` after the decision is committed, and consuming both in `employer` to update the company profile's verification status and rejection reason. Also covers the shared consumer added to `pkg/events` and the shared event fixtures in `tests/contract/fixtures`. No `.proto` contract or `events.proto` change.

Acceptance criteria used in the matrix:

- AC1: company-verification publishes both events through `pkg/events` with the Section 9 envelope, only after the decision is committed
- AC2: employer consumes both events from `employer.verification` and updates status and reason
- AC3: the employer consumer is idempotent by `event_id`
- AC4: unit tests with gomock; integration tests with Testcontainers RabbitMQ and PostgreSQL
- AC5: consumer tests use shared JSON fixtures in `tests/contract`
- AC6: no changes to `.proto` contracts or `events.proto`
- S1: runs in Docker Compose (shared Sprint 2 criterion)

Rules agreed for JT-56 (AGENTS.md Section 15):

- Company-verification stores the decision and its event in one transaction (`outbox_events`); a relay publishes unpublished events oldest first every second and stops at the first failure. `occurred_at` is the decision time.
- The shared consumer uses the JT-61 policy: at most 4 attempts with 1, 2 and 4 s backoff, then `<queue>.dead`; `events.Permanent` errors go to the dead queue at once.
- Employer claims the `event_id` in `processed_events` and stores the decision in `employer_verifications` in one transaction. Only a later `occurred_at` replaces the stored decision; an equal or older one is stale. An approval clears the reason.
- A decision for an employer without a profile is kept and copied by `Create`; both take a per-employer advisory lock.
- Event checks: UUID `event_id` and `employer_id`, `occurred_at` set, data type matching the routing key, rejection reason 1 to 500 characters after trimming (no NUL or invalid UTF-8), no reason on an approval.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT56-01 | AC5 | Both fixtures decode with `events.UnmarshalEnvelope`, file name equals routing key, producer, data type, round trip; rejection fixture older than approval | Equivalence partitioning | Both decode and round trip unchanged; order holds | Contract, `tests/contract` | Pass |
| JT56-02 | AC1 | `DecisionEvent` for approved, rejected, pending and unspecified requests | Equivalence partitioning | `employer.verified`; `employer.verification_rejected` with reason; no event for undecided requests; `occurred_at` is the decision time | Unit, `company-verification/internal/app` | Pass |
| JT56-03 | AC1 | Approve and Reject pass the event to `Decide`; two decisions get different event IDs; trimmed reason in the event | Equivalence partitioning | Correct routing key and data; new UUID per event | Unit, `internal/app` (gomock) | Pass |
| JT56-04 | AC1 | Decision that does not commit: already decided, unknown request, second approval (unique index), invalid event, duplicate event ID | Equivalence partitioning | No outbox row; request still PENDING; mapped errors unchanged | Integration, `company-verification/internal/adapters/postgres` | Pass |
| JT56-05 | AC1 | Two admins approve and reject the same request at once | Concurrency | Exactly one decision and one outbox event | Integration | Pass |
| JT56-06 | AC1 | Relay `PublishPending`: two events, none, read error, publish failure, mark failure, second publish failure | State transition | Oldest first, each marked after the broker confirms; stops at the first failure; earlier events stay published | Unit, `internal/app` (gomock) | Pass |
| JT56-07 | AC1 | Relay loop: failure then success, full batch, servers stop, context ends | State transition | Failure logged and retried; full batch read again at once; loop stops with the servers | Unit, `company-verification/cmd/server` | Pass |
| JT56-08 | AC1 | Outbox `Pending` order and limit; `MarkPublished` twice | Boundary value | Oldest first, limit 2 respected, limit 0 refused; first `published_at` kept | Integration | Pass |
| JT56-09 | AC1, AC4 | Approve and reject, run the relay against real RabbitMQ, run it again | End to end (component) | Both envelopes delivered once, oldest first, with their routing keys; nothing sent on the second run | Integration, Testcontainers PostgreSQL and RabbitMQ | Pass |
| JT56-10 | AC4 | Shared consumer configuration: queue name, bindings, handler, logger | Equivalence partitioning | `<service>.<purpose>` in kebab-case only; no wildcard or dead queue names; required fields enforced | Unit, `pkg/events` | Pass |
| JT56-11 | AC4 | Settling one delivery: success, malformed body, invalid count header, permanent error, transient error on attempts 1, 3 and 4, shutdown | Boundary value | Ack; dead queue; retry on attempts 1 to 3; dead queue on attempt 4; requeue on shutdown; settled exactly once | Unit, `pkg/events` | Pass |
| JT56-12 | AC4 | Shared consumer against RabbitMQ: success, permanent, flaky (fails twice), always failing, malformed | Boundary value | Handled 1, 1, 3 and 4 times; permanent, always failing and malformed in the dead queue; source queue empty | Integration, `pkg/events` | Pass |
| JT56-13 | AC2 | `NewVerificationDecision`: reason of 0, 1, 500 and 501 characters, NUL, invalid UTF-8, approval with reason, unverified and unspecified status, non-UUID IDs, zero time | Boundary value, equivalence partitioning | Valid at 1 and 500; all others `ErrInvalidVerificationEvent`; IDs lower-cased, time in UTC | Unit, `employer/internal/domain` | Pass |
| JT56-14 | AC2, AC5 | Handler applies both shared fixtures | Equivalence partitioning | Exact decision passed to the repository; outcome logged with `event_id` | Unit, `employer/internal/adapters/rabbitmq` (gomock) | Pass |
| JT56-15 | AC2 | Invalid events: unknown routing key, data not matching the key, missing data, missing `occurred_at`, non-UUID IDs, rejection without reason; database error | Equivalence partitioning | Invalid events `events.Permanent` without touching the database; database error retried | Unit, `internal/adapters/rabbitmq` | Pass |
| JT56-16 | AC2 | Rejection then later approval on an existing profile; edit the profile afterwards | State transition | REJECTED with reason, then VERIFIED with empty reason; `updated_at` set, `created_at` unchanged; editing keeps the status | Integration, `employer/internal/adapters/postgres` | Pass |
| JT56-17 | AC3 | Same `event_id` applied twice | Equivalence partitioning | Second is a duplicate; nothing changes | Unit, `internal/app`; integration | Pass |
| JT56-18 | AC2, AC3 | Older rejection after a newer approval, its redelivery, and a decision with the same time | State transition | Stale and marked processed, status stays VERIFIED; redelivery is a duplicate; equal time is stale | Integration | Pass |
| JT56-19 | AC2 | Rejection for an employer with no profile, then `Create` | State transition | Stored without a profile; no profile invented; `Create` returns REJECTED with the reason; other employers start UNVERIFIED; second `Create` `ALREADY_EXISTS` | Integration | Pass |
| JT56-20 | AC3 | Write that fails (reason of 501 characters past the domain, unverified status) | Boundary value | Error with driver details hidden; `event_id` not marked processed | Integration | Pass |
| JT56-21 | AC2 | 20 employers with `Create` and approval at the same time | Concurrency | Every profile ends VERIFIED | Integration | Pass |
| JT56-22 | AC2, AC3, AC4, AC5 | Shared fixtures through real RabbitMQ: newer approval, older rejection, approval again | End to end (component) | Profile VERIFIED with empty reason; 2 events processed; source and dead queues empty | Integration, Testcontainers PostgreSQL and RabbitMQ | Pass |
| JT56-23 | S1 | Employer consumer next to the servers: servers stop, server error, consumer failure, shutdown | State transition | Consumer stopped and waited for; consumer failure stops the servers with its error | Unit, `employer/cmd/server` | Pass |
| JT56-24 | AC1, AC2, S1 | Docker Compose through the gateway: employer registers, creates a profile, requests verification; admin approves. Second employer: admin rejects | Manual end to end | Profile UNVERIFIED, then VERIFIED; second profile REJECTED with "Synthetic: licence expired"; no manual SQL | Manual, Compose (`jt56-compose-check.sh`) | Pass |
| JT56-25 | AC6 | `git diff origin/main --stat -- proto/ gen/` | Review | No changes | Review | Pass |

## How to run

- Unit: `go test ./pkg/events/... ./tests/contract/... ./services/company-verification/... ./services/employer/...`
- Integration: the same packages with `-tags=integration` (pull `postgres:18-alpine` and `rabbitmq:4.1-alpine` first)
- Manual: start `postgres rabbitmq auth employer company-verification gateway` in Compose, then run the check script from the JT-56 PR

## Not covered

- Outbox rows are kept after publishing; there is no cleanup (AGENTS.md Section 16).
- Running more than one company-verification replica (two relays could publish an event twice; consumers ignore it by `event_id`).
- Broker failures during queue declaration in the shared consumer, which would need a mock AMQP layer; they are reconnected and logged.
