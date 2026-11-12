# JT-59 Resume outbox persistence test cases

Traceability: JT-59. Service owner: Kinley. Verification date: 12 November 2026.

This first increment covers atomic persistence for AC1 only. RabbitMQ dispatch,
validation-result consumption and the three shared event fixtures remain future
JT-59 increments. Existing GetResumeFile security wiring is preserved.

| Case | Requirement | Technique | Expected result | Automated location | Result |
|---|---|---|---|---|---|
| JT59-01 | AC1 | Equivalence partitioning | Upload submits a PENDING resume and matching typed envelope with a separate UUID, producer resume and UTC time | internal/app/resumes_test.go: TestResumes_Upload | Pass |
| JT59-02 | AC1 | Boundary values / equivalence partitioning | Empty content/name, oversized content, non-PDF content and wrong MIME type never call persistence | internal/app/resumes_test.go: TestResumes_Upload | Pass |
| JT59-03 | AC1 | Authorization | Employer/admin denied before persistence | internal/app/resumes_test.go: TestResumes_Upload | Pass |
| JT59-04 | AC1 | Equivalence partitioning | Both records commit; original envelope round-trips unchanged; published_at is NULL | internal/adapters/postgres/outbox_integration_test.go: TestResumes_OutboxAtomicity | Pass |
| JT59-05 | AC1 | Equivalence partitioning | Resume insert failure leaves neither new record | internal/adapters/postgres/outbox_integration_test.go: TestResumes_OutboxAtomicity | Pass |
| JT59-06 | AC1 | Equivalence partitioning | Duplicate outbox event ID rolls back the preceding resume insert and preserves existing records | internal/adapters/postgres/outbox_integration_test.go: TestResumes_OutboxAtomicity | Pass |
| JT59-07 | AC1 | Equivalence partitioning | Mismatched applicant IDs rejected without writes | internal/adapters/postgres/outbox_integration_test.go: TestResumes_OutboxAtomicity | Pass |
| JT59-08 | AC1 | Boundary values / equivalence partitioning | Invalid uploads through the real app and repository leave both row counts unchanged | internal/adapters/postgres/outbox_integration_test.go: TestResumes_OutboxAtomicity | Pass |
| JT59-09 | AC3 regression | Authorization | Existing internal interceptor restricts GetResumeFile to resume-validator with valid token | cmd/server/register_test.go: TestServe_InternalInterceptorGuardsOnlyTheInternalService | Pass |

