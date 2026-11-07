# JT-32 test cases and verification evidence

## Scope and evidence rules

Jira: JT-32, Implement employer service basic operations (Sprint 1, Must). Branch: `JT-32-employer-basic-operations`. Follow [AGENTS.md](../../AGENTS.md), especially Sections 4, 6, 9, 10 and 13. The case identifiers below are local to JT-32.

All results were run by Jigden Shakya on his laptop (WSL Ubuntu, Docker Desktop) against commit `72920a6` with a clean working tree (`git status --short` printed nothing). Times are UTC. Commands below are also reproduction instructions; run them from the repository root.

The service was created with `bash scripts/new-service.sh employer` from the empty `.gitkeep` placeholder (commit `7374e22`). The public contract `proto/jobtrack/employer/v1/employer.proto` and the generated code are unchanged.

Test design follows AGENTS.md Section 10: equivalence partitioning (each caller role, owner versus non-owner, valid and invalid input classes), boundary values (each length limit tested on and either side of the limit, counted in characters) and state-based cases (no profile, existing profile, rejected profile edited after a verification change).

The task asks for the repository integration test to follow the JT-20 pattern. JT-20 is not merged yet, so the auth integration test from JT-30 (`services/auth/internal/adapters/postgres/users_integration_test.go`) was used as the reference: Testcontainers with `postgres:18-alpine`, build tag `integration`, migrations applied twice.

## Decisions approved for JT-32

The contract and AGENTS.md left these open. Jigden Shakya (task owner) approved them on 7 Nov 2026; they are also recorded in the AGENTS.md decision log.

1. `GetCompanyProfile` is open to any authenticated role (applicant, employer, admin), so job-posting can call it with any forwarded user identity.
2. `rejection_reason` is returned only to the owning employer or an admin; everyone else gets an empty string.
3. An `employer_id` that is not a UUID is `INVALID_ARGUMENT`; a valid UUID with no profile is `NOT_FOUND`.
4. `CreateCompanyProfile`, `GetMyCompanyProfile` and `UpdateCompanyProfile` are employer only; applicants and admins get `PERMISSION_DENIED`. The role is checked before the input.
5. `UpdateCompanyProfile` replaces all four editable fields, so `company_name` is required. GetMy or Update with no profile is `NOT_FOUND`. Update never changes `verification_status` or `rejection_reason`.
6. Field limits, after trimming spaces and counted in characters: see the boundary table.
7. A non-empty `website` must be an absolute `http://` or `https://` URL with a host.

A forwarded identity whose user ID is not a UUID or whose role is unknown is treated as missing (`UNAUTHENTICATED`), because the gateway never sets such values. New profiles start as `VERIFICATION_STATUS_UNVERIFIED` (the verified flag is false by default).

## Execution results

| Check | Command | Result (`72920a6`, 7 Nov 2026) |
|---|---|---|
| Formatting | `gofmt -l services/employer` | PASS, no output |
| Static checks | `go vet ./services/employer/...` and `go vet -tags=integration ./services/employer/...` | PASS, no output |
| Unit tests | `go test -count=1 -cover ./services/employer/...` | PASS, all 6 packages with tests `ok` (18:09) |
| Integration tests | `go test -count=1 -tags=integration ./services/employer/...` | PASS, all 6 packages `ok` (postgres 15.3 s, rabbitmq 15.7 s, real containers) |
| Compose build and start | `docker compose -f deploy/compose/docker-compose.yml up -d --build employer` | PASS, image built, container started after Postgres and RabbitMQ were healthy |
| Compose health | `docker compose ... ps employer` | PASS, `Up 3 minutes (healthy)` (18:04, same image); ports 8080 and 9090 not published |
| Health endpoint | `docker compose ... exec employer wget -S -qO- http://localhost:8080/healthz` | PASS, `HTTP/1.1 200 OK`, body `ok` (18:10:33) |
| Startup logs | `docker compose ... logs --tail 3 employer` | PASS, `servers starting` with `"service":"employer"`, no errors |
| Migrations on startup | `docker compose ... exec postgres psql -U postgres -d employer -c '\dt'` | PASS, `company_profiles` and `goose_db_version`, both owned by `employer` (18:00) |
| Missing configuration | `docker compose ... run --rm -e DATABASE_URL= employer` | PASS, exits 1 with `invalid startup configuration` and `DATABASE_URL is required`; no password in the output |
| Through the gateway | Manual smoke checks, see JT32-S-03 | PASS, 11 of 11 (18:06) |

