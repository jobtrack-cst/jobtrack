# JT-58 Application integration test cases

Traceability: JT-58. Verification date: 12 November 2026.

Changes are limited to Application, its Compose wiring, shared application event
fixtures and this document. No protobuf schemas, dependencies, other services,
Git branches or Jira records are changed.

| Case | Acceptance criteria | Technique | Expected result | Automated location |
|---|---|---|---|---|
| JT58-01 | 1, 3 | Equivalence partitioning | Both public RPC clients forward authenticated applicant identity and translate valid metadata into domain values | `internal/adapters/clients/eligibility_test.go` |
| JT58-02 | 1, 3 | Boundary values | Missing/mismatched IDs, invalid owners, missing/invalid timestamps and unknown enum values fail closed | `internal/adapters/clients/eligibility_test.go` |
| JT58-03 | 1, 3 | Error partitions | NOT_FOUND, PERMISSION_DENIED, FAILED_PRECONDITION, UNAVAILABLE, DEADLINE_EXCEEDED, CANCELED and INTERNAL remain errors; UNAVAILABLE uses shared retries | `internal/adapters/clients/eligibility_test.go` |
| JT58-04 | 3 | Boundary values/state transitions | Only a published job with deadline strictly after now and an owned validated resume can be submitted; duplicate precedence and sanitized error mappings remain intact | `internal/app/application_test.go` |
| JT58-05 | 2 | Configuration partitions | JOB_POSTING_ADDR, RESUME_ADDR and RABBITMQ_URL required; Compose supplies service addresses; startup succeeds without either gRPC dependency running | `cmd/server/main_test.go`, `main_integration_test.go` |
| JT58-06 | 4, 6 | Transaction isolation | Application and event are both invisible while their transaction is blocked before commit | `internal/adapters/postgres/outbox_integration_test.go` via repository integration test |
| JT58-07 | 4, 6 | Failure injection | Outbox insert failure rolls back create and withdrawal; deferred commit failure rolls back both records | same as JT58-06 |
| JT58-08 | 4, 5 | Contract validation | Submitted/withdrawn events contain exact existing message fields, unique event UUIDs, producer application and committed UTC change timestamps | same as JT58-06; `tests/contract/application_fixtures_test.go` |
| JT58-09 | 4, 6 | State transitions/concurrency | Duplicate create and repeated/concurrent withdrawal produce no extra events; persisted events remain pending across failed publishing | repository integration tests |
| JT58-10 | 4, 6 | Failure injection/retry | A post-commit publish failure leaves application and event stored; failed marking after success retries the identical envelope and event ID | outbox integration test; `internal/app/outbox_test.go` |
| JT58-11 | 4, 6 | Error partitions | Relay stops at first failure, marks only confirmed events, and bounds each publish by 10 seconds | `internal/app/outbox_test.go` |
| JT58-12 | 4, 6 | Recovery/lifecycle | Failed publisher connection is dropped, reconnection failures are sanitized and retried; relay stops with servers and drains full batches | `cmd/server/publisher_test.go`, `relay_test.go` |
| JT58-13 | 4, 5, 6 | Broker integration | Both shared fixtures route to an application.* binding through pkg/events with persistent delivery and broker confirmations; canceled publish fails | `internal/adapters/rabbitmq/publisher_integration_test.go` |
| JT58-14 | 4, 6 | Error partitions | Invalid stored envelopes and canceled database operations fail without marking events | outbox integration test |

Paths above are relative to `services/application`, except shared contract tests.
All listed cases passed in the targeted combined run.

## Executed verification

From the repository root:

```sh
go test ./...
go generate ./services/application/internal/ports/...
gofmt -w services/application tests/contract/application_fixtures_test.go
go test -coverprofile=/tmp/jt58-unit.out ./services/application/... ./tests/contract/...
go test -tags=integration -coverprofile=/tmp/jt58-combined.out ./services/application/... ./tests/contract/...
go vet ./services/application/... ./tests/contract/...
go vet -tags=integration ./services/application/... ./tests/contract/...
docker compose -f deploy/compose/docker-compose.yml config --quiet
git diff --check
```