Verified on the uncommitted JT-59-resume-events-security working tree:

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go generate ./services/resume/internal/ports/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/postgres/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet ./services/resume/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./services/resume/...
```

All commands above passed. The temporary GOCACHE avoids an unavailable entry in
the default cache. Unit tests required execution outside the sandbox to bind local
ports; integration tests required Docker socket access. No RabbitMQ integration
suite, full monorepo suite, CI or coverage run was performed for this increment.
JT-59 is not complete.

## Durable claims and retry-state increment

Only persistence is implemented here. No dispatcher is started, no events are
published, and main.go, RabbitMQ transport and deployment wiring are unchanged.

`OutboxRepository.Claim` commits a short transaction using database
`statement_timestamp()`, ordered candidates and `FOR UPDATE SKIP LOCKED`.
It returns the original envelope text and a fresh ownership UUID. The adapter
borrows the existing Resume pool and uses a two-second database operation context;
no transaction or connection is retained after return. Durations must be whole
microseconds (positive lease, nonnegative retry delay).

Completion and retry updates require the current token and an unpublished row.
Stale operations return false without changing anything. Expiry allows reclamation;
an expired token may still complete if no worker has replaced it. Concurrent
reclamation/completion is serialized by PostgreSQL row updates. Retry with zero
delay releases immediately; positive delay schedules eligibility using database
time. Payload and event ID are never rewritten. The existing pending index is
retained; schema migration 00004 adds nullable state and a claim/lease pairing
constraint without rewriting migration 00003 or existing event data.

| Case | Requirement | Technique | Expected result | Automated location | Result |
|---|---|---|---|---|---|
| JT59-10 | AC1 persistence foundation | Boundary values | Reject zero/negative/submicrosecond lease, negative/fractional retry, malformed IDs before DB access | domain/outgoing_event_test.go; adapters/postgres/outbox_test.go | Pass |
| JT59-11 | AC1 persistence foundation | State transition | Stable claim order, DB-relative lease, committed claim has no remaining row lock; active claims excluded | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationLifecycle | Pass |
| JT59-12 | AC1 persistence foundation | State transition | Expired lease reclaimed by another repository instance with fresh token and identical payload/event ID | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationLifecycle | Pass |
| JT59-13 | AC1 persistence foundation | Equivalence partitioning | Stale completion/retry and unknown event are harmless; current claim survives | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationLifecycle | Pass |
| JT59-14 | AC1 persistence foundation | State transition | Future retry excluded, due retry eligible, zero delay immediately eligible across repository instances | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationLifecycle | Pass |
| JT59-15 | AC1 persistence foundation | State transition | Completion sets publication time, clears claim/retry state; repeated completion/retry are harmless; published rows excluded | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationLifecycle | Pass |
| JT59-16 | AC1 persistence foundation | Equivalence partitioning | Eight concurrent claims skip a separately locked candidate and uniquely claim the seven remaining rows | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationConcurrencyAndRollback | Pass |
| JT59-17 | AC1 persistence foundation | Equivalence partitioning | Forced claim UPDATE constraint failure rolls back; connection/lock released, no partial ownership, retry succeeds | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationConcurrencyAndRollback | Pass |
| JT59-18 | AC1 persistence foundation | State transition | DB rejects unpaired claim fields; canceled claim returns cancellation; expired unreplaced token can complete | adapters/postgres/claims_integration_test.go: TestOutbox_IntegrationConcurrencyAndRollback | Pass |
| JT59-19 | AC1 regression | Equivalence partitioning | Duplicate-ID subtest seeds its own conflicting event and passes when selected alone | adapters/postgres/outbox_integration_test.go: TestResumes_OutboxAtomicity/outbox_duplicate_event_ID_rolls_back_inserted_resume | Pass |

Actual verification on the uncommitted branch for this increment (12 November 2026):

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go generate ./services/resume/internal/ports/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/internal/domain/... ./services/resume/internal/adapters/postgres/... ./services/resume/internal/app/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/postgres/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/postgres -run '^TestResumes_OutboxAtomicity$/^outbox_duplicate_event_ID_rolls_back_inserted_resume$' -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./services/resume/...
git diff --check
```

All commands above passed. gofmt ran on the new Go files and changed atomicity
test. PostgreSQL tests used disposable Testcontainers databases; constraint-failure
injection modifies only those databases. Docker/local-port tests ran with sandbox
escalation. No RabbitMQ integration tests, full monorepo suite, coverage or CI run
was performed in this increment. Migration Down/Up cycling was not tested.

### Outstanding blocker: routing safety

Do not implement or enable dispatch until routing safety is resolved. The shared
publisher uses nonmandatory RabbitMQ publication; broker confirmation does not
prove that a matching durable queue received a message. Compose currently places
resume-validator-forwarder under the optional `functions` profile, so its queue
and binding may not exist when Resume is running. A later increment must establish
and verify safe routing before allowing pending events to be marked published.
This increment does not alter queues, transport, functions or Compose.

Remaining JT-59 work: resolve routing, implement dispatcher/recovery/backoff and
lifecycle wiring, validate confirmed delivery and duplicate-delivery recovery,
consume validation results idempotently, add the three shared JSON fixtures, and
complete end-to-end deployment verification. MarkPublished is only a persistence
operation: its future caller must enforce confirmation/routing prerequisites.
JT-59 remains incomplete.

## Independent topology provisioning increment

The earlier routing-blocker section records the preceding increment. Owner/DevOps
approval now permits coordinated JT-61 topology provisioning under JT-59. Resume
startup provisions the topology after migrations and before publisher/listeners,
using the same declarations as the forwarder. Dispatch remains disabled. See
[deployment details](../deployment/jt-59-topology.md) for startup, compatibility and
remaining runtime-routing limits. Existing JT-61 historical results are unchanged;
JT-62 owns the real validator and end-to-end function verification.

