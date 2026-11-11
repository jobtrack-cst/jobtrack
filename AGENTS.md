# JobTrack: Master Project Context

This file is the single source of truth for the JobTrack project. It is written for both team members and AI assistants (Claude, ChatGPT, GitHub Copilot).

- Last updated: 11 Nov 2026 (JT-38 job-posting service decisions)
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
| 18 | `offer` | Phuntsho | Offers; applicant accepts or declines | `offer.made`, `offer.accepted`, `offer.declined` | `application.submitted` |
| 19 | `messaging` | Phuntsho | Message threads per application | `message.sent` | `application.submitted` |
| 20 | `job-alert` | Phuntsho | Applicant subscriptions; matches new jobs | `job_alert.matched` | `job.published` |

### API Gateway (not counted in the 20)

- Folder: `gateway`. Owner: Jigden. Backup: Sangay.
- Built with grpc-gateway from the HTTP annotations in each service's `.proto`. It registers only public services, never `*InternalService`.
- Verifies the JWT on every request (except register and login), then forwards `x-user-id` and `x-user-role` as gRPC metadata.

### JWT claims format

- Issued by `auth` on login and signed with HS256 using `JWT_SECRET`. Lifetime 1 hour. There are no refresh tokens; the user logs in again.
- Signed and verified with `github.com/golang-jwt/jwt/v5`. `JWT_SECRET` must be at least 32 bytes; `auth` refuses to start otherwise, and the gateway applies the same check (JT-31).

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

Functions are stateless, receive CloudEvents over HTTP, and scale to zero. JT-29 selected the Go RabbitMQ-to-HTTP forwarder fallback: preserve the domain envelope as CloudEvent data, map its event ID/type into CloudEvent headers, and acknowledge RabbitMQ only after HTTP success. See `docs/deployment/jt-29-cluster-spike.md` for the verified spike and production follow-ups.

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
| `application` | `resume` | `ResumeService.GetResume` (forwards user identity) | Resume validated check | No |
| `job-posting` | `employer` | `EmployerService.GetCompanyProfile` (forwards user identity) | Employer verified check | No |
| `job-alert` | `category-tag` | `CategoryTagInternalService.ResolveNames` | Validate category/tag IDs and resolve their names | Yes |
| `notification-dispatcher` | `notification-inbox`, `auth` | `NotificationInboxInternalService.CreateNotification`, `AuthInternalService.GetUser` | Write notification, get email | No |
| `weekly-summary` | `analytics` | `AnalyticsInternalService.GetInternalSummary` | Statistics | No |
| `weekly-summary` | `auth` | `AuthInternalService.ListUsersByRole` | Admin emails | No |
| `deadline-reminder` | `job-posting`, `saved-jobs` | `JobPostingInternalService.ListJobsClosingBetween`, `SavedJobsInternalService.ListJobSavers` | Closing jobs and who saved them | Yes |
| `resume-validator` | `resume` | `ResumeInternalService.GetResumeFile` | Fetch file | No |

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

Status: layer 1 is in place (JT-23 contracts). JT-44 provides the layer 2 server interceptor in `pkg/authctx` and configured internal client interceptor in `pkg/grpcclient`; service owners must wire them into their servers and internal callers before protection is active. The gateway already removes client-sent `x-user-*`, `x-internal-*` and `x-caller-*` metadata (JT-31). NetworkPolicies remain pending.

#### Internal-call wiring (JT-44)

In a service's `main.go`, load `INTERNAL_TOKEN` only when it serves or calls internal RPCs. Tokens must be nonempty visible ASCII without whitespace; invalid configuration must stop startup. Never log the token or the configuration containing it. For an internal server (imports: `os`, `github.com/jobtrack-cst/jobtrack/pkg/authctx`, `google.golang.org/grpc`):

```go
internalAuth, err := authctx.InternalUnaryServerInterceptor(os.Getenv("INTERNAL_TOKEN"))
if err != nil {
    return err // from the startup function
}
grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(internalAuth))
// Register this service's public and internal handlers on grpcServer.
```

