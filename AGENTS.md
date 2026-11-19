# JobTrack: Master Project Context

This file is the single source of truth for the JobTrack project. It is written for both team members and AI assistants (Claude, ChatGPT, GitHub Copilot).

- Last updated: 19 Nov 2026 (JT-67 Sprint 4 extended by one day)
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
| 8 | `category-tag` | Jigden | Categories, skill tags, locations (seeded, read-only) | | |
| 9 | `job-search` | Sangay | Search read model built from job events | | `job.published`, `job.updated`, `job.closed` |
| 10 | `saved-jobs` | Sangay | Bookmarks | | `job.closed` |
| 11 | `notification-inbox` | Sangay | Stores and lists in-app notifications, mark as read | | (written by Notification Dispatcher via gRPC) |

### Team A (Phuntsho and Kinley)

| # | Service (folder) | Owner | Responsibility | Publishes | Consumes |
|---|---|---|---|---|---|
| 12 | `applicant-profile` | Kinley | Applicant details, education, skills | | |
| 13 | `resume` | Kinley | Upload and store resumes (in its database, 2 MB limit), validation status | `resume.uploaded` | `resume.validated`, `resume.rejected` |
| 14 | `application-status` | Kinley | Status lifecycle state machine and history | `application.status_changed` | `application.submitted`, `application.withdrawn`, `review.completed`, `interview.scheduled`, `offer.made`, `offer.accepted`, `offer.declined` |
| 15 | `review` | Kinley | Employer scores and shortlist decisions | `review.completed` | `application.submitted` |
| 16 | `interview` | Kinley | Schedule and record interviews | `interview.scheduled` | `application.submitted` |
| 17 | `application` | Phuntsho | Submit and withdraw applications, eligibility checks | `application.submitted`, `application.withdrawn` | |
| 18 | `offer` | Phuntsho | Offers; applicant accepts or declines | `offer.made`, `offer.accepted`, `offer.declined` | `application.submitted` |
| 19 | `messaging` | Phuntsho | Message threads per application | `message.sent` | `application.submitted` |
| 20 | `job-alert` | Phuntsho | Applicant subscriptions; matches new jobs | `job_alert.matched` | `job.published` |

### API Gateway (not counted in the 20)

- Folder: `gateway`. Owner: Jigden. Backup: Sangay.
- Built with grpc-gateway from the HTTP annotations in each service's `.proto`. It registers only public services, never `*InternalService`.
- Verifies the JWT on every request (except register and login, and CORS preflight requests from `CORS_ALLOWED_ORIGIN`, which the gateway answers itself, JT-53), then forwards `x-user-id` and `x-user-role` as gRPC metadata.
- Rejects request bodies larger than 3 MiB (3,145,728 bytes) with HTTP 413 before they reach any service (JT-54, Section 15).

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
| `email-sender` | Sangay | Direct HTTP JSON call from other functions | Sends email through SMTP to Mailpit; [shared HTTP contract](docs/deployment/email-sender-http-contract.md) |
| `weekly-summary` | Jigden | Weekly schedule (PingSource) | Gets statistics from Analytics and admin emails from Auth, then emails a summary to admins. |
| `resume-validator` | Phuntsho | Event: `resume.uploaded` | Fetches the file from Resume, checks type (PDF only), size (max 2 MB) and content signature. Publishes `resume.validated` or `resume.rejected`. |
| `deadline-reminder` | Kinley | Daily schedule (PingSource) | Finds jobs closing within 48 hours (Job Posting) and who saved them (Saved Jobs). Publishes `deadline.reminder` events. |

JT-61 Compose execution decision: functions run as ordinary long-running HTTP
containers on the Compose network; Compose does not run Knative or scale to zero.
The shared Go forwarder runs as one supervised container per event-driven function,
using a durable `<function>.events` queue and `<function>.events.dead` queue.
The `functions` Compose profile is opt-in until the resume-validator image is
available from its implementation task. k3d uses the same forwarder image as a
Deployment targeting the Knative Service URL. The forwarder carries no internal
gRPC credentials; only functions calling internal RPCs receive `INTERNAL_TOKEN`.
The project owner approved restart-safe bounded retries for JT-61: source and dead
queues use RabbitMQ quorum queues. Source queues set `x-delivery-limit=3`,
`x-dead-letter-strategy=at-least-once`, `x-overflow=reject-publish`, and route to
`<function>.events.dead`. One HTTP attempt per broker delivery permits at most
four HTTP attempts, with 1/2/4 second backoff before redelivered attempts.
Broker redelivery counts survive consumer reconnects and process restarts; crashes
and cancellations consume the budget, even if no HTTP attempt completed.
Only HTTP 2xx is acknowledged. HTTP failures use `basic.reject(requeue=true)`;
malformed or exhausted deliveries use `basic.nack(requeue=false)`. RabbitMQ retains
dead letters until the destination confirms receipt. Functions must deduplicate
event IDs because HTTP success before an uncertain AMQP ack can duplicate work.
Existing classic queues must be migrated with DevOps review; never delete queues
or data automatically. Compose starts Resume and its dependencies for the validator.
See `docs/deployment/jt-61-forwarder.md` for execution and verification.

Functions are stateless and scale to zero in Knative. Event-triggered functions receive CloudEvents over HTTP; email-sender instead accepts direct `POST /send-email` JSON requests from Notification Dispatcher and Weekly Summary. JT-29 selected the Go RabbitMQ-to-HTTP forwarder fallback: preserve the domain envelope as CloudEvent data, map its event ID/type into CloudEvent headers, and acknowledge RabbitMQ only after HTTP success. See `docs/deployment/jt-29-cluster-spike.md` for the verified spike and production follow-ups.

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

JT-76 wires Job Alert's `ports.NameResolver` to `CategoryTagInternalService.ResolveNames`
at `CATEGORY_TAG_ADDR`, using `grpcclient.InternalUnaryClientInterceptor(INTERNAL_TOKEN, SERVICE_NAME)`
with `SERVICE_NAME=job-alert`. Startup validates the token, exact service name and
host:port configuration without logging values. The connection is lazy and closes
after the servers and workers stop. Compose supplies these settings but has no
`depends_on: category-tag`; unavailable or unauthorized resolver calls fail closed
with `UNAVAILABLE`, while location-only subscriptions need no resolver call.
Category Tag already restricts ResolveNames to caller `job-alert` (JT-39).
No internal credentials are added to Job Alert's public server or the gateway.

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
- `CORS_ALLOWED_ORIGIN` only in `gateway`: the one browser origin allowed to call the API (Compose default `http://localhost:5173` when unset; an empty value turns CORS off; an invalid value stops startup; JT-53).
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
- New consumers can use the shared `events.Consume` from `pkg/events` (JT-56; employer uses it). It declares the quorum queue `<service>.<purpose>` and `<service>.<purpose>.dead` with the JT-61 policy (at most 4 attempts with 1, 2 and 4 s backoff, then the dead queue), handles one event at a time with a 10 s limit per handler call and reconnects after a broker restart. A handler returns nil to acknowledge, `events.Permanent(err)` for an event that can never be applied (dead queue at once), or any other error to retry. Resume's validation consumer (JT-59) applies the same policy in its own adapter. Either way, store the handled `event_id` in a `processed_events` table in the same transaction as the change.
- A service that publishes because of a database change stores the event in an outbox table in the same transaction and publishes it afterwards, so an event exists exactly when the change is committed (`resume` with leased claims for several workers, JT-59; `company-verification` with one relay, JT-56; `moderation` with one relay, JT-87).
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
| Contract | `buf breaking` in CI, event fixtures in `tests/contract/fixtures/<routing key>.json` | | Consumers test against shared JSON fixtures of each event |
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
| Sprint 2 | 9 to 13 Nov | Core flow working end to end with events and the first functions |
| Sprint 3 | 14 to 16 Nov | Full business logic, all 5 functions, frontend for all roles, integration tests |
| Sprint 4 | 17 to 20 Nov (extended by one day on 19 Nov) | End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze |

Planned project completion: 20 Nov 2026 (schedule revised on 11 Nov 2026 and Sprint 4 extended by one day on 19 Nov 2026, see Section 15). There is no buffer. Sprint 4 ends on 20 Nov, so any slip affects the final date directly (see the risk register in `docs/pm`).

Presentation date: not yet announced (likely to be postponed).

Ceremonies: sprint planning (1 hour), daily stand-up (15 minutes), combined sprint review and retrospective (1 hour). Sprint lengths stay fixed (the only changes are the agreed Sprint 2 extension, Section 15, 11 Nov 2026, and the one-day Sprint 4 extension, Section 15, 19 Nov 2026); if a goal is met early, pull the next items from the backlog.

### Current sprint

- Sprint: Sprint 4 (17 to 20 Nov, Jira sprint 38), started on 17 Nov 2026 at 09:00 and extended on 19 Nov 2026 to end on 20 Nov 2026 at 23:59 (Asia/Thimphu). This is the last sprint: there is no Sprint 5 and no further extension.
- Goal: End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze. Committed minimum: the Must tasks (39 points).
- Tasks: the 8 Sprint 3 carryovers (19 points), JT-90 from the backlog and JT-91 to JT-103: 22 issues, 57 points (39 Must, 18 Should), plus JT-104 (bug, unestimated). See Jira and `docs/pm/sprint-4-planning.md`, which lists the dependencies of every task.
- Scope: Cypress end to end against the running Compose stack (JT-95), the JMeter 200-user run (JT-96), UAT (JT-99), Prometheus and Grafana in Compose (JT-93), all 5 functions deployed to k3d (JT-92), the remaining Sprint 3 work (notification-dispatcher JT-71, job-search JT-69, frontend job pages JT-73), and the final documentation and reports.
- Commitment: Must tasks (Jira priority High) are the committed minimum and are never cut. Should tasks (Medium) are planned; an unfinished Should task is recorded as not delivered in the closure report (JT-100) instead of moving to another sprint (Section 15, 18 Nov 2026).
- Slip order: JT-66 first, then JT-102 and JT-103, then the other Should tasks.
- Sprint 4 assignments outside Section 4 and 5 ownership: JT-73 (frontend job pages) to Phuntsho and JT-70 (saved-jobs consumer) to Jigden, both reviewed by Sangay as owner; JT-90 (job-posting bug) to Jigden as job-posting backup. JT-72 (audit-log consumer) returns to Jigden as audit-log owner, reviewed by Sangay. Ownership in Sections 4 and 5 is unchanged.
- Deployment for WEB303: all 5 functions run on Knative in k3d (JT-92); the 20 services stay in Docker Compose. To be confirmed against the official WEB303 assessment requirements (Phuntsho).
- Profile images (JT-102, JT-103): Should tasks with additive protobuf changes; each proto diff needs Phuntsho's explicit approval before implementation (Section 15, 18 Nov 2026).
- No sub-tasks in Sprint 4; the acceptance criteria in each task are the checklist.
- Sprint reviews: `docs/pm/sprint-1-review.md` (15 of 26 tasks, 49 of 80 points), `docs/pm/sprint-2-review.md` (21 of 23 tasks, 60 points, with the two-day extension) and `docs/pm/sprint-3-review.md` (17 of 25 tasks, 43 of 62 points, 34 of 44 Must points; 8 tasks and 19 points carried into Sprint 4).
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
| 11 Nov 2026 | Job-posting checks verification through an `EmployerVerifier` port; unexpected employer errors are `INTERNAL`. Until JT-57 adds the real `EmployerService.GetCompanyProfile` client, a placeholder never verifies, so `PublishJob` returns `INTERNAL` with "employer verification is not available until JT-57" (approved by Jigden during JT-38) | No job can be published without a real verification check. Superseded by the JT-57 row below |
| 11 Nov 2026 | Job-posting verifies employers through `EmployerService.GetCompanyProfile` using `grpcclient.UnaryClientInterceptor()`: a public call that forwards the caller's `x-user-id` and `x-user-role` and sends no `INTERNAL_TOKEN`. `EMPLOYER_ADDR` is required; the connection is lazy and Compose has no `depends_on: employer`. Only `VERIFIED` counts as verified; `UNVERIFIED`, `REJECTED` and `NOT_FOUND` (no profile) are not verified (`FAILED_PRECONDITION`); `UNSPECIFIED`, unknown values and every other error, including unavailable and timeouts, are `INTERNAL` with the fixed message "employer verification check failed" (`ErrEmployerCheckFailed`). The placeholder verifier is removed (approved by Jigden during JT-57) | Fails closed and never leaks employer error details. Through the gateway, an unreachable employer can surface as `DEADLINE_EXCEEDED` (HTTP 504), because both hops share the 2 s `pkg/grpcclient` timeout; the job is still not published |
| 11 Nov 2026 | Resume `UploadResume`, `GetResume`, `ListMyResumes` and `DownloadResume` are applicant only; employers and admins get `PERMISSION_DENIED`, checked before input. `GetResume` and `DownloadResume` are owner only: another applicant gets `PERMISSION_DENIED`, an unknown resume `NOT_FOUND`, a `resume_id` that is not a UUID `INVALID_ARGUMENT`; a missing or malformed identity or unknown role is `UNAUTHENTICATED`. `GetResumeFile` is protected by the JT-44 internal interceptor (resume-validator only) and resume needs `INTERNAL_TOKEN` (approved by Phuntsho during JT-48) | Matches the existing owner-only contract and the wrong-owner mapping `application` already uses |
| 11 Nov 2026 | Resume upload limits from the contract: `content` 1 to 2,000,000 bytes; `content_type` exactly `application/pdf`; `content` starts with `%PDF-` (deeper checks stay in `resume-validator`); `file_name` 1 to 255 characters after trimming, without `/`, `\`, NUL or invalid UTF-8; every failure is `INVALID_ARGUMENT`. The database enforces the same size, type and length limits (approved by Phuntsho during JT-48) | Sets the Section 16 limits for resume and the boundary values for testing; a stored name can never be read as a path |
| 11 Nov 2026 | Resume `ListMyResumes` lists only the caller's resumes, newest first (`resume_id` breaks ties), and metadata reads never load the file content; every resume stays PENDING until the Sprint 2 `resume.validated` and `resume.rejected` consumers, which alone set `rejection_reason` (approved by Phuntsho during JT-48) | Keeps lists small with 2 MB files and leaves validation to resume-validator |
| 11 Nov 2026 | Application-status records a new application as SUBMITTED with a first history entry (`previous_status` UNSPECIFIED); every transition stores its history entry in the same transaction, through a conditional update, so of two concurrent transitions only one is stored (the other is `FAILED_PRECONDITION`). Any transition not in Section 6 is `FAILED_PRECONDITION`; recording an application twice is `ALREADY_EXISTS`. Transitions have no RPC: the Sprint 2 event consumers drive them (approved by Phuntsho during JT-49) | Every change is in history, and status and history can never disagree |
| 11 Nov 2026 | Application-status `GetApplicationStatus` and `ListApplicationStatusHistory` are readable only by the application's applicant (role applicant) and its employer (role employer). Admins get `PERMISSION_DENIED` before input; other users `PERMISSION_DENIED`; an unknown application `NOT_FOUND`; an `application_id` that is not a UUID `INVALID_ARGUMENT`. History is listed oldest first. Application-status has no internal RPCs and no `INTERNAL_TOKEN` (approved by Phuntsho during JT-49) | Matches the contract's applicant-or-employer access and the messaging and offer mappings |
| 11 Nov 2026 | The Section 10 coverage target is measured on production code, excluding generated gomock files, with unit and integration tests together; the unit-only figure is reported alongside (as in JT-43) (approved by Phuntsho during JT-49) | Repository code is tested with Testcontainers by design, so unit tests alone cannot reach it |
| 12 Nov 2026 | Applicant-profile `CreateMyProfile`, `GetMyProfile` and `UpdateMyProfile` are applicant only and always act on the caller's own profile; employers and admins get `PERMISSION_DENIED`, checked before input. A second create is `ALREADY_EXISTS`; get or update without a profile is `NOT_FOUND`. `UpdateMyProfile` replaces the summary, education and skills (an empty list clears it) and keeps `created_at`. No internal RPCs and no `INTERNAL_TOKEN` (approved by Phuntsho during JT-50) | The contract has no RPC for other roles, and the request names no applicant |
| 12 Nov 2026 | Applicant-profile limits, in characters after trimming: `summary` 0 to 2000 (line breaks allowed); up to 20 education entries, `institution` and `qualification` 1 to 150 on one line; `start_year` 1950 to next year; `end_year` 0 (ongoing) or `start_year` to 10 years ahead; up to 50 skills, each 1 to 50 on one line, no duplicates ignoring case; no invalid UTF-8 or other control characters; every failure is `INVALID_ARGUMENT` naming the entry. The database enforces the same limits except the clock-based year bounds (approved by Phuntsho during JT-50) | Sets the Section 16 limits for applicant-profile and the boundary values for testing |
| 12 Nov 2026 | Company-verification publishes `employer.verified` and `employer.verification_rejected` through a transactional outbox: `Decide` stores the decision and the event (table `outbox_events`) in one transaction, and a relay in the same process publishes unpublished events oldest first through `pkg/events` every second, stops at the first failure and then sets `published_at`. Each publish waits at most 10 s for the broker confirmation, and a failed publish drops the connection so the next one reconnects. The envelope producer is `company-verification`, `event_id` a new UUID and `occurred_at` the decision time. A crash between a confirmed publish and `published_at` publishes the event again, which consumers ignore by `event_id` (approved by Jigden during JT-56) | Publishing after the commit without an outbox could lose an event for good: a decided request cannot be decided again and an approved employer cannot resubmit |
| 12 Nov 2026 | `pkg/events` gets the shared consumer `events.Consume` with the JT-61 queue policy (quorum queue, `x-delivery-limit` 3, at-least-once dead-lettering to `<queue>.dead`, `reject-publish`), prefetch 1, a 10 s limit per handler call (a timeout is retried), backoff before redelivery, reconnection with backoff, and `events.Permanent` for errors that go straight to the dead queue; the message body and broker URL are never logged (proposed in JT-56 for review by Phuntsho as contracts lead and Sangay as DevOps lead) | Every Sprint 2 consumer can reuse one tested implementation instead of writing its own |
| 12 Nov 2026 | Employer consumes both verification events from the queue `employer.verification`. `ApplyVerification` claims the `event_id` in `processed_events` and stores the decision in `employer_verifications` (one row per employer) in one transaction, then updates the profile's status, reason and `updated_at`. Only a later `occurred_at` replaces the stored decision (an equal or older one is stale: marked processed, nothing changed), so events in the wrong order cannot undo a newer decision; an approval clears the reason. A decision for an employer without a profile is kept and copied by `Create`; both take a per-employer transaction advisory lock, so a decision arriving during `Create` is never lost (approved by Jigden during JT-56) | Company-verification cannot check that a profile exists, and RabbitMQ redelivery and retries can repeat or reorder events |
| 12 Nov 2026 | Employer verification event checks: the producer must be `company-verification`, `event_id` and `employer_id` must be UUIDs, `occurred_at` set, the data type must match the routing key, a rejection needs a trimmed reason of 1 to 500 characters (the JT-33 limit, no NUL or invalid UTF-8) and an approval no reason. A failing event goes straight to the dead queue; database errors are retried. The `employer_verifications` columns enforce the same rules (JT-56) | Sets the boundary values for testing and keeps bad events from blocking the queue |
| 12 Nov 2026 | Shared event fixtures live in `tests/contract/fixtures/<routing key>.json`, are checked against `pkg/events` by a contract test, and are loaded by the consumer tests; the first are `employer.verified` and `employer.verification_rejected` (JT-56) | Producer and consumer test against the same JSON |
| 12 Nov 2026 | JT-39 (category-tag) reassigned from Sangay to Jigden, and Section 4 lists Jigden as owner | JT-39 is a Must carryover in Sprint 2; Jigden was the backup owner |
| 12 Nov 2026 | Category-tag seeds categories, skill tags and locations by migration with fixed UUIDs: categories `10000000-0000-4000-8000-0000000000NN`, tags `20000000-...`, locations `30000000-...` (NN counts up). The first seed has 12 categories, 30 tags and 21 locations (the 20 dzongkhags and Remote). An ID is never changed or reused; new rows come in a new migration. There are no write RPCs (approved by Jigden during JT-39) | IDs are the same in Compose, k3d, CI and other services' tests |
| 12 Nov 2026 | Category-tag `ListCategories`, `ListTags` and `ListLocations` are open to any authenticated role; a missing or malformed identity or an unknown role is `UNAUTHENTICATED`, checked before input. Lists are sorted by name ignoring case (`lower(name) COLLATE "C"`, then ID) and use the Section 9 pagination rules (approved by Jigden during JT-39) | Applicants, employers and admins all need the lists, and the order is the same on every database locale |
| 12 Nov 2026 | Category-tag `ResolveNames` is protected by the JT-44 internal interceptor (job-alert only) and has no role check. IDs that are unknown or not UUIDs are left out of the response without an error; duplicates are removed ignoring case; more than 100 IDs in either list is `INVALID_ARGUMENT`; an empty request returns empty lists; results are sorted like the lists. Category-tag needs `INTERNAL_TOKEN` (approved by Jigden during JT-39) | Follows the contract, and job-alert already turns any missing ID into `INVALID_ARGUMENT` |
| 12 Nov 2026 | Category-tag name limits: 1 to 100 characters, no leading or trailing spaces, no control characters, unique ignoring case; database CHECK constraints and unique indexes enforce them. There is no free-text request field (JT-39) | 100 matches the job-posting `location` limit, so every location name fits in a job |
| 12 Nov 2026 | Review maintains a local `application_participants` table from `application.submitted` (consumer in Sprint 2), following the Offer and Messaging design, and Section 4 lists review consuming `application.submitted`. `CompleteReview` requires role `employer` and the stored `employer_id` of the application; unknown or not yet processed applications are `NOT_FOUND`, another employer `PERMISSION_DENIED`, a second review `ALREADY_EXISTS` (JT-51, approved by Phuntsho during JT-51) | Settles the contract's open question of how application ownership is verified without a new synchronous call |
| 12 Nov 2026 | Review `GetReview` is readable only by the owning employer; applicants, admins and other employers get `PERMISSION_DENIED`, checked by role before input. Score must be 1 to 5 and the decision SHORTLISTED or REJECTED (`INVALID_ARGUMENT` otherwise); scores do not decide shortlisting. Review has no free-text fields, no internal RPCs and no `INTERNAL_TOKEN` (JT-51, approved by Phuntsho during JT-51) | The score is the employer's internal assessment; the applicant sees the outcome through application-status |
| 15 Nov 2026 | Review consumes `application.submitted` from the queue `review.applications` through `events.Consume`, following the JT-74 Offer design. The producer must be `application` and all five IDs UUIDs (stored lowercase); any other event goes to the dead queue at once. `RecordApplication` claims the `event_id` in `processed_events` and stores the participants in one transaction; a repeated `event_id` changes nothing, the same participants under a new `event_id` are accepted unchanged, and different participants for a stored application go to the dead queue without keeping the claim. Database errors are retried under the JT-61 policy (JT-79, approved by Phuntsho during JT-79) | Review learns the owning employer without a synchronous call, and redelivery or a corrupt event can never change who owns an application |
| 15 Nov 2026 | Review `review.completed` carries `application_id` and `decision` `SHORTLISTED` or `REJECTED` (the values review stores; application-status reads them in JT-81), producer `review` and `occurred_at` = the review's `created_at` in UTC. `CompleteReview` stores the event in `outbox_events` in the review's transaction, so a second review adds no event; one relay publishes oldest first every second (at once after a full batch of 50), waits up to 10 s for each confirmation, marks rows after confirmation and keeps published rows; a reconnecting publisher wraps `pkg/events.Publisher`. A crash after confirmation resends the same `event_id`. Fixture: `tests/contract/fixtures/review.completed.json` (JT-79, approved by Phuntsho during JT-79) | An event exists exactly when the review is committed, as AGENTS.md Section 9 requires |
| 15 Nov 2026 | Interview consumes `application.submitted` from the queue `interview.applications` through `events.Consume`, with the same rules as review (JT-79): producer `application` and five UUID IDs (stored lowercase) or the dead queue at once; the `event_id` claim and participants in one transaction; a repeated `event_id` changes nothing; the same participants under a new `event_id` are accepted; different participants go to the dead queue without keeping the claim; database errors are retried (JT-80, approved by Phuntsho during JT-80) | Interview, review and offer handle the same event the same way |
| 15 Nov 2026 | Interview publishes one `interview.scheduled` per scheduled interview, with `interview_id`, `application_id` and `scheduled_at` in UTC, producer `interview` and `occurred_at` = the interview's `created_at`; `details` are never in the event. `ScheduleInterview` stores it in `outbox_events` in the interview's transaction, so `ALREADY_EXISTS` (same application and time) adds no event; the relay, publisher and kept rows are as in review (JT-79). Fixture: `tests/contract/fixtures/interview.scheduled.json` (JT-80, approved by Phuntsho during JT-80) | An event exists exactly when the interview is committed; consumers such as notification-dispatcher can rely on the fixture |
| 12 Nov 2026 | Interview maintains a local `application_participants` table from `application.submitted` (consumer in Sprint 2), following the Offer, Messaging and Review design, and Section 4 lists interview consuming `application.submitted`. `ScheduleInterview` requires role `employer` and the stored `employer_id` of the application; the applicant comes from the table, never the request. Unknown or not yet processed applications are `NOT_FOUND`, another employer `PERMISSION_DENIED` (JT-52, approved by Phuntsho during JT-52) | Ownership is verified without a new synchronous call |
| 12 Nov 2026 | Interview `GetInterview` and `ListInterviews` are readable by the application's applicant and owning employer; admins, other applicants and other employers get `PERMISSION_DENIED`, checked by role before input. `ListInterviews` is ordered by `scheduled_at`, then `interview_id`, and uses the Section 9 pagination rules. Interview has no internal RPCs and no `INTERNAL_TOKEN` (JT-52, approved by Phuntsho during JT-52) | Both sides of the application need to see when the interview is |
| 12 Nov 2026 | Interview `scheduled_at` is required, stored in UTC to the microsecond, and must be strictly after the current time and at most 365 days ahead (`INVALID_ARGUMENT` otherwise). An application can have several interviews, but not two at the same time (`ALREADY_EXISTS`, unique constraint). `details` is 0 to 1000 characters after trimming; newline, carriage return and tab are allowed, other control characters and invalid UTF-8 are `INVALID_ARGUMENT` (JT-52, approved by Phuntsho during JT-52) | Allows several interview rounds and gives boundary values for testing |
| 11 Nov 2026 | Revised sprint schedule: Sprint 2 extended to 9 to 13 Nov, Sprint 3 14 to 16 Nov, Sprint 4 17 to 19 Nov; planned project completion 19 Nov (previously 17 Nov); still no buffer. Roles and service ownership in Sections 2, 4 and 5 are unchanged. Decided by Jigden as Scrum Master on 11 Nov 2026 and approved by Phuntsho as Product Owner on 12 Nov 2026 (JT-64) | Sprint 1 finished 15 of 26 tasks (49 of 80 points), so 11 tasks (31 points) were carried into Sprint 2 on top of its event and function work, and Kinley and Sangay were not available at the start of Sprint 2 |
| 12 Nov 2026 | Scope priorities if time runs short, agreed with Phuntsho as Product Owner: (1) end-to-end apply-for-a-job flow, JT-58 before JT-60; (2) remaining basic service operations; (3) JT-53 before JT-63; (4) JT-54 and JT-55 by whether they block integration or testing. Lower-priority scope is kept minimal while each task keeps its required acceptance criteria (JT-64) | The revised schedule still has no buffer, so the core flow must be protected first |
| 13 Nov 2026 | Gateway CORS instead of a Vite dev proxy: the gateway allows browser calls from one origin set in `CORS_ALLOWED_ORIGIN` (exactly lowercase `scheme://host[:port]`; Compose default `http://localhost:5173` when unset; an empty value turns CORS off; an invalid value stops startup), and the shared frontend API client sends the JWT as `Authorization: Bearer <token>`. CORS wraps the JWT middleware: a preflight (`OPTIONS` with `Origin` and `Access-Control-Request-Method`) from that origin for `GET`, `POST`, `PUT`, `PATCH` or `DELETE`, asking only for `Authorization` and `Content-Type`, gets 204 with `Access-Control-Max-Age` 600 and never reaches authentication or a service; any other preflight gets 403. Other requests from that origin get `Access-Control-Allow-Origin` and still pass the JT-31 JWT check and header stripping; other origins get no CORS headers. No credentials or cookies, no new library (proposed by Jigden during JT-53, agreed by Phuntsho as frontend shell owner in PR #47) | Port 5173 and host port 8000 are different origins. CORS also works for a built frontend, which a Vite proxy does not cover, and the JWT check stays in front of every real request |
| 14 Nov 2026 | Sprint 3 is planned at 62 points (44 Must, 18 Should) on purpose, above the figure of about 50 from the Sprint 1 retrospective (Option 1). Must tasks are the committed minimum and stay within that figure; unfinished Should tasks move to the top of Sprint 4, and Sprint 3 is not extended. The carryovers JT-54, JT-66 and JT-67 are Should tasks, keep their size and count inside each owner's share; JT-66 and JT-67 slip first. Agreed by Jigden as Scrum Master with Phuntsho as Product Owner on 14 Nov 2026 (JT-89) | Sprint 2 delivered 60 points, but over 5 days; the schedule has no buffer, so Sprint 4 absorbs unfinished Should work instead of an extension |
| 14 Nov 2026 | JT-54 (gateway body size limit) is done early in Sprint 3 instead of slipping with the other carryovers (proposed by Jigden as Scrum Master during JT-89 and agreed by Phuntsho as Product Owner in PR #50 on 14 Nov 2026) | JT-83 (Must) needs it for resume upload through the gateway |
| 14 Nov 2026 | For Sprint 3 only, JT-77 (email-sender) is assigned to Phuntsho as DevOps backup and JT-72 (audit-log consumer) to Sangay as Jigden's backup; Sangay and Jigden, as the owners, review those PRs. JT-66 (error logging) is assigned to Phuntsho as architecture lead. Ownership in Sections 4 and 5 is unchanged (agreed with Phuntsho as Product Owner on 14 Nov 2026, JT-89) | Keeps the load at 15 to 16 points per member without a handover of ownership |
| 14 Nov 2026 | In Sprint 3, a consumer whose producer is not ready builds and tests against the shared fixtures in `tests/contract/fixtures/<routing key>.json`, and is verified end to end in Docker Compose once the producer is merged (JT-89) | No task waits for another to merge; follows the R4 retrospective action |
| 14 Nov 2026 | No sub-tasks in Sprint 3 (JT-89) | Short sprint; acceptance criteria already work as the checklist |
| 14 Nov 2026 | JT-77 email-sender accepts direct HTTP JSON requests at `POST /send-email` with `recipient`, `subject` and `body`; Phuntsho, Sangay (Notification Dispatcher) and Jigden (Weekly Summary) agreed to this request format. Returns 204 after SMTP acceptance, 4xx for invalid requests and 503 for SMTP failures. The [shared HTTP contract](docs/deployment/email-sender-http-contract.md) documents the interface and retry limits: lost SMTP acceptance replies or HTTP responses leave delivery uncertain, so retries can duplicate email; there is no idempotent delivery guarantee | Gives both calling functions one written interface while preserving the JT-29/JT-61 transport for event-triggered functions |
| 15 Nov 2026 | Gateway limits every request body to 3 MiB (3,145,728 bytes), agreed by Kinley as resume owner on 15 Nov 2026 (JT-54). The largest valid `UploadResume` request (`body: "*"`: 2,000,000 bytes of content as base64, a 255 character file name and `application/pdf`) is about 2.67 MB | Every valid resume upload fits with about 475 KB to spare, larger bodies stop at the gateway, and the limit stays below gRPC's 4 MiB default |
| 15 Nov 2026 | Gateway body limit behaviour: `middleware.LimitBody` sits after `Authenticate` and before grpc-gateway, so a request without a valid token gets 401 before its body is read, and register and login are limited too. A `Content-Length` above the limit is rejected without reading the body; other bodies, including chunked ones, are read through `http.MaxBytesReader`. A body over the limit gets HTTP 413 with `{"code":3,"message":"request body exceeds 3145728 bytes","details":[]}` (code 3 is `INVALID_ARGUMENT`, since grpc-gateway maps no gRPC code to 413); an unreadable body gets HTTP 400 with code 3. `GET /healthz` and `GET /metrics` have no body and are not limited (JT-54) | One clear error on every route, and anonymous clients cannot make the gateway read large bodies |
| 15 Nov 2026 | Analytics consumes the eight outcome events from the quorum queue `analytics.outcomes` with `events.Consume`. The handler calls the `RecordEvent` use case: a duplicate `event_id` is acknowledged without a new row, a database error is retried, and an event that can never be counted goes to `analytics.outcomes.dead` with `events.Permanent`. An event is invalid when its producer is not the Section 4 producer for its type (`job.*` from `job-posting`, `application.submitted` and `application.withdrawn` from `application`, `application.status_changed` from `application-status`, `offer.*` from `offer`), its data type does not match the routing key, `occurred_at` is missing, or it fails the JT-36 rules. IDs inside the data are not checked, because analytics does not store them (approved by Jigden during JT-85) | Same producer rule as JT-56, so a mislabelled event is never counted, without checks on fields analytics never uses |
| 15 Nov 2026 | JT-85 adds the shared fixtures `job.published`, `job.closed` and `application.status_changed` in `tests/contract/fixtures`, with contract tests; job-posting and application-status test their producers against them | Analytics needed them before the producers were ready (R4 retrospective action) |
| 15 Nov 2026 | Auth serves `AuthInternalService` behind the JT-44 internal interceptor: `INTERNAL_TOKEN` is required at startup (an invalid value stops it), `ListUsersByRole` is allowed only for `weekly-summary` and `GetUser` only for `notification-dispatcher`; the public `AuthService` RPCs are unchanged. Compose sets `INTERNAL_TOKEN` in the auth block (decided by Jigden as auth owner during JT-86) | `ListUsersByRole` returned admin emails to any caller without a token. Notification-dispatcher must call `GetUser` through `grpcclient.InternalUnaryClientInterceptor` |
| 15 Nov 2026 | Weekly-summary: `POST /` runs once (the PingSource CloudEvent body is ignored, at most 64 KiB). The period is the rolling 7 days before the run, ending now in UTC truncated to the microsecond. Admins come from `ListUsersByRole(ROLE_ADMIN)` with page size 100 until `next_page_token` is empty; a repeated token or more than 100 pages fails the run, users whose role is not admin are skipped and duplicate emails (ignoring case) are sent once. One plain-text email per admin goes through `POST /send-email` with a 30 s timeout, and only 204 counts. All reads happen before any email, so a failed read sends nothing; a failed email does not stop the others but the run returns 503. 204 only when every email was accepted or there are no admins; 409 while a run is in progress; each run is limited to 4 minutes. Dates in the email are UTC (decided by Jigden as owner during JT-86) | A retried run emails every admin again; accepted because email-sender has no deduplication (JT-77 contract) |
| 15 Nov 2026 | Weekly-summary configuration: `SERVICE_NAME=weekly-summary`, `ANALYTICS_ADDR`, `AUTH_ADDR`, `EMAIL_SENDER_URL` (the full endpoint; Compose `http://email-sender:8080/send-email`), `INTERNAL_TOKEN` and optional `HTTP_PORT`; startup fails on any missing or invalid value. Compose runs it in the `functions` profile without a host port, triggered by `docker compose --profile functions exec weekly-summary wget -S -O- --post-data='' http://localhost:8080/`. `deploy/knative/weekly-summary.yaml` holds the Knative Service (`cluster-local` visibility, `containerConcurrency` 1, 300 s timeout, token from the Secret `weekly-summary-credentials`, `EMAIL_SENDER_URL` set at deploy time from the email-sender Knative Service address because Kourier does not route the bare name) and a PingSource every Monday at 08:00 Asia/Thimphu (JT-86) | Follows the JT-61 execution decision; the email-sender contract asks each caller to document its URL setting |
| 15 Nov 2026 | Deadline-reminder: `POST /` runs once (the PingSource CloudEvent body is ignored, at most 64 KiB). The window is `[now, now + 48 h)` in UTC truncated to the microsecond. Jobs come from `ListJobsClosingBetween` and savers from `ListJobSavers`, both with page size 100 until `next_page_token` is empty; a repeated token or more than 100 pages fails that read. All jobs are read before anything is published, so a failed job read publishes nothing; a failed saver read or publish does not stop the other jobs but the run returns 503. A job or saver listed twice is reminded once, IDs are published lowercase and a saver ID that is not a UUID is skipped. One `deadline.reminder` per saver and job through `pkg/events`, waiting up to 10 s for each broker confirmation; the publisher is opened for each run and closed after it. 204 when every reminder was published or nothing is closing; 409 while a run is in progress; each run is limited to 4 minutes (JT-82, approved by Phuntsho during JT-82) | A function has no database and Knative scales it to zero, so it keeps no state between runs |
| 15 Nov 2026 | Deadline-reminder duplicate rule: each reminder's `event_id` is a UUIDv5 (namespace `6a1f3c2e-5d4b-4f8a-9c7e-2b1d0e9f8a71`) of `job_id/applicant_id/deadline` (deadline in RFC 3339 with nanoseconds, UTC). A retried run, a second run in the same window and the next day's run (the 48 hour windows overlap) republish the same `event_id`, and consumers ignore an `event_id` they have already handled (Section 9). A changed deadline gives a new `event_id`, so the applicant is reminded of the new date. `occurred_at` is the run time. Fixture `tests/contract/fixtures/deadline.reminder.json` (JT-82, approved by Phuntsho during JT-82) | Needs no store in the function; consumers already deduplicate by `event_id` |
| 15 Nov 2026 | Deadline-reminder configuration: `SERVICE_NAME=deadline-reminder`, `JOB_POSTING_ADDR`, `SAVED_JOBS_ADDR`, `RABBITMQ_URL` (`amqp://` or `amqps://` with a host), `INTERNAL_TOKEN` and optional `HTTP_PORT`; startup fails on any missing or invalid value. Both internal calls use `grpcclient.InternalUnaryClientInterceptor` with caller `deadline-reminder`. Compose runs it in the `functions` profile without a host port, triggered by `docker compose --profile functions exec deadline-reminder wget -S -O- --post-data='' http://localhost:8080/`. `deploy/knative/deadline-reminder.yaml` holds the Knative Service (`cluster-local` visibility, `containerConcurrency` 1, 300 s timeout, token from the Secret `deadline-reminder-credentials`, broker URL from `rabbitmq-credentials`) and a PingSource every day at 07:00 Asia/Thimphu (JT-82, approved by Phuntsho during JT-82) | Follows the JT-61 execution decision and the weekly-summary layout (JT-86) |
| 15 Nov 2026 | JT-83 frontend pages: `/applicant/profile`, `/applicant/resumes`, and for both roles `applications/:applicationId/status` and `applications/:applicationId/interviews`; `applications/:applicationId/review` for employers only. The JT-78 application cards link to Status, Interviews and (employer) Review. Before sending, the pages check the server rules (JT-50 profile limits, JT-48 PDF of 1 to 2,000,000 bytes starting with `%PDF-` and a 1 to 255 character name, JT-51 score 1 to 5 and decision, JT-52 interview time after now and within 365 days, details up to 1000 characters); the services stay authoritative. Gateway errors are shown as a fixed message per gRPC code (and HTTP 413), never the server's text. Shared helpers live in `frontend/src/features/application-status/shared.ts`. Interview times are entered in local time and sent in UTC. A review asks for confirmation because it cannot be changed. No new libraries (JT-83, approved by Phuntsho during JT-83) | Users see the problem before a 2 MB upload or a rejected request, and messages never repeat what the user typed |
| 15 Nov 2026 | Application-status maps the hiring events to the Section 6 lifecycle: `review.completed` SHORTLISTED or REJECTED, `interview.scheduled` INTERVIEW, `offer.made` OFFERED, `offer.accepted` ACCEPTED, `offer.declined` DECLINED. Nothing else reaches UNDER_REVIEW, so `review.completed` SHORTLISTED for a SUBMITTED application is stored as two steps (SUBMITTED to UNDER_REVIEW to SHORTLISTED) in one transaction, each with its own history row and `application.status_changed`; REJECTED from SUBMITTED is one step. `interview.scheduled` when the status is already INTERVIEW (a further interview) claims the `event_id` and changes nothing. Any other transition Section 6 does not allow (for example an interview before shortlisting, an offer before an interview, anything from a final state) is not applied and goes to the dead queue at once, so the queue keeps moving. An offer event whose `applicant_id` differs from the stored applicant is a conflict and goes to the dead queue (JT-81, approved by Phuntsho during JT-81) | Section 6 stays unchanged and the history shows every step; review, interview and offer do not check each other's order, so application-status enforces it |
| 15 Nov 2026 | Application-status reads the five hiring events from its own quorum queue `application-status.hiring` through `events.Consume`, next to the JT-60 queue `application-status.applications`. Each type must come from its producer (`review`, `interview`, `offer`), with a UUID `event_id`, valid `occurred_at`, matching payload, UUID IDs (stored lowercase) and decision `SHORTLISTED` or `REJECTED`. The `event_id` claim, status, history and outbox commit together; any failure releases the claim. An unknown application or a concurrent change is retried under the JT-61 policy (JT-81, approved by Phuntsho during JT-81) | Keeps the JT-60 queue unchanged and lets each queue be inspected separately |
| 16 Nov 2026 | The weekly-summary Knative Service uses the image `dev.local/jobtrack-weekly-summary:local`, tagged and imported with `k3d image import` (JT-86; verified by Phuntsho on `jobtrack-jt29`) | Knative skips registry tag resolution only for `dev.local/`, `kind.local/` and `ko.local/` names; the plain local name failed with a Docker Hub 401 |
| 16 Nov 2026 | JT-88 frontend pages: employer `/employer/company` and `/employer/verification`; admin `/admin/verification`, `/admin/moderation`, `/admin/analytics` and `/admin/audit-log`; `report-job` and `report-job/:jobId` for all three roles, linked from the JT-78 application cards. Admins get a workspace nav, and unknown `/admin/*` paths redirect to `/admin`. Before sending, the pages check the server rules (JT-32 company profile limits and an absolute http or https website, JT-33 registration number, notes and reject reason, JT-34 job ID format and report and takedown reasons, JT-35 event type format, JT-36 start before end); the services stay authoritative. Approve, reject, dismiss and remove ask for confirmation. Company details in the admin queue load only on request through `GetCompanyProfile`. Analytics counts from local midnight of the From date to local midnight after the To date, sent in UTC; audit-log times are entered in local time and sent in UTC. Gateway errors are shown as a fixed message per gRPC code, never the server's text. No new libraries (JT-88, decided by Jigden as owner of these services and approved by Phuntsho in PR #64) | Follows the JT-83 pattern, and the queue makes one call per page instead of one per row |
| 16 Nov 2026 | Moderation publishes `moderation.job_removed` through a transactional outbox, following JT-56: `RemoveJob` stores the takedown, the ACTIONED reports and the event (table `outbox_events`, only this routing key allowed) in one transaction, so a second takedown or any failure stores no event. One relay in the same process publishes unpublished events oldest first every second (at once after a full batch of 50), waits up to 10 s for each confirmation, marks rows after confirmation and keeps published rows; a reconnecting publisher wraps `pkg/events.Publisher`. Producer `moderation`, `event_id` a new UUID, `occurred_at` the removal time. The event carries `job_id` and the trimmed `reason` as in `events.proto`; the removing admin stays in `removed_jobs.removed_by` (JT-34), with no contract change. Takedowns made before JT-87 have no outbox row and are not backfilled (synthetic data). Fixture: `tests/contract/fixtures/moderation.job_removed.json` (decided by Jigden as moderation owner during JT-87) | An event exists exactly when the takedown is committed, and job-posting (JT-68) marks the job REMOVED; verified end to end in Docker Compose |
| 17 Nov 2026 | JT-76 consumes `job.published` through `events.Consume` on quorum queue `job-alert.jobs` with dead queue `job-alert.jobs.dead`. A valid envelope must come from `job-posting`, have UUID `event_id`, valid `occurred_at`, matching typed payload, UUID job/employer IDs and every tag ID, category ID empty or UUID (JT-38), a valid deadline and nonblank title. Malformed permanent input goes directly to the dead queue through `events.Permanent`; database failures use the JT-61 retry policy. Subscription locations are trimmed before validation, comparison and storage; whitespace-only location with no other filter is invalid (JT-76, requested by Jigden in PR #68 and approved by Phuntsho) | Accepts valid categoryless jobs and preserves the agreed subscription rules without changing contracts |
| 17 Nov 2026 | JT-76 claims `processed_events.event_id`, takes one matching-subscription snapshot, stores a `matched_subscriptions` row for EVERY matching subscription, and creates only ONE `job_alert.matched` outbox event per unique applicant for that published job, all in one transaction. Domain owns category membership AND any tag overlap AND exact location matching (empty individual filters unrestricted), alert ordering and applicant deduplication; app owns the transaction orchestration and event creation. PostgreSQL narrows candidates with equivalent SQL predicates to avoid loading all subscriptions, and domain rechecks them; tests cover SQL/domain parity, including categoryless jobs. Matches are ordered by alert ID and recipients by applicant ID. Duplicate source event IDs do not rematch or create events, including no-match deliveries; subscriptions created after the committed snapshot are not backfilled. A failure rolls back claim, matches and events together (JT-76, approved by Phuntsho after Jigden's PR #68 review) | Keeps Section 9 hexagonal ownership, efficient querying, atomic idempotency and one notification per applicant/job; payload remains job_id/applicant_id, with no alert_id contract change |
| 17 Nov 2026 | JT-76 stores `job_alert.matched` with producer `job-alert`, a new UUID saved once in `outbox_events` and the matching time in UTC. One relay publishes oldest first in batches of 50 every second (immediately after a full batch), waits up to 10 s for each broker confirmation and marks rows only afterwards. A reconnecting publisher wraps `pkg/events.Publisher`; failures retain rows and resends preserve event IDs. Consumer and relay stop and join before publisher/client/database close. Published rows and match snapshots are retained; existing processed events are not rebuilt (JT-76, following JT-75/JT-87, documented for Jigden's PR #68 review) | Restart-safe outbox delivery without new infrastructure; consumers must deduplicate a resend after uncertain confirmation or marking |
| 18 Nov 2026 | Job-posting applies `moderation.job_removed` (queue `job-posting.moderation`) in one transaction: it claims the `event_id` in `processed_events` (`ON CONFLICT DO NOTHING`), locks the job row, sets the status to `removed` from any other status and, only when the job was `published`, stores `job.closed` in its `outbox_events`, which the JT-68 relay publishes. A redelivered `event_id`, an unknown job or an already removed job changes nothing and stores no event; a draft or closed job is removed without `job.closed`. Producer `job-posting`, `event_id` a new UUID, `occurred_at` the removal time. Jobs removed before JT-90 are not backfilled (synthetic data) (JT-90, fixed by Jigden as job-posting backup; rule approved by Sangay as job-posting owner in PR #69) | Follows the 3 Nov 2026 decision so `job-search` and `saved-jobs` learn about removed jobs; a closed job already sent `job.closed` (analytics would count it twice) and a draft job was never published |
| 17 Nov 2026 | Sprint 4 carryover reassignment: JT-73 (frontend job pages, Must) to Phuntsho, and JT-72 (audit-log consumer) and JT-70 (saved-jobs consumer) to Jigden; Sangay reviews all three PRs. JT-71, JT-69, JT-66, JT-84 and JT-67 keep their owners. Ownership in Sections 4 and 5 is unchanged (agreed by Jigden as Scrum Master with Phuntsho as Product Owner, JT-98) | Sangay held 13 of the 19 carried points; he keeps the notification-dispatcher (JT-71), which the k3d deployment (JT-92) needs |
| 17 Nov 2026 | No sub-tasks in Sprint 4; the acceptance criteria in each task are the checklist (JT-98) | Short final sprint; same practice as Sprints 1 to 3 |
| 17 Nov 2026 | JT-101 (frontend polish: MUI theme, shared layout and consistent page styling) added to Sprint 4 as a Should task, using MUI theming only and no new libraries (JT-98) | Consistent pages for UAT without adding tools (Section 7) |
| 17 Nov 2026 | JT-90 (job-posting did not publish `job.closed` on moderation removal) moved from the backlog into Sprint 4 as a Must and assigned to Jigden as job-posting backup (approved by Phuntsho as Product Owner on 18 Nov 2026, JT-98) | Restores the 3 Nov 2026 rule that a moderation removal publishes `job.closed`, while Sangay, the owner, works on JT-71 |
| 18 Nov 2026 | WEB303 deployment: all 5 functions run on Knative in k3d and the 20 services stay in Docker Compose; JT-92 verifies the functions, their triggers, connectivity and scale to zero. Open action: confirm this against the official WEB303 assessment requirements (Phuntsho as Product Owner, JT-98) | JT-29 verified Knative for the functions on the 16 GiB host, but not capacity for all 20 services in k3d (Section 16) |
| 18 Nov 2026 | JT-102 (company logo) and JT-103 (applicant profile photo) approved as Should tasks that slip after JT-66: JPEG or PNG only, a 1 MB limit, file signature validation, ownership checks and additive protobuf changes. Each proto diff needs Phuntsho's explicit approval before implementation (Phuntsho as Product Owner and contracts lead, JT-98) | Follows the resume upload pattern (JT-48) and the rule that contract changes need agreement (Section 0) |
| 18 Nov 2026 | Sprint 4 slip order: JT-66 first, then JT-102 and JT-103, then the other Should tasks; a Must task is never cut. There is no Sprint 5 and no extension: an unfinished Should task is recorded as not delivered in the closure report (JT-100) (Phuntsho as Product Owner, JT-98) | Sprint 4 ends on the planned completion date (19 Nov) with no buffer, and Sprint 3 carried 19 points into it. The "no extension" part is superseded by the 19 Nov 2026 row below |
| 18 Nov 2026 | JT-102 `employer.proto` change approved by Phuntsho as Product Owner and contracts lead at 17:16: new RPCs `PutCompanyLogo` (`PUT /v1/employer/profile/logo`, body `"*"`, with `content_type` and `content` bytes) and `GetCompanyLogo` (`GET /v1/companies/{employer_id}/logo`, returns `content_type` and `content`), and `bool has_logo = 9` on `CompanyProfile`. Additive only, following the resume pattern, with no delete RPC. JPEG or PNG only, 1 MB limit, file signature validation and ownership checks; image bytes are never included in normal profile responses. Before coding, Sangay confirms the exact diff for Team B in the PR, and the PR must pass `buf lint`, `buf breaking` and CI. JT-102 stays a Should task and must not delay Must tasks (JT-98) | Adds logo upload without changing existing fields or responses, meeting the 18 Nov profile image conditions |
| 18 Nov 2026 | Cypress against the running stack: specs that need the Docker Compose stack live in `tests/e2e/stack/`, are excluded from `cypress.config.cjs` (so CI's `npm test` stays stubbed) and run locally with `npm run test:stack` (`frontend/cypress.stack.config.cjs`); CI does not run them. They use no `cy.intercept`, create synthetic accounts unique per run, read the admin login with `cy.env()` from `CYPRESS_ADMIN_EMAIL` and `CYPRESS_ADMIN_PASSWORD` (Cypress 16), and wait for event-driven changes by refreshing the page. `stack/main-workflow.cy.js` covers the main workflow from employer registration to the accepted offer and the status history (JT-95, approved by Jigden during JT-95) | Shows the real UI and services working together without making CI depend on a running stack |
| 19 Nov 2026 | Sprint 4 extended by one day: it now ends on 20 Nov 2026 at 23:59 (Asia/Thimphu), and planned project completion moves from 19 to 20 Nov 2026, still with no buffer. Scope, the Must and Should priorities, the slip order and ownership in Sections 2, 4 and 5 are unchanged; an unfinished Should task is still recorded as not delivered in the closure report (JT-100) at the end of 20 Nov. This replaces only the "no extension" part of the 18 Nov 2026 slip order row; there is no Sprint 5 and no further extension. Agreed by Phuntsho as Product Owner with Jigden as Scrum Master on 19 Nov 2026 (JT-67) | At midday on 19 Nov only 22 of 39 Must points were Done (17 points open, including JT-92, JT-99 and JT-100), and JT-73, which UAT (JT-99) needs, merged only at midday on 19 Nov |


---

## 16. Known issues and open questions

- Notification-dispatcher has no Compose RabbitMQ forwarder and no Knative deployment or forwarder manifest. This blocks verified event delivery for JT-71 and JT-92.
- JT-77 cluster deployment dependency: `deploy/knative/email-sender.yaml` sets `SMTP_ADDR=mailpit:1025`, which requires reachable Mailpit SMTP and the `mailpit` DNS name in the function's cluster namespace. Mailpit is configured only in Compose; cluster manifests and an explicit deployment task remain to be provided by DevOps before cluster email delivery can be verified.
- Offer application lifecycle eligibility remains unresolved for Sprint 2, including required lifecycle state, interview completion, employer verification and withdrawal handling. Whether Offer should consume `application.withdrawn` and which trusted data sources support these checks require agreement; Sprint 1 checks only local participants, employer ownership and lifetime offer uniqueness (JT-45).
- JT-29 confirmed 16 GiB physical RAM and the forwarder fallback; the spike does not prove capacity for the complete 20-service deployment. Docker Desktop currently has approximately 7.75 GiB allocated.
- Promote the JT-29 transport spike into shared deployment wiring in the appropriate implementation tasks: durable broker storage, consumer idempotency, retry/dead-letter policy, operational health and credentials. Native RabbitMQSource needs additional topology CRDs/operators and was not selected. See `docs/deployment/jt-29-cluster-spike.md`.
- Final presentation and submission dates not yet announced (likely to be postponed).
- Confirm the WEB303 deployment scope (5 functions on Knative in k3d, the 20 services in Docker Compose) against the official assessment requirements (Phuntsho, open action from 18 Nov 2026, JT-98).
- Deferred until the shared event contracts are coordinated: `actor_id` in `EventEnvelope` (for `audit-log`) and `description` in `job.published` and `job.updated` (for `job-search`). Until then audit-log stores no actor (JT-35).
- JT-44 shared internal-call helpers require adoption by each internal server/caller owner before layer 2 is active. Analytics wires the server interceptor for `GetInternalSummary` (JT-36), job-posting for `ListJobsClosingBetween` (JT-38), category-tag for `ResolveNames` (JT-39) and auth for `GetUser` and `ListUsersByRole` (JT-86). Job-alert wires the internal `ResolveNames` caller with `CATEGORY_TAG_ADDR`, `INTERNAL_TOKEN` and caller `job-alert` (JT-76). NetworkPolicy manifests in `deploy/k8s` remain pending; verify they allow intended Knative function pods. Gateway filtering is implemented in JT-31.
- Remaining risk: without mTLS, a compromised internal service that holds `INTERNAL_TOKEN` could call internal RPCs. Accepted by all four members in PR #6.
- Suggest a `.gitattributes` file with `* text=auto eol=lf` so line endings match on Windows and WSL.
- The `buf` CLI version is not pinned; the team should agree on one version.
- Maximum lengths for free-text fields (title, description, reason, notes) to be set in the implementation tasks and used for boundary value tests. Set so far: auth (JT-30), employer (JT-32), company-verification (JT-33), moderation (JT-34), audit-log (JT-35), analytics (JT-36), job-posting (JT-38), resume (JT-48), applicant-profile (JT-50), category-tag (JT-39) and interview (JT-52), see Section 15.
- Moderation cannot tell whether a reported job exists or is open, because it has no allowed call to job-posting; it only knows the jobs it removed itself (JT-34). Decide in Sprint 2 whether job events or an agreed call should feed it.
- How employers view an applicant's resume during review (GetResume and DownloadResume are owner-only).
- Recording interview outcomes ("record interviews" in Section 4) needs a contract change: `interview.proto` has no outcome field or RPC. JT-52 implements scheduling and reading only; the change needs team agreement (JT-52).
- Local `.env` passwords that contain `@ : / ? # %` break the `postgres://` and `amqp://` URLs in Docker Compose. Use letters and digits only (for example `openssl rand -hex 24`).
- `.github/pull_request_template.md` is listed in Section 8 but does not exist yet.
- JT-76 adds `tests/contract/fixtures/job_alert.matched.json` with shared-envelope contract validation. The event contains only `job_id` and `applicant_id`; all matching subscription IDs stay in Job Alert's database. Existing historical outbox rows are retained and not deduplicated retroactively.
- Team B contract fixtures exist only for `employer.verified` and `employer.verification_rejected` (JT-56) `job.published` and `job.closed` (JT-85) and `moderation.job_removed` (JT-87); the other Team B events have none yet.
- Company-verification, review (JT-79), interview (JT-80), moderation (JT-87) and job-alert (JT-76) keep published `outbox_events` rows as a record; there is no cleanup yet. With more than one replica, two relays could publish the same event twice, which consumers ignore by `event_id` (JT-56).
- Employers verified by manual SQL before JT-56 have no `employer_verifications` row; the next decision event for them still applies normally (JT-56).
- Application-status lists history by each producer's `occurred_at`. If producers' clocks or events disagree (the shared fixtures do: `interview.scheduled.json` is dated after `offer.made.json`), the history order and `updated_at` can differ from the processing order; `application.status_changed` events always follow processing order. Real hiring steps are minutes or days apart, so no change was made in JT-81 (JT-81).
- `pkg/events.Publisher` does not reconnect after a broker restart. Company-verification (JT-56), resume (JT-59), review (JT-79), interview (JT-80), moderation (JT-87) and job-alert (JT-76) each wrap it in their own reconnecting publisher; deadline-reminder opens a new one for each run (JT-82). Consider moving reconnection into `pkg/events`.
- Two consumer implementations follow the same JT-61 policy: the shared `events.Consume` (employer, JT-56) and resume's own validation consumer (JT-59). They differ in reconnect timing and in when the queue is declared (resume at startup, stopping on a mismatch; `events.Consume` on each connection, retrying). Decide whether resume moves to `events.Consume` and which declaration behaviour is the standard.
- Weekly-summary in k3d: the PingSource trigger was verified on `jobtrack-jt29` in a test namespace on 16 Nov 2026 (JT-86). Email delivery in k3d still needs analytics, auth and email-sender in the same namespace and Mailpit reachable from email-sender (see the JT-77 item above).
- Knative resolves image tags through a registry unless the image name starts with `dev.local/` (or `kind.local/`, `ko.local/`), so a plain local name such as `jobtrack-email-sender:local` fails with a Docker Hub 401 in k3d (found in JT-86). `weekly-summary.yaml` uses `dev.local/jobtrack-weekly-summary:local`; `email-sender.yaml`, `resume-validator.yaml` and `deadline-reminder.yaml` still use plain local names and need the same change by their owners.
- Saved-jobs `ListJobSavers` ignores `page_token` and never returns `next_page_token`, so deadline-reminder sees at most the first 100 savers of a job. Saved-jobs also does not wire `authctx.InternalUnaryServerInterceptor`, so `INTERNAL_TOKEN` and the Section 6 allowlist are not enforced on `ListJobSavers`. Deadline-reminder already sends the token and follows `next_page_token`; both fixes belong to Sangay as saved-jobs owner (requested during JT-82, pending) (JT-82).
- Deadline-reminder in k3d needs job-posting, saved-jobs and RabbitMQ in the same namespace with the Secret `rabbitmq-credentials`. JT-82 verified it in Docker Compose only; the PingSource run in k3d is not verified yet.
- JT-88 manual check (16 Nov 2026): the audit log stays empty until JT-72 adds the audit-log consumer; the JT-88 pages need no change. Removing a job in moderation now marks it REMOVED in job-posting (JT-87).
