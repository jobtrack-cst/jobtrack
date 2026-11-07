# JT-30 test cases and verification evidence

## Scope and evidence rules

Jira: JT-30, Implement auth service basic operations (Sprint 1, Must). Pull request: #13. Follow [AGENTS.md](../../AGENTS.md), especially Sections 4, 9, 10 and 13. The case identifiers below are local to JT-30.

All results were run by Jigden Shakya on his laptop (WSL Ubuntu, Docker Desktop) against commit `fa37c14` on branch `JT-30-auth-basic-operations`, with a clean working tree (`git status --short` printed nothing). Times are UTC. Commands below are also reproduction instructions; run them from the repository root.

Test design follows AGENTS.md Section 10: equivalence partitioning (valid and invalid classes for each input), boundary values (each length limit tested on and either side of the limit) and state-based cases (admin seed on an empty versus an existing database).

## Execution results

| Check | Command | Result (`fa37c14`, 6 Nov 2026) |
|---|---|---|
| Formatting | `gofmt -l services/auth` | PASS, no output |
| Static checks | `go vet ./services/auth/...` | PASS, no output |
| Unit tests | `go test -count=1 -cover ./services/auth/...` | PASS, all 7 packages `ok` |
| Integration tests | `go test -count=1 -tags=integration ./services/auth/...` | PASS, all 7 packages `ok` (postgres 13.8 s, rabbitmq 16.5 s, real containers) |
| Compose build and start | `docker compose -f deploy/compose/docker-compose.yml up -d --build auth` | PASS, image rebuilt with the new code, container started |
| Compose health | `docker compose ... ps auth` | PASS, `Up 21 seconds (healthy)`, ports 8080 and 9090 not published |
| Health endpoint | `docker compose ... exec auth wget -S -qO- http://localhost:8080/healthz` | PASS, `HTTP/1.1 200 OK`, body `ok` (18:42:32) |
| Startup logs | `docker compose ... logs --tail 4 auth` | PASS, `admin seed checked` with `created:false`, then `servers starting`; no email or password in the logs |
| Weak admin password | `docker compose ... run --rm -e ADMIN_PASSWORD=change-me auth` | PASS, exits with `invalid startup configuration`; the value is not printed |

Earlier evidence on the same local database (commit `31c9ad3`, 6 Nov 2026 15:34 UTC): the first start logged `admin seed checked` with `created:true`; after `docker compose ... restart auth` it logged `created:false`.

Phuntsho Namgyel reported unit, race and integration checks passing in his PR #13 review. Race tests were not run on Jigden's laptop because cgo is not enabled there (`-race requires cgo`).

## Unit test coverage (`fa37c14`)

Statement coverage from `go test -count=1 -cover ./services/auth/...`. AGENTS.md sets an 80 percent target only for `application` and `application-status`; for auth the rule is meaningful tests for every business rule.

| Package | Coverage |
|---|---:|
| `services/auth/internal/domain` | 96.8% |
| `services/auth/internal/app` | 97.1% |
| `services/auth/internal/adapters/grpc` | 90.9% |
| `services/auth/internal/adapters/security` | 88.2% |
| `services/auth/internal/adapters/rabbitmq` | 83.3% (template code, unchanged) |
| `services/auth/cmd/server` | 82.9% |
| `services/auth/internal/adapters/postgres` | 15.0% in the unit run; the repository is covered by the integration tests |

## Boundary values

| Input | Below or invalid | On the boundary (valid) | Above (invalid) | Test |
|---|---|---|---|---|
| Register password | 7 characters | 8 characters; 72 bytes | 73 bytes | `TestValidatePassword` |
| Login password | (any wrong password) | 72 ASCII bytes; 24 x "ཀ" (72 bytes) | 73 ASCII bytes; 24 x "ཀ" + "a"; 71 x "a" + "é" (72 characters, 73 bytes) | `TestAuth_Login_PasswordByteLimit`, `TestAuthHandler_Login` |
| Admin password | empty; `change-me`; 11 characters | 12 characters | 73 bytes | `TestValidateAdminPassword`, `TestLoadConfig` |
| Email | empty; spaces only; no `@`; display-name form | 254 characters | 255 characters | `TestNormalizeEmail` |
| Full name | empty; spaces only | 1 character; 100 characters; 100 multibyte characters | 101 characters | `TestNormalizeFullName` |
| `JWT_SECRET` | empty | 32 bytes | (no maximum) | `TestNewJWT_SecretLength`; 31 bytes in `TestLoadConfig` |
| `page_size` | -1 | 0 (default 20); 1; 100 | 101 (capped to 100) | `TestNewPage`, `TestAuth_ListUsersByRole` |
| `page_token` | `abc`; `-5`; `+5`; `05` | empty; `0`; `20` | | `TestNewPage` |
| User ID | empty; `abc`; no hyphens; extra character | lower- and upper-case UUID | | `TestValidateUserID` |

## Acceptance test cases

