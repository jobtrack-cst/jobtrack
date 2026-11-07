# JT-31 test cases and verification evidence

## Scope and evidence rules

Jira: JT-31, Implement API gateway with JWT verification (Sprint 1, Must). Branch: `JT-31-api-gateway`. Follow [AGENTS.md](../../AGENTS.md), especially Sections 4, 6, 9, 10 and 13. The case identifiers below are local to JT-31.

All results were run by Jigden Shakya on his laptop (WSL Ubuntu, Docker Desktop). The automated checks ran against commit `2437b3d` with a clean working tree (`git status --short` printed nothing). Times are UTC. Commands below are also reproduction instructions; run them from the repository root.

Test design follows AGENTS.md Section 10: equivalence partitioning (valid tokens versus each class of invalid token, reserved versus ordinary headers), boundary values (secret length, expiry time, port range) and route classes (public, protected, health). Each rejection rule is tested on its own: the test helpers sign valid claims and change exactly one thing.

## Execution results

| Check | Command | Result (`2437b3d`, 7 Nov 2026 16:06) |
|---|---|---|
| Formatting | `gofmt -l gateway` | PASS, no output |
| Static checks | `go vet ./gateway/...` | PASS, no output |
| Unit tests | `go test -count=1 -cover ./gateway/...` | PASS, all 4 packages `ok` |
| Whole repository after merging `main` | `go test -count=1 ./...` | PASS on merge commit `a922897`, no `FAIL` |
| Compose health | `docker compose -f deploy/compose/docker-compose.yml ps gateway auth` | PASS, both `Up 6 minutes (healthy)`, gateway on `0.0.0.0:8000->8080/tcp`, auth ports not published |
| Health endpoint | `curl http://localhost:8000/healthz` | PASS, `HTTP 200`, body `ok` |
| No internal token | `docker compose ... exec gateway env \| grep -c INTERNAL_TOKEN` | PASS, `0` |

The manual register and login check (JT31-A-05) ran at 16:00 UTC on the working tree that was committed unchanged as `2437b3d` (only `deploy/compose/docker-compose.yml` was uncommitted at that time).

The standalone image check (JT31-S-01) ran on commit `41a1f70` on 6 Nov 2026 at 18:23 UTC. The gateway code has not changed since then.

## Unit test coverage (`2437b3d`)

Statement coverage from `go test -count=1 -cover ./gateway/...`. AGENTS.md sets an 80 percent target only for `application` and `application-status`; for the gateway the rule is meaningful tests for every rule.

| Package | Coverage |
|---|---:|
| `gateway/internal/jwtauth` | 100.0% |
| `gateway/internal/middleware` | 100.0% |
| `gateway/internal/backends` | 100.0% |
| `gateway/cmd/server` | 53.5% |

In `cmd/server`, `loadConfig`, `newHandler` and `newAPI` are unit tested. `main`, `run` and `serve` (process start, listening and graceful shutdown) have no unit tests; they were exercised by the container run (JT31-S-01) and the Compose check (JT31-A-05).

## Boundary values

| Input | Below or invalid | On the boundary (valid) | Above or invalid | Test |
|---|---|---|---|---|
| `JWT_SECRET` | empty; 31 bytes | 32 bytes | (no maximum) | `TestNewVerifier_SecretLength`, `TestLoadConfig` |
| Token `exp` | 1 second in the past; missing | 1 second in the future | | `TestVerifier_Verify_AcceptsValidTokens`, `TestVerifier_Verify_RejectsInvalidTokens` |
| Token `alg` | `none`, `HS384`, `HS512`, `RS256` | `HS256` | | `TestVerifier_Verify_RejectsInvalidTokens` |
| Token `role` | empty; `superuser`; `ADMIN` (wrong case) | `applicant`, `employer`, `admin` | | `TestVerifier_Verify_AcceptsValidTokens`, `TestVerifier_Verify_RejectsInvalidTokens` |
| `HTTP_PORT` | `0`; `abc` | unset (default 8080); `9000` | `65536` | `TestLoadConfig` |
| Public route match | `GET /v1/auth/register`; `POST /v1/auth/login/` (trailing slash) | `POST /v1/auth/register`; `POST /v1/auth/login` | | `TestAuthenticate_RejectsWithUnauthenticated`, `TestAuthenticate_AllowsPublicRoutesAndValidTokens` |
| `Authorization` header | missing; `Basic ...`; `Bearer ` with no token; token without scheme | `Bearer <token>`; `bearer <token>` | | `TestAuthenticate_RejectsWithUnauthenticated`, `TestAuthenticate_AllowsPublicRoutesAndValidTokens` |

## Acceptance test cases

### JT31-A-01: Public services only, never any `*InternalService`

