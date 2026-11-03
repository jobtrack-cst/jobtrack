# JobTrack: Master Project Context

This file is the single source of truth for the JobTrack project. It is written for both team members and AI assistants (Claude, ChatGPT, GitHub Copilot).

- Last updated: 3 Nov 2026 (JT-23 Team B contracts)
- Lives in the repo root as `AGENTS.md`. Changes go through a pull request like code.
- If something here conflicts with a chat message, an old copy or an AI suggestion, this file wins. If this file is wrong, fix it through a PR.

---

## 0. How to use this file

### For team members

1. Open a new chat in Claude or ChatGPT.
2. Attach this file (always the latest version from `main`).
3. Paste your Jira task (key, summary, description and acceptance criteria).
4. Start with: "I am <name>. Help me complete this task step by step, following AGENTS.md."
5. Run, test and review everything the AI produces before you open a pull request. Generated code is not done until it builds, passes tests and meets the acceptance criteria.

In VS Code, GitHub Copilot reads this file through `.github/copilot-instructions.md`.

### Rules for AI assistants

When helping a JobTrack team member, you must:

1. Follow the architecture, contracts, conventions and ownership in this file. Do not introduce new frameworks, services, databases, message brokers or tools unless the member says the team has agreed to it.
2. Never change a `.proto` contract, event schema or another team's service unless the task explicitly says so. If the task seems to need a contract change, stop and tell the member to raise it with the team.
3. Stay within the assigned Jira task. Do not implement extra features.
4. Guide step by step for anything involving setup, Git, Docker or Kubernetes. Give exact commands and file paths.
5. Write tests together with the code (see Section 10) and explain how to run them.
6. Do not claim that code works. Tell the member exactly how to verify it.
7. If information is missing from this file, ask the member rather than guessing, and suggest adding the answer to this file.
8. Remind the member at the end to: run tests locally, commit with the Jira key, push, open a PR, and move the Jira card to In Review.

---

## 1. Project overview

JobTrack is a job and internship application portal for students and employers, built as one integrated final project for three modules at the College of Science and Technology (CST), Royal University of Bhutan:

- WEB303 Microservices and Serverless Applications: microservices, serverless components, deployment
- SWE302 Software Testing and Quality Assurance: testing in code, test case documentation
- SWE303 Software Project Management: project management plan, Scrum, Jira

### Users and roles

| Role | Can do |
|---|---|
| `applicant` | Build a profile, upload resumes, search and save jobs, subscribe to job alerts, apply, track status, message employers, accept or decline offers |
| `employer` | Create a company profile, request verification, post and manage jobs, review and shortlist applicants, schedule interviews, make offers, message applicants |
| `admin` | Placement coordinator. Verify employers, moderate job postings, view analytics, receive weekly summaries |

### Main workflow

Employer registers and gets verified, then posts a job. Applicant finds it, applies with a validated resume. Employer reviews, shortlists, interviews and makes an offer. Applicant accepts or declines. Every step notifies the right people.

### Out of scope

Payments, real SMS, real email providers (Mailpit is used instead), automated resume parsing or ML matching, mobile apps, production hosting, integration with national systems such as BLMIS. All data is synthetic.

---

## 2. Team, roles and ownership

| Member | Team | Scrum role | Lead area (extra responsibility) |
|---|---|---|---|
| Phuntsho Namgyel | A | Product Owner and developer | Architecture, contracts, service template, frontend shell |
| Kinley Tobgay Lhendrup | A | Developer | Testing and QA (test plan, Cypress, JMeter, test reports) |
| Jigden Shakya | B | Scrum Master and developer | Scrum, Jira, PM documents |
| Sangay Tenzin | B | Developer | DevOps (Docker Compose, cluster, Knative, CI, monitoring) |

- Pairs: Team A is Phuntsho and Kinley. Team B is Jigden and Sangay.
- Your pair partner is the backup for every service you own and reviews your pull requests.
- Lead area backups: Contracts (Phuntsho, backup Sangay), DevOps (Sangay, backup Phuntsho), Testing (Kinley, backup Jigden), Scrum and Jira (Jigden, backup Kinley).
- Everyone builds services, writes tests for their own services, builds the frontend pages for their own services, and contributes to documentation.

---

## 3. Architecture