## Unit test coverage (`72920a6`)

Statement coverage from `go test -count=1 -cover ./services/employer/...`. AGENTS.md sets an 80 percent target only for `application` and `application-status`; for employer the rule is meaningful tests for every business rule.

| Package | Coverage |
|---|---:|
| `services/employer/internal/domain` | 100.0% |
| `services/employer/internal/app` | 100.0% |
| `services/employer/internal/adapters/grpc` | 97.3% |
| `services/employer/cmd/server` | 83.3% |
| `services/employer/internal/adapters/rabbitmq` | 83.3% (template code, unchanged) |
| `services/employer/internal/adapters/postgres` | 28.3% in the unit run; the repository is covered by the integration tests |

## Boundary values

All text limits count characters after trimming spaces. The database columns (`VARCHAR(n)`, which also counts characters) back up each limit.

| Input | Below or invalid | On the boundary (valid) | Above (invalid) | Test |
|---|---|---|---|---|
| `company_name` | empty; spaces only | 1 character; 150 multibyte characters (`ä`) | 151 characters | `TestNewProfileFields`, `TestProfiles_DatabaseLimits` |
| `description` | | empty; 2000 characters | 2001 characters | `TestNewProfileFields`, `TestProfiles_DatabaseLimits` |
| `website` | no scheme; `ftp://`; `javascript:`; no host; space in host | empty; 255 characters | 256 characters | `TestNewProfileFields`, `TestProfiles_DatabaseLimits` |
| `location` | | empty; 100 characters | 101 characters | `TestNewProfileFields`, `TestProfiles_DatabaseLimits` |
| `employer_id` | empty; `abc`; braces; no hyphens; 35 characters | lower- and upper-case UUID (upper case is lowercased) | | `TestNormalizeEmployerID`, `TestProfiles_Get` |
| Forwarded `x-user-id` and `x-user-role` | missing; user ID not a UUID; role `superuser` or empty | UUID with `applicant`, `employer` or `admin` | | `TestNewCaller`, `TestEmployerHandler_RejectsBadIdentityOnEveryRPC` |

## Authorization matrix

Every cell is covered by table-driven unit tests in `internal/app/profiles_test.go` and `internal/adapters/grpc/employer_test.go`.

| RPC | Employer (owner) | Other employer | Applicant | Admin | No identity |
|---|---|---|---|---|---|
| `CreateCompanyProfile` | OK, keyed by own ID | (own profile only) | `PERMISSION_DENIED` | `PERMISSION_DENIED` | `UNAUTHENTICATED` |
| `GetMyCompanyProfile` | OK, with `rejection_reason` | (own profile only) | `PERMISSION_DENIED` | `PERMISSION_DENIED` | `UNAUTHENTICATED` |
| `UpdateCompanyProfile` | OK | reaches only own profile (`NOT_FOUND` if none) | `PERMISSION_DENIED` | `PERMISSION_DENIED` | `UNAUTHENTICATED` |
| `GetCompanyProfile` | OK, with `rejection_reason` | OK, reason hidden | OK, reason hidden | OK, with `rejection_reason` | `UNAUTHENTICATED` |

## Acceptance test cases

### JT32-A-01: Employer creates, gets and updates their own profile; one profile per employer

- Criterion: Employer creates, gets and updates their own company profile; a second create returns `ALREADY_EXISTS`.
- Inputs: valid create with all fields and with spaces to trim; a second create for the same employer; GetMy with and without a profile; update with new fields; update with no profile.
- Expected: create stores the profile keyed by the caller's user ID with trimmed fields; the second create returns `ALREADY_EXISTS` (enforced by the `employer_id` primary key); GetMy returns the caller's profile or `NOT_FOUND`; update replaces all four editable fields and `updated_at`, keeps `created_at`, or returns `NOT_FOUND`.
- Tests: `internal/app/profiles_test.go` (`TestProfiles_Create`, `TestProfiles_GetMine`, `TestProfiles_Update`); `internal/adapters/grpc/employer_test.go` (`TestEmployerHandler_CreateCompanyProfile`, `TestEmployerHandler_GetMyCompanyProfile`, `TestEmployerHandler_UpdateCompanyProfile`); `internal/adapters/postgres/profiles_integration_test.go` (`TestProfiles_CreateAndGet`, `TestProfiles_Update_KeepsVerificationAndCreatedAt`).
- Command: `go test -count=1 ./services/employer/internal/...` and `go test -count=1 -tags=integration ./services/employer/internal/adapters/postgres/`
- Status: PASS.

