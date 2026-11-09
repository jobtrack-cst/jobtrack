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

## 6. Entry and exit criteria

### Entry criteria (task level)

Testing of a Jira task starts when:

- The task is In Progress and has acceptance criteria.
- The `.proto` contract and any events it uses exist on `main`.
- The service builds, and the local environment is available (Docker Compose for service checks; Docker running for Testcontainers, with images pulled as in AGENTS.md Section 10).

### Exit criteria (task level)

A task's testing is complete when, in line with the Definition of Done (AGENTS.md Section 13):

- Every acceptance criterion is covered by at least one test case.
- Unit tests pass, and integration tests pass where the task involves a database or events.
- CI is green on the pull request.
- Coverage is at least 80 percent line coverage for `application` and `application-status`.
- Test cases are recorded in `docs/testing/jt-<number>-test-cases.md` using the template in `docs/testing/test-case-template.md`, traced to the Jira key.
- Proposed: no open Critical or Major bug is linked to the task.

### Entry criteria (system level)

End-to-end and load testing start when:

- The main workflow is merged to `main` and runs in Docker Compose (planned for Sprint 3, AGENTS.md Section 14).
- Synthetic test data for all three roles (applicant, employer, admin) is available.

### Exit criteria (system level)

System testing is complete by the end of Sprint 4 (17 Nov) when:

- The Cypress main workflow test in `tests/e2e` passes.
- The JMeter deadline-rush scenario with 200 concurrent users has been run and its report saved in `docs/testing`.
- A SonarQube report has been saved in `docs/testing` for each sprint.
- Proposed: no Critical bug is open, and every open Major bug is agreed with the Product Owner.

## 7. Defect process

Bugs found in testing are logged in Jira as issue type Bug (AGENTS.md Section 10).

### Required fields

| Field | Content |
|---|---|
| Summary | Starts with the service or page, for example `application-status: allows ACCEPTED to SUBMITTED` |
| Description | Steps to reproduce, expected result, actual result, environment (local, CI or cluster) and the commit or branch |
| Severity | One label: `severity-critical`, `severity-major`, `severity-minor` or `severity-trivial` |
| Priority | Jira Priority field (Highest to Lowest), confirmed by the Product Owner |
| Links | The story or task it affects, and the test case ID if one exists |
| Assignee | The owner of the affected service (AGENTS.md Section 4) |

### Severity levels

| Severity | Meaning | Example |
|---|---|---|
| Critical | Main workflow blocked, data loss or a security flaw, no workaround | Gateway accepts an expired JWT |
| Major | Feature wrong but a workaround exists, or a non-core feature fails | `page_size` above 100 is not capped |
| Minor | Small functional or UI problem with low impact | Wrong error message text |
| Trivial | Cosmetic only | Misaligned button or spelling mistake |

Severity is the impact, set by the tester. Priority is how soon to fix it, set or confirmed by the Product Owner.

### Lifecycle

1. The tester creates the Bug in To Do with the fields above.
2. The service owner moves it to In Progress and fixes it on a branch named with the bug's Jira key.
3. The fix is merged through a pull request with green CI (AGENTS.md Section 11). The bug moves to In Review while the PR is open.
4. The tester re-runs the steps to reproduce. If the bug is fixed, it moves to Done; if not, it goes back to In Progress with a comment.
5. New bugs found during a sprint go to the backlog unless they block the sprint goal (AGENTS.md Section 12).

## 8. Open questions for team review

- Pass thresholds for the load test (response time and error rate) are not yet set.
- The team should confirm the severity labels and the Proposed criteria above.
