# JT-19 test cases and verification evidence

## Scope and evidence rules

Jira: JT-19, Build the golden service template. Follow [AGENTS.md](../../AGENTS.md), especially Sections 10 and 13. These owner-approved case identifiers are local to JT-19, not a project-wide naming convention. The template is a reference service, not a 21st production microservice.

Implementation verified in the preceding task reports: `21a7ac0d851c35bf88cc829620a944ba940f551d` on `JT-19-golden-service-template`. This document transcribes those execution reports; no Go tests, Docker builds, integration tests or Compose environments were rerun while writing it. Commands below are reproduction instructions, not claims of new execution. Run Go commands from the repository root.

## Recent execution reports

| Task step | Command or check | Reported result |
|---|---|---|
| Step 3 | `go test ./...` | PASS, exit 0 after authorized sandbox retry; some package results cached |
| Step 3 | `go test -cover ./...` | PASS, exit 0; package coverage below |
| Step 3 | `go vet ./...` | PASS, exit 0 |
| Step 3 | `go build ./...` | PASS, exit 0 |
| Step 3 | `bash scripts/new-service_test.sh` | PASS, exit 0; generated build and uncached generated-service tests passed in a temporary fixture |
| Step 4 | `docker build --progress=plain -f services/service-template/Dockerfile -t jobtrack-jt19-verification:21a7ac0 .` | PASS, exit 0; cached build layers reused |
| Step 4 | `docker image inspect jobtrack-jt19-verification:21a7ac0` and stopped-container file inspection | PASS; executable, CA certificates and matching migration present; inspection container never started and was removed |
| Step 4 | `kubectl kustomize deploy/k8s/service-template` | PASS, exit 0; local rendering only |
| Step 5 | `go test -count=1 -tags=integration ./pkg/... ./services/service-template/...` | PASS, exit 0 after explicit authorization to retry outside the sandbox; no cached test results |
| Step 6 | Archived HEAD, generated `jt19-smoke`, temporary Compose build/start, readiness, HTTP, Goose, broker connection and cleanup checks | PASS; checked commands exited 0; generated image build reused cached layers |

Initial sandbox attempts returned exit 1 for restricted cache, Docker socket or local-listener access. Those attempts were BLOCKED by the environment, not evidence that integration assertions passed. The successful retries were explicitly authorized. Step 5 confirmed all disposable test containers, including Ryuk, disappeared and pre-existing container IDs remained present. Step 6 confirmed its containers, network, volumes, image and directory were removed; the 22 pre-existing container IDs and real repository file hashes were unchanged.

Step 6 workspace was `/private/tmp/jt19-compose.jS7O0H` with project `jt19-smoke-413c9d32ab`; it no longer exists. Its commands included `docker compose -p jt19-smoke-413c9d32ab -f /private/tmp/jt19-compose.jS7O0H/compose.yaml config`, `up -d --build`, and `down --volumes`. Use the guide's unique-workspace recipe for a new run rather than reusing deleted paths. This document does not contain retained raw logs and does not substitute for independently reproduced CI evidence.

## Reported package coverage

These are statement percentages from Step 3, not aggregate repository coverage or an assertion that every package meets a threshold.

| Package | Coverage |
|---|---:|
| `pkg/authctx` | 100.0% |
| `pkg/db` | 82.4% |
| `pkg/events` | 81.4% |
| `pkg/grpcclient` | 90.5% |
| `pkg/logging` | 100.0% |
| `services/service-template/cmd/server` | 83.2% |
| `services/service-template/internal/adapters/grpc` | 100.0% |
| `services/service-template/internal/adapters/postgres` | 90.0% |
| `services/service-template/internal/adapters/rabbitmq` | 83.3% |
| `services/service-template/internal/app` | 100.0% |
| `services/service-template/internal/domain` | 100.0% |

Generated protobuf packages reported 0.0%; template ports had no test files; `tests/contract` reported no statements. AGENTS.md's explicit 80% target applies to `application` and `application-status`, not a blanket threshold for JT-19.

## Acceptance test cases

Each case below records the relevant criterion, inputs, expected result, exact existing test references, reproduction command and reported execution status. The focused commands were not necessarily run separately in the recent sequence; their tests were included in the broader commands recorded above.

### JT19-A-01: Hexagonal architecture and gRPC health behavior