```
React web app (applicant, employer, admin)
        |  REST + JWT
        v
API Gateway (Go, grpc-gateway; verifies JWT)
        |  gRPC (user ID and role passed in metadata)
        v
20 Go microservices (hexagonal, each with its own database)
        |  publish domain events
        v
RabbitMQ (topic exchange: jobtrack.events)
        |  consumed by services and by serverless functions
        v
5 Knative serverless functions (3 event-driven, 2 scheduled)
        |
        v
Mailpit (test email inbox)

PostgreSQL: one server, one database and user per service
Monitoring: Prometheus and Grafana
```

Principles:

- Each microservice owns its data. No service reads another service's database.
- Services talk synchronously through gRPC only when they need an answer right now. Everything else goes through events.
- Cross-team synchronous calls are kept to a minimum (listed in Section 6).
- The gateway is the only entry point from the frontend. It is not counted as one of the 20 microservices.

---

## 4. Microservices (20)

Initial design. Exact RPCs and fields are defined in the `.proto` files (Sprint 0 tasks JT-18, JT-21, JT-23). Update this table if responsibilities change.

### Team B (Jigden and Sangay)

Contracts written in JT-23: `proto/jobtrack/<service_snake>/v1/<service_snake>.proto`.

| # | Service (folder) | Owner | Responsibility | Publishes | Consumes |
|---|---|---|---|---|---|
| 1 | `auth` | Jigden | Register, login, issue JWT, user lookup (name, email, role) | `user.registered` | |
| 2 | `employer` | Jigden | Company profiles, verified flag | | `employer.verified`, `employer.verification_rejected` |
| 3 | `company-verification` | Jigden | Verification requests and admin decisions | `employer.verified`, `employer.verification_rejected` | |
| 4 | `moderation` | Jigden | Reports on job postings, admin takedowns | `moderation.job_removed` | |
| 5 | `audit-log` | Jigden | Records key actions from all events (admin view) | | all events |
| 6 | `analytics` | Jigden | Outcome statistics built from events | | `job.published`, `job.closed`, `application.submitted`, `application.withdrawn`, `application.status_changed`, `offer.made`, `offer.accepted`, `offer.declined` |
| 7 | `job-posting` | Sangay | Create, edit, publish and close vacancies | `job.published`, `job.updated`, `job.closed` (also on moderation removal) | `moderation.job_removed` |
| 8 | `category-tag` | Sangay | Categories, skill tags, locations (seeded, read-only) | | |
| 9 | `job-search` | Sangay | Search read model built from job events | | `job.published`, `job.updated`, `job.closed` |
| 10 | `saved-jobs` | Sangay | Bookmarks | | `job.closed` |
| 11 | `notification-inbox` | Sangay | Stores and lists in-app notifications, mark as read | | (written by Notification Dispatcher via gRPC) |

### Team A (Phuntsho and Kinley)

| # | Service (folder) | Owner | Responsibility | Publishes | Consumes |
|---|---|---|---|---|---|
| 12 | `applicant-profile` | Kinley | Applicant details, education, skills | | |
| 13 | `resume` | Kinley | Upload and store resumes (in its database, 2 MB limit), validation status | `resume.uploaded` | `resume.validated`, `resume.rejected` |
| 14 | `application-status` | Kinley | Status lifecycle state machine and history | `application.status_changed` | `application.submitted`, `application.withdrawn`, `review.completed`, `interview.scheduled`, `offer.made`, `offer.accepted`, `offer.declined` |
| 15 | `review` | Kinley | Employer scores and shortlist decisions | `review.completed` | |
| 16 | `interview` | Kinley | Schedule and record interviews | `interview.scheduled` | |
| 17 | `application` | Phuntsho | Submit and withdraw applications, eligibility checks | `application.submitted`, `application.withdrawn` | |
| 18 | `offer` | Phuntsho | Offers; applicant accepts or declines | `offer.made`, `offer.accepted`, `offer.declined` | |
| 19 | `messaging` | Phuntsho | Message threads per application | `message.sent` | |
| 20 | `job-alert` | Phuntsho | Applicant subscriptions; matches new jobs | `job_alert.matched` | `job.published` |

### API Gateway (not counted in the 20)