| Case | Requirement | Technique | Expected result | Automated location | Result |
|---|---|---|---|---|---|
| JT59-20 | AC1 routing prerequisite | Equivalence partitioning | One shared declaration preserves exact JT-61 exchange, quorum queue, DLX, overflow and binding arguments | pkg/forwarder/topology_test.go | Pass |
| JT59-21 | AC1 routing prerequisite | Equivalence partitioning | Invalid config rejected before declarations; every failing declaration stops subsequent operations with non-secret error | pkg/forwarder/topology_test.go | Pass |
| JT59-22 | AC1 routing prerequisite | State transition | Fresh broker provisions both durable queues and binding without forwarder/validator; repeat initialization retains source and DLQ messages | pkg/forwarder/topology_integration_test.go | Pass |
| JT59-23 | AC1 routing prerequisite | Equivalence partitioning | Classic source/DLQ and incompatible quorum arguments fail closed and preserve pre-existing messages | pkg/forwarder/topology_integration_test.go | Pass |
| JT59-24 | AC1 routing prerequisite | Equivalence partitioning | Incompatible exchange fails without replacing it | pkg/forwarder/topology_integration_test.go | Pass |
| JT59-25 | AC1 routing prerequisite | State transition | Migrations precede provisioning, which precedes publisher/listeners; migration or topology failure prevents later startup | services/resume/cmd/server/topology_test.go | Pass |
| JT59-26 | AC1 routing prerequisite | State transition | Resume adapter provisions twice with functions absent; normal publication in the isolated test reaches the named validator inbox | services/resume/internal/adapters/rabbitmq/publisher_integration_test.go | Pass |

Verification commands and outcomes for this increment are recorded below after
execution. No dispatcher, validator, validation consumer, fixture or dependency
was added. No live queues/data were deleted or migrated. Synthetic messages are
only consumed/acknowledged in disposable test brokers.

Actual results on the uncommitted branch (12 November 2026):

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./pkg/forwarder ./cmd/rabbitmq-forwarder ./services/resume/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./pkg/forwarder -run '^TestTopology_' -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./pkg/forwarder -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/rabbitmq -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./pkg/forwarder ./cmd/rabbitmq-forwarder ./services/resume/...
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml config --quiet
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml --profile functions config --quiet
git diff --check
```

All commands passed. The full forwarder tagged suite took 55.238 seconds and
included the final incompatible-exchange test plus existing JT-61 delivery,
restart, crash-budget and dead-letter tests. Resume RabbitMQ tests took 5.160
seconds. New/changed Go files were formatted with gofmt. Unit/runtime tests used
sandbox escalation for local ports and Docker. Parsed Compose JSON assertions
also passed for profile service membership without printing secrets.

The profile checks are configuration-only; the application Compose stack and k3d
were not started. Fresh broker, repeat provisioning, incompatibility and retention
were actual RabbitMQ Testcontainers runtime tests. No PostgreSQL integration,
full monorepo, race, coverage or CI suite was rerun for this topology increment.
JT-59 remains incomplete and JT-62 verification was not attempted.

## Transactional outbox dispatcher increment

This increment enables Resume's one synchronous application worker only after the
existing migration/topology/publisher startup gates. Earlier sections record the
state and results of earlier increments. No shared package, topology, protobuf,
function, Compose or dependency change was made for this dispatcher increment.

| Case | Requirement | Technique | Expected result | Automated location | Result |
|---|---|---|---|---|---|
| JT59-27 | AC1 | State transition | Claim committed before publication; successful confirmation precedes conditional completion | internal/app/outbox_test.go: TestOutboxDispatcher_Step; adapters/postgres/dispatcher_integration_test.go | Pass |
| JT59-28 | AC1 | Equivalence partitioning | Empty polls, claim/complete/retry DB errors, stale completion/retry tokens handled without discarding events | internal/app/outbox_test.go: TestOutboxDispatcher_Step | Pass |
| JT59-29 | AC1 | Equivalence partitioning | Malformed envelope, wrong envelope identity/type or invalid payload IDs rescheduled without publishing | internal/app/outbox_test.go: TestOutboxDispatcher_Step | Pass |
| JT59-30 | AC1 | Boundary values | Backoff increases 1/2/4/8/16/30 seconds, caps at 30 and resets on successful completion | internal/app/outbox_test.go: TestOutboxDispatcher_BackoffCapsAndResets | Pass |
| JT59-31 | AC1 | State transition | Cancellation before claim, after claim, during publish and during idle/error waiting leaves recoverable state | internal/app/outbox_test.go; adapters/postgres/dispatcher_integration_test.go | Pass |
| JT59-32 | AC1 | State transition | Returned transport errors close old session before reconstruction; reconnect failures retry, cleanup failure is fatal, errors are sanitized | cmd/server/outbox_test.go: TestRecoveringPublisher_ReconnectsOnlyAfterClosingFailedSession | Pass |
| JT59-33 | AC1 lifecycle | State transition | Worker failure, server failure and cancellation cancel/join both before owners close resources | cmd/server/outbox_test.go: TestRunWithDispatcher_JoinsBothBeforeReturning | Pass |
| JT59-34 | AC1 | State transition | Real Postgres/RabbitMQ: publication retained in resume-validator.events with zero consumers; published_at absent until confirmed Publish returns | adapters/postgres/dispatcher_integration_test.go | Pass |
| JT59-35 | AC1 | State transition | Closed real publisher, simulated uncertain confirmation, simulated post-confirmation DB failure, cancellation after broker acceptance leave unpublished state; new worker/publisher recovers original identity | adapters/postgres/dispatcher_integration_test.go | Pass |
| JT59-36 | AC1 | State transition | Two workers claim distinct rows; expired row reclaimed and completed; original late worker cannot complete new ownership; duplicate deliveries keep identical event_id | adapters/postgres/dispatcher_integration_test.go | Pass |
| JT59-37 | AC1 | Equivalence partitioning | Fatal cleanup stops Run, waits cancel promptly and no per-attempt goroutines accumulate | internal/app/outbox_test.go; cmd/server/outbox_test.go | Pass |

Actual commands/results on the uncommitted JT-59 branch, 12 November 2026:

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go generate ./services/resume/internal/ports/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/internal/app/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/postgres -run '^TestOutboxDispatcher_' -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/postgres/... ./services/resume/internal/adapters/rabbitmq/... -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -race ./services/resume/internal/app/... ./services/resume/cmd/server/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./services/resume/...
git diff --check
```

