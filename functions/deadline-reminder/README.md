# Deadline reminder (JT-82)

Scheduled function that publishes one `deadline.reminder` event for every
applicant who saved a job closing within the next 48 hours. It has no database;
notification-dispatcher turns the events into notifications.

## What one run does

1. Takes the window `[now, now + 48 h)` in UTC, truncated to the microsecond.
2. Opens a RabbitMQ publisher for this run only (closed when the run ends).
3. Reads every closing job from `JobPostingInternalService.ListJobsClosingBetween`
   (page size 100, following `next_page_token`). If this read fails, nothing is
   published.
4. For each job, reads the savers from `SavedJobsInternalService.ListJobSavers`
   (page size 100, following `next_page_token`) and publishes one
   `deadline.reminder` per saver through `pkg/events`, waiting up to 10 s for
   each broker confirmation.

A repeated page token or more than 100 pages stops that read. A job or saver
listed twice is reminded once; a saver ID that is not a UUID is skipped and
counted. Both internal calls use `grpcclient.InternalUnaryClientInterceptor`
with `INTERNAL_TOKEN` and caller `deadline-reminder`, which the job-posting and
saved-jobs allowlists accept (AGENTS.md Section 6).

## No duplicates

Each reminder's `event_id` is a UUIDv5 of the job ID, the applicant ID and the
deadline. A retried run, a second run on the same day and the next day's run
(the 48 hour windows overlap) all republish the same `event_id`, and consumers
ignore an `event_id` they have already handled (AGENTS.md Section 9). A changed
deadline gives a new `event_id`, so the applicant is reminded of the new date.
`occurred_at` is the run time. Fixture:
`tests/contract/fixtures/deadline.reminder.json`.

## Trigger

`POST /` runs once. The body (the PingSource CloudEvent) is ignored and limited
to 64 KiB.

| Status | Meaning |
| --- | --- |
| 204 | Every reminder was published, or nothing is closing |
| 405 | Not a POST (`Allow: POST`) |
| 409 | A run is already in progress |
| 413 | Body over 64 KiB |
| 503 | The broker, job-posting or saved-jobs failed, or at least one publish failed |

A failed saver list or publish does not stop the other jobs, but the run
returns 503 so it can be retried; the retry is safe because every reminder
keeps its `event_id`. A run is limited to 4 minutes. Logs contain the counts
(`jobs`, `reminders`, `published`, `failed`, `skipped`), a fixed failure reason
and the CloudEvent `Ce-Id` as `request_id`, never user IDs, the broker URL, the
token or upstream error text.

`GET /healthz` and `GET /metrics` are on the same port.

## Configuration

| Variable | Required | Value |
| --- | --- | --- |
| `SERVICE_NAME` | yes | exactly `deadline-reminder` |
| `HTTP_PORT` | no | default `8080` |
| `JOB_POSTING_ADDR` | yes | for example `job-posting:9090` |
| `SAVED_JOBS_ADDR` | yes | for example `saved-jobs:9090` |
| `RABBITMQ_URL` | yes | `amqp://` or `amqps://` URL with credentials |
| `INTERNAL_TOKEN` | yes | same value as job-posting and saved-jobs |

Startup fails on any missing or invalid value. Error messages never contain
the values.

## Docker Compose

Compose runs the function as a normal container in the `functions` profile
(JT-61) with no host port, so trigger it from inside the container:

```bash
cd deploy/compose
docker compose --profile functions up -d --build job-posting saved-jobs deadline-reminder
docker compose --profile functions exec deadline-reminder \
  wget -S -O- --post-data='' http://localhost:8080/
docker compose logs --tail=5 deadline-reminder
```

`HTTP/1.1 204 No Content` means every reminder was published; the log line
"deadline reminders published" shows the counts.

## k3d (Knative)

`deploy/knative/deadline-reminder.yaml` holds the Knative Service and a
PingSource that runs every day at 07:00 Asia/Thimphu. The Service is
`cluster-local`, so it has no public route. Deploy into the namespace that
already runs job-posting, saved-jobs and RabbitMQ, where the Secret
`rabbitmq-credentials` (key `amqp-url`) already exists for resume-validator:

```bash
NS=<project-namespace>
k3d image import jobtrack-deadline-reminder:local -c <existing-cluster-name>
kubectl -n "$NS" create secret generic deadline-reminder-credentials \
  --from-literal=internal-token="$INTERNAL_TOKEN"
kubectl -n "$NS" apply -f deploy/knative/deadline-reminder.yaml
kubectl -n "$NS" get ksvc deadline-reminder
kubectl -n "$NS" get pingsource deadline-reminder-schedule
```

To run once without waiting for 07:00, call it from inside the cluster:

```bash
kubectl -n "$NS" run deadline-reminder-trigger --rm -i --restart=Never \
  --image=curlimages/curl -- curl -si -X POST \
  "http://deadline-reminder.$NS.svc.cluster.local/"
```

## Tests

```bash
go test -cover ./functions/deadline-reminder/...
```
