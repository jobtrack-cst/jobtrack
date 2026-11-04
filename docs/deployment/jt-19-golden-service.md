# JT-19 golden service guide

## Scope and prerequisites

Follow the repository-root [AGENTS.md](../../AGENTS.md). The reference service in `services/service-template/` is not a 21st production microservice and does not implement job-related business operations or a new protobuf contract.

Use the Go toolchain pinned in `go.mod` (currently `1.27.3`), Bash and standard shell utilities. Docker with an accessible daemon and Docker Compose is needed for container checks; an existing `kubectl` is needed for Kustomize rendering. Do not install or reconfigure tools as part of verification without authorization.

Run the following commands from the repository root unless a block explicitly changes directory. The full JobTrack application is still under development. [JT-19 test cases](../testing/jt-19-test-cases.md) distinguish prior execution reports from pending checks.

## Configuration

Configuration is read from the environment, not from an automatically loaded `.env` file.

| Variable | Requirement |
|---|---|
| `SERVICE_NAME` | Required for direct Go execution; lowercase kebab-case matching `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`. The template image defaults to `service-template`; generated images default to their generated name. |
| `DATABASE_URL` | Required, nonblank PostgreSQL connection URL supplied by the caller. Use a dedicated database and user; do not use production credentials for checks. |
| `RABBITMQ_URL` | Required, nonblank RabbitMQ connection URL supplied by the caller. |
| `GRPC_PORT` | Optional; default `9090`. Integer from 1 to 65535. |
| `HTTP_PORT` | Optional; default `8080`. Integer from 1 to 65535, different from `GRPC_PORT`. |

Do not put real credentials in commands saved to documentation or Git. `JWT_SECRET` and internal RPC security configuration are not implemented by this reference service.

## Startup and migrations

`cmd/server/main.go` loads configuration, opens and verifies the pgx pool through `pkg/db.OpenPool`, runs `pkg/db.Migrate`, initializes the RabbitMQ publisher, then binds the gRPC and HTTP listeners. Dependency initialization failures prevent serving. SIGINT/SIGTERM trigger graceful server shutdown and resource cleanup.

For local Go execution, the default migration path is `services/service-template/migrations` relative to the repository root. The generated service uses `services/<service-name>/migrations`. The migration directory can be injected by tests; there is no migration-directory environment variable.

`00001_initial.sql` is a Goose-tracked no-op (`SELECT 1`); it creates no business tables. The database must already exist and its user must be able to run migrations. Startup does not provision service databases or users.

The RabbitMQ publisher declares durable topic exchange `jobtrack.events`. It publishes existing protobuf envelopes as persistent JSON using the event type as routing key and waits for broker confirmation. Confirmation does not guarantee consumer processing. No event is published merely because the service starts; consumers and automatic reconnection are not implemented here.

## Build and run from the repository root

```sh
go build ./...
go test ./...
go vet ./...
```

For coverage, generator safety and Docker-backed integration checks:

```sh
go test -cover ./...
bash scripts/new-service_test.sh
go test -count=1 -tags=integration ./pkg/... ./services/service-template/...
```

The integration command creates disposable Testcontainers databases and brokers; it does not need service URLs from your environment. Use `-count=1` on ordinary Go tests too when uncached execution is required.

After privately supplying URLs for dedicated local test dependencies:

```sh
export SERVICE_NAME=service-template
: "${DATABASE_URL:?Supply a dedicated local PostgreSQL URL}"
: "${RABBITMQ_URL:?Supply a dedicated local RabbitMQ URL}"
go run ./services/service-template/cmd/server
```

The standard RPC is `grpc.health.v1.Health/Check` with an empty service name. Unknown service names return `NOT_FOUND`. HTTP `GET /healthz` returns `200` with `ok` when ready, otherwise `503` without internal details. `GET /metrics` returns Prometheus text exposition with `jobtrack_service_ready` equal to `1` or `0`. Readiness currently checks PostgreSQL; it does not continuously monitor RabbitMQ. Logs are JSON with a `service` attribute.

## Generate a service

The following intentionally creates a new directory in the checkout; use the temporary recipe below for smoke tests:

```sh
bash scripts/new-service.sh example-service
go build ./services/example-service/...
go test ./services/example-service/...
```

The generator resolves its repository from its script location, so it can be invoked from another working directory. It accepts exactly one lowercase kebab-case name and reserves `service-template`. An existing empty directory or a directory containing only an empty `.gitkeep` is accepted. Existing implementations, nonempty placeholders and destination symlinks are rejected.

It preserves hexagonal directories, Go code/tests and SQL migrations; rewrites template references in copied source and the Dockerfile, including imports, migration paths, binary names and default `SERVICE_NAME`; and leaves the template unchanged. Copying completes in a staging directory before publication, with placeholder restoration on failure. Only supported source artifacts are copied; environment files and secret/credential stores are excluded. Do not store secrets in source artifacts. Compose and Kubernetes files are not generated or edited.

## Docker image and runtime inspection

The build context must be the repository root because the repository has one Go module:

```sh
docker build --progress=plain -f services/service-template/Dockerfile -t jobtrack-jt19-verification:21a7ac0 .
docker image inspect jobtrack-jt19-verification:21a7ac0
```

Expect the Go `1.27.3` Alpine builder and Alpine `3.22` runtime, `CGO_ENABLED=0`, runtime user `10001:10001`, working directory `/app`, entry point `/usr/local/bin/service-template`, and exposed TCP ports `9090` and `8080`. Database and broker URLs must be supplied at runtime.

To inspect packaged files without starting the service, run this Bash block. It creates and removes only its own stopped inspection container and temporary files:

```bash
(
  set -eu
  inspection_dir=$(mktemp -d)
  inspection_id=
  cleanup_inspection() {
    if [ -n "$inspection_id" ]; then
      docker rm "$inspection_id" || return 1
    fi
    rm -rf "$inspection_dir"
  }
  trap cleanup_inspection EXIT
  inspection_id=$(docker create --network none jobtrack-jt19-verification:21a7ac0)
  docker cp "$inspection_id:/usr/local/bin/service-template" "$inspection_dir/server"
  docker cp "$inspection_id:/app/services/service-template/migrations/00001_initial.sql" "$inspection_dir/00001_initial.sql"
  docker cp "$inspection_id:/etc/ssl/certs/ca-certificates.crt" "$inspection_dir/ca-certificates.crt"
  test -x "$inspection_dir/server"
  test -s "$inspection_dir/ca-certificates.crt"
  cmp services/service-template/migrations/00001_initial.sql "$inspection_dir/00001_initial.sql"
)
```

## Kubernetes manifest rendering

```sh
kubectl kustomize deploy/k8s/service-template
```

This local render does not contact a cluster. Expect exactly Deployment `service-template` (one replica) and ClusterIP Service `service-template`, matching `app.kubernetes.io/name: service-template` labels/selectors, ports `9090`/`8080`, readiness probe `/healthz` on `8080`, and UID/GID `10001`.

The manifest references image `jobtrack-service-template:jt-19` with `IfNotPresent`, whereas the verification command above uses a separate verification tag. Before any separately authorized cluster test, the manifest's image tag must be built and available to the cluster. Secret `service-template-secrets` must supply keys `DATABASE_URL` and `RABBITMQ_URL`; no Secret values are included in the manifests. This guide does not deploy resources or claim cluster validation.

## Reproducible isolated Compose smoke test

This is a disposable JT-19 verification recipe, **not Sangay's official JT-27 Compose configuration**. It uses only synthetic credentials, PostgreSQL `17-alpine`, RabbitMQ `4.1-alpine`, an archived committed snapshot and a unique project. No host ports are published, including gRPC `9090`.

Run the complete block with Bash from the real repository root. It requires Compose support for `up --wait --wait-timeout`. It does not copy untracked changes or `.git`. The EXIT trap cleans up on success and failure; failed cleanup retains the temporary directory for diagnosis. Dependency healthchecks and the 150-second wait bound readiness; each HTTP request has a five-second timeout.