- Criterion: Registers grpc-gateway handlers (from `gen/go`) for public services only, never any `*InternalService`.
- Inputs: every `*.pb.gw.go` file in `gen/go`; the gateway's service list; requests to a public route and to an internal RPC path.
- Expected: exactly 20 services, each listed once with a unique `<SERVICE>_ADDR`; the list equals the set of generated `Register...HandlerFromEndpoint` functions; no generated handler name contains `Internal`; `GET /v1/auth/me` is routed (not 404); `POST /jobtrack.auth.v1.AuthInternalService/GetUser` returns 404.
- Tests: `gateway/internal/backends/backends_test.go` (`TestAll_ListsEachServiceOnce`, `TestBackend_AddrEnv`, `TestAll_MatchesGeneratedPublicHandlers`, `TestAll_RegistersOnMuxWithoutInternalRoutes`).
- Command: `go test -count=1 -v ./gateway/internal/backends/`
- Status: PASS. `TestAll_MatchesGeneratedPublicHandlers` fails if HTTP annotations are ever added to an internal service, or if a new public service is generated without being added to the gateway.

### JT31-A-02: JWT verified on every request except register and login

- Criterion: Verifies the JWT on every request except register and login; rejects a bad signature, any algorithm other than HS256, a wrong `iss`, an expired `exp` or an unknown `role` with `UNAUTHENTICATED` (HTTP 401).
- Inputs: see the token rows of the boundary table, plus an empty token, a value that is not a JWT, a token signed with another secret, a missing `iss` and a missing `sub`.
- Expected: every invalid token returns `ErrInvalidToken` from the verifier; through HTTP, every rejection returns status 401 with body `{"code":16,"message":"missing or invalid token","details":[]}` and never reaches the backend; register and login pass without a token (a bad token on login is ignored); a valid token attaches the caller's identity to the request.
- Tests: `gateway/internal/jwtauth/verifier_test.go` (`TestNewVerifier_SecretLength`, `TestVerifier_Verify_AcceptsValidTokens`, `TestVerifier_Verify_RejectsInvalidTokens`, 15 rejection subtests); `gateway/internal/middleware/middleware_test.go` (`TestAuthenticate_RejectsWithUnauthenticated`, 13 subtests; `TestAuthenticate_AllowsPublicRoutesAndValidTokens`, 5 subtests).
- Command: `go test -count=1 -v ./gateway/internal/jwtauth/ ./gateway/internal/middleware/`
- Status: PASS.

### JT31-A-03: Client-sent identity and internal metadata removed

