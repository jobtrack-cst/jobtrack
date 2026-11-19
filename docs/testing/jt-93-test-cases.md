# JT-93 monitoring validation — 19 November 2026

Branch: `JT-93-prometheus-grafana-monitoring`. Approved shared instrumentation
covers the gateway and all 20 services. Protobuf contracts, business logic,
authorization, local `.env` and unrelated `jt83_setup.py` were preserved.

## Automated checks

Commands run from the repository root; `GOCACHE` selects a writable sandbox cache.

```bash
GOCACHE=/private/tmp/jobtrack-jt93-go-cache go test ./...
GOCACHE=/private/tmp/jobtrack-jt93-go-cache go test -race ./pkg/requestmetrics
GOCACHE=/private/tmp/jobtrack-jt93-go-cache go vet ./pkg/requestmetrics ./gateway/... ./services/... ./deploy/compose/monitoring/...
bash deploy/compose/smoke-test_test.sh
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml config --quiet
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml --profile functions config --quiet
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml --profile monitoring config --quiet
docker compose --env-file deploy/compose/.env.example -f deploy/compose/docker-compose.yml --profile functions --profile monitoring config --quiet
git diff --check
```

All commands above passed on the final instrumentation code. The Go suite needed
sandbox escalation for existing tests that bind local listeners. An initial
sandboxed run failed on those binds; this was not an application failure. The
first unrestricted run exposed old readiness assertions that required readiness
to be the final metric line. Those assertions now check the unchanged readiness
sample and both new metric types. The subsequent complete suite passed.

| Test area | Evidence |
|---|---|
| gRPC success/failure | Response/error preserved; OK and InvalidArgument counters |
| gRPC context errors | Raw cancellation/deadline errors map to Canceled/DeadlineExceeded, matching gRPC's server conversion |
| Duration | Positive observed sum, matching histogram count and +Inf count |
| Safe labels | 100 unrecognized method strings collapse to `unknown`; requests, error messages, IDs, tokens and bodies absent |
| Authentication | Existing gateway auth/CORS/body-limit suites pass; new gateway test verifies 401 count and public login routing; shared interceptor observes InternalService rejection without invoking handler |
| HTTP behavior | Implicit 200, 403 and 500; interim 103 and repeated headers; optional Flusher preserved |
| Exclusions | Repeated health/metrics scrapes and gRPC health calls leave request metrics unchanged |
| Concurrency | Concurrent observations/snapshots preserve 2,000 exact samples; race detector passes |
| Panics | Counted as Internal/500 and propagated unchanged |
| Service wiring | Existing server lifecycle tests verify readiness plus appended metric types |
| Dashboard | All six live panels, datasource/provisioning references, service aggregation, interval and idle handling |
| Scrape inventory | Exactly 21 application targets, correct network names/port/path, no duplicates |
| Grafana credentials | Missing/example/short credentials rejected without disclosure |
| Smoke regression | IPv4/IPv6/remapped bindings, malformed ports, curl failures, DB count checks |

No new dependency or tool was installed; `httpsnoop`, gRPC, protobuf and YAML
were already pinned. No database/event implementation changed, so tagged
integration suites were not required or run.

## Live verification

Docker Engine 29.9.2 is available with sandbox escalation. The initially running
stack independently reported **21/21 UP**, and Grafana `/api/health` reported
`database: ok`. The original live smoke test also passed:

```bash
bash deploy/compose/smoke-test.sh
```

It confirmed healthy infrastructure, exactly 20 databases, per-user connections,
cross-database isolation, RabbitMQ management and Mailpit. This is stronger than
the earlier configuration-only evidence; it does not itself prove new request
instrumentation is loaded.

The full application rebuild is performed in explicit batches of three services
because this Compose version handed a whole-stack build to BuildKit concurrently
even with `COMPOSE_PARALLEL_LIMIT=2`. Two over-parallel builds were canceled before
startup. No data volume was deleted. Final image/traffic/query/screenshot results
are recorded below; they were verified against the rebuilt applications.


## Final runtime results

All 21 images rebuilt with the repository Dockerfiles using `-mod=readonly`, and
all 21 applications restarted successfully. Exact bounded build/start sequence:

```bash
services=(auth gateway analytics applicant-profile application application-status audit-log category-tag company-verification employer interview job-alert job-posting job-search messaging moderation notification-inbox offer resume review saved-jobs)
for ((i=0; i<${#services[@]}; i+=3)); do
  docker compose -f deploy/compose/docker-compose.yml --profile monitoring build "${services[@]:i:3}" || exit 1
done
docker compose -f deploy/compose/docker-compose.yml --profile monitoring up -d
```