All commands passed. Targeted dispatcher integration took 6.396 seconds. Combined
PostgreSQL and RabbitMQ suites passed in 20.932 and 12.643 seconds respectively.
Race-enabled app/server tests passed. gofmt ran on touched Go files. Docker and
local-port tests used sandbox escalation. Only disposable brokers/databases were
used; no production queues or data were deleted or migrated.

The uncertain-confirmation and post-confirmation DB failure windows are deliberately
injected around real broker acceptance; this is not a claim of reproducing a real
network partition or process kill. Lease expiry/next-attempt times are advanced in
the disposable DB using database time to avoid long sleeps. Connection reconstruction
and failed cleanup use controlled unit-test sessions; real closed-connection failure
and new-session recovery are covered in integration.

No application Compose end-to-end run, real validator/function delivery, monorepo
suite, CI or coverage run was performed for this increment. No hard AMQP cancellation
or bounded Close guarantee is claimed. Maintained topology is the approved invariant;
later administrative deletion/unbinding can defeat nonmandatory routing and requires
coordinated maintenance. Remaining JT-59 work: validation-result consumers and durable
event-ID deduplication, the three event fixtures, complete verification and eventual
PR/CI workflow. JT-62 owns the real validator. JT-59 is not complete.

## Corrective increment: malformed-event progress

The dispatcher originally used the persisted event retry delay as its global wait.
An oldest malformed envelope therefore became eligible again at every next poll,
starving newer valid events. Invalid saved envelopes now receive a 30-second durable
retry schedule without advancing the transient worker-backoff level. After a
successful reschedule, Step returns zero wait so Run promptly claims another
eligible event. This applies to all existing event-specific saved-envelope failures:
malformed JSON/protobuf, mismatched identity/type/producer and invalid payload IDs.

Transient publication and database failures retain worker-wide exponential backoff
(1/2/4/8/16/30 seconds). A failed malformed-event reschedule uses that DB backoff;
a stale reschedule token uses the ordinary idle wait. No eligible event still uses
the cancellation-aware one-second idle wait. Original IDs, payloads, leases and
conditional claim-token updates are unchanged; malformed rows are never published,
marked published, deleted or acknowledged. This supersedes the earlier description
that the worker waits for every successfully scheduled malformed-event retry.

| Case | Requirement | Technique | Expected result | Automated location | Result |
|---|---|---|---|---|---|
| JT59-38 | AC1 progress regression | State transition | Run defers older malformed event, immediately claims/publishes newer valid event and cancels cleanly | internal/app/outbox_test.go: TestOutboxDispatcher_MalformedEventDoesNotStarveValidEvent | Pass |
| JT59-39 | AC1 retry regression | Equivalence partitioning | Event schedule and global wait differ; successful deferral does not alter worker backoff; DB errors back off, stale tokens idle, empty polls idle | internal/app/outbox_test.go: TestOutboxDispatcher_EventDeferralAndWorkerBackoffAreSeparate | Pass |
| JT59-40 | AC1 progress regression | State transition | Real PostgreSQL/RabbitMQ: newer event reaches resume-validator.events while malformed older row stays unpublished with future next_attempt_at, unchanged payload and released claim | internal/adapters/postgres/dispatcher_integration_test.go: TestOutboxDispatcher_IntegrationMalformedDoesNotStarve | Pass |