```bash
(
  set -eu
  repo_root=$(git rev-parse --show-toplevel)
  smoke_dir=$(mktemp -d "${TMPDIR:-/tmp}/jt19-smoke.XXXXXX")
  project="jt19-$(basename "$smoke_dir" | tr '[:upper:]' '[:lower:]' | tr '.' '-')"
  compose_file="$smoke_dir/compose.yaml"
  compose() { docker compose -p "$project" -f "$compose_file" "$@"; }
  cleanup_smoke() {
    result=$?
    trap - EXIT
    if [ -f "$compose_file" ]; then
      if ! compose down --volumes; then
        printf 'Cleanup failed; retained %s\n' "$smoke_dir" >&2
        exit 1
      fi
      for kind in container network volume; do
        case "$kind" in
          container) remaining=$(docker ps -a -q --filter "label=com.docker.compose.project=$project") ;;
          network) remaining=$(docker network ls -q --filter "label=com.docker.compose.project=$project") ;;
          volume) remaining=$(docker volume ls -q --filter "label=com.docker.compose.project=$project") ;;
        esac
        if [ -n "$remaining" ]; then
          printf 'Project resources remain; retained %s\n' "$smoke_dir" >&2
          exit 1
        fi
      done
      if docker image inspect "$project:smoke" >/dev/null 2>&1; then
        docker image rm "$project:smoke" || exit 1
      fi
    fi
    if [ -f "$smoke_dir/containers-before" ]; then
      docker ps -a --no-trunc --format '{{.ID}}' | sort > "$smoke_dir/containers-after"
      if ! cmp "$smoke_dir/containers-before" "$smoke_dir/containers-after"; then
        printf 'Container inventory changed; retained %s for inspection\n' "$smoke_dir" >&2
        exit 1
      fi
    fi
    rm -rf "$smoke_dir"
    exit "$result"
  }
  trap cleanup_smoke EXIT
  trap 'exit 1' HUP INT TERM
  printf 'Temporary workspace: %s; Compose project: %s\n' "$smoke_dir" "$project"
  docker ps -a --no-trunc --format '{{.ID}}' | sort > "$smoke_dir/containers-before"
  mkdir "$smoke_dir/repository"
  git -C "$repo_root" archive HEAD --format=tar --output="$smoke_dir/head.tar"
  tar -xf "$smoke_dir/head.tar" -C "$smoke_dir/repository"
  cd "$smoke_dir/repository"
  cp -R services/service-template "$smoke_dir/template-before"
  bash scripts/new-service.sh jt19-smoke
  diff -r "$smoke_dir/template-before" services/service-template
  if grep -R -n 'service-template' services/jt19-smoke; then exit 1; fi
  cat > "$compose_file" <<YAML
services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: jt19_smoke
      POSTGRES_USER: jt19_smoke
      POSTGRES_PASSWORD: synthetic-db-only
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: [CMD-SHELL, "pg_isready -U jt19_smoke -d jt19_smoke"]
      interval: 1s
      timeout: 3s
      retries: 90
  rabbitmq:
    image: rabbitmq:4.1-alpine
    environment:
      RABBITMQ_DEFAULT_USER: jt19_smoke
      RABBITMQ_DEFAULT_PASS: synthetic-broker-only
    volumes:
      - rabbitmq-data:/var/lib/rabbitmq
    healthcheck:
      test: [CMD-SHELL, "rabbitmq-diagnostics -q check_running && rabbitmq-diagnostics -q check_port_connectivity"]
      interval: 2s
      timeout: 10s
      retries: 60
  jt19-smoke:
    image: "$project:smoke"
    build:
      context: "$smoke_dir/repository"
      dockerfile: services/jt19-smoke/Dockerfile
    environment:
      SERVICE_NAME: jt19-smoke
      GRPC_PORT: "9090"
      HTTP_PORT: "8080"
      DATABASE_URL: "postgres://jt19_smoke:synthetic-db-only@postgres:5432/jt19_smoke?sslmode=disable"
      RABBITMQ_URL: "amqp://jt19_smoke:synthetic-broker-only@rabbitmq:5672/"
    depends_on:
      postgres:
        condition: service_healthy
      rabbitmq:
        condition: service_healthy
    healthcheck:
      test: [CMD, wget, -q, -O, /dev/null, "http://127.0.0.1:8080/healthz"]
      interval: 1s
      timeout: 3s
      retries: 60
volumes:
  postgres-data: {}
  rabbitmq-data: {}
YAML
  chmod 600 "$compose_file"
  compose config
  if ! compose up -d --build --wait --wait-timeout 150; then
    compose logs --no-color --tail 60
    exit 1
  fi
  compose exec -T postgres pg_isready -U jt19_smoke -d jt19_smoke
  compose exec -T rabbitmq rabbitmq-diagnostics -q check_running
  compose exec -T jt19-smoke wget -S -O - -T 5 http://jt19-smoke:8080/healthz > "$smoke_dir/health.body" 2> "$smoke_dir/health.headers"
  grep 'HTTP/1.1 200' "$smoke_dir/health.headers"
  test "$(cat "$smoke_dir/health.body")" = ok
  compose exec -T jt19-smoke wget -S -O - -T 5 http://jt19-smoke:8080/metrics > "$smoke_dir/metrics.body" 2> "$smoke_dir/metrics.headers"
  grep 'HTTP/1.1 200' "$smoke_dir/metrics.headers"
  printf '# HELP jobtrack_service_ready Whether the service readiness check succeeds.\n# TYPE jobtrack_service_ready gauge\njobtrack_service_ready 1\n' > "$smoke_dir/metrics.expected"
  cmp "$smoke_dir/metrics.expected" "$smoke_dir/metrics.body"
  compose exec -T postgres psql -U jt19_smoke -d jt19_smoke -At -c 'SELECT version_id, is_applied FROM goose_db_version ORDER BY id;' > "$smoke_dir/goose.actual"
  printf '0|t\n1|t\n' > "$smoke_dir/goose.expected"
  cmp "$smoke_dir/goose.expected" "$smoke_dir/goose.actual"
  compose exec -T rabbitmq rabbitmqctl -q list_connections user
  compose logs --no-color --no-log-prefix jt19-smoke > "$smoke_dir/startup.log"
  cat "$smoke_dir/startup.log"
  grep '"msg":"servers starting"' "$smoke_dir/startup.log"
  if grep '"level":"ERROR"' "$smoke_dir/startup.log"; then exit 1; fi
  for service in postgres rabbitmq jt19-smoke; do
    container_id=$(compose ps -q "$service")
    docker inspect --format '{{.State.Running}} {{.State.Health.Status}} {{.RestartCount}} {{json .HostConfig.PortBindings}}' "$container_id"
  done
  cat "$smoke_dir/health.body" "$smoke_dir/metrics.body" "$smoke_dir/goose.actual"
  cd "$repo_root"
  git status --short
  git rev-parse HEAD
)
```

