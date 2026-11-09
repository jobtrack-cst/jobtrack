# JobTrack test plan

## 1. Document control

| Item | Value |
|---|---|
| Project | JobTrack, integrated project for WEB303, SWE302 and SWE303 |
| Jira | JT-22 (parent JT-15, Testing and Quality Assurance) |
| Owner | Kinley Tobgay Lhendrup (testing and QA lead) |
| Backup | Jigden Shakya |
| Status | Draft for team review |
| Version | 0.2, 9 Nov 2026 |
| Source of truth | [AGENTS.md](../../AGENTS.md). If this plan conflicts with it, AGENTS.md wins and this plan is corrected through a pull request. |

## 2. Scope

### In scope

- The 20 microservices (AGENTS.md Section 4), the API gateway and the 5 Knative functions (Section 5).
- Event flows over the RabbitMQ topic exchange `jobtrack.events` and the allowed synchronous gRPC calls (Section 6).
- The main workflow end to end: an employer registers and is verified, posts a job, an applicant applies with a validated resume, the employer reviews, shortlists, interviews and makes an offer, the applicant accepts or declines, and the right people are notified at each step.
- Authorisation: role and ownership checks in every public RPC.
- The frontend pages for the applicant, employer and admin roles.

### Out of scope

Payments, real SMS, real email providers (Mailpit is used), automated resume parsing or ML matching, mobile apps, production hosting, integration with national systems such as BLMIS, and mTLS between services (AGENTS.md Sections 1 and 16). All test data is synthetic.

### Not testable yet

Internal RPC protection is only partly in place (AGENTS.md Section 6). Layer 1 (separate `*InternalService` definitions) exists. JT-44 provides the layer 2 `INTERNAL_TOKEN` interceptors in `pkg/authctx` and `pkg/grpcclient`, but protection is active only after each service owner wires them in. Layer 3 NetworkPolicies are still pending. Tests for each layer are added in the tasks that wire or build it.

## 3. Test levels and approach

| Level | Tool | Where | Rule |
|---|---|---|---|
| Unit | Go testing, testify, gomock | `*_test.go` next to the code, mainly `internal/domain` and `internal/app` | Table-driven tests. Mock ports, not concrete adapters. |
| Integration | Testcontainers | `internal/adapters/...` with build tag `integration` | Real PostgreSQL and RabbitMQ in containers |
| Contract | `buf breaking` in CI, event fixtures | `tests/contract` | Consumers test against shared JSON fixtures of each event |
| End to end and GUI | Cypress | `tests/e2e` | Main workflow through the real UI |
| Load | Apache JMeter | `tests/load` | Deadline-rush scenario, 200 concurrent users |
| Code quality | SonarQube (local) | report in `docs/testing` | Run once per sprint |

Test design techniques: equivalence partitioning, boundary values and state transitions. Test names describe behaviour, for example `TestApplication_Submit_RejectsClosedJob`.

Coverage target: at least 80 percent line coverage on `application` and `application-status`. Other services need meaningful tests for every business rule.

Commands:

- Unit tests: `go test ./...`
- Integration tests: `go test -tags=integration ./...`
- Coverage: `go test -coverprofile=coverage.out ./...` then `go tool cover -func=coverage.out`

CI runs both the unit and coverage suite and the integration suite on every pull request (JT-20).

Sprint approach: in Sprint 1 each service is tested on its own, with cross-service calls as gomock-mocked ports. From Sprint 2, events and real cross-service clients are added and tested (AGENTS.md Sections 14 and 15).

## 4. Tools

| Tool | Purpose |
|---|---|
| Go testing and testify | Unit test framework and assertions |
| gomock (`go.uber.org/mock`, `go tool mockgen`) | Mocks of ports, committed in `internal/ports/mocks` |
| Testcontainers | Real PostgreSQL and RabbitMQ for integration tests |
| buf (lint and `buf breaking`) | Contract checks on `.proto` files in CI |
| Cypress | End-to-end and GUI tests |
| Apache JMeter | Load tests |
| SonarQube (run locally) | Code quality report, once per sprint |
| Mailpit | Test email inbox for notification checks |
| GitHub Actions | CI |
| Jira | Test tasks and defect tracking |

No other tools are added without team agreement (AGENTS.md Section 7).

## 5. Environments

| Environment | Purpose | Notes |
|---|---|---|
| Local development | Unit and integration tests, service checks | Docker Compose with Postgres, RabbitMQ, Mailpit and the services. Gateway on host port 8000. gRPC port 9090 is never published. |
| CI | Automatic checks on every pull request and push to `main` | GitHub Actions: gofmt, `go vet`, unit tests with coverage, integration tests, `buf` checks, generated code check, Docker builds |
| Cluster | Function tests, end-to-end and load tests | k3d with Knative Serving and Eventing on Phuntsho's laptop (16 GiB RAM, Docker Desktop about 7.75 GiB). Capacity for all 20 services is not yet proven (AGENTS.md Section 16). |

Test data is synthetic. Local `.env` passwords use letters and digits only, and no secrets are committed.