- Folder: `gateway`. Owner: Jigden. Backup: Sangay.
- Built with grpc-gateway from the HTTP annotations in each service's `.proto`. It registers only public services, never `*InternalService`.
- Verifies the JWT on every request (except register and login), then forwards `x-user-id` and `x-user-role` as gRPC metadata.

### JWT claims format

- Issued by `auth` on login and signed with HS256 using `JWT_SECRET`. Lifetime 1 hour. There are no refresh tokens; the user logs in again.

| Claim | Meaning | Example |
|---|---|---|
| `sub` | User ID (UUID from `auth`) | `3f2b8c1e-...` |
| `role` | `applicant`, `employer` or `admin` | `employer` |
| `iss` | Always `jobtrack-auth` | `jobtrack-auth` |
| `iat` | Issued at, Unix seconds | `1793700000` |
| `exp` | Expires at, Unix seconds (`iat` + 3600) | `1793703600` |

- The gateway rejects a token with a bad signature, any algorithm other than HS256, a wrong `iss`, an expired `exp` or an unknown `role` with `UNAUTHENTICATED` (HTTP 401).
- The gateway first removes any `x-user-*`, `x-internal-*` and `x-caller-*` metadata sent by the client (for example `Grpc-Metadata-X-User-Role` headers), then sets `x-user-id` = `sub` and `x-user-role` = `role`. Clients can never set their own identity.
- `role` values map to the `jobtrack.auth.v1.Role` enum: `applicant` = `ROLE_APPLICANT`, `employer` = `ROLE_EMPLOYER`, `admin` = `ROLE_ADMIN`.

---

## 5. Serverless functions (5, Knative)

| Function (folder) | Owner | Trigger | Responsibility |
|---|---|---|---|
| `notification-dispatcher` | Sangay | Events: `application.submitted`, `application.status_changed`, `interview.scheduled`, `offer.made`, `message.sent`, `job_alert.matched`, `employer.verified`, `deadline.reminder` | Decides who is notified and how. Writes to Notification Inbox (gRPC) and calls Email Sender for emails. Gets user emails from Auth. |
| `email-sender` | Sangay | HTTP call from other functions | Sends email through SMTP to Mailpit |
| `weekly-summary` | Jigden | Weekly schedule (PingSource) | Gets statistics from Analytics and admin emails from Auth, then emails a summary to admins. |
| `resume-validator` | Phuntsho | Event: `resume.uploaded` | Fetches the file from Resume, checks type (PDF only), size (max 2 MB) and content signature. Publishes `resume.validated` or `resume.rejected`. |
| `deadline-reminder` | Kinley | Daily schedule (PingSource) | Finds jobs closing within 48 hours (Job Posting) and who saved them (Saved Jobs). Publishes `deadline.reminder` events. |

Functions are stateless, receive CloudEvents over HTTP, and scale to zero. The exact RabbitMQ to Knative connection is decided by the cluster spike (JT-29) and will be recorded in the Decision log.

---

## 6. Key flows and synchronous calls

### Apply for a job (saga with compensation)

1. Applicant uploads a resume. `resume` stores it as PENDING and publishes `resume.uploaded`.
2. `resume-validator` checks it and publishes `resume.validated` or `resume.rejected`. `resume` updates its status.
3. Applicant applies. `application` calls `job-posting` (is the job open and before deadline?) and `resume` (is the resume VALIDATED and owned by this applicant?). If both pass, it saves and publishes `application.submitted`.
4. `application-status` creates the status SUBMITTED. `notification-dispatcher` notifies the applicant and employer.

### Application status lifecycle (owned by `application-status`)

```
SUBMITTED -> UNDER_REVIEW -> SHORTLISTED -> INTERVIEW -> OFFERED -> ACCEPTED
                                                               \-> DECLINED
REJECTED: allowed from SUBMITTED, UNDER_REVIEW, SHORTLISTED, INTERVIEW
WITHDRAWN: allowed from any non-final state (applicant action)
Final states: ACCEPTED, DECLINED, REJECTED, WITHDRAWN
```

Any other transition must be rejected with `FAILED_PRECONDITION`. Every transition is stored in history and publishes `application.status_changed`.

### Job publishing

`job-posting` publishes `job.published`. `job-search` updates its read model, `job-alert` matches subscriptions and publishes `job_alert.matched`, `analytics` and `audit-log` record it.

### Allowed synchronous gRPC calls

