# JT-59 independent validator inbox provisioning

Owner and DevOps approved independent provisioning of the existing JT-61 topology
under JT-59. The topology increment did not start a dispatcher; the subsequent
dispatcher increment described below now enables it. JT-61 historical
transport evidence is unchanged; JT-62 owns the real validator and its end-to-end
verification.

## Mechanism and ordering

Resume now runs `rabbitmq.ProvisionTopology` during startup, after its database
migrations and before opening its publisher or HTTP/gRPC listeners. The provisioner
calls `forwarder.EnsureTopology` with `ResumeValidatorTopology`. It does not start a
consumer, HTTP sink, extra container or service. No new environment variables or
dependencies are needed. The existing Resume Dockerfile already copies `pkg/`.

The forwarder and Resume both use `forwarder.Topology.Declare`; the queue arguments
are defined once in `pkg/forwarder/topology.go`. The named topology is:

- `jobtrack.events`: durable topic exchange.
- `resume-validator.events.dead`: durable quorum queue, nonexclusive, no auto-delete.
- `resume-validator.events`: durable quorum queue, nonexclusive, no auto-delete,
  `x-delivery-limit=3`, `x-dead-letter-exchange=""`,
  `x-dead-letter-routing-key=resume-validator.events.dead`,
  `x-dead-letter-strategy=at-least-once`, `x-overflow=reject-publish`.
- Binding: `resume.uploaded` from `jobtrack.events` to `resume-validator.events`.

Provisioning uses synchronous AMQP declarations/bindings and a ten-second caller
context, reusing JT-61's cancellable connection setup and bounded connection close.
Success, not an ordinary process-health check, gates the rest of startup. Permission,
connectivity or incompatible-topology errors stop Resume before listeners start.
Errors identify the failing stage without broker credentials or raw broker details.
Repeated equivalent declarations preserve queues and queued messages. Concurrent
Resume/forwarder startup may repeat the same declarations safely. Provisioning is
not an AMQP transaction: on a later declaration failure, earlier successful
additive declarations remain; no destructive rollback is attempted.

This runs whenever Resume starts, including with the Compose `functions` profile
absent. The optional profile still controls only validator/forwarder execution.
Starting RabbitMQ alone does not provision queues; starting Resume does. With no
consumer, messages remain in the durable inbox until the forwarder is enabled.
The same Resume startup gate applies outside Compose; no k3d deployment or actual
validator execution was verified in this increment.

## Local commands

Run from the repository root with an existing local Compose environment configured:

```sh
docker compose -f deploy/compose/docker-compose.yml up -d --build resume
docker compose -f deploy/compose/docker-compose.yml logs resume
```

The default command starts Resume and its existing PostgreSQL/RabbitMQ dependencies,
without enabling functions. Do not use example passwords for runtime verification.
Do not print the interpolated Compose configuration, which can contain credentials.

Configuration-only checks using the committed placeholders:

```sh
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml config --quiet
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml --profile functions config --quiet
```

Both configuration checks passed. Parsed configuration checks also verified that
Resume is enabled by default, validator/forwarder are absent by default and present
with `--profile functions`. These checks do not start containers. Actual RabbitMQ
runtime tests use disposable Testcontainers brokers, without a validator or
forwarder consumer; see `docs/testing/jt-59-test-cases.md` for results. The application
Compose stack was not launched or rebuilt for this increment.

## Compatibility and operations

Classic queues cannot be silently converted to quorum queues. Any incompatible
queue/exchange declaration stops provisioning with a fixed diagnostic. Arrange a
reviewed migration with DevOps; preserve pending source and dead messages. Nothing
in the provisioner deletes, recreates, drains, downgrades or migrates queues. It
adds the approved binding but never removes other bindings automatically. Review
obsolete bindings explicitly, as JT-61 requires. Runtime permissions must allow
these existing declarations/bindings. RabbitMQ broker storage remains unchanged.

## Remaining routing limits

The initial profile/startup race is addressed by this startup gate. The Resume
outbox dispatcher now starts only after this gate succeeds. Provisioning does
not protect against later administrative queue deletion, unbinding, broker policy
changes or broker-data loss. Reprovisioning occurs at the next startup, not through
a background repair loop. Operational topology changes require coordinated
maintenance with publishers stopped.

`pkg/events.Publisher` remains nonmandatory. A broker confirmation alone still
does not prove receipt by the validator inbox if routing is changed after startup.
The dispatcher relies on the approved maintained-topology invariant and tests
confirmation, retry and recovery with that topology present. Mandatory/return-aware publishing, if separately approved,
would detect zero matching queues but cannot identify a particular destination
when other subscribers match. No shared `pkg/events` change was made here.

