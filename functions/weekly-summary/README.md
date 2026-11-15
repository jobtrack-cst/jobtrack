# Weekly summary (JT-86)

Scheduled function that emails every admin the outcome statistics of the last
7 days. It has no database and no broker connection.

## What one run does

1. Takes the period `end = now (UTC)`, `start = end - 7 days`.
2. Gets the counts from `AnalyticsInternalService.GetInternalSummary`.
3. Gets admin emails from `AuthInternalService.ListUsersByRole` (`ROLE_ADMIN`,
   page size 100), following `next_page_token` until it is empty. A repeated
   token or more than 100 pages stops the run. Users whose role is not admin
   are skipped; duplicate emails (ignoring case) are sent once.
4. Sends one plain-text email per admin through email-sender
   (`POST /send-email`, see the
   [email-sender HTTP contract](../../docs/deployment/email-sender-http-contract.md)).

Both internal calls use `grpcclient.InternalUnaryClientInterceptor` with
`INTERNAL_TOKEN` and caller `weekly-summary`, which the analytics and auth
allowlists accept (AGENTS.md Section 6).

## Trigger

`POST /` runs once. The body (the PingSource CloudEvent) is ignored and limited
to 64 KiB.

| Status | Meaning |
| --- | --- |
| 204 | Every admin's email was accepted by email-sender, or there were no admins |
| 405 | Not a POST (`Allow: POST`) |
| 409 | A run is already in progress |
| 413 | Body over 64 KiB |
| 503 | Analytics or auth failed (no email sent), or at least one email failed |

All reads happen before any email is sent. If one email fails, the others are
still sent and the run returns 503 so it can be retried. A retry emails every
admin again, including those who already got it; email-sender has no
deduplication, so a retried 503 can also duplicate the email that failed.

A run is limited to 4 minutes; each email-sender call to 30 s. Logs contain the
counts, a fixed failure reason and the CloudEvent `Ce-Id` as `request_id`,
never email addresses, the token or upstream error text.

`GET /healthz` and `GET /metrics` are on the same port.

## Configuration

| Variable | Required | Value |
| --- | --- | --- |
| `SERVICE_NAME` | yes | exactly `weekly-summary` |
| `HTTP_PORT` | no | default `8080` |
| `ANALYTICS_ADDR` | yes | for example `analytics:9090` |
| `AUTH_ADDR` | yes | for example `auth:9090` |
| `EMAIL_SENDER_URL` | yes | full endpoint, for example `http://email-sender:8080/send-email` |
| `INTERNAL_TOKEN` | yes | same value as analytics and auth |

Startup fails on any missing or invalid value. Error messages never contain
the values.

## Docker Compose

Compose runs the function as a normal container in the `functions` profile
(JT-61) with no host port, so trigger it from inside the container:

```bash
cd deploy/compose
docker compose --profile functions up -d --build analytics auth email-sender mailpit weekly-summary
docker compose --profile functions exec weekly-summary \
  wget -S -O- --post-data='' http://localhost:8080/
```

`HTTP/1.1 204 No Content` means the emails were accepted; open Mailpit to see
one "JobTrack weekly summary" email per admin. The seeded admin is
`ADMIN_EMAIL` from `.env`.

## k3d (Knative)

`deploy/knative/weekly-summary.yaml` holds the Knative Service and a PingSource
that runs every Monday at 08:00 Asia/Thimphu. The Service is `cluster-local`,
so it has no public route. Deploy into the namespace that already runs
analytics, auth and email-sender:

```bash
NS=<project-namespace>
k3d image import jobtrack-weekly-summary:local -c <existing-cluster-name>
kubectl -n "$NS" create secret generic weekly-summary-credentials \
  --from-literal=internal-token="$INTERNAL_TOKEN"
EMAIL_URL="$(kubectl -n "$NS" get ksvc email-sender -o jsonpath='{.status.address.url}')/send-email"
echo "$EMAIL_URL"   # for example http://email-sender.<ns>.svc.cluster.local/send-email
sed "s#set-at-deploy-time#${EMAIL_URL}#" deploy/knative/weekly-summary.yaml | kubectl -n "$NS" apply -f -
kubectl -n "$NS" get ksvc weekly-summary
kubectl -n "$NS" get pingsource weekly-summary-schedule
```

`EMAIL_SENDER_URL` must be the email-sender address that Knative reports,
because Kourier routes by host name and Knative registers only
namespace-qualified host names, not the bare name `email-sender`. If the placeholder is applied unreplaced, the function does not
start.

To run once without waiting for Monday, call it from inside the cluster:

```bash
kubectl -n "$NS" run weekly-summary-trigger --rm -i --restart=Never \
  --image=curlimages/curl -- curl -si -X POST \
  "http://weekly-summary.$NS.svc.cluster.local/"
```

Email delivery in the cluster also needs Mailpit there (AGENTS.md Section 16).

## Tests

```bash
go test -cover ./functions/weekly-summary/...
```
