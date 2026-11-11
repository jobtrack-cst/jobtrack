# JT-61 test cases and execution evidence

All cases trace to JT-61. Unit tests use external package `forwarder_test` and
synthetic envelopes with extra whitespace to detect re-encoding.

| Case | Expected result | Automated coverage |
| --- | --- | --- |
| HTTP 200/204/299 | Exact envelope bytes and CloudEvent headers; single Ack | Unit |
| HTTP 300/400/500 | No Ack; counted broker retry rejection | Unit |
| Failure followed by success | First delivery retries, next delivery Acks | Unit |
| HTTP timeout / connection refusal | Retry without acknowledgment | Unit |
| 307 redirect with Location | Redirect target never contacted | Unit |
| Malformed JSON / incomplete envelope | Nack without requeue to safe DLX | Unit + RabbitMQ |
| Active request cancellation | No Ack, Nack or Reject; session close redelivers | Unit + RabbitMQ restart |
| Cancellation during backoff | No HTTP request or settlement | Unit |
| Ack / Nack / Reject error | Error propagates; session closes on settlement uncertainty | Unit; Run behavior inspected |
| Fresh handlers with broker counts 0–3 | Four total attempts, final Nack | Unit |
| Delivery count >=4 | Dead-letter without any HTTP request | Unit |
| TCP/AMQP stalled handshake | Context cancellation interrupts setup | Unit |
| Valid AMQP -> HTTP -> Ack | After stopping consumer, source has no redelivery | RabbitMQ |
| Four failing HTTP attempts | Original bytes reach per-function quorum DLQ | RabbitMQ |
| Consumer replaced during first request | Next process resumes broker budget and eventually dead-letters | RabbitMQ |
| Consumer closed during every request | Broker delivery limit prevents endless crash/redelivery loop | RabbitMQ |
| DLQ temporarily missing | Source retains dead letter; restoring DLQ permits eventual transfer | RabbitMQ |
| Actual validator in Compose and k3d | Event ID correlated in real validator and source drained | **BLOCKED: actual validator image absent** |

Integration uses isolated Testcontainers RabbitMQ 4.1 with real AMQP and a local
transport-test HTTP receiver. The isolated test broker sets
`dead_letter_worker_publisher_confirm_timeout=1000` milliseconds to keep DLQ
recovery verification bounded; production broker settings remain unchanged. The receiver is not resume-validator and does not
satisfy its end-to-end acceptance criterion. No project broker queues, data,
secrets or workloads are modified by integration tests.

## Verification commands

```bash
go test -race -timeout=45s -count=1 ./pkg/forwarder ./cmd/rabbitmq-forwarder
go test -race -tags=integration -timeout=120s -count=1 ./pkg/forwarder
go vet ./...
go test -timeout=90s ./...
go build -o /tmp/jobtrack-jt61-forwarder ./cmd/rabbitmq-forwarder
buf lint
git diff --check
env POSTGRES_PASSWORD=synthetic SERVICE_DB_PASSWORD=synthetic RABBITMQ_USER=synthetic RABBITMQ_PASSWORD=synthetic JWT_SECRET=synthetic-test-secret-at-least-32-bytes ADMIN_EMAIL=admin@example.test ADMIN_PASSWORD=synthetic-test-password INTERNAL_TOKEN=synthetic docker compose -f deploy/compose/docker-compose.yml --profile functions config --quiet
kubectl kustomize deploy/knative/forwarder
docker -H unix:///var/run/docker.sock build -f cmd/rabbitmq-forwarder/Dockerfile -t jobtrack-rabbitmq-forwarder:jt61 .
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig --context k3d-jobtrack-jt29 -n jt29-spike apply --dry-run=server -k deploy/knative/forwarder
```

The explicit Docker socket matches the existing local Testcontainers runtime;
other hosts should use their configured socket. The kubectl command validates
against the existing isolated spike cluster without creating a workload.

## Execution notes (11 November 2026, Asia/Thimphu)

Initial sandboxed tests could not bind HTTP test ports; elevated runs were used.
The initial cancellation receiver hung because it did not drain the request body;
the fixture was corrected and focused race checks then passed. A DLQ outage test
initially checked the ready-message count, which does not include internally
retained dead letters; it now checks eventual transfer after restoring the DLQ.
These failed intermediate runs are not evidence of successful verification.

The previous implementation's recorded test results do not verify this policy.
The first DLQ recovery check used a 25-second wait, shorter than RabbitMQ's
three-minute default internal DLX retry schedule. Only the isolated test broker
now uses a one-second confirm timeout. Final results follow below.

## Final results

| Command/check | Actual result |
| --- | --- |
| Focused `go test -race -timeout=45s -count=1 ./pkg/forwarder ./cmd/rabbitmq-forwarder` | PASS; forwarder 8.465 s, command package has no tests |
| `go test -race -tags=integration -timeout=120s -count=1 ./pkg/forwarder` | PASS; 43.247 s; all six RabbitMQ scenarios plus unit cases |
| `go vet ./...` | PASS |
| `go test -timeout=90s ./...` | PASS; final forwarder unit run 7.465 s; unchanged packages may use Go cache |
| `go build -o /tmp/jobtrack-jt61-forwarder ./cmd/rabbitmq-forwarder` | PASS |
| `buf lint` | PASS |
| `gofmt -l pkg/forwarder cmd/rabbitmq-forwarder` | PASS; no unformatted files |
| `git diff --check` | PASS |
| Compose config command above, with synthetic environment | PASS; configuration validation only |
| `kubectl kustomize deploy/knative/forwarder` | PASS |
| Docker image build command above | PASS; local image `jobtrack-rabbitmq-forwarder:jt61` |
| k3d server-side dry run above | PASS; `deployment.apps/resume-validator-forwarder created (server dry run)` |
| Actual validator delivery in Compose and k3d | **BLOCKED**; validator implementation/image absent |

No live application Compose deployment, classic-queue migration, or real validator
end-to-end verification was performed. Broker node/disk loss and RabbitMQ 4.3
compatibility were not fault-tested; RabbitMQ 4.1 consumer redelivery and safe
DLQ transfer were tested. Existing queue migration requires DevOps review.

| JT-61 acceptance criterion | Status |
| --- | --- |
| Shared code, envelope bytes, identity headers and success-only Ack | Verified by code and transport tests |
| Durable per-function queues, restart-safe retry and safe dead-letter policy | Verified on isolated RabbitMQ 4.1; migration caveat documented |
| Compose execution decision in AGENTS.md | Verified |
| resume.uploaded to actual validator in Compose and k3d | **BLOCKED** |
| Unit mapping and acknowledgment tests | Verified |
| Sections 5, 9, 10; no new tools/dependencies/contracts | Inspected; external package tests and event_id logging implemented |
| Jira-key PR, green CI, merge, time logged | Unverified / intentionally untouched pending owner review |
