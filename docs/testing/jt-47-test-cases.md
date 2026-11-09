# JT-47 Job-alert subscriptions: Sprint 1

Implements CreateJobAlert, ListMyJobAlerts and DeleteJobAlert using the existing contracts. Scaffold created with `bash scripts/new-service.sh job-alert`. No contract or event schema changes, job.published consumer, matching logic or job_alert.matched publishing.

CategoryTagInternalService.ResolveNames is represented by `ports.NameResolver` and mocked with gomock. The application validates every supplied category/tag ID against its response and persists resolved names with the subscription. No real client is wired in Sprint 1: category/tag creation returns UNAVAILABLE in Compose; location-only creation, listing and deletion are available. No fake reference data or internal credentials are added.

Location is limited to **100 Unicode characters**, checked by the application and PostgreSQL `varchar(100)`. Invalid UTF-8 and NUL are rejected. The location name is preserved exactly, without trimming or separate location validation, as required by the contract. Category/tag input lists are limited to 20 entries each before normalization; UUIDs are lowercased, sorted and deduplicated. Normalized sets plus exact location identify duplicates per applicant. Duplicate detection precedes the 10-subscription limit. Creation uses a transaction and per-applicant advisory lock to serialize concurrent creates, plus a database unique constraint. Lists use creation time then alert UUID for deterministic ordering and a consistent count/page snapshot.

| Case | Acceptance criterion / partition | Verification |
|---|---|---|
| JA-01 | Applicant role required for all three operations; missing/invalid identity UNAUTHENTICATED; employer/admin PERMISSION_DENIED | domain `TestCaller_RequiresApplicant`; app `TestAlerts_AllOperationsRejectUnauthorizedCallers`; gRPC trusted identity tests |
| JA-02 | Owner derived from metadata; list scoped to owner; another applicant cannot delete; missing alert NOT_FOUND | app scope tests; gRPC response tests; PostgreSQL persistence/ownership test |
| JA-03 | All-empty filters invalid; each individual empty filter unrestricted; bad UUIDs invalid; 20/21 entries; normalized sorting/deduplication | domain filter boundary table; app normalized resolver test |
| JA-04 | Location 100/101 Unicode characters, NUL/invalid UTF-8, exact name preserved | domain filter table; PostgreSQL migration constraints test |
| JA-05 | Names obtained through mocked ResolveNames; omitted categories/tags invalid; resolver failure propagates; unavailable production resolver fails closed | domain resolved-names table; app create tests; gRPC UNAVAILABLE test |
| JA-06 | Normalized duplicate ALREADY_EXISTS; tenth allowed, eleventh FAILED_PRECONDITION; duplicate at limit stays ALREADY_EXISTS | app error table; PostgreSQL concurrent limit and duplicate tests |
| JA-07 | UUID and UTC timestamps; persist IDs, names and exact location; delete then recreate | app create test; PostgreSQL persistence test |
| JA-08 | Default 20, max 100; negative size and malformed/overflow tokens invalid; decimal offsets including int64 max | domain pagination table; app and gRPC tests |
| JA-09 | Total count, next token, final page and past-end offset; isolated applicants | gRPC pagination table; PostgreSQL persistence test |
| JA-10 | gRPC error mapping and sanitized unexpected errors; nil requests | gRPC error table and response tests |
| JA-11 | Migrations repeatable; filter and length constraints; database failures | Testcontainers PostgreSQL tests and inherited readiness tests |
| JA-12 | Public service registered; health/metrics; startup migration ordering, failure cleanup and graceful shutdown | `cmd/server` tests, including `TestServe_RegistersJobAlertPublicRPC` |
| JA-13 | Compose build/start, DB credentials, internal ports, HTTP health 200 | Compose smoke verification |

Run from the repository root, with Docker running for integration tests:

```sh
go generate ./services/job-alert/internal/ports/...
go vet ./services/job-alert/...
go test -cover ./services/job-alert/...
go test -tags=integration ./services/job-alert/...
go test ./...
go test -tags=integration ./...
docker compose --env-file deploy/compose/.env -f deploy/compose/docker-compose.yml up -d --build --wait job-alert
docker compose --env-file deploy/compose/.env -f deploy/compose/docker-compose.yml exec -T job-alert wget -S -O- http://localhost:8080/healthz
```

Compose interpolates every service's configuration, including Auth settings when starting only job-alert. The local `.env` currently lacks ADMIN_EMAIL, ADMIN_PASSWORD and JWT_SECRET. For this job-alert smoke check only, commands used temporary synthetic process values (`env ADMIN_EMAIL=admin@example.test ADMIN_PASSWORD=synthetic-test-password JWT_SECRET=synthetic-compose-test-secret-32-bytes docker compose ...`). No environment files or existing credentials were changed.

## Executed verification — 8 November 2026

- Mock generation, gofmt and `git diff --check`: passed.
- `go vet ./services/job-alert/...`: passed.
- `go test -cover ./services/job-alert/...`: passed; app 100%, gRPC 100%, domain 100%, startup 82.2%; PostgreSQL behavior is covered by integration tests rather than the unit coverage figure.
- `go test -tags=integration ./services/job-alert/...`: passed with real PostgreSQL and RabbitMQ, including concurrent duplicate/limit checks. The scaffold's transport tests use isolated queues; business-event publishing is absent.
- `go test ./...` and `go test -tags=integration ./...`: passed, using cached results for unchanged packages where applicable.
- Compose built the job-alert image and started the service with healthy PostgreSQL/RabbitMQ dependencies. Explicit container HTTP probe returned **200 OK**, body `ok`. No host ports are published for job-alert. This verifies startup with existing local dependencies, not a fresh-volume cold start.
- Initial restricted runs encountered Go-cache/local-listener/Docker-socket permissions; reruns with required access passed. Initial Compose interpolation failed on the missing Auth settings described above.

Remaining delivery steps: review the diff, rerun checks locally, create a `JT-47-...` branch, commit with `JT-47:` after approval, push and open a Jira-keyed PR after approval, obtain green CI, move JT-47 to In Review and log actual time. Merge only after the project Definition of Done is met. No commits, pushes, PRs or Jira writes were performed.