Run that sequence in Bash. The post-rebuild checks passed:

```bash
bash deploy/compose/smoke-test.sh
python3 deploy/compose/monitoring/check-targets.py
docker compose -f deploy/compose/docker-compose.yml --profile monitoring run --rm --no-deps --entrypoint /bin/promtool prometheus check config /etc/prometheus/prometheus.yml
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /private/tmp/jt93-probe ./deploy/compose/monitoring/probe
docker compose -f deploy/compose/docker-compose.yml cp /private/tmp/jt93-probe gateway:/tmp/jt93-probe
docker compose -f deploy/compose/docker-compose.yml exec -T gateway /tmp/jt93-probe
```

The probe was compiled from the same source in a temporary file before being
added under `monitoring/probe`; the committed package also passed Go test/vet.
It received Unauthenticated on four read-only calls per service and OK on a
synthetic Category Tag lookup. Repeated probes and 20 bad HTTP logins plus 20
missing-JWT HTTP requests per round provided traffic between scrapes. HTTP
failures were all 401. No account, job or other data was created by these probes.

A Python standard-library check fetched the **actual provisioned Grafana dashboard**
via its authenticated API (local credentials held only in memory), compared every
query to the committed JSON, substituted `$service=.*` and `$__rate_interval=1m`,
and evaluated all six queries via `/api/v1/query`. Every query returned exactly
21 finite, nonnegative values. Error fractions were within 0–1, mean latency was
positive, readiness and availability were all 1. Request totals and histogram
counts were positive for all 21 services. Sanitized values and capture time are in
[jt-93-runtime-evidence.json](jt-93-runtime-evidence.json).

An additional idle-service check evaluated the first three actual dashboard
queries with `$service=review` and `$__rate_interval=30s` after its traffic stopped:
request rate was exactly zero, while error fraction and latency returned no
series. This verifies the intended absence handling in Prometheus, not just a
structural assertion about the JSON query.

Every live `/metrics` response was fetched via `docker compose exec -T <service>
wget -qO- http://localhost:8080/metrics` and piped to the deployed
`promtool check metrics`. All 21 passed, with both request metric types present.
The final target checker again reported **21/21 UP** after the traffic and idle
checks. The live smoke test again passed after application recreation.

## Report screenshots and limits

The installed Cypress 16.2.0 ran a temporary monitoring-only spec. It logged in
through Grafana's API with request logging disabled, checked all three request
panel titles and readiness, asserted no BLOCKED panel remained, and opened
Prometheus targets. The initial Electron run passed 2/2 checks; its screenshot
clipped part of the wide dashboard, so capture was repeated with an explicit
viewport. The final headless Chrome run passed **2/2** checks and produced five
reviewed screenshots. An intermediate small-viewport capture failed to find the
offscreen virtualized readiness panel; the capture spec was adjusted to check the
visible request panels, while readiness remained independently verified by API.
The inherited `ELECTRON_RUN_AS_NODE` environment flag also had to be removed from
the screenshot runner. Credentials were never placed in commands, reports or
screenshots.

- [Dashboard request panels](jt-93-screenshots/jt-93-dashboard.png)
- [Request rate with all service legends](jt-93-screenshots/jt-93-requests.png)
- [Error rate with all service legends](jt-93-screenshots/jt-93-errors.png)
- [Mean latency with all service legends](jt-93-screenshots/jt-93-latency.png)
- [All 21 targets UP](jt-93-screenshots/jt-93-targets.png)

Live label inspection also passed: every observed method was a registered RPC or
fixed `api`, results were success/error, and all labels belonged to the intended
bounded key set. Prometheus adds `exported_service` when the emitted service label
collides with its static target label; their values were verified equal on all
21 services. This does not change aggregation by `service`.

For recapture steps and interpretation, see the
[deployment guide](../deployment/jt-93-monitoring.md). High error fractions in
these screenshots are expected because the deliberate probes test authentication
failures; they do not indicate scrape failures. Brief `up=0` history around the
rebuild reflects intentional container recreation; the final state is 21/21 UP.

No 200-user load benchmark, long-running capacity test, tagged DB/event integration
suite, CI run or merge is claimed. Review and normal project delivery steps remain.