Actual corrective verification on the uncommitted branch, 12 November 2026:

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/internal/app/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/postgres -run '^TestOutboxDispatcher_' -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -race ./services/resume/internal/app/... ./services/resume/cmd/server/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./services/resume/...
git diff --check
```

All passed. App unit tests: 1.315 seconds; dispatcher PostgreSQL/RabbitMQ integration
suite: 11.820 seconds; race-enabled app/server tests: 1.411/1.507 seconds. gofmt ran
on the three modified Go files. Runtime tests used disposable databases/brokers
with sandbox escalation; the regression injected incomplete-envelope JSON into only
its disposable DB while keeping envelope ID/type constraints. It verified queue
receipt, publication state, future retry eligibility and no malformed publication.

No migrations, shared packages, topology, Compose, security or dependency changes
were made. No application Compose end-to-end, real validator, full monorepo or CI
verification was attempted. The previously documented nonmandatory-routing,
duplicate-delivery and potentially blocking AMQP cleanup limitations remain.
JT-59 remains incomplete.

## Atomic validation-result persistence increment

This increment adds only typed domain/application processing and PostgreSQL
persistence. It does not parse protobuf envelopes, start a RabbitMQ consumer,
settle deliveries, or alter the existing dispatcher/topology/security. The inspected
ResumeValidated payload contains resume_id/applicant_id; ResumeRejected adds reason.
Neither contract supplies a rejection-reason length/disposition policy.

Migration 00005 creates Resume-owned processed_events(event_id UUID PRIMARY KEY,
processed_at TIMESTAMPTZ DEFAULT now()), following analytics/audit-log patterns.
ApplyValidation claims the ID with ON CONFLICT DO NOTHING, locks the target resume,
checks its applicant, and conditionally updates only PENDING status in the same
transaction. Exactly one successfully committed final result wins. Distinct events
for an already-final matching resume are recorded but ignored; duplicates return
ValidationDuplicate. Final status, rejection reason and updated_at are untouched
by later results. Outcomes distinguish applied, duplicate and already-final.

Rejected reasons are stored verbatim; validated results use an empty reason,
clearing any previous pending reason. The application supplies processing time via
the existing Clock port in UTC; PostgreSQL retains its existing microsecond precision.
processed_at uses database time. PostgreSQL transaction atomicity prevents a
partially committed processed-event claim and validation status update. A
server-rejected transaction or confirmed rollback leaves neither effect committed.
A lost connection while receiving the COMMIT result leaves the outcome uncertain:
the transaction may have committed completely or not committed. Retrying the SAME
event_id safely resolves this uncertainty through durable idempotency. Existing
integration tests cover server-rejected commit, not simulated loss of the commit
response; that network-failure scenario has not been verified.
Unknown resumes/applicant mismatches
return distinguishable errors without marking IDs processed. This preserves data
and leaves eventual broker disposition open. No consumer is wired to this use case.

| Case | Requirement | Technique | Expected result | Automated location | Result |
|---|---|---|---|---|---|
| JT59-41 | AC2 domain | State transition | Only PENDING accepts VALIDATED/REJECTED; both final states reject further mutation | internal/domain/validation_test.go: TestValidationStatus_FirstFinalResultWins | Pass |
| JT59-42 | AC2 input | Equivalence partitioning | Normalize UUIDs, reject malformed identifiers/unsupported target states, enforce protobuf/Postgres text compatibility; preserve reason verbatim without invented length limit | internal/domain/validation_test.go: TestNewValidationResult_Input | Pass |
| JT59-43 | AC2 application | Equivalence partitioning | Apply delegates normalized typed result and UTC Clock time to repository; errors/outcomes pass through; invalid input never accesses repository | internal/app/validation_test.go: TestValidationResults_Apply | Pass |
| JT59-44 | AC2 persistence | State transition | PENDING->VALIDATED/REJECTED commits once with processed ID; rejection reason stored, validated reason cleared; timestamps UTC | internal/adapters/postgres/validation_integration_test.go: TestValidation_IntegrationAtomicResults | Pass |
| JT59-45 | AC2 idempotency | State transition | Replay through new adapter instance returns duplicate; same/different later results never overwrite final state/reason/time | internal/adapters/postgres/validation_integration_test.go | Pass |
| JT59-46 | AC2 concurrency | State transition | Twelve concurrent distinct/conflicting event IDs cause one applied transition and eleven ignored final results; all IDs recorded | internal/adapters/postgres/validation_integration_test.go | Pass |
| JT59-47 | AC2 concurrency | Equivalence partitioning | Twelve concurrent deliveries of one ID yield one applied result and eleven duplicates | internal/adapters/postgres/validation_integration_test.go | Pass |
| JT59-48 | AC2 atomicity | Equivalence partitioning | Forced UPDATE constraint failure rolls back processed ID; same ID succeeds after test constraint removed | internal/adapters/postgres/validation_integration_test.go | Pass |
| JT59-49 | AC2 atomicity | Equivalence partitioning | Deferred test trigger causes COMMIT failure after both writes; both roll back; same ID can be retried | internal/adapters/postgres/validation_integration_test.go | Pass |
| JT59-50 | AC2 input safety | Equivalence partitioning | Unknown/mismatched targets, invalid UUID/text and cancellation commit nothing | internal/adapters/postgres/validation_integration_test.go | Pass |

Actual verification on the uncommitted JT-59 branch, 12 November 2026:

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go generate ./services/resume/internal/ports/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/internal/domain/... ./services/resume/internal/app/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./services/resume/internal/adapters/postgres -run '^TestValidation_' -count=1
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -race ./services/resume/internal/domain/... ./services/resume/internal/app/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./services/resume/...
git diff --check
```

