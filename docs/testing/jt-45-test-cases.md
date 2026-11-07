# JT-45 Offer service test cases

Traceability: [JT-45](https://jobtrack-cst.atlassian.net/browse/JT-45). Owner: Phuntsho Namgyel. Verification date: 8 November 2026.

## Scope

All four public Offer RPCs use the existing contract. MakeOffer uses the trusted local application participant projection; GetOffer and decisions use stored offer participants. No new synchronous dependency, public seed endpoint, business event publisher or consumer is introduced. The original seven domain transition tests are preserved.

Terms are trimmed and must contain 1 to 2000 Unicode characters. PostgreSQL enforces nonempty terms and the same character maximum. IDs use 36-character UUID strings; uppercase hexadecimal is accepted and normalized. Caller identity is obtained from pkg/authctx; malformed identity or unknown roles are UNAUTHENTICATED. Admins cannot view offers in Sprint 1.

## Test matrix

| Case | Technique | Expected result | Automated coverage |
|---|---|---|---|
| JT45-01 Owning employer makes offer | Valid partition | PENDING offer, UUID, trusted IDs, trimmed terms, UTC timestamps | app, gRPC, PostgreSQL |
| JT45-02 Applicant/admin creates offer | Role partitions | PERMISSION_DENIED, no write | app, gRPC |
| JT45-03 Wrong employer | Ownership | PERMISSION_DENIED, no write | app |
| JT45-04 Unknown/unprocessed application | Missing data | NOT_FOUND | participant integration, app, gRPC error mapping |
| JT45-05 Missing/malformed identity or role | Invalid partitions | UNAUTHENTICATED on all RPCs | domain, app, gRPC |
| JT45-06 Invalid/empty identifiers | Invalid partition | INVALID_ARGUMENT | participant unit/integration, app, gRPC |
| JT45-07 Empty/whitespace terms | Invalid partition | INVALID_ARGUMENT | domain, app |
| JT45-08 Terms length 1, 2000, 2001 | Unicode boundary values | First two accepted, last rejected | domain; PostgreSQL 2000/2001 |
| JT45-09 Applicant/employer views associated offer | Ownership | Offer returned; all proto fields preserved | app, gRPC |
| JT45-10 Unrelated caller/admin views offer | Role/ownership | PERMISSION_DENIED | app, gRPC |
| JT45-11 Unknown offer | Missing data | NOT_FOUND | app, PostgreSQL, gRPC mapping |
| JT45-12 Intended applicant accepts/declines | State transition | ACCEPTED/DECLINED, updated timestamp; other fields preserved | domain, app, gRPC, PostgreSQL |
| JT45-13 Wrong applicant/employer/admin responds | Role/ownership | PERMISSION_DENIED | app, gRPC; PostgreSQL owner predicate |
| JT45-14 Repeat decision or reversal | State transitions | FAILED_PRECONDITION | original domain tests, app, PostgreSQL |
| JT45-15 Duplicate offer, including after decline | Lifetime uniqueness | ALREADY_EXISTS; original retained | app, PostgreSQL |
| JT45-16 Simultaneous MakeOffer requests | Concurrency | One insert succeeds, other ALREADY_EXISTS | PostgreSQL integration |
| JT45-17 Concurrent accept/decline and same decisions | Concurrency | Exactly one transition succeeds; loser FAILED_PRECONDITION | app concurrent loser; PostgreSQL integration with race detector |
| JT45-18 Repository failures/cancellation | Failure partitions | Distinct infrastructure errors; safe INTERNAL/cancellation mapping | participant unit, app, PostgreSQL closed pool, gRPC mapping |
| JT45-19 Migrations reapplied | Integration | Migration history preserved, all migrations applied once | JT-20 readiness and integration setup |
| JT45-20 UUID, NOT NULL, status, length and uniqueness constraints | Invalid partitions | PostgreSQL rejects invalid rows; no cross-service foreign keys | participant/offer integration |
| JT45-21 Registration, metadata, health, metrics, shutdown | Wiring/lifecycle | Public service registered; real gRPC uses metadata; existing endpoints/shutdown preserved | server unit tests |
| JT45-22 Embedded, trailing and standalone NUL in terms | Invalid partition/regression | ErrInvalidTerms / INVALID_ARGUMENT before any repository call | domain; app and gRPC mocks reject unexpected repository calls |

Integration tests use synthetic SQL fixtures only. No production bypass populates participants.

## Sprint 2 follow-ups

Implement the idempotent application.submitted consumer and offer event publishing. Until then the runtime participant table is empty unless existing trusted data is present, so MakeOffer returns NOT_FOUND. Agree lifecycle eligibility, interview completion, employer verification and withdrawal handling and their trusted sources before adding checks. Application Status continues to own lifecycle transitions.

## Local verification

Run from the repository root:

```sh
go generate ./services/offer/internal/ports/...
go test ./services/offer/...
go test -tags=integration ./services/offer/...
go test -race -tags=integration ./services/offer/internal/adapters/postgres/...
go vet ./services/offer/...
go test ./tests/contract/...
go test ./...
git diff --check
```

Docker must be available for Testcontainers. For Compose, set the required auth/gateway variables in your local environment or untracked .env; do not commit credentials. Then run:

```sh
docker compose -f deploy/compose/docker-compose.yml config --quiet
docker compose -f deploy/compose/docker-compose.yml build offer
docker compose -f deploy/compose/docker-compose.yml up -d --wait offer
docker compose -f deploy/compose/docker-compose.yml exec -T offer wget -S -O- http://localhost:8080/healthz
```

Offer exposes neither 9090 nor 8080 on the host. The gateway already routes to offer:9090. Resolve local infrastructure port conflicts before the standard Compose startup; do not stop another project's containers without agreement.

## Verification results

Executed on 8 November 2026:

- `go generate ./services/offer/internal/ports/...`: passed; generated repository and clock mocks are included.
- `go test ./services/offer/...`: passed.
- `go test -tags=integration ./services/offer/...`: passed; PostgreSQL package completed in 7.868 seconds, including original participant tests and new Offer tests.
- `go test -race -tags=integration ./services/offer/internal/adapters/postgres/...`: passed in 9.481 seconds.
- `go vet ./services/offer/...`: passed.
- `go test ./tests/contract/...`: passed.
- `go test ./...`: passed across the repository.
- `gofmt -l services/offer` and `git diff --check`: no output, passed.
- Focused unit coverage: app 100.0%, domain 64.3%, gRPC adapters 98.7% of statements. Domain package coverage excludes its use by app tests; no 80% requirement applies to Offer.
- Compose configuration validation and Offer image build passed with temporary synthetic auth variables supplied on the command line because the local environment lacked ADMIN_EMAIL, ADMIN_PASSWORD and JWT_SECRET. The local .env was not changed.
- Standard Compose startup failed because another project's `hotel-rabbitmq` owns host ports 5672/15672. No other project's container was stopped. A separate `jt45-check` Compose project, using a temporary override to remove only PostgreSQL/RabbitMQ host port publishing, built and started all three services successfully. Offer became healthy and in-container `wget -S -O- http://localhost:8080/healthz` returned `HTTP/1.1 200 OK` and `ok`.

Go build-cache and Docker access initially needed sandbox retries. Early test compilation caught unused imports during implementation; the final test commands above passed after correction. No CI run, commit, push, PR or Jira update was performed. The task's merge/CI/time-tracking Definition of Done remains outside this implementation run.

To reproduce the isolated check while the host RabbitMQ ports are occupied:

```sh
cat > /tmp/jobtrack-jt45-compose-override.yml <<'YAML'
services:
  postgres:
    ports: !reset []
  rabbitmq:
    ports: !reset []
YAML
export ADMIN_EMAIL=offer-check@example.test
export ADMIN_PASSWORD=OfferCheckSynthetic123
export JWT_SECRET=offer-check-synthetic-secret-32-bytes
docker compose -p jt45-check -f deploy/compose/docker-compose.yml -f /tmp/jobtrack-jt45-compose-override.yml up -d --build --wait --wait-timeout 60 offer
docker compose -p jt45-check -f deploy/compose/docker-compose.yml -f /tmp/jobtrack-jt45-compose-override.yml exec -T offer wget -S -O- http://localhost:8080/healthz
docker compose -p jt45-check -f deploy/compose/docker-compose.yml -f /tmp/jobtrack-jt45-compose-override.yml down
```

These synthetic auth variables satisfy configuration interpolation only; the check starts Offer, PostgreSQL and RabbitMQ, not auth or gateway. Existing PostgreSQL/RabbitMQ credentials still come from the local Compose environment. The isolated containers/network were removed after verification; test volumes were retained. The initial standard startup recreated the JobTrack PostgreSQL/RabbitMQ containers before encountering the port conflict; it did not remove volumes.


## Final review fixes

Offer terms now reject U+0000 using the existing ErrInvalidTerms error. Embedded/trailing NUL, empty/whitespace terms and over-limit Unicode terms have application and gRPC regression coverage that permits no repository calls. Domain tests also cover NUL-only input while preserving valid terms and length boundaries. The NUL tests reproduced the review finding before the validator fix.

Response status expectations use explicit protobuf PENDING, ACCEPTED and DECLINED constants, independently of the production statusToProto helper.