JT-59 still needs validation-result consumers, idempotency, shared fixtures and
complete verification. JT-62 owns real validator/function verification.


## Resume outbox dispatcher

Resume startup now follows this order: database migrations, exact JT-61 topology
provisioning, publisher creation, then one application dispatcher worker alongside
the existing HTTP/gRPC servers. The database pool is shared with the outbox adapter.
No dispatcher runs before successful migrations/provisioning. No optional function
profile or live validator is needed to publish into the durable inbox.

The worker claims one row for 30 seconds using the existing PostgreSQL lease API.
The claim transaction and connection have been released before any AMQP work. It
decodes the saved envelope, validates uploaded-event identity/type/payload, publishes
through the existing Resume adapter, and calls MarkPublished only after Publish
returns successful broker confirmation. Event IDs, payloads and timestamps are
never regenerated or rewritten. Serialization by the existing publisher can change
JSON formatting across builds; the original envelope content and identity remain.

With maintained topology, recoverable DB/broker availability and a progressing or
restarted worker, this provides at-least-once broker publication. It does not mean
exactly-once processing, HTTP function success or resume validation completion.
Confirmation followed by a DB error, cancellation or process crash can publish the
same event ID again. Expiry permits another worker to reclaim the row; its fresh
claim token prevents stale completion/retry updates from changing the new claim.
No global publication ordering is promised across workers or retries.

Consecutive unsuccessful attempts back off 1, 2, 4, 8, 16, then 30 seconds (capped).
A successful owned completion resets the in-memory backoff. A failed publication
or malformed stored envelope uses Retry to persist its next eligible time; the
worker also waits using cancellation-aware timers. The backoff level is local to
one worker and resets on restart; the scheduled timestamp survives restart. Empty
polls and lost ownership wait one second. Claim/complete/retry DB errors do not
mark events published; unresolved claims recover after expiry. Malformed events
remain pending for repair; they are never discarded or sent to a new DLQ.

The publisher connection is owned by service-local recoveringPublisher. A returned
Publish error closes the old session before the next attempt reconstructs it. A
reconnect failure is retried with backoff. A cleanup error stops the worker and
servers so the process can restart. No attempt starts another publishing goroutine.
Logs contain fixed diagnostics and event IDs, never payloads, URLs or credentials.

Each Publish receives a ten-second context for confirmation waiting. This does
not impose a hard bound on AMQP socket writes, connection construction or Close.
Those operations are synchronous and may block; repeated abandoned goroutines are
not spawned. If one never returns, this worker and graceful shutdown can stall and
require operator/container termination. Its lease still expires in PostgreSQL,
allowing another process to recover. No socket-level hard deadline is claimed.

Shutdown cancels the worker and servers, joins both, then closes the current
publisher and finally the database pool. Confirmation waiting, idle/backoff waits
and DB calls observe cancellation; a canceled claim stays pending until expiry.
Existing JT-44 checks, public handlers and health/metrics endpoints are unchanged.
Health remains the existing DB/process health, not a new dispatcher-progress probe.

Current verification uses unit tests, fixed-worker lifecycle/race tests, and real
PostgreSQL/RabbitMQ Testcontainers tests. It does not constitute an application
Compose end-to-end run or delivery to the real JT-62 validator. See the JT-59 test
case document for commands and actual results.

## Resume validation consumer (12 November 2026)

The earlier sections document preceding increments. Resume startup now also
provisions its own `resume.validation` and `resume.validation.dead` durable quorum
queues after database migrations and before opening the publisher/listeners.
They do not depend on the optional functions profile. This is distinct from the
unchanged JT-61 validator inbox: Resume still uses the shared JT-61 declaration
for that inbox, while the Resume adapter owns result-consumer topology.

`resume.validation` is bound only to `resume.validated` and `resume.rejected` on
`jobtrack.events`. It uses x-delivery-limit=3, x-dead-letter-strategy=at-least-once,
x-overflow=reject-publish, x-dead-letter-exchange="" and
x-dead-letter-routing-key=resume.validation.dead. Both queues are durable,
nonexclusive and never automatically deleted. Incompatible declarations stop
initial startup; existing queues/messages are never deleted or migrated. A
reviewed migration is required to resolve incompatible topology or historical
rejection reasons rejected by database migration 00006.