### JT30-A-01: Register applicants and employers; admins cannot self-register

- Criterion: Register and login for applicants and employers; admins cannot self-register.
- Inputs: roles applicant, employer, admin, unspecified, unknown enum value 99; invalid email, name and password; duplicate email.
- Expected: applicant and employer are created with a hashed password and a database-generated UUID; admin, unspecified and unknown roles return `INVALID_ARGUMENT`; invalid input never reaches storage; duplicate email returns `ALREADY_EXISTS`; the password hash is never returned.
- Tests: `internal/domain/user_test.go` (`TestRole_CanSelfRegister`, `TestNewRegistration`); `internal/app/auth_test.go` (`TestAuth_Register`); `internal/adapters/grpc/auth_test.go` (`TestAuthHandler_Register`); `internal/adapters/postgres/users_integration_test.go` (`TestUsers_CreateAndGet`, duplicate email subtest).
- Command: `go test -count=1 ./services/auth/internal/...` and `go test -count=1 -tags=integration ./services/auth/internal/adapters/postgres/`
- Status: PASS.

### JT30-A-02: Login returns a token; failed login gives one UNAUTHENTICATED message

- Criterion: Failed login returns `UNAUTHENTICATED` with the same message for wrong email and wrong password.
- Inputs: valid credentials (email in mixed case); unknown email; wrong password; malformed email; database failure; token signing failure.
- Expected: valid login returns the token, expiry and user; unknown email, wrong password and malformed email all return `UNAUTHENTICATED` with the identical message `invalid email or password`; an unknown email still runs a dummy bcrypt check so timing does not reveal registered emails; a database failure returns `INTERNAL` with the fixed message `internal error` and no SQL detail.
- Tests: `internal/app/auth_test.go` (`TestAuth_Login`, `TestAuth_Login_SameErrorForWrongEmailAndWrongPassword`); `internal/adapters/grpc/auth_test.go` (`TestAuthHandler_Login`); `internal/adapters/security/security_test.go` (`TestBcrypt_HashAndCompare`, empty-hash dummy subtest); `cmd/server/auth_setup_test.go` (`TestServe_AnswersAuthRPCs`, over a real gRPC connection).
- Command: `go test -count=1 ./services/auth/...`
- Status: PASS.

### JT30-A-03: Login passwords over 72 bytes (PR #13 review regression)

- Criterion: bcrypt refuses passwords over 72 bytes, so login must treat them as invalid credentials, not as an internal error.
- Inputs: see the login password row of the boundary table, including the multibyte cases.
- Expected: 72-byte passwords (ASCII and 3-byte characters) are checked by bcrypt normally; anything over 72 bytes returns the same `UNAUTHENTICATED` error as a wrong password, without reaching storage or bcrypt; bytes are counted, not characters.
- Tests: `internal/app/auth_test.go` (`TestAuth_Login_PasswordByteLimit`, 5 subtests); `internal/adapters/grpc/auth_test.go` (`TestAuthHandler_Login`, 73-byte subtest compares the message with a wrong password).
- Command: `go test -count=1 -run 'PasswordByteLimit' -v ./services/auth/internal/app/`
- Status: PASS. The new subtests were also confirmed to fail when the fix is removed.

### JT30-A-04: Passwords hashed with bcrypt