Expect every container inspection to show `true healthy 0` with no host port bindings, HTTP 200 with `ok`, the readiness gauge `1`, Goose applied rows `0|t` and `1|t`, and a RabbitMQ connection for `jt19_smoke`. Confirm logs contain neither initialization failures nor credentials. Save redacted evidence outside the temporary directory before the block exits if it must be retained. The trap removes only this project's containers, volumes, network, unique image and temporary files. Never use global prune commands.

## Troubleshooting and limitations

- If Docker socket or listener access is sandbox-blocked, record BLOCKED and request authorization before retrying outside the sandbox. Do not change test assertions or daemon configuration.
- Missing configuration, occupied ports, inaccessible migration paths, database permissions and an unavailable broker prevent startup. Use the specific project's `logs` and `ps`; do not probe existing user-managed dependencies.
- A successful image build may reuse cached layers. Rendering manifests is not cluster schema validation or deployment. Publisher confirmation is not consumer processing, and no production consumer is exercised by the Compose recipe.
- The documented shell recipes have been inspected and syntax-checked during documentation work, not executed end-to-end in that step. Prior task reports record equivalent image/Compose checks; see the test document for evidence boundaries.
- Official JT-27 Compose integration and GitHub Actions under JT-28 remain pending. Actual Kubernetes cluster deployment has not been tested. Internal RPC security work remains deferred as stated in AGENTS.md Section 16.