### JT32-A-02: Only the owning employer can update; verified flag false by default

- Criterion: Only the owning employer can update a profile; verified flag is false by default.
- Inputs: update as the owner, as another employer, as an applicant and as an admin; a new profile; a profile rejected by a verification change after the employer read it.
- Expected: the update request has no `employer_id`, so the key is always the caller's own ID and another employer can only reach their own profile; applicants and admins get `PERMISSION_DENIED`; new profiles are `VERIFICATION_STATUS_UNVERIFIED`; an update never writes `verification_status` or `rejection_reason`, so a stale read cannot undo a verification change.
- Tests: `internal/domain/caller_test.go` (`TestCaller_RequireEmployer`); `internal/domain/profile_test.go` (`TestNewCompanyProfile_StartsUnverifiedInUTC`, `TestCompanyProfile_WithFields_KeepsVerificationAndCreatedAt`); `internal/app/profiles_test.go` (`TestProfiles_Update`, "another employer only reaches their own profile"); `internal/adapters/postgres/profiles_integration_test.go` (`TestProfiles_Update_KeepsVerificationAndCreatedAt`, stale copy written back).
- Command: `go test -count=1 ./services/employer/internal/...` and `go test -count=1 -tags=integration ./services/employer/internal/adapters/postgres/`
- Status: PASS.

### JT32-A-03: GetCompanyProfile works with forwarded user identity

- Criterion: `GetCompanyProfile` works with forwarded user identity (used by job-posting in Sprint 2).
- Inputs: calls as the owner, another employer, an applicant and an admin; an ID that is not a UUID; an upper-case UUID; an unknown employer; a database failure; a real gRPC call with `x-user-id` and `x-user-role` set as outgoing metadata.
- Expected: any authenticated role gets the profile and its verification status; `rejection_reason` is shown only to the owner and admins; bad IDs return `INVALID_ARGUMENT`; unknown employers return `NOT_FOUND` (job-posting treats this as not verified); database failures return `INTERNAL` with the fixed message `internal error` and no driver detail.
- Tests: `internal/domain/profile_test.go` (`TestCompanyProfile_ViewFor_HidesRejectionReason`); `internal/app/profiles_test.go` (`TestProfiles_Get`); `internal/adapters/grpc/employer_test.go` (`TestEmployerHandler_GetCompanyProfile`, `TestEmployerHandler_HidesUnexpectedErrors`); `cmd/server/employer_setup_test.go` (`TestServe_AnswersEmployerRPCsWithForwardedIdentity`, over a real gRPC connection).
- Command: `go test -count=1 ./services/employer/...`
- Status: PASS. The real job-posting client adapter is Sprint 2.

### JT32-A-04: Role and identity checks use pkg/authctx with the Section 9 codes

- Criterion: Role and ownership checks use `pkg/authctx` and return the gRPC codes from AGENTS.md Section 9.
- Inputs: on every RPC: no metadata, missing role, user ID not a UUID, unknown role; each domain error.
- Expected: identity problems return `UNAUTHENTICATED` on all four RPCs; wrong role `PERMISSION_DENIED`; bad input `INVALID_ARGUMENT`; missing profile `NOT_FOUND`; duplicate `ALREADY_EXISTS`; canceled and expired contexts `CANCELED` and `DEADLINE_EXCEEDED`; anything else `INTERNAL`.
- Tests: `internal/domain/caller_test.go` (`TestNewCaller`, `TestParseRole_RejectsUnknownValues`); `internal/adapters/grpc/employer_test.go` (`TestEmployerHandler_RejectsBadIdentityOnEveryRPC` and the per-RPC tables).
- Command: `go test -count=1 ./services/employer/internal/...`
- Status: PASS.

### JT32-S-01: Migrations, repository and database constraints

