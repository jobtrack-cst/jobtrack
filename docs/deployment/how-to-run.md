# Run JobTrack: Compose and k3d/Knative (JT-94)

Repository snapshot: 19 November 2026. Start with [AGENTS.md](../../AGENTS.md) and
[system architecture](../architecture/system-architecture.md). Run commands from
the repository root unless a block explicitly changes directory. Use synthetic
data and Mailpit. This guide does not install new tools or change deployment scope.

## 1. Choose the execution mode and check prerequisites

| Mode | Services | Functions | Current evidence/boundary |
|---|---|---|---|
| Local development | 20 services, gateway and infrastructure in Compose; frontend on host | Opt-in `functions` profile: ordinary HTTP containers, no Knative/scale to zero; schedules triggered manually | [JT-95](../testing/jt-95-test-cases.md) records the main UI workflow against Compose; this is prior evidence, not a JT-94 rerun |
| WEB303 deployment decision | The same 20 services stay in Compose | All five functions intended on Knative in k3d | JT-92 must verify all functions, triggers, connectivity and scale to zero. Phuntsho must confirm scope against official assessment requirements (AGENTS.md Section 16) |
| JT-29 transport spike | Isolated disposable broker and hello-world sink | Sample CloudEvent receiver and minute PingSource | Proven spike, not the application or capacity proof for 20 services |

Have Docker Engine/Desktop running with Compose v2, Bash, curl, Python 3, OpenSSL,
Node.js 22/npm (matching frontend CI), and the Go version pinned in
[go.mod](../../go.mod) if running Go checks. k3d/Knative additionally needs the
existing k3d and kubectl tools, image/release download access and valid credentials
for the intended context. Check installations without changing contexts:

```bash
docker version
docker compose version
node --version
npm --version
python3 --version
go version
k3d version
kubectl version --client
```

The JT-29 host had 16 GiB RAM, about 7.75 GiB allocated to Docker, and a 5 GiB k3d
node limit. These observations are not full-stack capacity requirements or proof;
monitor actual usage with `docker stats --no-stream`. Free ports 5432, 5672,
15672, 1025, 8025, 8000 and 5173 for Compose/frontend; optional monitoring uses
loopback 9090 and 3000, and the spike uses loopback 6550 and 18080.

## 2. Configure Compose once

Keep an existing `deploy/compose/.env`. Copy only when absent:

```bash
if [ ! -f deploy/compose/.env ]; then
  cp deploy/compose/.env.example deploy/compose/.env
fi
```

Edit that ignored file privately. Do not paste its contents or an interpolated
`docker compose config` dump into reports. Compose loads the file beside
`deploy/compose/docker-compose.yml`; application binaries themselves read env vars.

| Setting | Required configuration |
|---|---|
| `POSTGRES_PASSWORD` | Replace placeholder; PostgreSQL bootstrap superuser only |
| `SERVICE_DB_PASSWORD` | Replace placeholder; local shared password for the 20 separate service users/databases |
| `RABBITMQ_USER`, `RABBITMQ_PASSWORD` | Local broker login; replace placeholder password |
| `JWT_SECRET` | Replace placeholder with at least 32 bytes; goes only to Auth and gateway |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | Synthetic placement coordinator; password must differ from placeholder and have at least 12 characters; Auth creates it only if no admin exists |
| `INTERNAL_TOKEN` | Replace placeholder; same nonempty visible ASCII value without whitespace for approved internal servers/callers, never gateway/frontend |
| `CORS_ALLOWED_ORIGIN` | Unset defaults to `http://localhost:5173`; use exactly that origin for this guide; empty disables CORS |
| `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD` | Needed only for monitoring; separate non-placeholder credentials, password at least 12 characters |

Generate values individually with `openssl rand -hex 24` for URL-embedded
passwords, `openssl rand -base64 48` for JWT and `openssl rand -hex 32` for the
internal token; place them privately in `.env`, never Git. Use letters/digits for
PostgreSQL/RabbitMQ passwords because some Compose URLs interpolate them without
encoding (`@ : / ? # %` break those URLs). Keep `.env` shell-compatible for the
existing smoke script. Changing env values does not rotate credentials already
stored in PostgreSQL, RabbitMQ, the seeded Auth account or Grafana. Coordinate
credential changes with DevOps; preserve data volumes.

Validate configuration without starting containers or exposing values:

```bash
docker compose -f deploy/compose/docker-compose.yml config --quiet
docker compose -f deploy/compose/docker-compose.yml --profile functions config --quiet
```