- Criterion: Removes client-sent `x-user-*`, `x-internal-*` and `x-caller-*` metadata (for example `Grpc-Metadata-X-User-Role` headers), then sets `x-user-id` = `sub` and `x-user-role` = `role`.
- Inputs: valid applicant token together with `Grpc-Metadata-X-User-Id: attacker`, `Grpc-Metadata-X-User-Role: admin`, plain `X-User-Id` and `X-User-Role`, a lowercase `grpc-metadata-x-user-role` key, `Grpc-Metadata-X-Internal-Token` and `Grpc-Metadata-X-Caller-Service`; the same spoofed headers on the public register route.
- Expected: the metadata a backend receives has exactly one `x-user-id` (the token's `sub`) and one `x-user-role` (`applicant`); no `x-internal-token` or `x-caller-service`; a public route carries no identity at all; non-reserved headers behave exactly like grpc-gateway's `DefaultHeaderMatcher`.
- Tests: `gateway/internal/middleware/middleware_test.go` (`TestGateway_HeaderSpoofing`, 6 subtests through grpc-gateway's real `runtime.AnnotateContext`; `TestIncomingHeaderMatcher_DropsReservedKeys`, 6 subtests; `TestIncomingHeaderMatcher_OtherKeysUseDefault`, 4 subtests); `gateway/cmd/server/main_test.go` (`TestGateway_ForwardsVerifiedIdentityToBackend`, real gateway wiring and `pkg/grpcclient` against a fake auth gRPC server on `127.0.0.1`).
- Command: `go test -count=1 -v ./gateway/internal/middleware/ ./gateway/cmd/server/`
- Status: PASS, and confirmed live in Compose (JT31-A-05, spoofed `admin` headers still returned the applicant's own account).

### JT31-A-04: Host port 8000, `JWT_SECRET`, never `INTERNAL_TOKEN`

- Criterion: Exposed on host port 8000; has `JWT_SECRET` but never `INTERNAL_TOKEN`.
- Inputs: configuration with `INTERNAL_TOKEN` set; missing, empty and 31-byte `JWT_SECRET`; the resolved Compose configuration; the running container's environment.
- Expected: the gateway refuses to start when `INTERNAL_TOKEN` is set or `JWT_SECRET` is shorter than 32 bytes, without printing the value; the Compose gateway block publishes `8000:8080`, sets `JWT_SECRET` from `.env` and has no `INTERNAL_TOKEN` and no RabbitMQ settings.
- Tests: `gateway/cmd/server/main_test.go` (`TestLoadConfig`, 11 subtests).
- Manual: `docker compose -f deploy/compose/docker-compose.yml config | grep -n INTERNAL_TOKEN` printed nothing; `docker compose ... exec gateway env | grep -c INTERNAL_TOKEN` printed `0`; `ps` showed `0.0.0.0:8000->8080/tcp`; a direct run with `JWT_SECRET=short` logged `invalid startup configuration` with `JWT_SECRET must be at least 32 bytes` and exited with code 1.
- Status: PASS.

### JT31-A-05: Compose block and register and login through the gateway

- Criterion: Gateway block added to `deploy/compose/docker-compose.yml`; manual check: register and login through the gateway to auth in Compose.
- Prerequisites: `deploy/compose/.env` with `JWT_SECRET`, `ADMIN_EMAIL` and `ADMIN_PASSWORD` (see `.env.example`).
- Command: `docker compose -f deploy/compose/docker-compose.yml up -d --build gateway`, then the requests below to `http://localhost:8000`.
- Results (7 Nov 2026, 16:00 UTC):

| Request | Result |
|---|---|
| `POST /v1/auth/register` (applicant, no token) | HTTP 200, user created with `role: ROLE_APPLICANT` |
| `POST /v1/auth/login` (correct password) | HTTP 200, `accessToken` of 243 characters, `expiresAt` exactly 1 hour later |
| `GET /v1/auth/me` with the token | HTTP 200, the caller's own account |
| `GET /v1/auth/me` with the token and spoofed `admin` headers | HTTP 200, the same account, still `ROLE_APPLICANT` |
| `GET /v1/auth/me` without a token | HTTP 401, `code` 16, `missing or invalid token` |
| `GET /v1/auth/me` with `Bearer not-a-jwt` | HTTP 401, `code` 16 |
| `POST /v1/auth/login` (wrong password) | HTTP 401 from auth, `invalid email or password` |

- Status: PASS. The gateway log showed `gateway starting` with `"backends":20` and one `request rejected` line for the invalid token; no token or secret appeared in the logs.

### JT31-A-06: AGENTS.md Section 9 conventions

- Criterion: Logging, `/healthz`, `/metrics` and configuration through environment variables.
- Expected: JSON logs through `pkg/logging` with `service: gateway`; `GET /healthz` returns `ok` and `GET /metrics` returns `jobtrack_service_ready 1` without a token; `POST /healthz` is not exempt and returns 401; configuration only from `SERVICE_NAME`, `HTTP_PORT`, `JWT_SECRET` and the 20 `<SERVICE>_ADDR` variables, with a startup error naming any missing address; every backend call goes through `pkg/grpcclient`, with one interceptor per connection.
- Tests: `gateway/cmd/server/main_test.go` (`TestNewHandler_Routes`, 5 subtests; `TestLoadConfig`, including missing `AUTH_ADDR` and blank `SAVED_JOBS_ADDR`).
- Command: `go test -count=1 -v ./gateway/cmd/server/`; health and metrics also checked on the running container (JT31-S-01) and in Compose.
- Status: PASS.

### JT31-S-01: Docker image builds and runs on its own

- Criterion: `gateway/Dockerfile` follows the service template (multi-stage, non-root user, no secrets in the image).
- Command: `docker build -f gateway/Dockerfile -t jobtrack-gateway:dev .`, then `docker run` with an env file holding a generated `JWT_SECRET` and the 20 addresses, publishing `8000:8080`.
- Expected and observed: image built (34.5 MB); `/healthz` returned `ok`; `/metrics` returned `jobtrack_service_ready 1`; `/v1/auth/me` without a token returned HTTP 401; the startup log showed `http_address` `[::]:8080` and `backends` 20; the container environment had no `INTERNAL_TOKEN`.
- Status: PASS on `41a1f70`. A first build attempt failed while downloading modules from `proxy.golang.org` (a network stream error); the retry succeeded without any change.

## Not covered or pending

- Internal-call protection on `*InternalService` (internal token interceptor, `pkg/grpcclient` internal headers, NetworkPolicies): not part of JT-31. The internal-token check is tracked in JT-44 (see the JT-30 test cases); the rest is listed in AGENTS.md Section 16.
- Live calls to services other than auth: only auth runs in Compose so far. Routes to services that are not running return 503 through the gateway until their owners add them.
- A request with no `Authorization` header gets 401 without a log line; only tokens that fail verification are logged.
- Race tests: not run on Jigden's laptop (cgo disabled).
- GitHub Actions CI on the final commit: check on the JT-31 pull request before merging.