- Criterion: goose migrations run on startup; maximum lengths for free-text fields are set; repository integration test with Testcontainers.
- Prerequisites: Docker; `docker pull postgres:17-alpine postgres:18-alpine rabbitmq:4.1-alpine` (17 is used by the template readiness test).
- Expected: migrations apply and can run twice; timestamps are stored and returned in UTC with microsecond precision; one profile per employer; unknown IDs map to `ErrProfileNotFound`; the database rejects values over each limit, an empty company name and an `employer_id` that is not a UUID.
- Tests: `internal/adapters/postgres/profiles_integration_test.go` (`TestProfiles_CreateAndGet`, `TestProfiles_Update_KeepsVerificationAndCreatedAt`, `TestProfiles_DatabaseLimits` with 7 subtests); `internal/adapters/postgres/profiles_test.go` (`TestStatusMapping_RoundTrips`, `TestStatusMapping_RejectsUnknownValues`); `internal/adapters/postgres/readiness_integration_test.go` (template).
- Command: `go test -count=1 -tags=integration ./services/employer/internal/adapters/postgres/`
- Status: PASS.

### JT32-S-02: Docker Compose startup and health

- Criterion: Service block in `deploy/compose/docker-compose.yml`; `docker compose up` starts it and `GET /healthz` returns 200.
- Prerequisites: `deploy/compose/.env` from `.env.example`.
- Expected: the image builds; employer waits for healthy Postgres and RabbitMQ, applies migrations and becomes healthy; `/healthz` returns 200; ports 9090 and 8080 are not published to the host; the database password is passed as `PGPASSWORD`, not inside the URL.
- References: `deploy/compose/docker-compose.yml` (employer block), `services/employer/Dockerfile`, `cmd/server/main_test.go` (`TestHealthHandler_HTTPStatus`, startup order tests).
- Command: the Compose commands in the execution results table.
- Status: PASS on `72920a6`.

### JT32-S-03: Public RPCs through the API gateway (manual)

- Purpose: extra evidence that the four routes work end to end with real JWTs from auth. Not required by the acceptance criteria.
- Setup: `docker compose -f deploy/compose/docker-compose.yml up -d --build auth employer gateway`; a new synthetic employer and applicant registered and logged in through `POST /v1/auth/register` and `POST /v1/auth/login` on `http://localhost:8000`.
- Result on `72920a6` (7 Nov 2026 18:06 UTC), run with a local script that is not committed:

| # | Request | Caller | Expected | Result |
|---|---|---|---|---|
| 1 | `POST /v1/employer/profile` | employer | 200, own `employerId`, `VERIFICATION_STATUS_UNVERIFIED` | PASS |
| 2 | `POST /v1/employer/profile` again | employer | 409 (`ALREADY_EXISTS`) | PASS |
| 3 | `GET /v1/employer/profile` | employer | 200, own profile | PASS |
| 4 | `PUT /v1/employer/profile` with name and location only | employer | 200, website cleared (full replace) | PASS |
| 5 | `PUT /v1/employer/profile` with a spaces-only name | employer | 400 (`INVALID_ARGUMENT`) | PASS |
| 6 | `GET /v1/companies/{employer_id}` | applicant | 200, no `rejectionReason` | PASS |
| 7 | `POST /v1/employer/profile` | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 8 | `PUT /v1/employer/profile` | applicant | 403 (`PERMISSION_DENIED`) | PASS |
| 9 | `GET /v1/companies/not-a-uuid` | applicant | 400 (`INVALID_ARGUMENT`) | PASS |
| 10 | `GET /v1/companies/00000000-0000-4000-8000-000000000000` | applicant | 404 (`NOT_FOUND`) | PASS |
| 11 | `GET /v1/employer/profile` without a token | none | 401 (`UNAUTHENTICATED`) | PASS |

## Not covered or pending

- Consuming `employer.verified` and `employer.verification_rejected`: Sprint 2, not part of JT-32. Until then every profile stays `UNVERIFIED`; the update path already leaves the verification columns alone.
- Real job-posting client and event consumers: Sprint 2. Employer calls no other services, so there are no client ports in Sprint 1.
- Internal-token protection: not relevant, because employer has no `*InternalService`.
- Race tests on Jigden's laptop: not run (cgo disabled).
- Whole repository after merging `main`, and GitHub Actions CI on the final commit: checked before merging the PR.