These validate interpolation/Compose structure, not secret strength or connectivity.
For credential-free documentation checks use `--env-file deploy/compose/.env.example`
before `-f`; example credentials are unsuitable for runtime.

## 3. Start the services, then opt into functions

```bash
docker compose -f deploy/compose/docker-compose.yml up -d --build
docker compose -f deploy/compose/docker-compose.yml ps
bash deploy/compose/smoke-test.sh
```

The default configuration builds the 20 services and gateway and starts PostgreSQL,
RabbitMQ and Mailpit. PostgreSQL's first-volume initialization creates all 20
service databases/users; service startup runs its Goose migrations. Keep gRPC
9090 inside the Docker network. The gateway is reachable at http://localhost:8000.
The smoke script checks infrastructure, 20 databases, owning-user connections,
one forbidden cross-database connection, RabbitMQ management and Mailpit. It does
not prove all services, event consumers or functions are ready. Containers without
a healthcheck can be Running without application readiness.

Enable the functions explicitly after reviewing queue compatibility:

```bash
docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build
docker compose -f deploy/compose/docker-compose.yml --profile functions ps
docker compose -f deploy/compose/docker-compose.yml logs --tail=50 resume resume-validator resume-validator-forwarder notification-dispatcher
```

The profile builds five actual function images plus the validator forwarder from
existing Dockerfiles. No separate validator image download is now required: JT-62
provides [its implementation](../../functions/resume-validator/README.md).
AGENTS.md and the [JT-61 guide](jt-61-forwarder.md) retain the historical opt-in
rationale; keep the profile opt-in. Functions have no host-published HTTP ports.
Resume provisions the durable validator inbox even with the profile off, so
uploads can queue pending later processing; see [JT-59 topology](jt-59-topology.md).
The [JT-61 retry/dead-letter and migration rules](jt-61-forwarder.md) still apply.

**Current wiring limit:** the Compose file has only `resume-validator-forwarder`.
`notification-dispatcher` starts an HTTP CloudEvents receiver, but no Compose
forwarder delivers its eight assigned RabbitMQ event types. A Running receiver
does not establish notification delivery. Its handler also needs owner review
against the unchanged envelope and durable deduplication requirements before
attaching a shared forwarder. JT-94 documents this gap without changing code or
wiring. Do not run both Compose and k3d consumers on the same function queue for a
comparison: they compete for deliveries rather than each receiving a copy.

## 4. Start the browser app and verify behavior

In a second terminal, from the repository root:

```bash
cd frontend
npm ci
npm run dev
```

Open http://localhost:5173 using `localhost` so the browser origin matches CORS.
The client calls the gateway directly; see [frontend instructions](../../frontend/README.md).
Use the seeded admin login from your private `.env`. Register synthetic employer
and applicant accounts through the UI; public registration cannot create admins.

Read-only operational probes, from the repository root:

```bash
curl -fsS http://localhost:8000/healthz
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8000/v1/auth/me
curl -fsS http://localhost:8025/api/v1/info > /dev/null
docker compose -f deploy/compose/docker-compose.yml exec -T resume wget -qO- http://localhost:8080/healthz
docker compose -f deploy/compose/docker-compose.yml exec -T rabbitmq rabbitmqctl list_queues name type messages_ready messages_unacknowledged consumers
```

Expect gateway/resume health success, HTTP 401 on `/v1/auth/me` without a JWT,
and a successful Mailpit API response. Inspect validator source/result/dead queues
and consumers; a drained queue alone does not prove a successful business effect.
In the UI: employer creates a company and requests verification; admin approves;
employer refreshes until verified and publishes a job; applicant uploads a supported
synthetic PDF, refreshes until VALIDATED and applies; employer shortlists, schedules
an interview and makes an offer; applicant accepts. Wait for each status before
the next hiring step. Check final history, not just HTTP acceptance. Resume's
[supported PDF subset](../../functions/resume-validator/README.md) is deliberately
limited. Check notifications and Mailpit separately; the dispatcher gap above
prevents treating the main hiring workflow as proof of notification delivery.

For the existing automated main workflow, follow
[tests/e2e/README.md](../../tests/e2e/README.md) to privately export
`CYPRESS_ADMIN_EMAIL`/`CYPRESS_ADMIN_PASSWORD`, keep Vite running, then run
`npm run test:stack` from `frontend`. This creates persistent synthetic records.
`npm test` uses stubbed gateway responses and is not live-stack evidence.

Compose has no PingSource scheduler. Trigger the scheduled functions manually:

```bash
docker compose -f deploy/compose/docker-compose.yml --profile functions exec -T weekly-summary wget -S -O- --post-data='' http://localhost:8080/
docker compose -f deploy/compose/docker-compose.yml --profile functions exec -T deadline-reminder wget -S -O- --post-data='' http://localhost:8080/
```