| Caller | Calls | RPC | Why | Cross-team |
|---|---|---|---|---|
| `gateway` | all services | Public services only (RPCs with HTTP annotations) | Frontend requests | Yes |
| `application` | `job-posting` | `JobPostingService.GetJob` (forwards user identity) | Job open check | Yes |
| `application` | `resume` | (defined in JT-21) | Resume validated check | No |
| `job-posting` | `employer` | `EmployerService.GetCompanyProfile` (forwards user identity) | Employer verified check | No |
| `job-alert` | `category-tag` | `CategoryTagInternalService.ResolveNames` | Category and tag names | Yes |
| `notification-dispatcher` | `notification-inbox`, `auth` | `NotificationInboxInternalService.CreateNotification`, `AuthInternalService.GetUser` | Write notification, get email | No |
| `weekly-summary` | `analytics` | `AnalyticsInternalService.GetInternalSummary` | Statistics | No |
| `weekly-summary` | `auth` | `AuthInternalService.ListUsersByRole` | Admin emails | No |
| `deadline-reminder` | `job-posting`, `saved-jobs` | `JobPostingInternalService.ListJobsClosingBetween`, `SavedJobsInternalService.ListJobSavers` | Closing jobs and who saved them | Yes |
| `resume-validator` | `resume` | (defined in JT-21) | Fetch file | No |

Any new synchronous call must be agreed by both teams and added here.

### Public and internal RPCs

- Public RPCs live in `<Name>Service` and have HTTP annotations. The gateway exposes them, and they always check the caller's role and ownership through `pkg/authctx`.
- Internal RPCs live in a separate `<Name>InternalService` with no HTTP annotations. The gateway never registers them. They do not check a user role, because they are called by functions and event consumers where no user is involved.
- When a service calls a public RPC of another service while handling a user request (for example `job-posting` calling `EmployerService.GetCompanyProfile` during `PublishJob`), `pkg/grpcclient` forwards the incoming `x-user-id` and `x-user-role` metadata.

Internal RPCs are restricted to their intended callers in three layers:

1. Contract: internal RPCs are only in `*InternalService`, so they can never get an HTTP route, and servers can apply a stricter interceptor to them.
2. Application: every `*InternalService` call must carry `x-internal-token` equal to `INTERNAL_TOKEN` (constant-time compare) and `x-caller-service` set to the caller's service name. The server rejects a missing or wrong token with `UNAUTHENTICATED`, and a caller not allowed for that RPC (the table above) with `PERMISSION_DENIED`. `pkg/grpcclient` adds both headers. The gateway never has `INTERNAL_TOKEN`.
3. Network: in k3d, a Kubernetes NetworkPolicy per service allows ingress on port 9090 only from its callers in the table above. In Docker Compose, port 9090 is never published to the host.

`x-caller-service` is not proof of identity on its own; any holder of `INTERNAL_TOKEN` could set it. Layers 2 and 3 together limit misuse to compromised internal services. Full per-service identity would need mTLS, which is out of scope (Section 16).

Status: layer 1 is in place (JT-23 contracts). Layers 2 and 3 are designed and agreed by all four members in PR #6, but they are not implemented yet. Until the internal interceptor, `pkg/grpcclient` headers, gateway filtering and NetworkPolicies are built (Section 16), internal RPCs are protected only by not being exposed through the gateway.

---

## 7. Tech stack

| Area | Choice |
|---|---|
| Services and functions | Go (current stable release, pinned in `go.mod`), single Go module for the monorepo |
| RPC | gRPC, Protocol Buffers, `buf` for linting and code generation |
| Gateway | grpc-gateway |
| Database | PostgreSQL (one server, one database and user per service), `pgx`, `goose` migrations run on startup |
| Messaging | RabbitMQ, topic exchange `jobtrack.events` |
| Serverless | Knative Serving and Eventing (PingSource for schedules) |
| Frontend | React, TypeScript, Vite, MUI, React Router |
| Local development | Docker Compose (Postgres, RabbitMQ, Mailpit, services) |
| Cluster | k3d on one host machine |
| Kubernetes manifests | Plain YAML with Kustomize |
| CI | GitHub Actions, images pushed to GitHub Container Registry |
| Monitoring | Prometheus and Grafana, JSON logs |
| Resilience | Shared gRPC client with timeout, retry and circuit breaker (`sony/gobreaker`) |
| Testing | Go testing, testify, gomock (`go.uber.org/mock`), Testcontainers, Cypress, Apache JMeter, SonarQube (run locally) |
| Project management | Jira (space key JT), GitHub |