All passed. Tagged PostgreSQL tests took 2.207 seconds. Race-enabled domain/app tests
passed in 1.441/1.798 seconds. gofmt ran on the new Go files. New mock generated
using the existing go tool mockgen directive. Docker and local-port tests used
sandbox escalation. Fault injection affects only disposable Testcontainers databases;
no live service data, queues or schemas were changed.

### Decisions required before consumer connection

- Unknown resume/applicant mismatch: choose retry, terminal handling or another
  reviewed disposition. Current error and rollback behavior deliberately records
  neither as processed and does not imply any acknowledgment decision.
- Rejection reasons: define any required/nonblank/trim/maximum-length policy. The
  existing TEXT schema has no maximum; this increment preserves strings, including
  empty/long strings, without truncation. Invalid UTF-8 and NUL are refused because
  protobuf/PostgreSQL cannot represent/store them consistently, not as a new text
  business rule. Consumer poison handling for these failures remains undecided.
- Define acknowledgment, retry/backoff and dead-letter policy for malformed and
  failed validation deliveries. The JT-61 function-forwarder policy is not adopted
  for this service consumer.

No RabbitMQ validation consumer, fixtures, real validator, application Compose
end-to-end, full monorepo integration suite or CI run was performed. The existing
outbox/dispatcher/topology implementation and migrations 00003/00004 are unchanged.
JT-59 remains incomplete; consumer wiring requires the decisions above, shared
fixtures, full verification and eventual PR/CI review. JT-62 owns the validator.

### Validation RabbitMQ consumer and contract fixtures (12 November 2026)

This increment supersedes the preceding open consumer-policy decisions; the prior
verification evidence above remains historical. Approved behavior is now: unknown
resume and transient/uncertain database results retry the original event ID;
applicant mismatch and malformed/invalid content dead-letter immediately. Rejected
reasons are trimmed, required, 1–500 Unicode characters, valid UTF-8 and NUL-free;
they are never truncated. Migration 00006 adds a rejection-reason CHECK and fails
on incompatible historical data without repairing it automatically.

Resume provisions its own `resume.validation` durable quorum queue and
`resume.validation.dead` durable quorum queue before publisher/listener startup,
independently of the functions profile. Only `resume.validated` and
`resume.rejected` are bound. The source delivery limit is 3, with at-least-once
broker dead-lettering, default-exchange dead-letter routing and reject-publish
overflow. There are at most four broker deliveries including the original;
reconnects/cancellations consume this durable budget. Redelivery waits are 1/2/4
seconds, with prefetch 1. Failed processing uses basic.reject(requeue=true), the
fourth failure or invalid content uses basic.nack(requeue=false). Missing counts
are accepted only for an initial non-redelivered delivery; malformed/negative or
missing redelivery counts fail closed to the DLQ. Raw dead-letter bodies are
preserved. No automatic replay, deletion or queue migration occurs.