For an approved internal caller (imports: `os`, `github.com/jobtrack-cst/jobtrack/pkg/grpcclient`, `google.golang.org/grpc`, `google.golang.org/grpc/credentials/insecure`), use the configured interceptor instead of the default one:

```go
internalClient, err := grpcclient.InternalUnaryClientInterceptor(
    os.Getenv("INTERNAL_TOKEN"), os.Getenv("SERVICE_NAME"),
)
if err != nil {
    return err
}
conn, err := grpc.NewClient(os.Getenv("AUTH_ADDR"),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithChainUnaryInterceptor(internalClient),
)
if err != nil {
    return err
}
defer conn.Close()
// notification-dispatcher calls GetUser; weekly-summary calls ListUsersByRole.
```

This client includes the existing timeout, retries and circuit breaker. It replaces outgoing internal token/caller headers on internal calls, removes them on public calls, and never copies incoming security metadata. The server requires exactly one token and one caller value, compares token hashes in constant time, and applies the exact RPC allowlist above (unknown internal methods are denied). Public RPCs pass through the server interceptor unchanged. All current internal contracts are unary. Public-only callers, including gateway, keep `grpcclient.UnaryClientInterceptor()` with no `INTERNAL_TOKEN`. Do not add internal credentials to gateway or frontend.

---

## 7. Tech stack

| Area | Choice |
|---|---|
| Services and functions | Go (current stable release, pinned in `go.mod`), single Go module for the monorepo |
| RPC | gRPC, Protocol Buffers, `buf` for linting and code generation |
| Gateway | grpc-gateway |
| Database | PostgreSQL (one server, one database and user per service), `pgx`, `goose` migrations run on startup |
| Messaging | RabbitMQ, topic exchange `jobtrack.events`, Go client `github.com/rabbitmq/amqp091-go` |
| Serverless | Knative Serving and Eventing (PingSource for schedules) |
| Frontend | React, TypeScript, Vite, MUI, React Router |
| Local development | Docker Compose (Postgres, RabbitMQ, Mailpit, services) |
| Cluster | k3d on one host machine |
| Kubernetes manifests | Plain YAML with Kustomize |
| CI | GitHub Actions, images pushed to GitHub Container Registry |
| Monitoring | Prometheus and Grafana, JSON logs |
| Resilience | Shared gRPC client with timeout, retry and circuit breaker (`sony/gobreaker`) |
| Passwords and tokens | bcrypt from `golang.org/x/crypto`, JWT with `github.com/golang-jwt/jwt/v5` (`auth` and `gateway` only) |
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
    <other>/                     only when none of the above fit (for example security/ in auth for bcrypt and JWT)
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
- List RPCs use the shared pagination messages from `common.proto`:
  - `page_size`: 0 means the default of 20; values above 100 are capped at 100; negative is `INVALID_ARGUMENT`.
  - `page_token`: the offset as a plain decimal string (for example `"20"`); anything else is `INVALID_ARGUMENT`.
  - `next_page_token` is empty on the last page; `total_count` is always filled.

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
- Mocks: add `//go:generate go tool mockgen -destination=mocks/<file>_mock.go -package=mocks . <Interface>,...` in `internal/ports`, run `go generate ./services/<service>/internal/ports/...`, and commit the generated `mocks/` folder. `mockgen` is a Go tool in `go.mod`, so no separate install is needed.
- Integration tests: the first run may time out while Docker downloads images. Pull them first: `docker pull postgres:17-alpine postgres:18-alpine rabbitmq:4.1-alpine`.
- Design test cases with equivalence partitioning, boundary values and state transitions, and record them in the test case document with the Jira story they trace to.
- Coverage target: at least 80 percent line coverage on `application` and `application-status`. Other services: meaningful tests for all business rules.
- Commands: `go test ./...` (unit), `go test -tags=integration ./...` (integration), `go test -cover ./...` (coverage).
- Coverage report, as CI produces it: `go test -coverprofile=coverage.out ./...`, then `go tool cover -func=coverage.out`. For one service: `go test -cover ./services/<service>/...`.
- Folder layout: test files sit next to the code they test as `<file>_test.go`, in the external package (for example `package domain_test`). Integration tests are named `<file>_integration_test.go` and start with `//go:build integration`. Generated mocks live in `internal/ports/mocks`.
- Working examples to copy are in `services/service-template`: table-driven test with testify in `internal/domain/health_test.go`, gomock port mock in `internal/app/health_test.go` (mock from `internal/ports/mocks`), Testcontainers PostgreSQL test in `internal/adapters/postgres/readiness_integration_test.go`.
- CI runs both suites: `go test -coverprofile=coverage.out ./...` (unit and coverage) and `go test -tags=integration ./...` (integration). Both must pass before merging.
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
| Sprint 1 | 6 to 8 Nov | All 20 services and the gateway running in Docker Compose with their basic public operations and tests |
| Sprint 2 | 9 to 11 Nov | Core flow working end to end with events and the first functions |
| Sprint 3 | 12 to 14 Nov | Full business logic, all 5 functions, frontend for all roles, integration tests |
| Sprint 4 | 15 to 17 Nov | End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze |

There is no buffer. Sprint 4 ends on 17 Nov, so any slip affects the final date directly (see the risk register in `docs/pm`).

Presentation date: not yet announced (likely to be postponed).

Ceremonies: sprint planning (1 hour), daily stand-up (15 minutes), combined sprint review and retrospective (1 hour). Sprint lengths stay fixed; if a goal is met early, pull the next items from the backlog.

### Current sprint

- Sprint: Sprint 1 (6 to 8 Nov), started on 6 Nov 2026
- Goal: All 20 services and the gateway running in Docker Compose with their basic public operations and tests. Committed minimum: the Must tasks needed for the apply-for-a-job flow.
- Tasks: JT-30 to JT-52, plus JT-20, JT-22 and JT-29 carried over from Sprint 0 (see Jira)
- Scope: each service works on its own with public RPCs, authorisation, migrations, tests and Docker Compose. Cross-service calls are port interfaces mocked with gomock. Events and real cross-service clients move to Sprint 2.
- Commitment: Must tasks (Jira priority High) are the committed minimum. Should tasks (Medium) are planned; unfinished ones move to the top of Sprint 2.
- No sub-tasks in Sprint 1. The acceptance criteria in each task are the checklist.
- Cluster host machine: Phuntsho's laptop; 16 GiB physical RAM verified during JT-29 (Docker Desktop approximately 7.75 GiB allocated)
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
| 3 Nov 2026 | All four members agreed to `github.com/rabbitmq/amqp091-go` as the RabbitMQ Go client | Use one shared client with publisher confirmations for broker-acknowledged event publishing |
| 5 Nov 2026 | Record Team A and Team B's confirmed PR #11 Messaging decisions: consume only `application.submitted` to maintain local applicant/employer participants; derive sender from trusted metadata and recipient as the other participant; reject non-participants with `PERMISSION_DENIED` and unknown or unprocessed applications with `NOT_FOUND`; list oldest first; trimmed content must be 1 to 2000 characters (`INVALID_ARGUMENT` otherwise) | Authorize application threads without client-supplied participants or a new synchronous call |
| 5 Nov 2026 | Record Team A and Team B's confirmed PR #11 Job Alert decisions: category membership, any tag overlap and exact location name, combined with AND; empty individual filters are unrestricted, all-empty filters invalid; normalized category/tag sets identify duplicates; maximum 10 subscriptions per applicant and 20 IDs per filter list; validate IDs through `ResolveNames`; delete and recreate instead of editing | Define matching and validation within the existing Category Tag dependency; detailed error mappings are in the contract comments |
| 5 Nov 2026 | Record Team A and Team B's confirmed PR #11 submission error mapping: missing job `NOT_FOUND`, closed or expired job and unvalidated resume `FAILED_PRECONDITION`, wrong resume owner `PERMISSION_DENIED` | Make existing Job Posting and Resume eligibility checks consistent without changing their calls |
| 5 Nov 2026 | Reaffirm Team A and Team B's PR #11 Resume internal-call requirement: runtime `x-internal-token` verification, with `GetResumeFile` restricted to `resume-validator` and no HTTP route | Clarify the existing internal security design without changing signatures or public resume permissions |
| 5 Nov 2026 | Sprint dates (Option B): Sprint 1 6 to 8 Nov, Sprint 2 9 to 11, Sprint 3 12 to 14, Sprint 4 15 to 17; no buffer | Sprint 1 could not start before 6 Nov; keeps four 3-day sprints within the 17 Nov end date |
| 5 Nov 2026 | JT-29 (cluster spike) reassigned from Sangay to Phuntsho | The spike runs on Phuntsho's laptop, the cluster host |
| 5 Nov 2026 | Service ownership unchanged; Jigden keeps his six services and the gateway | Avoids handover cost mid-project; Sangay, as pair partner, can take over a task if Jigden falls behind |
| 5 Nov 2026 | `auth` hashes passwords with bcrypt from `golang.org/x/crypto` | Standard, maintained Go library; closes the Section 16 question |
| 5 Nov 2026 | Frontend shell moved to the Sprint 2 backlog | Sprint 1 focuses on services and the gateway running on their own |
| 5 Nov 2026 | Sprint 1 services run on their own; cross-service calls are gomock-mocked ports; events and real clients move to Sprint 2 | Each member can build and test without waiting for other services |
| 5 Nov 2026 | Must tasks are the committed Sprint 1 minimum; unfinished Should tasks move to the top of Sprint 2 | Protects the apply-for-a-job flow if the sprint runs short |
| 5 Nov 2026 | No sub-tasks in Sprint 1 | Short sprint; acceptance criteria already work as the checklist |
| 6 Nov 2026 | Pagination for all list RPCs: default page size 20, maximum 100, page token is the offset as a decimal string (agreed by Phuntsho during JT-30) | One simple rule every service can implement and test the same way |
| 6 Nov 2026 | `github.com/golang-jwt/jwt/v5` signs (auth) and verifies (gateway) JWTs; `JWT_SECRET` at least 32 bytes (JT-30) | Well-tested library instead of hand-written token code; HS256 keys shorter than 32 bytes are weaker |
| 6 Nov 2026 | `mockgen` added as a Go tool (`go tool mockgen`); generated mocks committed in `internal/ports/mocks` (JT-30) | Same gomock version for everyone with no separate install |
| 6 Nov 2026 | Login runs bcrypt against a dummy hash when the email is unknown (JT-30) | Response time does not reveal which emails are registered |
| 7 Nov 2026 | JT-29 verified k3d with Knative Serving/Eventing 1.24.0 and Kourier (approved by Phuntsho); select the Go forwarder fallback for RabbitMQ events | Native RabbitMQSource could not reconcile without RabbitMQ topology CRDs; the existing Go AMQP client delivered the unchanged domain envelope as a CloudEvent and PingSource delivery was verified. See `docs/deployment/jt-29-cluster-spike.md`. |
| 7 Nov 2026 | Gateway serves the REST API, `GET /healthz` and `GET /metrics` on container HTTP port 8080, published as host port 8000; it has no gRPC port (JT-31) | One port to publish, matching the 8080 and host port 8000 conventions |
| 7 Nov 2026 | Only `POST /v1/auth/register` and `POST /v1/auth/login` skip the JWT check, matched exactly, and grpc-gateway HTTP method override and POST-to-GET path length fallback are turned off; `GET /healthz` and `GET /metrics` are also open (JT-31) | Fail closed: any other method or path variant needs a token, and the method cannot change after the check |
| 7 Nov 2026 | Gateway keeps grpc-gateway v2.33's `DefaultHeaderMatcher` (it no longer forwards `Grpc-Metadata-*` headers) and adds its own filter that drops `x-user-*`, `x-internal-*` and `x-caller-*`; only `x-user-id` and `x-user-role` from the verified token are set (JT-31) | Two independent layers, so a client cannot set identity even if a default changes |
| 7 Nov 2026 | Gateway refuses to start if `INTERNAL_TOKEN` is set or any of the 20 `<SERVICE>_ADDR` variables is missing; Compose sets all 20 and gives the gateway no RabbitMQ settings (JT-31) | Enforces Section 9 at startup; routes to services not in Compose yet return 503 instead of blocking the gateway |
| 7 Nov 2026 | Gateway connects to each service through `pkg/grpcclient`, with one interceptor and circuit breaker per backend connection (JT-31) | Follows Section 9; one failing service cannot open the circuit for the others |
| 8 Nov 2026 | JT-20: Testcontainers integration tests run in CI with `go test -tags=integration ./...`, in addition to the unit and coverage checks (agreed by Phuntsho; ci.yml change coordinated with Sangay) | Template integration tests must pass in CI, not only locally |
| 7 Nov 2026 | Employer `GetCompanyProfile` is open to any authenticated role; `CreateCompanyProfile`, `GetMyCompanyProfile` and `UpdateCompanyProfile` are employer only (`PERMISSION_DENIED` otherwise) and always use the caller's own user ID; a forwarded user ID that is not a UUID or an unknown role is `UNAUTHENTICATED`; an `employer_id` that is not a UUID is `INVALID_ARGUMENT` (approved by Jigden during JT-32) | job-posting can check verification with any forwarded user, and no request field can target another employer's profile |
| 7 Nov 2026 | Employer `UpdateCompanyProfile` replaces all four editable fields (`company_name` required) and never changes `verification_status` or `rejection_reason`; `rejection_reason` is returned only to the owning employer or an admin (JT-32) | Follows the contract comments; only verification events change verification |
| 7 Nov 2026 | Employer field limits, in characters after trimming: `company_name` 1 to 150, `description` up to 2000, `website` up to 255 and an absolute `http` or `https` URL with a host, `location` up to 100; the database columns enforce the same lengths (JT-32) | Sets the Section 16 limits for employer and gives the boundary values for testing |
| 8 Nov 2026 | Offer maintains a local application-participants table using data from `application.submitted`, following the Messaging architecture (JT-45; proposed by Jigden, approved by Phuntsho as Offer service owner and contracts lead). `MakeOffer` requires role `employer` and matching stored `employer_id`; `AcceptOffer` and `DeclineOffer` require role `applicant` and matching stored `applicant_id`. Unauthorized callers receive `PERMISSION_DENIED`; unknown or unprocessed application participants receive `NOT_FOUND`. Sprint 1 implements the repository port, PostgreSQL adapter, migrations and mocked tests; Sprint 2 implements the idempotent RabbitMQ consumer. No new synchronous RPC dependencies or protobuf changes | Verify participants locally without introducing a synchronous dependency |
| 8 Nov 2026 | Offer `GetOffer` permits the associated applicant or associated employer to view their offer, using trusted stored participant IDs for authorization. Admins cannot view offers in Sprint 1; unrelated callers receive `PERMISSION_DENIED`; unknown offer IDs receive `NOT_FOUND` (JT-45; approved by Phuntsho as Offer service owner and contracts lead) | Restrict offer viewing to its associated participants |
| 8 Nov 2026 | Each application can have exactly one offer for its lifetime; a second `MakeOffer` request returns `ALREADY_EXISTS`, and a declined offer cannot be replaced. PostgreSQL must enforce uniqueness on `application_id`. Accepting or declining a pending offer changes its status to `ACCEPTED` or `DECLINED`; repeated acceptance, repeated decline or reversing a decision returns `FAILED_PRECONDITION` (JT-45; approved by Phuntsho as Offer service owner and contracts lead) | Enforce one lifetime offer per application and final offer decisions consistently in business logic and persistence |
| 8 Nov 2026 | Offer Sprint 1 eligibility requires trusted local application participants, a matching owning employer and no previous offer. No application lifecycle, interview completion, withdrawal or employer verification checks are implemented without an approved trusted data source (JT-45; approved by Phuntsho as Offer service owner and contracts lead) | Keep standalone eligibility within the approved participant projection |
| 8 Nov 2026 | Offer terms are required, trimmed and limited to 1 to 2000 Unicode characters after trimming; empty or whitespace-only terms and invalid terms return `INVALID_ARGUMENT`. Application validation and PostgreSQL storage enforce consistent length limits (JT-45; approved by Phuntsho as Offer service owner and contracts lead) | Define validation and boundary values for offer terms |
| 8 Nov 2026 | Company-verification `SubmitVerificationRequest` and `ListMyVerificationRequests` are employer only and always use the caller's own user ID; `ListVerificationRequests`, `ApproveVerificationRequest` and `RejectVerificationRequest` are admin only; other roles get `PERMISSION_DENIED`, checked before input; a `request_id` that is not a UUID is `INVALID_ARGUMENT` (approved by Jigden during JT-33) | Only employers have requests and only admins decide; no request field can target another employer |
| 8 Nov 2026 | Company-verification `ListMyVerificationRequests` is newest first and `ListVerificationRequests` is oldest first; an unknown status filter is `INVALID_ARGUMENT`. At most one PENDING and one APPROVED request per employer (partial unique indexes), and a decision only updates a row that is still PENDING, so concurrent decisions get `FAILED_PRECONDITION` (JT-33) | Admins handle the queue in order, and races cannot create duplicate or double decisions |
| 8 Nov 2026 | Company-verification field limits, in characters after trimming: `registration_number` 1 to 50, `notes` up to 1000, reject `reason` 1 to 500; NUL refused; the database columns enforce the same lengths (JT-33) | Sets the Section 16 limits for company-verification and gives the boundary values for testing |
| 8 Nov 2026 | Moderation `ReportJob` is open to applicant, employer and admin (the contract says any logged-in user) and always uses the caller's own user ID as `reporter_id`; `ListReports`, `DismissReport` and `RemoveJob` are admin only; other roles get `PERMISSION_DENIED`, checked before input; a `job_id` or `report_id` that is not a UUID is `INVALID_ARGUMENT` (approved by Jigden during JT-34) | Any user can flag a posting, only admins act on it, and no request field can set the reporter |
| 8 Nov 2026 | Moderation keeps its own `removed_jobs` table and does not check jobs against job-posting: any well-formed job ID can be reported, and `ReportJob` or `RemoveJob` on a job moderation already removed returns `FAILED_PRECONDITION`. A job with no reports can be removed; `RemoveJob` marks the job's OPEN reports ACTIONED and stores the reason and admin for `moderation.job_removed` in Sprint 2 (approved by Jigden during JT-34) | Section 6 allows no call from moderation to job-posting, so moderation only knows the jobs it removed itself |
| 8 Nov 2026 | Moderation `ListReports` is newest first with `report_id` breaking ties; an unknown status filter is `INVALID_ARGUMENT`. One OPEN report per reporter and job (partial unique index, `ALREADY_EXISTS`); a dismissed report can be reported again. `ReportJob` and `RemoveJob` take a per-job transaction advisory lock, and `DismissReport` only updates a row that is still OPEN, so concurrent takedowns and dismissals get `FAILED_PRECONDITION` (JT-34) | Races cannot create duplicate reports, double decisions or an OPEN report on a removed job |
| 8 Nov 2026 | Moderation field limits, in characters after trimming: report `reason` 1 to 1000, takedown `reason` 1 to 500; NUL refused; the database columns enforce the same lengths (JT-34) | Sets the Section 16 limits for moderation and gives the boundary values for testing |
| 8 Nov 2026 | Audit-log `ListAuditEntries` is admin only; other roles get `PERMISSION_DENIED`, checked before input, including the timestamps. Entries are newest first by `occurred_at` with `event_id` breaking ties; `start_time` is inclusive and `end_time` exclusive, and start must be before end when both are set. The `event_type` filter is checked only for format, so an unused type returns an empty list (approved by Jigden during JT-35) | Only admins see the audit trail, and adding an event type never needs an audit-log change |
| 8 Nov 2026 | Audit-log stores `event_id` (primary key), `event_type`, `producer`, `occurred_at` and `data_json` as TEXT exactly as received, plus an internal `recorded_at`. No actor column until `actor_id` is added to `EventEnvelope`; then a migration adds a nullable column. A separate `processed_events` table holds handled event IDs: `RecordEntry` claims the ID and stores the entry in one transaction, a duplicate returns not recorded without an error, and a failed insert marks nothing processed (approved by Jigden during JT-35) | The admin view shows payloads unchanged, and the Sprint 2 consumer is idempotent without losing events |
| 8 Nov 2026 | Audit-log field limits: `event_type` 1 to 100 characters, a lowercase dot-separated routing key (for example `job_alert.matched`); `producer` 1 to 50 characters, kebab-case; `data_json` a valid UTF-8 JSON object of at most 65536 bytes; values are not trimmed; the database columns enforce the same lengths (JT-35) | Sets the Section 16 limits for audit-log and gives the boundary values for testing |
| 8 Nov 2026 | Analytics `GetSummary` is admin only; other roles get `PERMISSION_DENIED`, checked before input, including the timestamps. `GetInternalSummary` has no role check and is protected by the JT-44 internal server interceptor (`weekly-summary` only); both services are registered on the analytics server, and the gateway never registers the internal one. Both RPCs require `start_time` and `end_time` (`INVALID_ARGUMENT` if either is missing or invalid, or if start is not before end), count events with `start_time <= occurred_at < end_time` (both bounds rounded up to a whole microsecond, the precision PostgreSQL stores), have no maximum range, echo the request times, and list only statuses with a count above 0, sorted by name (approved by Jigden during JT-36) | Only admins and the weekly summary see statistics, and both get the same counts for the same range |
| 8 Nov 2026 | Analytics stores one row per counted event in `outcome_events`: `event_id` (primary key), `event_type` (a CHECK allows only the eight consumed types), `new_status` (set only for `application.status_changed`), `occurred_at` and an internal `recorded_at`; no job, employer or applicant IDs; index on `(occurred_at) INCLUDE (event_type, new_status)`. A separate `processed_events` table holds handled event IDs: `RecordEvent` claims the ID and stores the event in one transaction, a duplicate returns not recorded without an error, and a failed insert marks nothing processed (approved by Jigden during JT-36) | Exact time ranges need one row per event, and the Sprint 2 consumers are idempotent without losing events |
| 8 Nov 2026 | Analytics field limits: neither RPC has a free-text field; `new_status` is trimmed, 1 to 32 characters, valid UTF-8, NUL refused, stored as sent and not checked against the lifecycle list; the column is TEXT with a length CHECK, because VARCHAR would silently cut trailing spaces (JT-36) | Sets the Section 16 limits for analytics and gives the boundary values for testing |
| 8 Nov 2026 | In Docker Compose, `INTERNAL_TOKEN` is set only in the blocks of services that serve or call internal RPCs (analytics is the first, JT-36), never in the shared `*service-env` anchor or the gateway; `.env.example` has a placeholder, and local values come from `openssl rand -hex 32` (proposed in JT-36 for review by Sangay as DevOps lead) | No service holds the token before its owner wires JT-44, and the gateway never can |
| 11 Nov 2026 | Job-posting `CreateJob`, `UpdateJob`, `PublishJob`, `CloseJob` and `ListMyJobs` are employer only, and the four changing RPCs are owner only; other roles and other employers get `PERMISSION_DENIED`, checked before input. `GetJob` is open to any authenticated role, but DRAFT and REMOVED jobs are visible only to the owner and admins (`NOT_FOUND` for others). A `job_id` that is not a UUID is `INVALID_ARGUMENT` (approved by Jigden during JT-38) | Only the owning employer manages a job, and unpublished jobs stay hidden |
| 11 Nov 2026 | Job-posting status rules: DRAFT -> PUBLISHED -> CLOSED; REMOVED only from `moderation.job_removed` (Sprint 2). `UpdateJob` is allowed in DRAFT or PUBLISHED, replaces all editable fields and never changes status. `PublishJob` needs DRAFT, a deadline after now and a verified employer (checked last). `CloseJob` needs PUBLISHED. A passed deadline does not change status; open means PUBLISHED and deadline after now. Status changes are conditional updates, so a concurrent change gets `FAILED_PRECONDITION`. `ListMyJobs` is newest first with `job_id` breaking ties; an unknown status filter is `INVALID_ARGUMENT` (approved by Jigden during JT-38) | Follows the contract comments and makes races safe |
| 11 Nov 2026 | Job-posting `ListJobsClosingBetween` lists PUBLISHED jobs with `start_time <= deadline < end_time` (both required, start before end, rounded up to a whole microsecond), ordered by deadline then `job_id`, and is protected by the JT-44 internal interceptor (deadline-reminder only); job-posting needs `INTERNAL_TOKEN` (JT-38) | Gives deadline-reminder an exact, paged window |
| 11 Nov 2026 | Job-posting field limits, in characters after trimming: `title` 1 to 150, `description` up to 5000, `location` up to 100; `category_id` empty or a UUID; at most 20 `tag_ids`, each a UUID (duplicates removed); `deadline` required and after now; NUL and invalid UTF-8 refused; the database columns enforce the same limits. Category, tag and location values are checked for format only, because Section 6 has no call from job-posting to category-tag (JT-38) | Sets the Section 16 limits for job-posting and gives the boundary values for testing |
| 11 Nov 2026 | Job-posting checks verification through an `EmployerVerifier` port; unexpected employer errors are `INTERNAL`. Until JT-57 adds the real `EmployerService.GetCompanyProfile` client, a placeholder never verifies, so `PublishJob` returns `INTERNAL` with "employer verification is not available until JT-57" (approved by Jigden during JT-38) | No job can be published without a real verification check |