The consumer validates the shared envelope, routing/type agreement, producer
`resume-validator`, UUIDs and exact typed payload before persistence. Rejected
reasons must contain 1–500 Unicode characters after trimming (valid UTF-8, no NUL).
Unknown resumes and database/uncertain outcomes retry the original message;
applicant mismatch or invalid content dead-letters without a processed-event claim.
Only committed/idempotent success (applied, duplicate or already-final) is ACKed.
First committed final validation result wins through the existing transaction.

Prefetch is one. Broker-maintained quorum delivery counts enforce FOUR total
broker deliveries including the initial delivery, surviving consumer reconnects.
Redelivery waits are 1/2/4 seconds; processing has a two-second context. Retry uses
basic.reject(requeue=true); invalid/exhausted events basic.nack(requeue=false).
Missing redelivery counts and malformed metadata dead-letter safely. Crashes or
cancellation can exhaust the budget with fewer than four completed processing
attempts. DLQ bodies/IDs remain available for reviewed manual investigation; no
automatic draining/replay occurs. Uncertain settlement closes the session without
attempting a second settlement. A successful database commit followed by lost ACK
may replay, safely handled by durable event-ID deduplication.

The consumer runs synchronously per session with cancellation-aware reconnect
backoff of five seconds. Connection ownership uses the existing context-aware
forwarder dial pattern and bounded connection close; cancellation closes the
session and joins the fixed workers before publisher/pool teardown. It creates
no per-delivery worker goroutines. Existing shared publisher blocking-I/O and
nonmandatory routing limitations remain: initial provisioning cannot guarantee
routing after later administrative queue deletion/unbinding. Health remains the
existing health/metrics behavior, not a consumer-progress guarantee. Logs exclude
credentials, broker URLs and envelope contents.

Actual verification is recorded in `docs/testing/jt-59-test-cases.md`: full Go
unit suites, focused race/vet checks and real PostgreSQL/RabbitMQ Testcontainers
suites passed. The final runtime evidence below verifies Compose startup and
topology; no real validator end-to-end is claimed. JT-62 owns that implementation
and verification. PR/CI and merge remain before JT-59 completion.

## Final verification: Compose startup and topology verified (12 November 2026)

The previous local Compose configuration blocker is resolved. After configuring
the local `.env`, the project owner verified these actual runtime results and
supplied them for this documentation finalization. No runtime or test command was
rerun during this documentation-only update. Earlier increment-specific evidence
remains historical; this section records the final Compose runtime result.

```sh
docker compose -f deploy/compose/docker-compose.yml config --quiet
docker compose -f deploy/compose/docker-compose.yml up -d --build postgres rabbitmq resume
docker compose -f deploy/compose/docker-compose.yml ps
```

Configuration validation passed. Compose successfully built and started Resume,
and `ps` showed PostgreSQL, RabbitMQ and Resume all healthy. The optional
`functions` profile was not enabled. Resume logged `servers starting` with gRPC
`:9090` and HTTP `:8080`.

Direct PostgreSQL inspection confirmed `resumes`, `outgoing_events`,
`processed_events` and `goose_db_version` in the `resume` database. Goose versions
0 through 6 all have `is_applied=true`, including the four JT-59 migrations.

Direct RabbitMQ inspection confirmed:

| Queue | Durable | Type |
|---|---|---|
| resume.validation | true | quorum |
| resume.validation.dead | true | quorum |
| resume-validator.events | true | quorum |
| resume-validator.events.dead | true | quorum |

The `jobtrack.events` bindings are `resume.validated` and `resume.rejected` to
`resume.validation`, and `resume.uploaded` to `resume-validator.events`. Both
source queues have `x-delivery-limit=3`, `x-overflow=reject-publish` and
`x-dead-letter-strategy=at-least-once`. This confirms runtime provisioning with
the optional forwarder/functions execution disabled, beyond `config --quiet`.

The previously executed automated suites, full tagged integrations, focused race
checks, vets and `git diff --check` passed; commands and coverage are recorded in
`docs/testing/jt-59-test-cases.md`. Container health was verified; no separate
direct HTTP response capture was supplied for this documentation update. No
actual JT-62 validator end-to-end execution or simulated loss of a PostgreSQL
COMMIT response is claimed. Existing maintained-topology/nonmandatory-publishing
and blocking publisher-I/O limitations remain.

There is no remaining local Compose configuration blocker in the supplied
verification evidence. PR creation, green CI and merge remain outstanding; JT-59
is not Done. This finalization edits only the two JT-59 documentation files,
retains all existing work and excludes local environment files, unreviewed setup
changes and generated `coverage.out` from the intended commit. No staging, commit,
push, PR creation or Jira change was performed.