- Criterion: Passwords hashed with bcrypt from `golang.org/x/crypto`.
- Expected: stored value is a `$2a$` bcrypt hash, never the plain password; each hash uses a new salt; wrong password returns invalid credentials; a malformed stored hash is reported as an error, not as wrong credentials; cost outside bcrypt's range is rejected.
- Tests: `internal/adapters/security/security_test.go` (`TestBcrypt_HashAndCompare`, `TestNewBcrypt_RejectsCostOutOfRange`); `cmd/server/auth_setup_test.go` (`TestSetupAuth_SeedsAdminWithoutLoggingSecrets` checks the seeded admin's stored hash with bcrypt).
- Command: `go test -count=1 ./services/auth/internal/adapters/security/ ./services/auth/cmd/server/`
- Status: PASS.

### JT30-A-05: JWT claims and signing

- Criterion: JWT signed with HS256 using `JWT_SECRET`, with the claims in AGENTS.md Section 4.
- Inputs: employer user issued at a time with fractional seconds in UTC+06:00; empty user ID; unspecified role; secrets of 0, 31 and 32 bytes.
- Expected: header `alg` is `HS256`; payload has exactly `sub`, `role`, `iss = jobtrack-auth`, `iat` and `exp = iat + 3600` in whole Unix seconds; expiry returned in UTC; the token verifies with the right secret and HS256 only, and fails with a wrong secret; bad input and secrets under 32 bytes are rejected.
- Tests: `internal/adapters/security/security_test.go` (`TestJWT_Issue_ClaimsMatchAgentsSection4`, `TestJWT_Issue_RejectsBadInput`, `TestNewJWT_SecretLength`); `internal/app/auth_test.go` (`TestAuth_Login`, clock time passed to the issuer).
- Command: `go test -count=1 ./services/auth/internal/adapters/security/`
- Status: PASS.

### JT30-A-06: Admin seeded at startup

- Criterion: Admin seeded from `ADMIN_EMAIL` and `ADMIN_PASSWORD` following AGENTS.md Section 9.
- States and inputs: empty database; database with an admin; admin email already used by a normal account; database failure; missing, placeholder and 11-character passwords; invalid email.
- Expected: one admin is created on an empty database; an existing admin is never updated; weak passwords stop startup before the database is touched; error messages and logs never contain the email or password.
- Tests: `internal/domain/user_test.go` (`TestValidateAdminPassword`); `internal/app/auth_test.go` (`TestAuth_SeedAdmin`); `cmd/server/main_test.go` (`TestLoadConfig`); `cmd/server/auth_setup_test.go` (`TestSetupAuth_SeedsAdminWithoutLoggingSecrets`, `TestSetupAuth_Failures`).
- Command: `go test -count=1 ./services/auth/...`; manual Compose checks in the execution results table.
- Status: PASS, including the live Compose checks (`created:true`, then `created:false` after restart; `change-me` refused).

### JT30-A-07: User lookup RPCs

- Criterion: User lookup RPCs implemented, including `AuthInternalService.GetUser` and `ListUsersByRole`.
- Inputs: `GetCurrentUser` with valid identity, no metadata, unknown role, malformed ID and a deleted account; `GetUser` with valid, malformed and unknown IDs; `ListUsersByRole` with pages, missing pagination, unspecified role and a bad token.
- Expected: `GetCurrentUser` needs `x-user-id` and `x-user-role` from the gateway (`UNAUTHENTICATED` otherwise) and returns only the caller's own account; `GetUser` returns `INVALID_ARGUMENT` or `NOT_FOUND` as appropriate; `ListUsersByRole` sorts by email, fills `total_count`, and returns `next_page_token` only when more results exist.
- Tests: `internal/app/auth_test.go` (`TestAuth_GetCurrentUser`, `TestAuth_GetUser`, `TestAuth_ListUsersByRole`); `internal/adapters/grpc/auth_test.go` (`TestAuthHandler_GetCurrentUser`, `TestInternalHandler_GetUser`, `TestInternalHandler_ListUsersByRole`); `internal/domain/page_test.go` (`TestNewPage`, `TestPage_NextToken`); `internal/adapters/postgres/users_integration_test.go` (`TestUsers_ListByRoleAndExistsWithRole`).
- Command: `go test -count=1 ./services/auth/internal/...` and `go test -count=1 -tags=integration ./services/auth/internal/adapters/postgres/`
- Status: PASS. Internal-token protection is not part of JT-30; it is tracked in JT-44.

### JT30-S-01: Migrations, repository and database constraints

- Criterion: goose migrations run on startup; maximum field lengths are set.
- Prerequisites: Docker; Testcontainers starts `postgres:18-alpine` (and `postgres:17-alpine` for the template readiness test).
- Expected: migrations apply and can run twice; the database generates UUIDs and UTC timestamps; unique, lowercase-email and role constraints hold; not-found cases map to `ErrUserNotFound`.
- Tests: `internal/adapters/postgres/users_integration_test.go` (`TestUsers_CreateAndGet`, `TestUsers_ListByRoleAndExistsWithRole`); `internal/adapters/postgres/readiness_integration_test.go` (`TestReadiness_RealPostgresAndInitialMigration`).
- Command: `go test -count=1 -tags=integration ./services/auth/internal/adapters/postgres/`
- Status: PASS.

### JT30-S-02: Docker Compose startup and health

- Criterion: Service block in `deploy/compose/docker-compose.yml`; `docker compose up` starts it and `GET /healthz` returns 200.
- Prerequisites: `deploy/compose/.env` with `JWT_SECRET`, `ADMIN_EMAIL` and `ADMIN_PASSWORD` (see `.env.example`); passwords used in connection URLs contain only letters and digits.
- Expected: the image builds, auth becomes healthy, `/healthz` returns 200, ports 9090 and 8080 are not published to the host.
- References: `deploy/compose/docker-compose.yml` (auth block), `services/auth/Dockerfile`, `cmd/server/main_test.go` (`TestHealthHandler_HTTPStatus`).
- Command: the Compose commands in the execution results table.
- Status: PASS on `fa37c14`.

## Not covered or pending

- Publishing `user.registered`: Sprint 2, not part of JT-30.
- Internal-token check on `AuthInternalService`: JT-44.
- A live gRPC call through the gateway: the gateway is JT-31. In-process gRPC calls are covered by `TestServe_AnswersAuthRPCs`.
- Race tests on Jigden's laptop: not run (cgo disabled); reported passing by Phuntsho.
- GitHub Actions CI on the final commit: check on PR #13 before merging.
