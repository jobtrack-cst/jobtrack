# JobTrack

JobTrack is a job and internship application portal for students, employers and placement coordinators at the College of Science and Technology, Royal University of Bhutan. It is a shared final project for WEB303, SWE302 and SWE303.

JobTrack is under development; the complete application is not finished. This branch includes shared contracts, Go infrastructure helpers and a runnable golden service template with tests, a Dockerfile, reference Kubernetes manifests and a service generator. Many production service directories remain placeholders.

The golden template at `services/service-template/` is a reference implementation, not a 21st production microservice. It demonstrates gRPC Health/Check, PostgreSQL startup migrations, RabbitMQ publisher initialization, JSON logging, `/healthz` and `/metrics`.

See [AGENTS.md](AGENTS.md) for the authoritative architecture, service ownership, conventions, testing requirements and team workflow.

- [Golden service build, configuration and generation guide](docs/deployment/jt-19-golden-service.md)
- [JT-19 test cases and verification evidence](docs/testing/jt-19-test-cases.md)

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
  README.md                 project introduction and development status
  go.mod, go.sum            single Go module and dependency checksums
  proto/jobtrack/           common, event and per-service v1 contract directories
  gen/go/                   generated Go code
  pkg/                      grpcclient, events, authctx, logging, db
  services/                 20 production service directories plus the reference template
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

Empty directories are tracked with `.gitkeep` files. The template has been verified in an isolated Docker Compose environment. Integration with the official JT-27 Compose configuration and GitHub Actions CI under JT-28 remains pending; Kubernetes cluster deployment has not been tested. See the JT-19 verification document for the scope and limitations of each check.
