# JT-60 application-status events

Verified 13 November 2026 on branch `JT-60-application-status-events`.
Scope: only `application.submitted` and `application.withdrawn`. Review,
interview and offer consumers remain deferred. Read-only RPCs and their
participant authentication are unchanged; there is no internal RPC or token.

## Processing and persistence

The shared `events.Consume` declares `application-status.applications` and
`application-status.applications.dead` as durable quorum queues. Only the two
application routing types are bound. The shared JT-61 policy supplies prefetch 1,
a 10-second handler deadline, reconnects, up to four deliveries with 1/2/4-second
backoff, and at-least-once dead-lettering. No existing queue is deleted or migrated.

The adapter validates the application producer, supported envelope type, matching
protobuf Any type, UUIDs (including job/resume IDs), and valid nonzero timestamp.
The shared consumer also validates the serialized envelope. The shared handler
API exposes the envelope type, not the AMQP delivery routing-key field.

Each successful input claims its event ID and commits status, history and outbox
in one PostgreSQL transaction. Duplicate IDs succeed without another write.
A new submission event ID for an existing application is a permanent conflict.
A withdrawal must match the stored applicant. A missing application is retriable
(the submission may be in flight); wrong participants and final-state transitions
are permanent failures. Failed transactions leave the input ID available for retry.
Withdrawals lock the row and retain the conditional status UPDATE.

All repository status changes, including the existing Start/Transition path,
append a serialized `StatusChanged` envelope to the outbox. Event status strings
use the existing contract test's uppercase unprefixed names (`SUBMITTED`,
`WITHDRAWN`); initial previous status is `UNSPECIFIED`, matching JT-49.
Database status strings stay lowercase. Outgoing envelopes have a fresh UUID,
producer `application-status`, UTC occurrence time and trusted application/applicant
IDs. Outbox insertion sequence breaks equal-timestamp ties.

The relay runs immediately and every second, in batches of 50, stops on failure,
and waits at most 10 seconds for each broker-confirmed publish before marking it.
A failed publish drops the publisher connection; the next attempt reconnects.
Uncertain acknowledgement or mark failure may duplicate a publish with the same
persisted outgoing event ID. Published rows are retained. Multiple service replicas
can publish the same pending row; downstream consumers must deduplicate event IDs.
Shutdown cancels and joins both workers before closing the publisher and pool.

## Test cases

| Case | Technique | Expected result | Evidence |
|---|---|---|---|
| Submit shared fixture | Contract / normal case | SUBMITTED, initial history, one processed ID and output | Consumer unit; PostgreSQL and broker integration |
| Withdraw shared fixture | State transition | WITHDRAWN and second history/output | Consumer unit; PostgreSQL and broker integration |
| Duplicate/redelivered ID | Equivalence / concurrency | No extra history or output; eight same-ID workers succeed | PostgreSQL integration; repeated broker deliveries |
| Distinct submission ID for recorded application | Conflict partition | Permanent conflict; no committed input ID/output | PostgreSQL integration |
| Withdrawal by wrong applicant | Authorization | Permanent conflict; trusted participants unchanged | PostgreSQL integration; handler classification unit |
| Withdrawal from ACCEPTED, DECLINED, REJECTED, WITHDRAWN | Final-state boundaries | Invalid transition; no extra input/history/output | Table-driven PostgreSQL integration |
| Different IDs concurrently withdraw | Concurrency | Exactly one change; other final-state conflict | PostgreSQL integration |
| Unknown application | Ordering / retry | Retriable, no processed ID | PostgreSQL integration; handler unit |
| Bad producer, type, Any, IDs or timestamp; nil input/data | Invalid partitions / timestamp boundary | Permanent failure before repository call | Table-driven fixture consumer unit |
| Invalid producer through RabbitMQ | Transport / dead letter | Original event reaches owned dead queue | Broker integration |
| Late outbox insert failure | Atomicity / fault injection | Status, history and input claim rolled back; retry succeeds | PostgreSQL CHECK fault injection |
| History insert failure during withdrawal | Atomicity / fault injection | Original status remains; retry succeeds | PostgreSQL CHECK fault injection |
| Output content and identity | Contract | Correct producer, statuses, participants and fresh UUID | PostgreSQL integration; broker envelope equality |
| Confirmed publication | Transport | Stored envelopes received by bound queue; published rows absent from Pending | PostgreSQL + RabbitMQ integration |
| Failed publish then retry | Recovery | Pending rows and original outgoing IDs retained, subsequently delivered | Broker integration; relay unit |
| Pending / publish / mark errors | Failure partitions | Stop batch, retry original envelope, mark only after publish | Table-driven gomock relay unit |
| Publisher fails / reconnect fails / closed | Recovery | Drop failed connection, reconnect on next publish, reject after close | Recovering publisher unit |
| Existing lifecycle, history, reads and auth | Regression | Existing JT-49 suite continues to pass | Unit and PostgreSQL integration |