| Case | Requirement / technique | Expected result | Automated location | Result |
|---|---|---|---|---|
| JT59-51 | AC2 state transitions/idempotency | Validated/rejected persist; reason trimmed; duplicates and different final conflicts ACK only after successful persistence | rabbitmq/validation_consumer_test.go; postgres/consumer_integration_test.go | Pass |
| JT59-52 | AC2 concurrency | Two actual consumers processing conflicting events preserve one final status; later replays preserve status/reason/time | postgres/consumer_integration_test.go | Pass |
| JT59-53 | AC2 retry boundary | Unknown resume receives four attempts then retained identical dead letter, no processed ID | postgres/consumer_integration_test.go | Pass |
| JT59-54 | AC2 invalid partitions | Applicant mismatch/malformed body immediately retained in DLQ without status update or processed ID | postgres/consumer_integration_test.go | Pass |
| JT59-55 | AC2 recovery | Injected transient database failure retries original ID and then commits once | postgres/consumer_integration_test.go | Pass |
| JT59-56 | AC2 restart boundary | Four interrupted sessions consume broker budget across reconnects and retain original body in DLQ | postgres/consumer_integration_test.go | Pass |
| JT59-57 | AC2 transport safety | Wrong routing/type/producer/Any payload, bad count metadata, invalid reason fail closed; settlement errors end session without second settlement | rabbitmq/validation_consumer_test.go | Pass |
| JT59-58 | AC2 input boundaries | Reasons 1 and 500 Unicode characters accepted; 0/501, invalid UTF-8/NUL refused; whitespace trimmed | domain/validation_test.go | Pass |
| JT59-59 | AC2 topology | Repeat provisioning preserves messages, exactly two bindings; incompatible existing classic source/dead queues fail without deleting retained data | postgres/consumer_integration_test.go; rabbitmq/validation_consumer_test.go | Pass |
| JT59-60 | AC4 contract | All three shared protojson fixtures decode exact typed payloads/producers and round-trip; consumer reads result fixtures | tests/contract/resume_events_test.go; postgres/consumer_integration_test.go | Pass |
| JT59-61 | Lifecycle / regression | Cancellation interrupts waits; fixed consumer/dispatcher/server workers join; existing outbox, starvation, leases, rollback and JT-44 tests remain green | cmd/server/outbox_test.go; existing Resume suites | Pass |

The wrong-exchange production guard in `rabbitmq/validation_consumer.go` was
code-inspected; no dedicated wrong-exchange unit case was executed.

