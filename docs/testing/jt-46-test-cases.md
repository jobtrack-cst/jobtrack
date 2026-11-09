# JT-46 Messaging Sprint 1 test cases

Scope: SendMessage and ListMessages using a local application-participants projection. No synchronous service calls, message.sent publishing, or application.submitted consumer. A new application returns NOT_FOUND until its participant projection is populated; production population is Sprint 2 work.

| Case | Acceptance criteria | Verification |
|---|---|---|
| MSG-01 | Trim content; 0, whitespace-only, 1, 2000, 2001 Unicode characters; reject NUL and invalid UTF-8 | domain `TestContent_Boundaries`; PostgreSQL `TestMessages_PersistenceOrderingAndPagination` |
| MSG-02 | Trusted caller UUID and role; invalid or absent identity is UNAUTHENTICATED | domain `TestCaller_Validation`; app `TestMessages_Authorization`; gRPC `TestMessages_StatusCodes` |
| MSG-03 | Applicant/employer may send and list only their own threads; reject admins, unrelated users, and mismatched role | app `TestMessages_Authorization` for both operations |
| MSG-04 | Recipient is the other stored participant; sender is caller; trim persisted content; UUID and UTC timestamps | app `TestMessages_Authorization`; gRPC `TestMessages_RequestAndResponseMapping` |
| MSG-05 | Unknown/unprocessed applications return NOT_FOUND; infrastructure failures remain distinct and sanitized | app authorization tests; gRPC status tests; participant repository unit and integration tests |
| MSG-06 | Default 20, cap 100, size 1, negative size, decimal/negative/plus/whitespace/overflow token rejection | domain `TestPage_Boundaries`; app validation tests; gRPC request mapping |
| MSG-07 | Count, next token, final page, empty thread, offset beyond end | gRPC response mapping; PostgreSQL persistence and pagination integration test |
| MSG-08 | Durable storage; oldest first with message UUID tie-breaker; isolated application threads | PostgreSQL `TestMessages_PersistenceOrderingAndPagination` |
| MSG-09 | Migration repeatability; UUID/non-null/primary key constraints; message content lengths and application FK | participant integration tests (migrate twice); message integration test |
| MSG-10 | Request mapping, nil requests, INVALID_ARGUMENT, PERMISSION_DENIED, NOT_FOUND, UNAUTHENTICATED, INTERNAL, cancellation and deadline mapping | gRPC message tests |
| MSG-11 | Repository failures propagate; invalid input never writes | app `TestMessages_ValidationAndRepositoryErrors` and gomock expectations |
| MSG-12 | Startup configuration, migrations, cleanup, readiness, health, metrics, graceful shutdown | scaffolded `cmd/server` tests, health tests, PostgreSQL readiness integration tests |
| MSG-13 | Compose service uses messaging DB credentials, internal 9090/8080, no host ports | Compose config validation and container health smoke check |

Run from the repository root:

```sh
go generate ./services/messaging/internal/ports/...
gofmt -w services/messaging
go vet ./services/messaging/...
go test ./services/messaging/...
go test -cover ./services/messaging/...
go test -tags=integration ./services/messaging/...
go test ./...
go test -tags=integration ./...
docker compose --env-file deploy/compose/.env -f deploy/compose/docker-compose.yml config --quiet
docker compose --env-file deploy/compose/.env -f deploy/compose/docker-compose.yml up -d --build --wait messaging
docker compose --env-file deploy/compose/.env -f deploy/compose/docker-compose.yml exec -T messaging wget -qO- http://localhost:8080/healthz
```

Integration tests require Docker. Existing template RabbitMQ adapter initialization is retained, without business-event publishing. Its inherited adapter integration tests exercise the transport in isolated test queues only.

## Executed verification (8 November 2026)

- Mock generation and gofmt completed.
- `go vet ./services/messaging/...`: passed.
- `go test ./services/messaging/...`: passed after rerunning outside sandbox restrictions on cache access and local listeners. The initial restricted attempt failed for those environmental reasons.
- `go test -cover ./services/messaging/...`: passed. app 100%, gRPC 100%, domain 84.8%, startup 78.5%, PostgreSQL 26.2% (database behavior is tested in the separate integration suite), RabbitMQ 83.3%.
- `go test -tags=integration ./services/messaging/...`: passed, including real PostgreSQL and RabbitMQ.
- `go test ./...`: passed.
- `go test -tags=integration ./...`: passed. Existing unchanged packages used Go's test cache where applicable.

### Historical troubleshooting (resolved)

- Compose config initially failed because the existing local `.env` lacks ADMIN_EMAIL, ADMIN_PASSWORD and JWT_SECRET. Retried with temporary synthetic process environment values: passed. No environment files were changed.
- Final registration/chronological ordering checks: targeted coverage, vet and Messaging integration suite passed again. Final Messaging unit suite passed after the repository interface assertion was added.
- Compose `up -d --build --wait messaging`: image build succeeded. RabbitMQ remained `starting` with timed-out health checks (exit code -1), leaving Messaging `created`. The blocked wait was interrupted (exit 130; Compose reported `dependency rabbitmq failed to start`).
- Explicit container `/healthz` probe: failed with `service "messaging" is not running`. Runtime smoke verification was blocked at that stage by local RabbitMQ health; the successful checks below supersede this blocker. PostgreSQL and pre-existing application/Mailpit containers were left intact; no volumes were removed. The attempt created the Messaging container and started the existing RabbitMQ container.
- `git diff --check`: passed. No branch changes, staging, commits, pushes, PRs, or Jira changes were performed.

### Final runtime verification (8 November 2026)

The following terminal results were verified after resolving the local RabbitMQ host-port conflict and disconnected Docker network. The earlier failures above remain troubleshooting history, not outstanding runtime blockers.

- The Messaging Docker image built successfully, and Messaging started with Docker Compose.
- `docker compose -f deploy/compose/docker-compose.yml up -d messaging` succeeded: Messaging was running, RabbitMQ was healthy, and PostgreSQL was healthy.
- `docker ps` showed `jobtrack-messaging-1` as `Up (healthy)`.
- `docker exec jobtrack-messaging-1 wget -S -O- http://127.0.0.1:8080/healthz` returned `HTTP/1.1 200 OK` with body `ok`.
- Messaging startup logs showed `servers starting`, with gRPC on port 9090 and HTTP on port 8080.
- Messaging's ports were not published to the host.
- The final read-only review confirmed that Messaging vet, unit tests, coverage and integration tests, plus repository-wide Go unit and integration tests, passed. Cached test results were used where applicable.
- A fresh cold-start dependency sequencing test has **not been performed**. Successful runtime health and the Compose command above do not establish cold-start dependency sequencing.