---

## 16. Known issues and open questions

- Offer application lifecycle eligibility remains unresolved for Sprint 2, including required lifecycle state, interview completion, employer verification and withdrawal handling. Whether Offer should consume `application.withdrawn` and which trusted data sources support these checks require agreement; Sprint 1 checks only local participants, employer ownership and lifetime offer uniqueness (JT-45).
- JT-29 confirmed 16 GiB physical RAM and the forwarder fallback; the spike does not prove capacity for the complete 20-service deployment. Docker Desktop currently has approximately 7.75 GiB allocated.
- Promote the JT-29 transport spike into shared deployment wiring in the appropriate implementation tasks: durable broker storage, consumer idempotency, retry/dead-letter policy, operational health and credentials. Native RabbitMQSource needs additional topology CRDs/operators and was not selected. See `docs/deployment/jt-29-cluster-spike.md`.
- Final presentation and submission dates not yet announced (likely to be postponed).
- Deferred until the shared event contracts are coordinated: `actor_id` in `EventEnvelope` (for `audit-log`) and `description` in `job.published` and `job.updated` (for `job-search`). Until then audit-log stores no actor (JT-35).
- JT-44 shared internal-call helpers require adoption by each internal server/caller owner before layer 2 is active. Analytics wires the server interceptor for `GetInternalSummary` (JT-36) and job-posting for `ListJobsClosingBetween` (JT-38). NetworkPolicy manifests in `deploy/k8s` remain pending; verify they allow intended Knative function pods. Gateway filtering is implemented in JT-31.
- Remaining risk: without mTLS, a compromised internal service that holds `INTERNAL_TOKEN` could call internal RPCs. Accepted by all four members in PR #6.
- Suggest a `.gitattributes` file with `* text=auto eol=lf` so line endings match on Windows and WSL.
- The `buf` CLI version is not pinned; the team should agree on one version.
- Maximum lengths for free-text fields (title, description, reason, notes) to be set in the implementation tasks and used for boundary value tests. Set so far: auth (JT-30), employer (JT-32), company-verification (JT-33), moderation (JT-34), audit-log (JT-35), analytics (JT-36) and job-posting (JT-38), see Section 15.
- Moderation cannot tell whether a reported job exists or is open, because it has no allowed call to job-posting; it only knows the jobs it removed itself (JT-34). Decide in Sprint 2 whether job events or an agreed call should feed it.
- How employers view an applicant's resume during review (GetResume and DownloadResume are owner-only).
- Local `.env` passwords that contain `@ : / ? # %` break the `postgres://` and `amqp://` URLs in Docker Compose. Use letters and digits only (for example `openssl rand -hex 24`).
- `.github/pull_request_template.md` is listed in Section 8 but does not exist yet.
- Team B services have no contract tests in `tests/contract/` yet.
- `PublishJob` in job-posting returns `INTERNAL` in Docker Compose until JT-57 replaces the placeholder `EmployerVerifier` with the real `EmployerService.GetCompanyProfile` client (JT-38).