Paths in this table are beneath `services/resume/internal/adapters/` unless fully
qualified or explicitly domain/cmd paths. Actual verification commands:

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./services/resume/... ./tests/contract/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test ./...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -race ./services/resume/... ./tests/contract/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -json -tags=integration ./services/resume/internal/adapters/postgres/... ./services/resume/internal/adapters/rabbitmq/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./services/resume/... ./tests/contract/...
git diff --check
```

All commands passed. Final tagged Testcontainers package durations were PostgreSQL
61.273 seconds and RabbitMQ 12.382 seconds. An earlier broader run failed because
the unknown-resume test counted unrelated valid event replays after uncertain ACK;
the instrumentation now counts only its own event ID. Transient-failure and
interrupted-delivery instrumentation is likewise event-specific. The corrected
full affected integration run passed. These are real container tests, including
existing dispatcher regressions, not configuration-only validation. Go files were
gofmt-formatted; ports did not change, so mock regeneration was unnecessary.

No simulated loss of a PostgreSQL COMMIT response was performed. Atomicity means
both effects commit or neither does; uncertain outcomes retry the same event ID.
Acknowledgment uncertainty can cause replay, which durable deduplication handles.
Existing nonmandatory publisher routing and blocking publisher I/O limitations
remain. No full Compose runtime, real JT-62 validator end-to-end, monorepo-wide
integration suite, or CI/PR run is claimed. Review and those deployment checks remain;
JT-59 is not declared complete.

## Final automated and runtime verification (12 November 2026)

Branch verified as `JT-59-resume-events-security`; all pre-existing tracked and
untracked work was retained. Only this document, the JT-59 deployment document,
and the requested new `coverage.out` artifact changed in this verification step.
No production code/tests were edited. JT59-57 above now distinguishes the
code-inspected exchange guard from the cases actually covered by unit tests.

Actual automated commands, all exit 0:

```sh
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -coverprofile=coverage.out ./...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go tool cover -func=coverage.out
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -tags=integration ./...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go test -race ./services/resume/... ./tests/contract/...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet ./...
GOCACHE=/private/tmp/jobtrack-jt59-go-cache go vet -tags=integration ./services/resume/... ./tests/contract/...
git diff --check
```

Both complete test commands reported 103 successful test packages and no failed
packages; packages without tests are additional. Some reusable unit results may
be cached. Full tagged execution included real disposable Testcontainers brokers
and databases: `pkg/forwarder` 82.907s, Resume PostgreSQL 61.300s, Resume RabbitMQ
17.460s. Focused race checks passed. No related or unrelated test failures were
observed. No Jira/PR/CI action was performed.

`go tool cover -func=coverage.out` reports 32.7% total repository statement
coverage, including generated packages/mocks. Resume unit-only production coverage
computed from that profile, excluding generated mocks, is:

| Package | Statement coverage |
|---|---:|
| cmd/server | 84.4% |
| internal/adapters/grpc | 98.8% |
| internal/adapters/postgres | 11.2% |
| internal/adapters/rabbitmq | 57.0% |
| internal/app | 97.8% |
| internal/domain | 100.0% |

These figures do not include integration coverage and do not establish the
combined application/application-status coverage target. `coverage.out` did not
exist before this step and was generated as requested; it was not staged.

Full command output is retained locally in `/private/tmp/jt59-final-unit.log`,
`jt59-final-coverage.log`, `jt59-final-integration.log`, `jt59-final-race.log`,
`jt59-final-vet.log` and `jt59-final-vet-integration.log` (all beneath
`/private/tmp/`). The empty vet logs correspond to successful exit codes. Raw
local Compose configuration/errors were not printed or added to repository docs.

### Verified Compose runtime: previous configuration blocker resolved

After the local `.env` was configured, the following runtime checks passed.
These actual results were supplied by the project owner for this documentation
finalization; the runtime commands and automated suites were not rerun during
this documentation-only step. This evidence supersedes the earlier missing-local-
configuration blocker without changing previously recorded automated results.

```sh
docker compose -f deploy/compose/docker-compose.yml config --quiet
docker compose -f deploy/compose/docker-compose.yml up -d --build postgres rabbitmq resume
docker compose -f deploy/compose/docker-compose.yml ps
```

- Configuration validation passed using the configured local environment.
- Compose successfully built and started Resume. PostgreSQL, RabbitMQ and Resume
  all appeared healthy in `docker compose ps`, without enabling the optional
  functions profile.
- Resume logged `servers starting` with gRPC `:9090` and HTTP `:8080`.
- The PostgreSQL `resume` database contains `resumes`, `outgoing_events`,
  `processed_events` and `goose_db_version`. Goose versions 0 through 6 all have
  `is_applied=true`, including migrations 00003–00006.
- Direct broker inspection confirmed all four queues are durable quorum queues:
  `resume.validation`, `resume.validation.dead`, `resume-validator.events`, and
  `resume-validator.events.dead`.
- `jobtrack.events` has `resume.validated` and `resume.rejected` bindings to
  `resume.validation`, and `resume.uploaded` to `resume-validator.events`.
- Both source queues have `x-delivery-limit=3`, `x-overflow=reject-publish`, and
  `x-dead-letter-strategy=at-least-once`.

Container health and direct database/broker inspection are verified runtime
evidence, beyond configuration-only validation. No separate direct HTTP response
capture was supplied for this update; the Compose health result is recorded as
observed. This is Resume startup/topology verification, not actual JT-62 validator
end-to-end execution. Simulated loss of a PostgreSQL COMMIT response remains
untested. The accepted nonmandatory-publication and blocking publisher-I/O
limitations remain documented.

Final local verification: automated checks PASS; Compose startup, health,
migrations and topology PASS. The previous configuration blocker is resolved.
PR creation, green CI, merge and the remaining Definition of Done steps are still
outstanding; JT-59 is not Done. In this documentation-finalization step only this
file and `docs/deployment/jt-59-topology.md` were edited. `coverage.out` is retained
but EXCLUDE from the intended commit, as are local environment configuration and
unreviewed setup artifacts. No code/tests were changed, and no staging, commits,
pushes, PR or Jira edits were performed.