Expect 204; inspect run counts and outcomes in logs. Weekly Summary should create
one Mailpit summary per admin; inspect http://localhost:8025. Deadline Reminder
requires saved jobs closing within 48 hours; 204 with zero reminders is valid and
does not test notification delivery. See [Weekly Summary](../../functions/weekly-summary/README.md)
and [Deadline Reminder](../../functions/deadline-reminder/README.md), including
pagination and current Saved Jobs security limitations in AGENTS.md Section 16.
For a direct email smoke request, use the existing
[Email Sender run instructions](../../functions/email-sender/README.md) and
[HTTP contract](email-sender-http-contract.md): expect 204 **and** inspect Mailpit.
SMTP acceptance alone does not prove inbox arrival; retries may duplicate emails.

Optional monitoring:

```bash
docker compose -f deploy/compose/docker-compose.yml --profile functions --profile monitoring up -d --build
python3 deploy/compose/monitoring/check-targets.py
```

Wait at least two scrapes; expect 21 exact application targets UP. Follow the
[JT-93 guide](jt-93-monitoring.md) for Grafana, readiness, request/error/latency
checks and evidence. These checks do not prove function delivery or load capacity.

## 5. Reproduce the k3d/Knative foundation separately

Follow [JT-29](jt-29-cluster-spike.md) for pinned versions, checksums, prior results
and the failed native RabbitMQSource attempt. The selected transport is the Go
forwarder, not RabbitMQSource. Review the scripts first: they create/reconcile
cluster resources in `jobtrack-jt29`, with explicit context and isolated credentials.
They do not deploy the five business functions or connect to the Compose broker.

```bash
bash deploy/knative/jt29/cluster.sh
bash deploy/knative/jt29/install.sh
bash deploy/knative/jt29/deploy.sh
python3 deploy/knative/jt29/verify.py
```

Keep `/private/tmp/jobtrack-jt29.kubeconfig` private (mode 600), never print/commit
it. If overriding `JT29_KUBECONFIG`, use that path consistently. The scripts do not
switch the default context. `deploy.sh` invokes verification already; rerunning
`verify.py` produces fresh synthetic spike events. The disposable spike RabbitMQ
uses `emptyDir`; never use it for application data. Successful hello/PingSource
verification does not verify application service access, credentials or email.

## 6. Deploy business functions only after the JT-92 gates are satisfied

This is a **conditional deployment procedure**, not a verified complete rollout.
Do not apply all manifests and call it done. DevOps must first provide/review:

| Gate | Required configuration/evidence |
|---|---|
| Scope | Phuntsho confirms 5 functions in Knative + 20 services in Compose meets official WEB303 assessment requirements |
| Cluster access | Intended cluster name, namespace, kubeconfig path and context, reachable API and permissions to reconcile Knative Services, Secrets, Deployments and PingSources |
| Cross-network connectivity | A tested route/DNS mapping from function pods to Compose Auth, Analytics, Resume, Job Posting and Saved Jobs gRPC, Notification Inbox, RabbitMQ AMQP and Mailpit SMTP; Compose DNS names do not automatically resolve in k3d. Do not expose gRPC 9090 on the host as a shortcut |
| Broker | Persistent application broker storage, reachable AMQP URL and reviewed queue topology; decide how the same event bus is reached, not an unrelated spike broker |
| Secrets | Namespace Secret `rabbitmq-credentials` key `amqp-url`; `resume-validator-credentials`, `weekly-summary-credentials`, `deadline-reminder-credentials` each with `internal-token` matching the approved Compose internal servers. Dispatcher credentials/deployment still need owner wiring. Provision privately through the team process; never log credential values or put them in manifests |
| Endpoints | Manifests currently use same-namespace `resume:9090`, `analytics:9090`, `auth:9090`, `job-posting:9090`, `saved-jobs:9090`, `mailpit:1025`; deployment configuration must resolve/replace these with the reviewed reachable endpoints |
| Security | Verify each internal interceptor and allowlist; fix pending Saved Jobs adoption/pagination with its owner. NetworkPolicies remain pending and must admit intended Knative pods and broker/HTTP traffic |
| All five functions | No `deploy/knative/notification-dispatcher.yaml` or dispatcher forwarder exists. Its eight event bindings, envelope compatibility, durable deduplication and activation path need implementation/verification by the owner |

Do not guess missing endpoints, create a new bridge or copy credentials from the
isolated spike. Record the agreed connectivity/configuration in AGENTS.md through
the normal review process. Existing function READMEs assume same-namespace
services; that assumption must be reconciled with the Compose/k3d deployment.

