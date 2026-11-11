# JT-61 RabbitMQ CloudEvents forwarder

The shared package is `pkg/forwarder`; `cmd/rabbitmq-forwarder` supervises sessions
with five-second reconnect delays. No new dependencies or contracts are introduced.
The JT-29 spike remains independently reproducible.

## Approved delivery and retry policy

The project owner approved restart-safe bounded retries. Each function has durable
quorum queues `<function>.events` and `<function>.events.dead`. Source arguments:

- `x-queue-type=quorum`, `x-delivery-limit=3`
- `x-dead-letter-exchange=""`, `x-dead-letter-routing-key=<function>.events.dead`
- `x-dead-letter-strategy=at-least-once`, `x-overflow=reject-publish`

RabbitMQ tracks failures in `x-delivery-count`. Each delivery performs at most one
HTTP request. Counts 0/1/2/3 permit four attempts, delayed 0/1/2/4 seconds respectively.
Consumer crashes, connection closures and cancellation with unacked deliveries
consume this same broker budget: reconnecting or restarting the forwarder does
not reset it. A delivery at count 4 or above never reaches HTTP. Broker delivery
limits provide a second bound if the consumer keeps crashing before settlement.
Invalid delivery-count metadata is dead-lettered rather than resetting the count.
Use RabbitMQ 4.1+; `basic.reject(requeue=true)` marks failure even with RabbitMQ 4.3's
changed `basic.nack` counting semantics. Prefetch is 1 per consumer.

The complete original validated protojson envelope is the binary CloudEvent's
`application/json` data, byte-for-byte including whitespace. `ce-id=event_id`,
`ce-type=type`, `ce-source=/jobtrack/rabbitmq`, `ce-specversion=1.0`. Redirects are
not followed; the HTTP timeout is 15 seconds.

HTTP 2xx alone causes `Ack(false)`. Other statuses, timeouts and connection errors
cause a requeue rejection until exhaustion. Malformed/exhausted deliveries use
`Nack(false,false)` to initiate the configured broker-side at-least-once dead-letter
transfer. This is not a positive acknowledgment: the quorum source retains the
message until the DLQ confirms receipt. No client publish/ack transfer window exists.
Settlement errors close the session so an uncertain message is redelivered.
Cancellation interrupts requests/backoff and leaves the message unacknowledged.
Connection setup/handshake has a five-second timeout and honors cancellation;
connection shutdown is limited to one second, and HTTP server shutdown to two.

At-least-once delivery still permits duplicate HTTP processing after success with
an uncertain acknowledgment. Functions must durably deduplicate event IDs before
returning success. Crashes can exhaust the budget without four completed requests;
inspect/replay DLQ events manually after repairing the cause, preserving event IDs.
At-least-once dead-letter retries can duplicate DLQ entries. Never auto-loop the DLQ.
Loss of broker disks or a quorum majority's data remains outside this guarantee;
a single-node local quorum queue is persistent but has no node-loss redundancy.
Publish persistent events through `pkg/events.Publisher` and retain broker volumes.

## Topology and migration

Existing classic queues cannot be redeclared as quorum queues. Startup fails closed
on a topology mismatch, without deleting anything. Coordinate a drain/migration
with DevOps before activating this configuration against existing queues. Preserve
pending source/dead messages; do not remove queues/volumes as an automatic fix.
Changing event bindings also requires reviewing obsolete bindings explicitly.

Compose already persists RabbitMQ data in `rabbitmq-data`. The JT-29 broker uses
`emptyDir` and must not be treated as the application broker. k3d application broker
storage, namespace, credentials and the actual validator Knative Service must exist
before deployment. Do not weaken dead-letter settings or remove the DLQ: RabbitMQ
will retain undeliverable dead letters and retry, consuming disk/resources. Broker-side DLQ recovery retries use RabbitMQ's own
schedule (normally up to three minutes), separate from HTTP backoff.
See [RabbitMQ quorum queue documentation](https://www.rabbitmq.com/docs/4.1/quorum-queues)
for the delivery-limit and at-least-once dead-letter guarantees and limitations.

## Compose

Functions run as ordinary long-running HTTP containers, with no local Knative or
scale-to-zero. The opt-in `functions` profile points to the actual validator image;
there is no substitute receiver that acknowledges real resume events.
From the repository root with existing Compose environment settings configured:

```bash
export RESUME_VALIDATOR_IMAGE=jobtrack-resume-validator:local
docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build postgres rabbitmq resume resume-validator resume-validator-forwarder
docker compose -f deploy/compose/docker-compose.yml logs resume-validator-forwarder
```

Validator dependencies now start Resume, PostgreSQL and RabbitMQ and wait for
health checks. The actual validator image must listen on 8080 and durably process
CloudEvent envelopes. Only the validator receives `INTERNAL_TOKEN` and `RESUME_ADDR`.
The forwarder receives no internal token, JWT secret or database credentials.

## k3d

Use the application namespace with a persistent broker, `rabbitmq-credentials`
Secret containing `amqp-url`, and the real Knative Service `resume-validator`:

```bash
docker build -f cmd/rabbitmq-forwarder/Dockerfile -t jobtrack-rabbitmq-forwarder:local .
k3d image import jobtrack-rabbitmq-forwarder:local -c <application-cluster>
kubectl --context <application-context> -n <application-namespace> apply -k deploy/knative/forwarder
kubectl --context <application-context> -n <application-namespace> rollout status deployment/resume-validator-forwarder
```

The same-namespace Knative URL supports activation from scale zero. Check network
policies permit broker and Knative traffic. `/healthz` and `/metrics` describe the
forwarder process, not broker or sink readiness.
Configuration: `RABBITMQ_URL`, `SINK_URL` (HTTP, no embedded credentials),
`FUNCTION_NAME`, comma-separated exact `EVENT_TYPES`, optional `HTTP_PORT` (8080).

## Verification and remaining blocker

```bash
go test -count=1 -race ./pkg/forwarder ./cmd/rabbitmq-forwarder
go test -count=1 -tags=integration ./pkg/forwarder
go vet ./...
go test ./...
go build -o /tmp/jobtrack-jt61-forwarder ./cmd/rabbitmq-forwarder
kubectl kustomize deploy/knative/forwarder
```

Tests exercise a transport-only HTTP sink, not validator business logic. The real
`resume.uploaded -> resume-validator` criterion remains **BLOCKED** until the actual
validator image exists and event IDs and acknowledgments are correlated in both
Compose and k3d. A passing integration test or Kubernetes dry run does not satisfy it.
See `docs/testing/jt-61-test-cases.md` for actual execution results.
