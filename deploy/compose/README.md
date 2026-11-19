# Local development environment (Docker Compose)

Use the [end-to-end run guide](../../docs/deployment/how-to-run.md) for prerequisites,
private `.env` setup, startup, the opt-in `functions` profile, frontend, verification,
stopping and troubleshooting. It also explains how Compose services relate to the
conditional k3d/Knative deployment. [AGENTS.md](../../AGENTS.md) is authoritative.

The current [Compose file](docker-compose.yml) defines all 20 services, the gateway
and the shared infrastructure; no service block needs copying to start them.
Functions and monitoring are opt-in. The old infrastructure-only startup description
is superseded by the run guide. Keep an existing `.env`; never overwrite it with
example values or print the interpolated configuration.

| Component | From your laptop | Inside Compose |
|---|---|---|
| Gateway REST/health/metrics | http://localhost:8000 | `gateway:8080` |
| PostgreSQL | `localhost:5432` | `postgres:5432` |
| RabbitMQ | AMQP `localhost:5672`, UI http://localhost:15672 | `rabbitmq:5672` |
| Mailpit | SMTP `localhost:1025`, inbox http://localhost:8025 | `mailpit:1025` |
| Frontend (host Vite process) | http://localhost:5173 | Browser calls gateway |
| Optional Prometheus/Grafana | http://localhost:9090 and http://localhost:3000 (loopback only) | `prometheus:9090`, `grafana:3000` |

Service gRPC 9090 and HTTP 8080 stay inside the Docker network. Functions also have
no host HTTP ports. See [architecture](../../docs/architecture/system-architecture.md)
for databases, owners, event flows and internal-call security.

From the repository root after configuring the ignored `.env`:

```bash
docker compose -f deploy/compose/docker-compose.yml config --quiet
docker compose -f deploy/compose/docker-compose.yml up -d --build
docker compose -f deploy/compose/docker-compose.yml ps
bash deploy/compose/smoke-test.sh
```

The smoke script checks infrastructure/database isolation; it does not prove the
main workflow or function delivery. See the run guide for those checks and the
current notification-dispatcher transport gap. Resume provisions its validator
inbox even before functions are enabled. See [JT-61](../../docs/deployment/jt-61-forwarder.md)
for delivery/retry policy and [JT-93](../../docs/deployment/jt-93-monitoring.md)
for optional monitoring and recorded results.

Named volumes retain PostgreSQL, RabbitMQ and monitoring data. `down` without `-v`
preserves them. Changing `.env` does not rotate existing database/broker/admin
credentials. Coordinate repairs with DevOps rather than deleting volumes or queues.