After those gates, set deployment-specific variables in your terminal. The checks
below deliberately stop if any are unset; use the intended private kubeconfig,
explicit context and namespace on every kubectl command:

```bash
: "${JT_DEPLOY_CLUSTER:?Set reviewed k3d cluster name}"
: "${JT_DEPLOY_KUBECONFIG:?Set private kubeconfig path}"
: "${JT_DEPLOY_CONTEXT:?Set intended Kubernetes context}"
: "${JT_DEPLOY_NAMESPACE:?Set application namespace}"
k() { kubectl --kubeconfig "$JT_DEPLOY_KUBECONFIG" --context "$JT_DEPLOY_CONTEXT" -n "$JT_DEPLOY_NAMESPACE" "$@"; }
k get namespace "$JT_DEPLOY_NAMESPACE"
k get ksvc,pingsource,deployments
```

Build/import the four functions with existing manifests plus the shared forwarder:

```bash
for fn in email-sender resume-validator weekly-summary deadline-reminder; do
  docker build -f "functions/$fn/Dockerfile" -t "dev.local/jobtrack-$fn:local" .
  k3d image import "dev.local/jobtrack-$fn:local" -c "$JT_DEPLOY_CLUSTER"
done
docker build -f cmd/rabbitmq-forwarder/Dockerfile -t jobtrack-rabbitmq-forwarder:local .
k3d image import jobtrack-rabbitmq-forwarder:local -c "$JT_DEPLOY_CLUSTER"
```

Knative skips tag resolution for `dev.local/`; plain mutable local names can fail
with Docker Hub 401 despite `IfNotPresent`. Only `weekly-summary.yaml` currently
uses this prefix. Render deployment copies of the other three manifests using the
matching `dev.local/` images (or reviewed registry digests). In those copies set
the endpoint values agreed above; do not apply the original unconfigured files.
No JT-94 command here generates the missing connectivity or dispatcher manifests.
For example, this image-only rendering is valid **only if** the listed original
endpoint names already resolve to the reviewed services:

```bash
sed 's#image: jobtrack-email-sender:local#image: dev.local/jobtrack-email-sender:local#' deploy/knative/email-sender.yaml | k apply -f -
k wait --for=condition=Ready ksvc/email-sender --timeout=180s
JT_EMAIL_BASE=$(k get ksvc email-sender -o jsonpath='{.status.address.url}')
: "${JT_EMAIL_BASE:?Email Sender must have a Ready Knative address}"
sed "s#set-at-deploy-time#${JT_EMAIL_BASE}/send-email#" deploy/knative/weekly-summary.yaml | k apply -f -
sed 's#image: jobtrack-resume-validator:local#image: dev.local/jobtrack-resume-validator:local#' deploy/knative/resume-validator.yaml | k apply -f -
sed 's#image: jobtrack-deadline-reminder:local#image: dev.local/jobtrack-deadline-reminder:local#' deploy/knative/deadline-reminder.yaml | k apply -f -
k apply -k deploy/knative/forwarder
k rollout status deployment/resume-validator-forwarder --timeout=180s
```

Use the reported Email Sender address plus `/send-email`, not bare `email-sender`:
Knative routing is host-based. The weekly manifest intentionally fails startup
if its URL placeholder is left unchanged. The two schedule manifests activate
PingSources when applied: weekly Monday 08:00 and daily 07:00 Asia/Thimphu.
Deploy only when those invocations are intended. The forwarder uses the real
Knative service address so incoming events can activate a zero-replica function.
Follow [JT-61](jt-61-forwarder.md) for queue migration, delivery and health limits.
Dispatcher deployment/trigger steps remain blocked pending owner-provided wiring.

## 7. Verify k3d behavior and retain evidence

With the `k` function and variables above, after dependencies are configured:

```bash
k wait --for=condition=Ready ksvc/email-sender ksvc/resume-validator ksvc/weekly-summary ksvc/deadline-reminder --timeout=180s
k wait --for=condition=Ready pingsource/weekly-summary-schedule pingsource/deadline-reminder-schedule --timeout=180s
k get ksvc,pingsource,deployments,pods
k logs deployment/resume-validator-forwarder --tail=50
```

Ready status and a healthy forwarder process are necessary, not delivery proof.
For **each of five functions**, capture name, image, context/namespace, time,
trigger, input ID, outcome, queue/dead-letter state where applicable and evidence:

1. Upload a supported PDF through the UI; correlate its `resume.uploaded` ID through
   the forwarder, validator result and Resume's VALIDATED status. Inspect rejected
   input too. Preserve queued messages while troubleshooting.
2. Once dispatcher wiring exists, exercise its eight assigned event types; correlate
   event IDs to intended in-app recipients and Mailpit messages, including retries.
   A direct HTTP receiver request alone does not prove the RabbitMQ trigger.
3. Observe actual PingSource invocations in function logs, not only manual HTTP runs;
   confirm weekly statistics/admin emails and daily saved-job reminders. A no-op
   204 is insufficient for positive behavior; prepare synthetic eligible records.
4. Confirm Email Sender's 204 plus the synthetic email in Mailpit from the cluster
   invocation; include function-to-function URL routing in the check.
5. Leave each Knative Service idle until its revision has zero workload replicas;
   inspect `k get deployments,pods`. Then invoke its real trigger and observe
   reactivation and the business outcome. Forwarder Deployments stay running and
   are not expected to scale to zero. Do not claim five-function scale to zero
   from the hello-world result or a `min-scale: "0"` annotation alone.

Save sanitized results/screenshots in `docs/testing/` under the deployment task.
No full business-function k3d rollout, connectivity, credentials or scale-to-zero
verification was performed for JT-94. Existing JT-86 evidence verifies its
PingSource in a test namespace, with full cluster email delivery still pending.

## 8. Stop and troubleshoot without discarding data

Stop the host Vite terminal with Ctrl-C. For Compose:

```bash
docker compose -f deploy/compose/docker-compose.yml --profile functions --profile monitoring stop
docker compose -f deploy/compose/docker-compose.yml --profile functions --profile monitoring down
```

`stop` retains containers; `down` removes containers/network but preserves named
volumes. Do not use `down -v` as routine troubleshooting. PostgreSQL init only runs
on an empty volume; coordinate credential/database repair instead of assuming an
env edit updates users. Inspect a failing service with
`docker compose -f deploy/compose/docker-compose.yml logs --tail=50 SERVICE`,
replacing `SERVICE` with its actual Compose name. A 503 can mean an unavailable
upstream; CORS failures require matching the browser origin; quorum redeclaration
failures require reviewed migration, never automatic queue deletion. Avoid sharing
logs containing user data or credentials.

For local password-authentication failures after an `.env` edit, first inspect the
failing service's logs and confirm its database/user/host settings without printing
passwords. Existing PostgreSQL users and RabbitMQ credentials survive restarts;
restore the last working local configuration or coordinate credential rotation
with DevOps. Restarting containers does not rerun PostgreSQL initialization on an
existing volume. Prefer these repairs over a data reset.

If a host port is already in use, inspect `docker compose -f
deploy/compose/docker-compose.yml ps` and the process reported in the bind error.
Stop only an identified, authorized conflicting process/container, or agree on a
local host-port override with DevOps. Keep gateway URLs, browser CORS and test URLs
consistent with any override; never publish service gRPC 9090 as a workaround.

An optional local reset is a last resort. **`down -v` permanently deletes named-volume
data**, including PostgreSQL databases, RabbitMQ messages and monitoring history.
It must never run automatically. Before executing it, obtain explicit authorization
and backup confirmation that required data is backed up and restorable. The owner
must explicitly authorize deletion of the local data. Only after those
confirmations, run from the repository root:

```bash
# DESTRUCTIVE: run only after explicit authorization and backup confirmation.
docker compose -f deploy/compose/docker-compose.yml --profile functions --profile monitoring down -v
# Recreate with the privately configured .env; PostgreSQL initialization runs again.
docker compose -f deploy/compose/docker-compose.yml --profile functions --profile monitoring up -d --build
```

This is a manual recovery option, not a required setup or verification step.

For k3d use explicit-context `k get events --sort-by=.lastTimestamp`, `k describe
ksvc FUNCTION` and the relevant revision logs after replacing `FUNCTION`. Check
image resolution, Secret key references, DNS/TCP access and internal allowlists.
Leave cluster/namespace teardown to DevOps after preserving application broker
storage and deployment evidence; the JT-29 cleanup notes apply only to the spike.

## JT-94 validation boundary

Documentation checks render both existing Kustomize roots, validate Compose with
example settings (default, functions, monitoring and combined profiles), check
relative links/paths, shell-block syntax, architecture inventories against
AGENTS.md and whitespace/diff scope. They do not install Knative, build/restart
containers, invoke schedules or create data. See the accompanying
[JT-94 validation record](../testing/jt-94-docs-validation.md) for checks actually
run and remaining teammate review. No peer review, green CI or task completion is
claimed by this guide.