- **Criterion A:** Domain, ports and adapters with a sample gRPC endpoint.
- **Objective and inputs:** Inspect infrastructure-independent domain and inward imports; exercise healthy, unhealthy, unknown/invalid status, empty/unknown service names, nil request, cancellation, deadline and application errors.
- **Expected:** Adapter delegates through `ports.HealthChecker`; empty-service healthy RPC returns SERVING; unknown service gives NOT_FOUND; errors do not leak application details. No new protobuf contract.
- **Test references:** `services/service-template/internal/domain/health_test.go` (`TestHealthStatus_IsHealthy`); `services/service-template/internal/app/health_test.go` (`TestHealth_CheckHealth`, `TestHealth_CheckHealth_UsesReadinessPort`); `services/service-template/internal/adapters/grpc/health_test.go` (`TestHealthHandler_Check_DelegatesAndMapsResults`, `TestHealthHandler_Check_RejectsInvalidRequestsBeforeApplication`, `TestHealthHandler_Check_ThroughGRPC`).
- **Command:** `go test -count=1 ./services/service-template/internal/...`
- **Evidence/status:** PASS in prior code inspection and recent Go runs, including uncached Step 5. Earlier live container RPC evidence is historical, described below.

### JT19-B-01: Database startup and migration failure handling

- **Criterion B:** pgx adapter and Goose migrations on startup.
- **Objective and inputs:** Missing/blank URLs, invalid configuration, failed connection/migration, missing directory, listener failure and shutdown with injected factories.
- **Expected:** Connect before migration; migrate before RabbitMQ/listeners; errors prevent serving, hide credentials and release owned resources while preserving caller pool ownership.
- **Test references:** `pkg/db/db_test.go` (`TestOpenPool_RejectsInvalidConfiguration`, `TestOpenPool_ReturnsConnectionFailureWithoutCredentials`, `TestMigrate_RejectsInvalidInputs`, cancellation tests); `services/service-template/cmd/server/main_test.go` (`TestLoadConfig`, `TestRunWithDatabase_OrdersStartupAndCleansUp`, `TestStartup_RabbitMQOrderFailuresAndCleanup`).
- **Command:** `go test -count=1 ./pkg/db ./services/service-template/cmd/server`
- **Evidence/status:** PASS in recent Step 3 and uncached Step 5 package runs; focused command is a reproduction instruction.

### JT19-B-02: Real PostgreSQL migrations and cancellation

- **Criterion B:** Real migration execution and resource cleanup.
- **Objective and prerequisites:** Disposable `postgres:17-alpine`, accessible Docker daemon; repeated migration, invalid SQL, duplicate version, cancellation and closed-pool readiness. No production URL environment variables required.
- **Expected:** Migration applied once, failed/canceled changes rolled back, canceled version not recorded, single-connection pool usable after cleanup; template version 1 recorded once and closed pool not ready.
- **Test references:** `pkg/db/db_integration_test.go` (`TestMigrate_AppliesSQLAndReleasesResources` and its rollback/provider-loading/cancellation subtests); `services/service-template/internal/adapters/postgres/readiness_integration_test.go` (`TestReadiness_RealPostgresAndInitialMigration`).
- **Command:** `go test -count=1 -tags=integration ./pkg/db ./services/service-template/internal/adapters/postgres`
- **Evidence/status:** PASS in uncached Step 5 combined integration run; no cleanup failures reported. Testcontainers containers and Ryuk were subsequently absent.

### JT19-C-01: RabbitMQ publishing and error handling

- **Criterion C:** RabbitMQ publisher adapter.
- **Objective and prerequisites:** Existing ResumeUploaded envelope; invalid metadata, canceled context, dial/channel/exchange/confirm errors, publish failure, NACK, missing confirmation, close errors; disposable `rabbitmq:4.1-alpine` for real routing.
- **Expected:** Durable topic exchange `jobtrack.events`, event-type routing, persistent `application/json`, unchanged envelope, awaited broker ACK; errors returned and resources cleaned exactly once. Broker ACK does not prove consumer processing.
- **Test references:** `pkg/events/envelope_test.go`; `pkg/events/publisher_test.go` (`TestPublisher_PublishesPersistentJSONAndWaitsForAck`, `TestPublisher_ReportsPublishAndConfirmationFailures`, initialization/validation/cleanup tests); `pkg/events/publisher_integration_test.go` (`TestPublisher_IntegrationRoutesEnvelopeToBoundQueue`); `services/service-template/internal/adapters/rabbitmq/publisher_test.go` (delegation, errors, close, blank URL); its `publisher_integration_test.go` (`TestPublisher_IntegrationRoutesEnvelopeToBoundQueue`).
- **Command:** `go test -count=1 -tags=integration ./pkg/events ./services/service-template/internal/adapters/rabbitmq`
- **Evidence/status:** PASS in Step 5 combined run, including both real broker round-trips. No publishing, startup or cleanup failures reported in the authorized run.