Do not add other tools without team agreement.

---

## 8. Repository structure

```
jobtrack/
  AGENTS.md                      this file
  README.md
  go.mod
  buf.yaml, buf.gen.yaml, buf.lock
  proto/jobtrack/
    common/v1/common.proto       shared types
    events/v1/events.proto       all domain events
    <service_snake>/v1/<service_snake>.proto   one per service (snake_case, see Section 9)
  gen/go/                        generated code (committed; CI checks it is up to date)
  pkg/                           shared Go libraries
    grpcclient/                  timeout, retry, circuit breaker, forwards user identity, adds internal headers
    events/                      RabbitMQ publisher and consumer helpers, event envelope
    authctx/                     read x-user-id and x-user-role from gRPC metadata
    logging/                     JSON logger setup
    db/                          pgx pool and goose migration helpers
  services/<service>/            one folder per microservice (see Section 9)
  functions/<function>/          one folder per serverless function
  gateway/
  frontend/
  deploy/
    compose/                     docker-compose.yml, Postgres init script
    k8s/                         Kustomize base per service (including NetworkPolicy)
    knative/                     function and event source manifests
  tests/
    e2e/                         Cypress
    load/                        JMeter
    contract/                    event fixtures and consumer tests
  scripts/new-service.sh         creates a service from the template
  docs/
    architecture/  pm/  testing/  deployment/
  .github/
    workflows/                   CI
    pull_request_template.md
    copilot-instructions.md      points to AGENTS.md
```

---

## 9. Coding conventions

### Service layout (hexagonal)

```
services/<service>/
  cmd/server/main.go             wiring only: config, adapters, start servers
  internal/domain/               entities, value objects, business rules (no imports of DB, gRPC or RabbitMQ)
  internal/ports/                interfaces: repositories, event publisher, clients to other services
  internal/app/                  use cases; depend only on domain and ports
  internal/adapters/
    grpc/                        gRPC handlers (convert proto <-> domain, call app)
    postgres/                    repository implementations
    rabbitmq/                    event publisher and consumers
    clients/                     gRPC clients to other services
  migrations/                    goose SQL files
  Dockerfile                     multi-stage build
```

### Naming and configuration

- Folder and service names: kebab-case (`application-status`). Same name in Compose, Kubernetes and Docker images.
- Proto package: `jobtrack.<service_snake>.v1` (for example `jobtrack.application_status.v1`).
- Proto folders and file names use the same snake_case name as the package (for example `proto/jobtrack/application_status/v1/application_status.proto`), because buf STANDARD lint requires the folder to match the package.
- Proto naming follows buf STANDARD lint: service names end in `Service`, each RPC has its own `<Rpc>Request` and `<Rpc>Response`, and enum zero values end in `_UNSPECIFIED`.
- Internal RPCs go in a separate `<Name>InternalService` in the same file, with no HTTP annotations. Each internal RPC's comment names its allowed callers.
- Database and DB user: service name in snake_case (`application_status`).
- Ports inside containers: gRPC on 9090, HTTP (health and metrics) on 8080. Gateway is exposed on host port 8000. Frontend dev server on 5173.
- Configuration only through environment variables: `SERVICE_NAME`, `DATABASE_URL`, `RABBITMQ_URL`, `GRPC_PORT`, `HTTP_PORT`, plus `<SERVICE>_ADDR` for each service it calls. `JWT_SECRET` only in `auth` and `gateway`.
- `INTERNAL_TOKEN` in every service and function that serves or calls an `*InternalService` RPC. Never in `gateway` or `frontend`.
- `ADMIN_EMAIL` and `ADMIN_PASSWORD` only in `auth`. At startup, `auth` creates one admin from them if no admin exists and never updates an existing admin. It stores only a password hash, never logs the values, and refuses to start if `ADMIN_PASSWORD` is missing, equals the `.env.example` placeholder, or is shorter than 12 characters.
- No secrets committed. Use `.env.example` files with placeholder values.

### Data and API rules

