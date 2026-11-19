# JT-93: Compose monitoring

Prometheus and Grafana run in the optional `monitoring` profile alongside the
existing 20 services and gateway. Functions are outside this task's scrape set.
The profile adds two containers capped at 256 MiB each; capacity under a full
workload still needs measurement on the 16 GiB host. Prometheus retains at most
7 days or 512 MB of TSDB blocks (head/WAL storage adds overhead).

## Request instrumentation

The team approved shared instrumentation for the gateway and all 20 services.
`pkg/requestmetrics` owns process-local counters and histograms; no new dependency
or contract change is required. Every service shares one recorder between an outer
gRPC unary interceptor and its existing `/metrics` handler. Existing authentication
interceptors remain in their original order after instrumentation, so rejected
calls count too. The gateway wraps its existing authenticated handler and preserves
ResponseWriter interfaces through the already-pinned `httpsnoop` dependency.
CORS preflight responses handled by the outer CORS policy are excluded.

Exported metrics:

| Metric | Labels | Meaning |
|---|---|---|
| `jobtrack_requests_total` | `service`, `method`, `result`, `status` | Completed calls, including errors; result is `success` or `error` |
| `jobtrack_request_duration_seconds_bucket` / `_sum` / `_count` | `service`, `method` (plus `le` on buckets) | Classic duration histogram in seconds |
| `jobtrack_service_ready` | Existing scrape service label | Existing readiness check, unchanged |

Service names are literals. RPC methods come from registered unary handlers;
unrecognized names collapse to `unknown`. Gateway method is the fixed `api` label
for all paths and HTTP methods. HTTP statuses are 100–599 or `other`; gRPC statuses
are canonical codes. No paths, identifiers, headers, credentials, bodies or error
messages become labels. HTTP statuses >=400 and non-OK gRPC statuses count as
errors, including expected authentication, validation and business failures.
Panics are counted as Internal/500 and propagate unchanged; this is no recovery
middleware. Unregistered gRPC methods rejected by gRPC before interception are
not counted. GET/HEAD health and metrics, and gRPC health calls, are excluded.

Histogram boundaries are 5, 10, 25, 50, 100, 250, 500 ms, 1, 2.5, 5 and 10 s,
plus +Inf. Histograms aggregate all outcomes per method. Registered methods start
with zero count, so idle services expose genuine zero request rates after at least
two scrapes. Counters reset on restart; dashboard queries use `rate` before summing.
Scraping snapshots counters under a short lock and writes after releasing it.

The dashboard now shows per-service requests/s, error fraction (displayed as a
percentage), and mean request latency in seconds, weighted across methods and
results. It uses `$__rate_interval`. With traffic and no error series, error rate
is zero; no traffic or insufficient samples leaves error rate and latency blank.
Gateway and service values represent separate hops; do not sum them for a global
request count. Scrape availability, readiness and scrape duration remain separate.
`up=1` does not imply readiness, and scrape duration is not request latency.

## Setup and access

Run from the repository root. Start Docker Desktop first. Keep the existing
`deploy/compose/.env`; only copy `.env.example` if `.env` does not yet exist.
Add separate credentials to that ignored file:

```dotenv
GRAFANA_ADMIN_USER=change-me
GRAFANA_ADMIN_PASSWORD=change-me
```

Replace both placeholders. Generate a password with `openssl rand -hex 24`.
The startup wrapper rejects missing/example credentials and passwords shorter
than 12 characters without printing values. Credentials go only to Grafana.
Do not print interpolated Compose configuration or include `.env` in reports.
Grafana seeds the admin only on first startup; later `.env` edits do not rotate
an existing account. Change its password in Grafana's account settings and keep
local configuration consistent. Do not delete data volumes to rotate passwords.

```bash
# All three configuration checks work without Docker Engine or real credentials.
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml config --quiet
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml --profile functions config --quiet
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml --profile monitoring config --quiet
# Validate with Prometheus's own parser (requires Docker Engine).
docker compose -f deploy/compose/docker-compose.yml --profile monitoring run --rm --no-deps --entrypoint /bin/promtool prometheus check config /etc/prometheus/prometheus.yml
# Start/rebuild the full application stack plus monitoring.
docker compose -f deploy/compose/docker-compose.yml --profile monitoring up -d --build
docker compose -f deploy/compose/docker-compose.yml --profile monitoring ps
```

Prometheus: http://localhost:9090/targets. Grafana: http://localhost:3000;
log in with the new Grafana credentials, then open **Dashboards → JobTrack →
JobTrack Compose monitoring**, or http://localhost:3000/d/jobtrack-compose.
The datasource uses `http://prometheus:9090` inside Compose. Both host ports bind
only to `127.0.0.1`; service metrics on 8080 and service gRPC remain unpublished.
Configuration mounts are read-only. Provisioning is authoritative; edit the
committed JSON rather than saving UI changes. Prometheus and Grafana data persist
in `prometheus-data` and `grafana-data`.

## Validation and screenshots

Wait at least 30 seconds after startup, then run:

```bash
bash deploy/compose/smoke-test_test.sh
bash deploy/compose/smoke-test.sh
python3 deploy/compose/monitoring/check-targets.py
go test ./deploy/compose/monitoring
go vet ./deploy/compose/monitoring
curl -fsS http://127.0.0.1:3000/api/health
```

The original smoke test still checks database isolation, infrastructure health,
RabbitMQ and Mailpit. It does not check application readiness or monitoring.
The new target check requires the exact 21 application names, job `jobtrack`,
`http://<service>:8080/metrics`, and UP status for every target. A healthy count
alone is insufficient. To inspect readiness separately, use Prometheus's query
page with `jobtrack_service_ready{job="jobtrack"}`; expect 21 samples of 1.
Grafana's health endpoint should return `database: ok`; confirm provisioning by
opening the dashboard with Service = All. Request rate should show all 21 services
after at least two scrapes; error rate and latency require traffic. Readiness and
scrape panels should show 21 series even without UI traffic.

