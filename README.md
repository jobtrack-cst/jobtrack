# JobTrack

JobTrack is a job and internship application portal for students, employers and placement coordinators at the College of Science and Technology, Royal University of Bhutan. It is a shared final project for WEB303, SWE302 and SWE303.

The project is being scaffolded and is not yet runnable. This repository currently contains project documentation and empty directory placeholders. Implementations, contracts, configuration and tests will be added in their assigned Jira tasks.

See [AGENTS.md](AGENTS.md) for the authoritative architecture, service ownership, conventions, testing requirements and team workflow.

## Agreed technology stack

- Backend: Go in a single module, 20 microservices with hexagonal architecture, gRPC, Protocol Buffers and buf; grpc-gateway with JWT authentication.
- Data and events: PostgreSQL with a database and user per service, pgx, goose migrations and RabbitMQ.
- Serverless and email: five Knative functions, Knative Eventing and PingSource schedules; Mailpit for test email.
- Frontend: React, TypeScript, Vite, MUI and React Router.
- Development and deployment: Docker Compose, k3d, Kubernetes YAML with Kustomize, GitHub Actions and GitHub Container Registry.
- Observability and resilience: Prometheus, Grafana, JSON logs and shared gRPC timeout, retry and circuit breaker support using sony/gobreaker.
- Testing: Go testing, testify, gomock, Testcontainers, Cypress, Apache JMeter and local SonarQube.
- Project management: Jira (JT) and GitHub Flow.

## Repository layout

```text
jobtrack/
  AGENTS.md                 authoritative project context
  README.md                 project introduction and scaffold status
  proto/jobtrack/           common, event and per-service v1 contract directories
  gen/go/                   generated Go code
  pkg/                      grpcclient, events, authctx, logging, db
  services/                 one directory for each of the 20 microservices
  functions/                one directory for each of the five Knative functions
  gateway/                  API gateway
  frontend/                 React web app
  deploy/
    compose/                local environment
    k8s/                    Kubernetes manifests
    knative/                function and event source manifests
  tests/
    e2e/                    Cypress
    load/                   JMeter
    contract/               event fixtures and consumer tests
  scripts/                  service scaffolding scripts
  docs/
    architecture/
    pm/
    testing/
    deployment/
  .github/
    workflows/              CI workflows
```

Empty directories are tracked with `.gitkeep` files. The Go module, buf configuration, proto files, service code, deployment configuration, scripts, GitHub templates and CI workflows described in AGENTS.md are deferred to their assigned tasks.