- IDs: UUID strings (plain `string` fields, as in `events.proto`). Timestamps: `google.protobuf.Timestamp`, stored in UTC.
- `employer_id` everywhere (events and contracts) is the employer's user ID from `auth`. Each employer account has at most one company profile.
- Authorisation: services read the caller from `pkg/authctx` and check role and ownership themselves (for example only the owning employer can update a job). See Section 6 for internal RPCs.
- Errors: use gRPC status codes. `INVALID_ARGUMENT` for bad input, `UNAUTHENTICATED` for a missing or invalid JWT, a missing or wrong internal token, and failed login (same message for wrong email and wrong password), `NOT_FOUND`, `PERMISSION_DENIED` for wrong role, owner or internal caller, `FAILED_PRECONDITION` for business rule violations (closed job, invalid status transition), `ALREADY_EXISTS`, `INTERNAL` only for unexpected errors.
- List RPCs use the shared pagination messages from `common.proto`.

### Events

- Published to exchange `jobtrack.events`, routing key = event type (for example `application.submitted`).
- Payload: JSON (protojson) of an envelope with `event_id` (UUID), `type`, `occurred_at`, `producer`, and `data` (the event message from `events.proto`).
- Each consuming service has its own durable queue named `<service>.<purpose>`.
- Consumers must be idempotent: ignore an `event_id` already processed.
- Adding a field to an event is allowed. Removing or renaming a field needs both teams' agreement (`buf breaking` enforces this in CI).

### General

- Logging: Go `log/slog` with JSON output through `pkg/logging`. Always include `service` and, where available, `event_id` or `request_id`.
- Every outgoing gRPC call uses `pkg/grpcclient` (default timeout 2 s, up to 3 retries on `UNAVAILABLE`, circuit breaker).
- Health endpoint `GET /healthz` and metrics `GET /metrics` on the HTTP port.
- Keep comments short and useful: explain why, not what.
- Format with `gofmt`. Code must pass `go vet`.

### Frontend

- Folder per feature under `frontend/src/features/<area>/`. Each member builds the pages for their own services.
- One shared API client in `frontend/src/api/` that calls the gateway and attaches the JWT.
- Use MUI components. No custom design system.

---

## 10. Testing conventions

| Level | Tool | Where | Rule |
|---|---|---|---|
| Unit | Go testing, testify, gomock | `*_test.go` next to the code, mainly `internal/domain` and `internal/app` | Table-driven tests. Mock ports, not concrete adapters. |
| Integration | Testcontainers | `internal/adapters/...` with build tag `integration` | Real PostgreSQL and RabbitMQ in containers |
| Contract | `buf breaking` in CI, event fixtures in `tests/contract` | | Consumers test against shared JSON fixtures of each event |
| End to end and GUI | Cypress | `tests/e2e` | Main workflow through the real UI |
| Load | JMeter | `tests/load` | Deadline-rush scenario, 200 concurrent users |
| Code quality | SonarQube (local) | | Run once per sprint, save the report in `docs/testing` |

- Test names describe behaviour: `TestApplication_Submit_RejectsClosedJob`.
- Design test cases with equivalence partitioning, boundary values and state transitions, and record them in the test case document with the Jira story they trace to.
- Coverage target: at least 80 percent line coverage on `application` and `application-status`. Other services: meaningful tests for all business rules.
- Commands: `go test ./...` (unit), `go test -tags=integration ./...` (integration), `go test -cover ./...` (coverage).
- Bugs found in testing are logged in Jira as Bug with severity and steps to reproduce.

---

## 11. Git and GitHub workflow (GitHub Flow)

1. Update main: `git checkout main && git pull`
2. Create a branch named with the Jira key: `git checkout -b JT-21-team-a-contracts`
3. Commit small, with the Jira key first: `git commit -m "JT-21: add application-status proto"`
4. Push: `git push -u origin JT-21-team-a-contracts`
5. All changes to `main` require a pull request. Title starts with the Jira key. Fill in the PR template.
6. Peer reviews are encouraged, especially for shared contracts, shared `pkg/`, and `AGENTS.md`, including cross-team review, but approval is not mandatory for merging.
7. Applicable CI checks must pass before merging, once JT-28 provides the checks. Squash and merge. Delete the branch.

Rules:

- Never push directly to `main` (it is protected).
- Merge at least once a day so `main` stays close to everyone's work.
- `main` must always build and run in Docker Compose.
- If you get a merge conflict, pull `main` into your branch, resolve, run tests, then push. Ask your partner if unsure.

---

## 12. Jira workflow

- Space: JobTrack, key `JT`, site `jobtrack-cst.atlassian.net`.
- Statuses: To Do, In Progress (you are working on it), In Review (PR open), Done (PR merged and meets the Definition of Done).
- Move your own cards as you work. Log time spent on each task (Time tracking field); these hours are used for earned value calculations.
- Estimates use story points (1, 2, 3, 5, 8).
- New work discovered during a sprint goes to the backlog and is discussed with the Product Owner, unless it blocks the sprint goal.

---

## 13. Definition of Done

A task or story is Done only when:

1. All acceptance criteria in the Jira task are met.
2. Code follows this file's conventions and is formatted.
3. Unit tests are written and pass; integration tests where the task involves a database or events.
4. CI is green.
5. A pull request is used for all changes to `main`; peer review is encouraged, but approval is not mandatory for merging.
6. It is merged to `main` and runs in Docker Compose.
7. Related documentation (this file, docs folder, test cases) is updated.
8. Time is logged in Jira.

---

## 14. Sprint plan

| Sprint | Dates | Goal |
|---|---|---|
| Sprint 0 | 2 to 3 Nov | Development foundation: repo, contracts for all 20 services, golden template, CI, local environment, proven cluster setup |
| Sprint 1 | 4 to 6 Nov | All 20 services and the gateway running with basic operations |
| Sprint 2 | 7 to 9 Nov | Core flow working end to end with events and the first functions |
| Sprint 3 | 10 to 12 Nov | Full business logic, all 5 functions, frontend for all roles, integration tests |
| Sprint 4 | 13 to 15 Nov | End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze |
| Buffer | 16 to 17 Nov | Only if needed |

Presentation date: not yet announced (likely to be postponed).

Ceremonies: sprint planning (1 hour), daily stand-up (15 minutes), combined sprint review and retrospective (1 hour). Sprint lengths stay fixed; if a goal is met early, pull the next items from the backlog.

### Current sprint

- Sprint: Sprint 0 (2 to 3 Nov), started on 2 Nov 2026
- Goal: Development foundation ready: repo, contracts for all 20 services, golden template, CI, local environment, and a proven cluster setup.
- Tasks: JT-16 to JT-29 (see Jira)
- Order: JT-16 (repo) and JT-18 (buf and event contracts) first, because JT-21 and JT-23 (service contracts) depend on them. Tasks that do not need the repo yet (charter, test plan, Docker Compose) start in parallel.
- Cluster host machine: Phuntsho's laptop; RAM must still be verified against the 16 GB minimum requirement
- Daily stand-up: Flexible, coordinated by the team each day (15 minutes).

---

## 15. Decision log

Add new decisions at the bottom with the date and a one-line reason.