The full repository unit suite (`go test ./...`), targeted unit/contract and
combined unit/integration suites passed. Both targeted vet commands, Compose
configuration validation and whitespace checks also passed. PostgreSQL 17
and RabbitMQ 4.1 tests ran in disposable Testcontainers containers. Go and Docker
commands used sandbox escalation for the build cache, local ports and Docker
socket. Compose validation did not print resolved secrets. No existing volumes,
queues or databases were removed.

Production coverage excludes generated `internal/ports/mocks` files, per
AGENTS.md. The unit-only result is **75.0% (372/496 statements)**; combined unit
and integration coverage is **94.4% (468/496)**, exceeding the 80% target.
Reproduce the filtered reports with:

```sh
awk '!/\/internal\/ports\/mocks\//' /tmp/jt58-unit.out > /tmp/jt58-unit-production.out
awk '!/\/internal\/ports\/mocks\//' /tmp/jt58-combined.out > /tmp/jt58-combined-production.out
go tool cover -func=/tmp/jt58-unit-production.out
go tool cover -func=/tmp/jt58-combined-production.out
```

## Runtime semantics and review notes

The single in-process relay follows the approved JT-56 pattern: query up to 50
committed rows oldest first, publish through pkg/events, wait for broker
confirmation, then mark published. Retry after one second on failure; a full
batch is followed immediately by another. Event IDs and payloads never change
on relay retry. Application RPC success means database commit; publishing is
asynchronous. Existing applications are not backfilled with historical events.

A crash after confirmation but before marking can duplicate delivery. Consumers
must deduplicate event IDs. More than one Application process may duplicate
publishes because this follows the single-relay pattern without leased claims.
Published rows are retained. Broker confirmation does not establish consumer
processing or the existence of a matching durable binding; downstream services
must provision their own queues. This task does not change downstream topology.

No real multi-service UI workflow, full repository integration suite, CI,
SonarQube, deployment or live broker-restart experiment was run. Schema and
ownership changes were unnecessary. Review these changes before committing.

## Changed-file inventory

The final working tree contains 31 changed or new files, all unstaged:

- `deploy/compose/docker-compose.yml`
- `services/application/cmd/server/main.go`
- `services/application/cmd/server/main_integration_test.go`
- `services/application/cmd/server/main_test.go`
- `services/application/internal/adapters/postgres/application.go`
- `services/application/internal/adapters/postgres/application_integration_test.go`
- `services/application/internal/app/application.go`
- `services/application/internal/ports/application.go`
- `services/application/internal/ports/mocks/application.go`
- `docs/testing/jt-58-test-cases.md`
- `services/application/cmd/server/publisher.go`
- `services/application/cmd/server/publisher_test.go`
- `services/application/cmd/server/relay.go`
- `services/application/cmd/server/relay_test.go`
- `services/application/internal/adapters/clients/eligibility.go`
- `services/application/internal/adapters/clients/eligibility_test.go`
- `services/application/internal/adapters/postgres/events.go`
- `services/application/internal/adapters/postgres/outbox.go`
- `services/application/internal/adapters/postgres/outbox_integration_test.go`
- `services/application/internal/adapters/rabbitmq/publisher.go`
- `services/application/internal/adapters/rabbitmq/publisher_integration_test.go`
- `services/application/internal/adapters/rabbitmq/publisher_test.go`
- `services/application/internal/app/outbox.go`
- `services/application/internal/app/outbox_test.go`
- `services/application/internal/ports/events.go`
- `services/application/internal/ports/mocks/outbox_mock.go`
- `services/application/internal/ports/outbox.go`
- `services/application/migrations/00003_outbox.sql`
- `tests/contract/application_fixtures_test.go`
- `tests/contract/fixtures/application.submitted.json`
- `tests/contract/fixtures/application.withdrawn.json`