Capture report evidence on the running machine:

1. Open Prometheus `/targets`, expand the `jobtrack` group and capture all 21
   targets UP (use multiple screenshots if needed).
2. Open Grafana's dashboard with Service = All and Last 30 minutes; wait for
   several samples. Capture the request/error/latency panels and the readiness/scrape
   charts, including the legends. Generate traffic first as described below. Do not label scrape duration as request latency.
3. Save screenshots under `docs/testing/` as `jt-93-targets-*.png` and
   `jt-93-dashboard-*.png`; record the capture time, branch and runtime results
   with the report. Exclude credentials, login forms, `.env` and populated
   configuration dumps. These are validation steps, not evidence already obtained.

If a target is DOWN, inspect its container with
`docker compose -f deploy/compose/docker-compose.yml logs --tail=50 <service>`
and correct its startup problem; do not remove targets to obtain a green count.
If Grafana restarts, check that both local credentials were replaced. If a port
is occupied, free it before validation. Apply Prometheus config edits with
`docker compose -f deploy/compose/docker-compose.yml --profile monitoring restart prometheus`.
Grafana rereads dashboard files periodically; restart Grafana after datasource
provisioning changes. To release monitoring memory without stopping applications:

```bash
docker compose -f deploy/compose/docker-compose.yml --profile monitoring stop grafana prometheus
```

Do not use `down -v`: it deletes existing application and monitoring data.

## Synthetic traffic and query verification

Rebuild the gateway and all services after changing Go instrumentation, then
wait for two scrapes. The monitoring-only restart does not rebuild applications:

```bash
docker compose -f deploy/compose/docker-compose.yml --profile monitoring up -d --build
python3 deploy/compose/monitoring/check-targets.py
```

Generate safe failures without logging in or changing data. A bad login exercises
both gateway and Auth; a protected route without JWT exercises gateway rejection:

```bash
for attempt in $(seq 1 20); do
  curl -sS -o /dev/null -X POST http://localhost:8000/v1/auth/login \
    -H 'Content-Type: application/json' \
    --data '{"email":"jt93-no-account@jobtrack.test","password":"synthetic-invalid-password"}'
  curl -sS -o /dev/null http://localhost:8000/v1/auth/me
  sleep 1
done
```

Use existing synthetic UI accounts for successful requests (do not paste their
JWTs into terminal history or reports). Browse reference data and profile/list
pages for relevant services. Services with no traffic correctly stay at zero
request rate and show blank latency/error fraction. The included read-only gRPC probe exercises all 20 services using existing
Go/gRPC dependencies. It sends four empty read requests per service without
identity, plus a successful category lookup using synthetic applicant metadata.
It never calls a write RPC or prints response bodies. Run on this ARM64 Docker
host (use GOARCH=amd64 on an x86 Docker host):

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /private/tmp/jt93-probe ./deploy/compose/monitoring/probe
docker compose -f deploy/compose/docker-compose.yml cp /private/tmp/jt93-probe gateway:/tmp/jt93-probe
docker compose -f deploy/compose/docker-compose.yml exec -T gateway /tmp/jt93-probe
```

Wait for a scrape before the first probe and repeat the probe after 20 seconds,
then wait for the next scrape. This supplies a counter increase between scrapes.
The synthetic identity is used only on the trusted local network for a read-only
Category Tag call, not as an authentication mechanism or public API credential.

In Prometheus's query page, evaluate these expressions while traffic is active:

```promql
sum by (service) (rate(jobtrack_requests_total{job="jobtrack"}[1m]))
sum by (service) (rate(jobtrack_requests_total{job="jobtrack",result="error"}[1m]))
sum by (service) (rate(jobtrack_request_duration_seconds_sum{job="jobtrack"}[1m])) / sum by (service) (rate(jobtrack_request_duration_seconds_count{job="jobtrack"}[1m]))
```

For exact dashboard expressions, open each panel's query inspector; confirm
request rates are nonnegative, errors reflect returned application statuses,
latency has seconds units, and idle services are not shown with fabricated zero
latency. Validate text exposition with Prometheus's parser:

```bash
curl -fsS http://localhost:8000/metrics | docker compose -f deploy/compose/docker-compose.yml --profile monitoring exec -T prometheus promtool check metrics
```

See `docs/testing/jt-93-test-cases.md` for verification actually performed and any
remaining screenshot steps. Earlier missing-instrumentation blockers are resolved
in code; runtime evidence must be collected against the rebuilt images.

Configuration references: [Prometheus configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/),
[Grafana provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/),
[Grafana Docker configuration](https://grafana.com/docs/grafana/latest/setup-grafana/configure-docker/).


## Verified outcome (19 November 2026)

The rebuilt stack passed live smoke checks, all 21 exposition parser checks and
21/21 target health checks. All six provisioned Grafana queries returned data for
21 services during synthetic traffic. An idle-service check confirmed zero rate
and blank error/latency ratios. The screenshot capture checks passed 2/2.
See the [test report](../testing/jt-93-test-cases.md),
[dashboard](../testing/jt-93-screenshots/jt-93-dashboard.png),
[latency panel](../testing/jt-93-screenshots/jt-93-latency.png) and
[21 UP targets](../testing/jt-93-screenshots/jt-93-targets.png).
Grafana's seconds unit automatically scales small values to ms. These are
functional monitoring checks, not a load/capacity benchmark.