### JT19-D-01: gRPC timeout, retries, circuit breaker and metadata

- **Criterion D:** Shared resilient outgoing gRPC helper.
- **Objective and inputs:** UNAVAILABLE recovery/exhaustion, other status codes, canceled/expired contexts, shorter caller deadline, repeated failures, complete/missing identity, public/internal RPC names and existing outgoing metadata.
- **Expected:** Two-second total default timeout, at most three retries after initial attempt only on UNAVAILABLE, caller cancellation respected, breaker opens after repeated unavailable calls; public identity deduplicated and metadata preserved across retries; no automatic incoming user identity forwarding to InternalService.
- **Test references:** `pkg/grpcclient/client_test.go`, including `TestUnaryClientInterceptor_RetriesOnlyUnavailable`, `TestUnaryClientInterceptor_DefaultTimeoutCancelsCall`, `TestUnaryClientInterceptor_OpensCircuitAfterRepeatedFailures`, `TestUnaryClientInterceptor_ForwardsOnlyCompletePublicCallerIdentity`, `TestUnaryClientInterceptor_DeduplicatesIdentityAcrossRetries` and context/deadline tests; `pkg/authctx/authctx_test.go` (`TestFromContext_ExtractsOnlyCompleteIncomingIdentity`).
- **Command:** `go test -count=1 ./pkg/grpcclient ./pkg/authctx`
- **Evidence/status:** PASS in recent Go runs and uncached Step 5. Half-open recovery is not specifically covered by these tests; internal-call security headers/interceptors remain deferred under AGENTS.md Section 16.

### JT19-E-01: Health, metrics and JSON logging

- **Criterion E:** Health checks, structured JSON logs and metrics.
- **Objective and inputs:** Healthy/unhealthy/unknown checker, internal error, unsupported HTTP methods, contextual log attributes, server failure and shutdown.
- **Expected:** `/healthz` returns 200 with `ok` or masked 503; `/metrics` returns 200, correct Prometheus content type/HELP/TYPE and readiness 1/0; JSON logs include service. Both endpoints share HTTP_PORT; graceful shutdown closes listeners.
- **Test references:** `services/service-template/cmd/server/http_test.go` (`TestMetrics_ReadinessExposition`, `TestHTTPRoutes_RejectUnsupportedMethods`); `services/service-template/cmd/server/main_test.go` (`TestHealthHandler_HTTPStatus`, `TestServe_RegistersHealthAndShutsDownBothServers`, server-failure/binding/shutdown tests); `pkg/logging/logging_test.go` (`TestNew_EmitsJSONWithServiceMessageAndLevel`, `TestNew_PreservesContextualAttributes`).
- **Command:** `go test -count=1 ./services/service-template/cmd/server ./pkg/logging`
- **Evidence/status:** PASS in recent Go runs; Step 6 also reported live HTTP health/metrics success, startup JSON logs without errors and zero restarts. The readiness gauge monitors the existing database check, not continuous broker health.

### JT19-F-01: Docker image build and runtime contents

- **Criterion F:** Multi-stage Dockerfile.
- **Objective and prerequisites:** Docker daemon, repository-root build context; inspect final image and packaged files without running the service.
- **Expected:** Runtime stage, non-root `10001:10001`, `/app`, executable entry point, CA certificates, ports 9090/8080 and migration at `/app/services/service-template/migrations/00001_initial.sql` matching source.
- **References:** `services/service-template/Dockerfile`, `services/service-template/migrations/00001_initial.sql`, `services/service-template/cmd/server/main.go` (`loadConfig`). No dedicated automated image-content test is committed.
- **Commands:** `docker build --progress=plain -f services/service-template/Dockerfile -t jobtrack-jt19-verification:21a7ac0 .`; `docker image inspect jobtrack-jt19-verification:21a7ac0`; stopped-container file checks in the [deployment guide](../deployment/jt-19-golden-service.md).
- **Evidence/status:** PASS in Step 4 with cached layers. Packaged executable, certificates and matching migration were checked using a container that was never started and was then removed. The guide's shell reproduction block was not executed in the documentation step.

### JT19-F-02: Kubernetes manifest rendering