## Actual verification

All commands below passed, using broader execution access for Docker and local
test listeners (the initial sandboxed unit run could not bind listeners).

```sh
gofmt -w services/application-status
go vet ./services/application-status/...
go test ./services/application-status/...
go test -tags=integration ./services/application-status/...
go test -coverprofile=/tmp/jt60-unit.out ./services/application-status/...
go test -tags=integration -coverprofile=/tmp/jt60-combined.out ./services/application-status/...
git diff --check
docker compose -f deploy/compose/docker-compose.yml config --quiet
docker compose -f deploy/compose/docker-compose.yml up -d --build application-status
docker compose -f deploy/compose/docker-compose.yml ps application-status
docker compose -f deploy/compose/docker-compose.yml exec -T application-status wget -qO- http://localhost:8080/healthz
docker compose -f deploy/compose/docker-compose.yml stop application-status
docker inspect --format '{{.State.ExitCode}}' jobtrack-application-status-1
docker compose -f deploy/compose/docker-compose.yml up -d application-status
```

Compose built successfully, reported healthy, returned `ok`, and stopped with
exit code 0. PostgreSQL and RabbitMQ dependencies were already running and healthy.
The service was restarted for review. RabbitMQ queue inspection confirmed both
owned queues are quorum queues and the source has delivery-limit 3, at-least-once
dead-lettering, and reject-publish overflow. Testcontainers used PostgreSQL 17/18 and
RabbitMQ 4.1. No local environment contents were printed or edited.

Coverage includes startup wiring and adapters, excludes generated
`internal/ports/mocks/`, and uses the combined tagged run (which runs both unit
and integration tests). Go's standard measure is statements; line coverage below
is the union of source line ranges in coverage blocks with nonzero execution,
divided by the union of all instrumented source line ranges.

| Suite | Production statements | Instrumented production lines |
|---|---:|---:|
| Unit only | 408/635 = 64.3% | 533/845 = 63.1% |
| Unit plus integration | 545/635 = 85.8% | 709/845 = 83.9% |

To reproduce the statement totals and line measure from either coverage profile:

```python
from pathlib import Path
for suite in ("unit", "combined"):
    rows = Path(f"/tmp/jt60-{suite}.out").read_text().splitlines()
    rows = [rows[0]] + [r for r in rows[1:] if "/ports/mocks/" not in r]
    Path(f"/tmp/jt60-{suite}-production.out").write_text("\n".join(rows) + "\n")
    statements = covered_statements = 0
    lines, covered_lines = set(), set()
    for row in rows[1:]:
        location, size, count = row.split()
        statements += int(size)
        if int(count):
            covered_statements += int(size)
        filename, span = location.rsplit(":", 1)
        first, last = span.split(",")
        block = {(filename, n) for n in range(
            int(first.split(".")[0]), int(last.split(".")[0]) + 1)}
        lines.update(block)
        if int(count):
            covered_lines.update(block)
    print(suite, covered_statements, statements, len(covered_lines), len(lines))
```

`go tool cover -func=/tmp/jt60-combined-production.out` reports the filtered
standard Go statement coverage.

Remaining scope limitation: strict equality between the AMQP delivery routing key
and envelope type cannot be checked by this service because the shared
`events.Handler` receives no delivery metadata. Supporting that check needs an
approved `pkg/events` change outside this task's authorized files. Envelope type
and Any type checks are implemented. CI, peer review, commit/push, PR, merge and
Jira changes were not performed; these remain the member's subsequent workflow.