| Date | Decision | Reason |
|---|---|---|
| 27 Oct 2026 | Build 20 real microservices, not counting the gateway or serverless functions | Safest reading of the tutor's "minimum 20 microservices" |
| 27 Oct 2026 | 5 serverless functions on Knative (SMS Sender from the proposal dropped) | Each function has a real job; SMS would be a stub with no value |
| 27 Oct 2026 | Admin split into Company Verification and Moderation; added Category and Tag, Messaging, Job Alert, Offer, Audit Log | Real features needed to reach 20 microservices without padding |
| 27 Oct 2026 | One PostgreSQL server with a separate database and user per service | Keeps data isolation and saves memory |
| 27 Oct 2026 | Resumes stored in the Resume service database, not a Kubernetes volume | Removes storage setup work |
| 27 Oct 2026 | Contract testing with `buf breaking` and event fixtures instead of Pact | Same purpose with much less tooling friction |
| 27 Oct 2026 | Scrum with Sprint 0 plus four 3-day sprints; Jira and GitHub | Accelerated 14 to 16 day plan with regular feedback |
| 27 Oct 2026 | GitHub Flow instead of GitFlow | Long-lived branches add merge work in a short project |
| 27 Oct 2026 | Jigden is Scrum Master; Phuntsho is Product Owner | Agreed by the team |
| 28 Oct 2026 | Single Go module for the monorepo; generated proto code is committed | Simpler builds and imports for everyone |
| 28 Oct 2026 | Container ports: gRPC 9090, HTTP 8080; gateway on host port 8000 | Consistent configuration across all services |
| 1 Nov 2026 | Development start moved from 28 Oct to 2 Nov; all sprint dates shifted by 5 days | Team was occupied with other coursework; Sprint 0 had not been started |
| 2 Nov 2026 | Sprint 0 started on 2 Nov 2026 | Begin the agreed development foundation work on schedule |
| 2 Nov 2026 | Use Phuntsho's laptop as the cluster host; verify at least 16 GB RAM | Select the host while keeping hardware readiness pending verification |
| 2 Nov 2026 | Daily stand-up: Flexible, coordinated by the team each day (15 minutes). | Allow daily coordination around team availability |
| 3 Nov 2026 | Transfer the jobtrack repository to the shared GitHub organization jobtrack-cst | Shared team ownership, consistent future Go import path, and enable GitHub for Jira integration |
| 3 Nov 2026 | All four members agreed to remove mandatory PR approvals while retaining mandatory PRs, protected main, encouraged peer and cross-team reviews, and applicable CI requirements | Avoid blocking the short Sprint 0; applicable CI checks must pass once JT-28 provides them |
| 3 Nov 2026 | Proto folders and file names use snake_case matching the package; service folders stay kebab-case (agreed by Phuntsho in PR #6) | buf STANDARD lint requires the folder to match the package |
| 3 Nov 2026 | `buf.yaml` depends on `buf.build/googleapis/googleapis`; `buf.gen.yaml` adds the grpc-gateway plugin; `buf.lock` committed (agreed by Phuntsho in PR #6) | Needed for HTTP annotations and generated gateway code |
| 3 Nov 2026 | `employer_id` is the employer's user ID from `auth`; one company profile per employer (agreed by Phuntsho in PR #6) | Notification Dispatcher and ownership checks only know user IDs |
| 3 Nov 2026 | Internal RPCs live in separate `*InternalService` definitions, protected by a shared `INTERNAL_TOKEN`, a per-RPC caller allowlist and NetworkPolicy; user requests forward `x-user-id` and `x-user-role`. Residual risk without mTLS accepted (agreed by all four members in PR #6) | The gateway cannot reach internal services, and direct calls are limited to known callers without new tools |
| 3 Nov 2026 | `weekly-summary` gets admin emails from `AuthInternalService.ListUsersByRole` (agreed by all four members in PR #6) | `auth` stays the single source of truth for users |
| 3 Nov 2026 | Admin accounts are seeded by `auth` at startup from `ADMIN_EMAIL` and `ADMIN_PASSWORD` | Admins cannot self-register and no password is committed |
| 3 Nov 2026 | Categories, tags and locations are seeded by migration and read-only | Admin management is not in scope |
| 3 Nov 2026 | Moderation removal sets the job to REMOVED and `job-posting` publishes `job.closed` | `job-search` and `saved-jobs` only consume `job.closed` |

---

## 16. Known issues and open questions

- Verify that Phuntsho's laptop, the chosen cluster host, has at least 16 GB RAM.
- RabbitMQ to Knative connection method pending the JT-29 spike.
- Final presentation and submission dates not yet announced (likely to be postponed).
- Deferred until the shared event contracts are coordinated: `actor_id` in `EventEnvelope` (for `audit-log`) and `description` in `job.published` and `job.updated` (for `job-search`).
- Internal-call protection still to build (not active yet): the internal interceptor in `pkg/`, internal headers in `pkg/grpcclient`, header stripping in `gateway`, and NetworkPolicy manifests in `deploy/k8s` (check they work with Knative function pods during JT-29). These need Jira tasks.
- Remaining risk: without mTLS, a compromised internal service that holds `INTERNAL_TOKEN` could call internal RPCs. Accepted by all four members in PR #6.
- Suggest a `.gitattributes` file with `* text=auto eol=lf` so line endings match on Windows and WSL.
- The `buf` CLI version is not pinned; the team should agree on one version.
- Password hashing library for `auth` to be agreed (suggestion: bcrypt from `golang.org/x/crypto`).
- Maximum lengths for free-text fields (title, description, reason, notes) to be set in the implementation tasks and used for boundary value tests.