- **Criterion F:** Kubernetes manifest.
- **Objective and prerequisites:** Existing kubectl; local Kustomize render, no cluster contact.
- **Expected:** Exactly Deployment and ClusterIP Service `service-template`; consistent selectors; ports 9090/8080; `/healthz` probe on 8080; UID/GID 10001; Secret `service-template-secrets` references `DATABASE_URL` and `RABBITMQ_URL`; no embedded real credentials or hostPort/NodePort/LoadBalancer.
- **References:** `deploy/k8s/service-template/deployment.yaml`, `deploy/k8s/service-template/service.yaml`, `deploy/k8s/service-template/kustomization.yaml`. No dedicated automated manifest test is committed.
- **Command:** `kubectl kustomize deploy/k8s/service-template`
- **Evidence/status:** PASS, Step 4 exit 0 plus rendered-output inspection. The manifest image `jobtrack-service-template:jt-19` differs intentionally from the separate verification tag. Actual cluster deployment NOT TESTED; rendering is not deployment validation.

### JT19-G-01: Service generator safety and buildability

- **Criterion G:** Copy script creates a new service that builds.
- **Objective and inputs:** Valid kebab-case name, missing/multiple/invalid/reserved names, implemented target, `.gitkeep` placeholder, path with spaces, invocation outside repository, synthetic secret artifacts and induced copy/rename failures.
- **Expected:** Rewritten imports/Dockerfile/default name/migration path; generated service builds and tests pass; existing source and template preserved; secrets excluded; no partial output; placeholder restored on failure.
- **Test references:** `scripts/new-service_test.sh`; implementation `scripts/new-service.sh`. Tests operate only on their own temporary fixture.
- **Command:** `bash scripts/new-service_test.sh`
- **Evidence/status:** PASS, Step 3 exit 0; generated build used `-mod=readonly` and generated tests used `-count=1`. Step 6 independently generated `jt19-smoke` from an archive of HEAD.

### JT19-G-02: Generated service running in isolated Compose

- **Criterion G:** Generated service runs with real local dependencies in Docker Compose.
- **Objective and prerequisites:** Exact committed snapshot outside checkout, unique Compose project, synthetic credentials, PostgreSQL 17 Alpine/RabbitMQ 4.1 Alpine, no host ports and bounded readiness.
- **Expected:** Generated Dockerfile builds; dependencies and service healthy with no unexpected restarts; HTTP health `ok`, readiness gauge 1; Goose rows `0|t` and `1|t`; broker connection established; no startup errors/secrets; all temporary resources removed and real checkout unchanged.
- **References:** `scripts/new-service.sh`, generated copy of `services/service-template/Dockerfile` and `cmd/server/main.go`/`http.go`, `services/service-template/migrations/00001_initial.sql`. This is a manual smoke check; no committed Compose smoke-test script is claimed.
- **Command:** Run the complete Bash block under “Reproducible isolated Compose smoke test” in the [deployment guide](../deployment/jt-19-golden-service.md), using its scoped `config`, `up -d --build --wait --wait-timeout 150`, exec checks and `down --volumes` cleanup.
- **Evidence/status:** PASS in Step 6's equivalent temporary verification, with exit 0 for configuration, build/start, HTTP, migration, broker and cleanup commands. Three containers healthy with zero restarts; no published ports. Its temporary YAML/harness was removed. The newly documented block is a reproduction procedure, not an additional executed run. Official JT-27 integration is still pending.

## Historical evidence

Earlier Step 28 ran a temporary Go client on the isolated Compose network against real `grpc.health.v1.Health/Check`: empty service returned OK (0)/SERVING, unknown service returned NOT_FOUND (5), and an already-expired caller deadline returned DEADLINE_EXCEEDED (4). Calls used two-second deadlines. This was not rerun as a live container RPC in the latest Compose pass. An already-expired deadline check does not establish slow-server timeout behavior.

## Pending checks and Definition of Done

- **Official JT-27 Compose integration:** pending; the isolated recipe does not modify or validate Sangay's configuration.
- **GitHub Actions CI under JT-28:** pending; no CI success is asserted or rechecked during documentation work.
- **Actual Kubernetes cluster deployment:** NOT TESTED. Earlier cluster dry-run was blocked by authentication; local rendering remains separate evidence.
- Documentation and test results alone do not mark JT-19 Done. AGENTS.md Sections 11 and 13 still govern PR workflow, applicable CI, merge, documentation and time tracking. No PR status, merge or Jira changes are part of this document update.
