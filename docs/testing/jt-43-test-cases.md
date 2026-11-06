# JT-43 application service test cases

Traceability: [JT-43](https://jobtrack-cst.atlassian.net/browse/JT-43). Owner: Phuntsho Namgyel. Verification date: 7 November 2026.

## Scope and agreed behavior

The service was created with `bash scripts/new-service.sh application` from an empty `.gitkeep` placeholder. Public contracts and generated protobuf code are unchanged. PostgreSQL owns application submission/withdrawal records; Application Status still owns the lifecycle state machine.

The task owner (Phuntsho Namgyel) explicitly approved the following service decisions in this implementation session:

- Applicants get/list only their own applications; employers get/list only applications addressed to their user ID; admins get/list all applications.
- Only applicants submit/withdraw, with the applicant ID derived from `pkg/authctx`.
- Repeat withdrawal returns `FAILED_PRECONDITION`. Withdrawal changes only the local `withdrawn` flag, without a lifecycle lookup.
- A unique applicant/job pair remains reserved after withdrawal. No resubmission or deletion is introduced.
- During the PR #15 edge-case review, the task owner explicitly approved duplicate precedence: after authentication and request validation, an existing applicant/job pair returns `ALREADY_EXISTS` before job/resume eligibility checks, including after the job closes or expires.

The existing repository decision log and proto override the generic eligibility-failure wording: another applicant's resume returns `PERMISSION_DENIED`; missing resources return `NOT_FOUND`; closed/expired jobs and unvalidated resumes return `FAILED_PRECONDITION` when no application already exists for that applicant/job pair.

## Input limits and PR notes

There are no free-text fields in the existing Application requests, so no title/reason/notes length limit applies. Resource IDs must be canonical lowercase, nonzero UUID strings (36 characters). Invalid, blank, noncanonical and oversized IDs return `INVALID_ARGUMENT`. Missing/invalid authenticated caller IDs return `UNAUTHENTICATED`; unknown roles return `PERMISSION_DENIED`.

Pagination uses the existing shared messages: default size 20, explicit sizes 1–100, negative/over-100 sizes invalid. Page tokens are canonical decimal offsets, at most 10 characters and at most 2147483547; negative, padded, nonnumeric or oversized values are invalid. Results sort by creation time descending, then UUID descending. Total count and page use one repeatable-read snapshot. Offset pagination can shift between requests if new applications arrive; tokens never bypass caller filters.

Real cross-service clients and events remain deferred to Sprint 2. Runtime Submit for a new applicant/job pair returns `UNAVAILABLE` until those clients are wired; it never bypasses eligibility. Get/List/Withdraw use the real repository. Health checks PostgreSQL readiness, not the deliberately deferred clients. Compose uses the standard pgx `PGPASSWORD` environment alongside a password-free `DATABASE_URL`. Application startup does not initialize or depend on RabbitMQ; the shared Compose environment remains available for Sprint 2.

## Test matrix

| Case | Technique | Expected result | Automated coverage |
|---|---|---|---|
| JT43-01 Valid published job, future deadline, owned validated resume | Valid partition | Persist generated ID, trusted employer, caller applicant, UTC timestamps | app submit; RPC submit; PostgreSQL persistence |
| JT43-02 First submission: closed job, expired job, exact deadline | Boundary values | `FAILED_PRECONDITION`; no resume lookup/write | app eligibility; domain deadline ±1 ns |
| JT43-03 Job missing, unavailable, malformed response | Error partitions | `NOT_FOUND`, `UNAVAILABLE`, `INTERNAL`; no write | app eligibility |
| JT43-04 Resume missing/unavailable/malformed response | Error partitions | Mapped dependency failure; no write | app eligibility |
| JT43-05 Resume owned by another applicant | Authorization | `PERMISSION_DENIED`; no write | app eligibility |
| JT43-06 Resume not validated | Invalid partition | `FAILED_PRECONDITION`; no write | app eligibility |
| JT43-07 Submit same applicant/job twice | Duplicate partition/concurrency | `ALREADY_EXISTS`; unique constraint accepts one concurrent insert | app duplicate; PostgreSQL race |
| JT43-08 Withdraw own application | State transition | Flag set, update time advances, data retained | app withdraw; RPC withdraw; PostgreSQL |
| JT43-09 Repeat and concurrent withdrawal | State transition/concurrency | `FAILED_PRECONDITION`; exactly one concurrent update succeeds | app withdraw; PostgreSQL race |
| JT43-10 Resubmit after withdrawal | State transition | Unique applicant/job constraint still rejects | PostgreSQL |
| JT43-11 Missing/malformed identity, unknown role | Authorization partitions | `UNAUTHENTICATED` / `PERMISSION_DENIED` for all operations | app authorization matrix |
| JT43-12 Employer/admin attempts submit or withdrawal | Authorization | `PERMISSION_DENIED`, no dependency or storage access | app authorization matrix |
| JT43-13 Applicant reads/withdraws another applicant's record | Ownership | `PERMISSION_DENIED` | app ownership/withdraw |
| JT43-14 Employer reads another employer's record | Ownership | `PERMISSION_DENIED` | app get |
| JT43-15 Allowed applicant/employer/admin get and list | Authorization | Matching caller filters, admin unrestricted | app get/list; repository filters |
| JT43-16 Blank, malformed, nil, uppercase and oversized UUID | Boundary/invalid partitions | Invalid UUID rejected, nil request rejected | domain; app input; RPC nil cases |
| JT43-17 Page sizes default/1/100/-1/101 | Boundary values | Valid defaults/bounds accepted, invalid bounds rejected | app list |
| JT43-18 Tokens empty/0/20/negative/padded/nonnumeric/over-limit | Boundary values | Correct next token or `INVALID_ARGUMENT` | app list |
| JT43-19 Job filter, empty page, count/order | Equivalence/pagination | Caller isolation and stable tie-break ordering | app list; repository integration |
| JT43-20 Repository failure and context cancellation | Failure partitions | Safe `INTERNAL`/cancellation codes, no private error content | app mappings; repository closed pool/cancellation |
| JT43-21 RPC conversion and service registration | Contract adapter | Every field/timestamp/pagination converted; public service registered | RPC tests; real listener server test |
| JT43-22 Migrations twice and readiness | Integration | Versions applied once, readiness fails after pool closes | generated JT-20 readiness integration pattern |
| JT43-23 Startup configuration and graceful shutdown | Boundaries/lifecycle | Invalid config rejected; listeners close and resources release | generated/adapted server tests |
| JT43-24 Deferred clients | Sprint boundary | Valid Submit fails closed with `UNAVAILABLE` | app unit test |
| JT43-25 Reserved database password characters | Regression partitions | pgx preserves `@`, `:`, `/`, `?`, `#`, `%`, combined characters, quotes, backslash and spaces without changing the hostname | table-driven pgx parser tests; live PostgreSQL startup authentication with combined reserved characters |
| JT43-26 Canceled/invalid startup | Failure/secret handling | Canceled startup never opens the database; invalid configuration errors contain no synthetic password | server unit tests |
| JT43-27 Existing application after job closes/expires | State/precedence regression | `ALREADY_EXISTS` before eligibility; submitted/withdrawn rows included; other applicants/jobs remain isolated | two-submit gomock regression; real PostgreSQL/use-case integration |
| JT43-28 Duplicate lookup failure | Failure partitions | Safe `INTERNAL` / cancellation error; no eligibility call or insert | app unit tests |

## Reproduction

Run from the repository root. Tests need Go 1.27.3; integration tests need Docker and use disposable Testcontainers resources, never Compose volumes.

```sh
gofmt -l services/application
go vet ./...
go test -race -coverprofile=/tmp/jt43-unit.cover ./...
go test -race -tags=integration -coverprofile=/tmp/jt43-integration.cover ./services/application/...
go build ./...
buf lint
buf breaking --against '.git#ref=main'
buf generate
git diff --exit-code -- proto gen
docker compose -f deploy/compose/docker-compose.yml config --quiet
docker compose -f deploy/compose/docker-compose.yml up -d --no-deps --build application
docker compose -f deploy/compose/docker-compose.yml exec -T application wget -q -S -O - http://127.0.0.1:8080/healthz
```

For a sandboxed agent, writable build cache was set with `GOCACHE=/tmp/jobtrack-go-cache`; TCP listeners, Docker, downloads and module cache access required escalation. Do not print interpolated Compose configuration or `.env`.

Generated gomock code is committed under `internal/ports/mocks`. Regenerate with `go generate ./services/application/internal/ports` (pinned mockgen v0.6.0). Coverage of production service code excludes generated mocks; report the raw figure separately if including generated artifacts. Go reports statement coverage, the available automated proxy for the project line-coverage target.

## Actual verification results

All commands below were executed; host permission failures were rerun with the required escalation.

| Check | Result |
|---|---|
| `git pull --ff-only` | PASS; main already current; feature branch `JT-43-application-basic-operations` created |
| `bash scripts/new-service.sh application` | PASS; placeholder inspected first |
| `gofmt -l services/application` | PASS; no output |
| `go vet ./...` | PASS; exit 0 |
| `go build ./...` | PASS; exit 0 |
| `go test -race -coverprofile=/tmp/jt43-unit.cover ./...` | PASS; complete repository unit/race suite |
| `go test -race -tags=integration -coverprofile=/tmp/jt43-integration.cover ./services/application/...` | PASS; disposable PostgreSQL, repository concurrency and actual startup/health |
| `buf lint` | PASS after registry access escalation |
| `buf breaking --against '.git#ref=main'` | PASS |
| `buf generate` and `git diff --exit-code -- proto gen` | PASS; no contract/generated-protobuf changes |
| `docker compose -f deploy/compose/docker-compose.yml config --quiet` | PASS |
| `docker compose -f deploy/compose/docker-compose.yml build application` | PASS |
| Compose `up -d --no-deps --build application` and container `/healthz` | PASS; actual application container returned HTTP 200 and `ok`; migration versions 1 and 2 confirmed in application database |
| `git diff --check` | PASS |

Coverage from Go profiles (statement-weighted; generated gomock source excluded for production totals):

| Scope | Unit + race | Integration + race |
|---|---:|---:|
| Domain | 100% | 100% |
| Application use cases | 100% | 100% |
| gRPC adapters | 100% | 100% |
| Entire production application service | 281/338 = **83.14%** | 319/338 = **94.38%** |
| Entire application service including generated mocks | 281/393 = 71.50% | 319/393 = **81.17%** |

The initial integration test exposed timezone-location differences on PostgreSQL timestamp decoding. The repository now normalizes decoded timestamps to UTC; the subsequent persistence and race runs passed.

### Compose password regression: resolved

The earlier hostname-resolution diagnosis was caused by raw password interpolation in `DATABASE_URL`, not Docker DNS. Literal `postgres` resolved correctly in Go, while pgx parsed a different hostname containing `@`. The application Compose block now uses a password-free `DATABASE_URL` and supplies the unchanged password through `PGPASSWORD`, supported by the existing pgx library. No new dependency or shared-package change was needed.

Table-driven tests invoke `pgxpool.ParseConfig` and assert the exact password, host, port, user and database for synthetic passwords containing `@`, `:`, `/`, `?`, `#`, `%`, combined reserved characters, quotes, backslash and spaces. The startup integration test authenticates against disposable PostgreSQL using the combined reserved-character password through `PGPASSWORD`, applies migrations and receives HTTP 200. Its URL is built without a password because the Testcontainers connection-string helper also embeds raw passwords.

The actual persisted Compose application was rebuilt and started with `up -d --no-deps --build application`. `exec -T application wget -q -S -O - http://127.0.0.1:8080/healthz` returned **HTTP 200** and `ok`. A read-only PostgreSQL query confirmed migration versions 1 and 2. Application, PostgreSQL, RabbitMQ and Mailpit were running afterward. Only the application container was recreated; healthy infrastructure and all persisted volumes were preserved. No credentials were rotated, printed or committed.

RabbitMQ authentication is not required or exercised by the Sprint 1 application service. The separately reported broker credential problem remains outside JT-43; no broker authentication or full-stack event verification is claimed. The application’s actual Compose health criterion is now verified.

## Acceptance criteria review

| AC | Status | Evidence |
|---|---|---|
| 1 Own submit/withdraw only | PASS | Trusted identity; role/ownership matrix; atomic owner-filtered update |
| 2 Duplicate job application | PASS | `ALREADY_EXISTS` mapping and PostgreSQL unique pair, including concurrency/after withdrawal |
| 3 Job/resume eligibility with gomock | PASS | Published/deadline boundaries; validated/owned resume; required error mappings |
| 4 authctx and gRPC codes | PASS | Every operation authorizes; dependency/storage errors safely mapped |
| 5 All existing public RPCs | PASS | Submit, Withdraw, Get, List handlers; registration tested; contracts unchanged |
| 6 Hexagonal layout | PASS | Domain, app, ports, gRPC/PostgreSQL adapters, wiring |
| 7 Approved generator | PASS | Generated from inspected empty placeholder |
| 8 pgx/goose startup | PASS | Persistence, idempotent migrations, real startup integration |
| 9 Validation/maximum lengths | PASS | UUID/pagination limits documented; no free-text request fields exist |
| 10 Eligibility port interfaces | PASS | `internal/ports/application.go`, generated gomock implementations |
| 11 Sprint 2 clients/events deferred | PASS | No real clients/events; runtime Submit fails closed until wiring |
| 12 Table-driven unit tests | PASS | Valid/invalid partitions, permissions, ownership, state transitions and boundaries |
| 13 Testcontainers integration | PASS | Integration build tag; JT-20 PostgreSQL/migration pattern; isolated cleanup |
| 14 At least 80% coverage | PASS | Production unit 83.14%; integration 94.38%; raw integration including mocks 81.17% |
| 15 Compose/startup/health | PASS | Compose config/image and actual persisted Compose application startup/HTTP 200; startup migrations verified |
| 16 Traceable test cases | PASS | JT43-01 through JT43-28 above |

## Changed files and review notes

- `services/application/`: replaces the empty placeholder with generated/adapted health/startup/Dockerfile, application domain/use cases/ports/mocks, all four RPC handlers, PostgreSQL repository, initial and business migrations, unit and integration tests. Empty client/event adapter directories retain placeholders for Sprint 2.
- `deploy/compose/docker-compose.yml`: application image/environment and PostgreSQL readiness dependency; no published gRPC port and no dependency restart/recreation.
- `go.mod`, `go.sum`: AGENTS.md-required `go.uber.org/mock` v0.6.0; existing UUID module becomes a direct dependency. No new framework or datastore.
- `docs/testing/jt-43-test-cases.md`: this traceability, limits, decisions, evidence and review checklist.

Final review found no changes to protobuf contracts, generated protobuf files, other owners' services, shared Go source or `.env`; no secrets added. Full cluster, frontend, real eligibility clients and event publishing remain outside JT-43. The owner authorized committing, pushing and opening a PR after these checks; merge and Jira updates are excluded.

Review the PR and its CI results, then rerun local checks as needed. No merge or Jira update is performed by this workflow. The task is not Done under the project Definition of Done until the remaining review/CI, merge and time-tracking requirements are met.


## PR #15 duplicate-precedence follow-up

The focused `TestApplication_Submit_DuplicateAfterJobBecomesIneligible` first reproduced the issue against the original service: after a successful submission, both closing and expiring the job returned `FailedPrecondition` where the regression expected `AlreadyExists`. JT-43 states that duplicate submissions return `ALREADY_EXISTS`; AGENTS.md maps closed/expired jobs to `FAILED_PRECONDITION`, but neither explicitly states precedence when both apply. The task owner then explicitly confirmed: **duplicates take precedence after authentication and request validation, before job/resume eligibility**. No proto rule was invented or contract changed.

The eight-line production correction reuses the existing repository `List` port with the authenticated applicant ID, requested job ID and limit 1. It includes withdrawn applications and maps lookup failures safely. Eligibility checks for new pairs remain unchanged. The lookup is supplementary: the existing PostgreSQL unique applicant/job constraint still rejects concurrent inserts if two requests both initially see no application. The existing unit case returning `ErrDuplicate` after a clear lookup and PostgreSQL concurrent-insert tests retain coverage of this race.

New regression coverage includes two sequential submissions followed by closed/expired jobs, duplicate detection with deferred eligibility clients, withdrawn records, lookup failures, and use-case integration with the real PostgreSQL repository. The integration tests also verify a different applicant or different job does not cause a false duplicate. Existing authorization/input tests ensure those checks still run before repository access.

Follow-up verification executed successfully:

- `go test ./services/application/internal/app ./services/application/internal/adapters/grpc -count=1`
- `go test -race -coverprofile=/tmp/jt43-unit.cover ./...`
- `go test -race -tags=integration -coverprofile=/tmp/jt43-integration.cover ./services/application/...`
- `go vet ./...`, `go build ./...`, formatting and `git diff --check`

Coverage in the table above is refreshed from these follow-up profiles. Earlier Compose verification remains historical evidence from the initial PR implementation; this follow-up does not alter Compose, credentials or infrastructure and does not rebuild/restart the running service. Real cross-service clients and event publishing remain deferred to Sprint 2.
