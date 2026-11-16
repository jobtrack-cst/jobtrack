# JT-76 Job Alert event integration

Initial verification on 16 November 2026; real PublishJob end-to-end verification
on 17 November 2026 (Asia/Thimphu), on branch `JT-76-job-alert-event-integration`.
The branch and clean working tree were checked before editing; repository-root
AGENTS.md was read. No contracts, shared packages, dependencies or other service
implementations changed.

## Behavior and acceptance criteria

| AC | Implementation and evidence |
|---|---|
| 1 | Production `ports.NameResolver` uses Category Tag's `ResolveNames`. The configured internal interceptor supplies token and caller `job-alert`. Startup requires that exact service name, a valid DNS/IP host and numeric port in `CATEGORY_TAG_ADDR`, and a valid `INTERNAL_TOKEN`; errors omit values. The connection closes after workers and servers stop. The bufconn client test uses the real server auth interceptor. Public subscription validation, ownership, limits and APIs remain unchanged. |
| 2 | `events.Consume`, queue `job-alert.jobs`, binding `job.published`; shared envelope validation, exact producer `job-posting`, typed payload, mandatory UUID job/employer IDs, UUID tag IDs, an empty or UUID category ID and valid deadline. Invalid input returns `events.Permanent`; database failures retry. Matching uses category membership AND any tag overlap AND exact location; each empty filter is unrestricted. |
| 3 | One PostgreSQL transaction claims `processed_events.event_id`, snapshots and records matched subscriptions, and writes all outgoing envelopes. Duplicates return successfully without rematching, including jobs with no matches. Failure rolls back the claim, matches and events. A single relay reads batches of 50 in sequence order, publishes with a 10-second confirmation timeout and marks only confirmed rows. Failed publishes reconnect on the next attempt; publish/mark failures retain the original event ID. Workers stop and join before publisher, gRPC connection and database close. |
| 4 | Shared `job_alert.matched.json` fixture and contract round trip; resolver, startup, consumer and relay unit tests; real PostgreSQL and RabbitMQ Testcontainers tests. Cases below. |
| 5 | Focused unit, integration, contract and vet checks passed. The real public API flow passed on 17 November: registered employer/applicant, public admin verification approval, real PublishJob through the gateway, observed job.published and job_alert.matched, and matching persisted claim/match/confirmed outbox rows. The earlier direct-event transport smoke remains documented separately below. |

Each matching subscription records one match and one outgoing event, even when
several subscriptions belong to one applicant. Events contain only the approved
`job_id` and `applicant_id` fields, producer `job-alert`, and a new UUID stored
once in the outbox. Matches retain subscription IDs after subscription deletion.
No matching event is backfilled for subscriptions created after a source event
was processed. Published outbox rows are retained, following Messaging and
Moderation. Run one relay per service process; multiple replicas may resend the
same stored event ID, which downstream consumers must deduplicate.

## Test cases

| Case | Partition/boundary | Expected result and automated coverage |
|---|---|---|
| JT-76-01 | Valid category lookup; unknown tag omitted | Real internal auth succeeds; returned names mapped; missing requested ID fails existing validation. `TestNames_AuthenticatedInternalRPCAndMissingIDs`. |
| JT-76-02 | Missing/invalid token, wrong service, malformed address, port 0/65536 | Startup fails with fixed errors without values. `TestConfig_RejectsUnsafeInternalSettings`; existing config tests cover valid/default/port boundaries. |
| JT-76-03 | Category-only, tag-only, location-only, each pair, all three | All seven filter combinations match. Multi-tag filter succeeds with any overlap. `TestJobs_MatchingCombinationsDuplicatesAndRollback`. |
| JT-76-04 | Category miss, tag miss, location case difference | No match when any required condition fails; location comparison is exact. Same integration test. |
| JT-76-05 | All filters empty, duplicate/unknown IDs, subscription limits | Existing domain/app/PostgreSQL subscription tests remain in the focused suite; validation behavior is preserved. |
| JT-76-06 | Same event delivered twice | One claim, one set of matches/outbox rows; real broker test checks counts after the following invalid event reaches the dead queue. `TestJobConsumer_BrokerDuplicatesInvalidAndShutdown`. |
| JT-76-07 | No subscriptions match | Claim commits with no matches or outgoing events. `TestJobs_MatchingCombinationsDuplicatesAndRollback`. |
| JT-76-08 | Nil envelope, wrong producer/type/payload, invalid IDs, missing time/deadline | Permanent error without repository invocation. `TestJobHandler_ValidationAndRetry`; real RabbitMQ test verifies wrong producer reaches `job-alert.jobs.dead` without a claim. |
| JT-76-09 | Transient repository failure | Original error propagates for shared consumer retry. `TestJobHandler_ValidationAndRetry`. |
| JT-76-10 | Outbox constraint failure after match insert | No claim or match survives; removing the injected constraint lets the same event retry. PostgreSQL matching/rollback test. |
| JT-76-11 | Publish failure or failure to mark after confirmation | No row marked; retry resends identical event ID. `TestRelay_RetriesSameIDAfterPublishOrMarkFailure`; PostgreSQL/RabbitMQ `TestJobs_OutboxConfirmedDeliveryAndRetry` retries a failed publisher and observes the original ID on the broker. |
| JT-76-12 | Connection failure/reconnect; stop during serving; full batch | Reconnect retries, Close prevents future publishing, workers stop, full batches proceed immediately. `publisher_test.go`, `relay_test.go`, real consumer shutdown test. |
| JT-76-13 | Shared notification contract | Fixture decodes as `JobAlertMatched`, correct producer/type/UUID payload, lossless envelope round trip. `TestJobAlertFixtures_SharedEnvelope`. |
| JT-76-14 | Published job omits category; malformed nonempty category; required IDs/tags | `TestJobHandler_CategorylessJobsValidateRemainingIDs` loads the shared fixture with category_id omitted and verifies it is recorded; malformed/whitespace categories, empty or malformed job/employer IDs, and empty/malformed tag entries are permanent errors without recording. No tags is valid. |
| JT-76-15 | Categoryless event through handler and PostgreSQL | `TestJobs_CategorylessEventsMatchOnlyUnrestrictedCategories` verifies tag-only, location-only and combined tag/location subscriptions match, category-only and combined category filters do not, tag/location misses do not match, duplicate delivery preserves exactly the expected matches/outbox recipients, and a malformed category creates no processed-event claim or outgoing event. |


## Commands and results

Run from the repository root. A temporary cache was needed because the sandbox
denied the normal macOS Go cache; sandboxed listener tests also failed with
`bind: operation not permitted`. The same suites passed with local listener and
Docker access. No environment blocker remains for the checks below.

```sh
git status --short --branch
git branch --show-current
# Clean; JT-76-job-alert-event-integration.

GOCACHE=/private/tmp/jobtrack-go-cache go generate ./services/job-alert/internal/ports/...
GOCACHE=/private/tmp/jobtrack-go-cache go test ./services/job-alert/... ./tests/contract
GOCACHE=/private/tmp/jobtrack-go-cache go test -tags=integration ./services/job-alert/... ./tests/contract
GOCACHE=/private/tmp/jobtrack-go-cache go vet ./services/job-alert/... ./tests/contract
git diff --check
```

Results: generation, focused unit/contract suite, focused integration suite, vet
and whitespace checks passed. Testcontainers used `postgres:17-alpine` and
`rabbitmq:4.1-alpine`. The existing adapter integration tests also ran.
Full-repository unit tests (`go test ./...`), integration tests (`go test -tags=integration ./...`) and `go vet ./...` passed locally. GitHub CI and Kubernetes deployment were not run.

## Compose evidence and reproduction

Required running dependencies: PostgreSQL, RabbitMQ and Category Tag with its
internal token configured. Compose now gives Job Alert `CATEGORY_TAG_ADDR` and
`INTERNAL_TOKEN` and waits for Category Tag health. Secrets remain in the existing
local environment; no values are recorded here. Only Job Alert was rebuilt and
recreated; running dependencies were preserved.

```sh
docker compose -f deploy/compose/docker-compose.yml up -d --build --no-deps job-alert
# Use GOARCH=amd64 on an amd64 Docker engine; verified engine here is arm64.
GOCACHE=/private/tmp/jobtrack-go-cache GOOS=linux GOARCH=arm64 CGO_ENABLED=0   go build -o /private/tmp/jt76-smoke services/job-alert/scripts/smoke.go
docker compose -f deploy/compose/docker-compose.yml cp /private/tmp/jt76-smoke job-alert:/tmp/jt76-smoke
docker compose -f deploy/compose/docker-compose.yml exec -T job-alert /tmp/jt76-smoke
```

Observed:

```text
PASS: real ResolveNames -> subscription -> duplicate job.published -> job_alert.matched via outbox
```

The smoke script creates a synthetic applicant subscription with all three
filters, using seeded category/tag IDs through the public gRPC handler and real
authenticated resolver. It binds a temporary observation queue, publishes the
same matching synthetic job event twice, and checks the outgoing job/applicant
and producer. It deletes its subscription afterwards; match, processed-event
and outbox evidence remain. The synthetic source is published directly through
`pkg/events`, as in the JT-75 smoke pattern: this does not exercise Job Posting's
`PublishJob` RPC or notification delivery.

Database evidence from the first run:

```text
source_event_id: a6ba566b-65ac-4a92-972a-5c5d870becdf
job_id:         85d4aab8-8b62-4e33-a756-34903ce09f1c
matched subscriptions: 1
confirmed outgoing events: 1
```

Inspection command (read-only, no credentials printed):

```sh
docker compose -f deploy/compose/docker-compose.yml exec -T postgres psql -U postgres -d job_alert <<'SQL'
SELECT m.source_event_id,m.job_id,count(*) AS matches,
 (SELECT count(*) FROM outbox_events o
  WHERE o.payload::jsonb->'data'->>'job_id'=m.job_id::text
    AND o.published_at IS NOT NULL) AS confirmed_events
FROM matched_subscriptions m GROUP BY m.source_event_id,m.job_id;
SQL
```

No existing queues or data were deleted. An existing classic
`job-alert.jobs` queue would need DevOps-reviewed migration rather than automatic
deletion; the verified environment accepted the quorum topology.

## Changed files

- `services/job-alert/cmd/server/`: `main.go`, `main_test.go`, new `config_test.go`, `publisher.go`, `publisher_test.go`, `relay.go`, `relay_test.go`.
- `services/job-alert/internal/adapters/clients/`: new `names.go`, `names_test.go`.
- `services/job-alert/internal/adapters/postgres/`: `alerts.go`; new `jobs.go`, `outbox.go`, `jobs_integration_test.go`, `broker_integration_test.go`, `consumer_integration_test.go`.
- `services/job-alert/internal/adapters/rabbitmq/`: new `job_consumer.go`, `job_consumer_test.go`.
- `services/job-alert/internal/app/`: new `outbox.go`, `outbox_test.go`.
- `services/job-alert/internal/ports/`: new `outbox.go`, generated `mocks/outbox_mock.go`.
- New `services/job-alert/migrations/00003_events.sql` and `services/job-alert/scripts/smoke.go`.
- `deploy/compose/docker-compose.yml`: Job Alert settings/dependency only.
- New `tests/contract/fixtures/job_alert.matched.json`, `tests/contract/job_alert_fixtures_test.go`, and this document.

Next: review the diff and rerun local checks; when ready, commit with JT-76,
push, open a PR and move the card to In Review. None of those actions, Jira
transitions or time logging were performed by this implementation session.

## Blocking review follow-up: optional published-job category

The handler now accepts an omitted/empty category ID per AGENTS.md Section 15,
while still rejecting malformed nonempty categories. Job and employer IDs remain
mandatory, and every supplied tag must be a UUID. PostgreSQL matching needed no
change: categoryless jobs match only subscriptions whose category filter is
unrestricted.

After this fix, all four checks in the commands section passed again:
focused unit/contract tests, the complete focused Testcontainers integration suite
(including JT-76-14 and JT-76-15), go vet, and git diff --check. Existing
uncommitted JT-76 work was preserved. This follow-up changed only
job_consumer.go, job_consumer_test.go, jobs_integration_test.go and this document.


## Real PublishJob end-to-end verification — 17 November 2026

**Result: PASS after refreshing a stale Job Posting container.** This verification
uses real public APIs through the JWT-authenticated gateway; no job event is
published directly by the test. Only this JT-76 test document was changed in the
repository during this verification session. All pre-existing uncommitted work
was preserved on `JT-76-job-alert-event-integration`.

### Environment finding and recovery

The first attempt legitimately registered synthetic accounts, created a company
profile, submitted a verification request, obtained public admin approval, waited
for the employer profile to become VERIFIED, created a matching subscription,
created a DRAFT job and called the real PublishJob endpoint. PublishJob returned
PUBLISHED, but the observer timed out without receiving both required events:
**that first attempt did not pass**.

Read-only inspection found the running Job Posting database at goose version 2,
with no `public.outbox_events`. The current, unchanged repository implementation
stores the event in `Jobs.UpdateStatus` / `events.go` and runs a relay, requiring
the existing `00003_events.sql`. Thus the running container was stale. Its first
attempt job remains stored as published:

- Job: `95fe98b3-9a49-4a46-a326-fa0b77dfef22`
- Employer: `d9fedc57-655e-41f3-8827-8432adb53ce8`
- Title/run: `JT76 real PublishJob dc0c936de651`

The following commands rebuilt only Job Alert (to include the categoryless
correction) and then the stale Job Posting service from unchanged source. Existing
volumes, queues, records and other containers were preserved:

```sh
docker compose -f deploy/compose/docker-compose.yml ps --format '{{.Service}} {{.State}} {{.Health}}'
docker compose -f deploy/compose/docker-compose.yml up -d --build --no-deps job-alert
# After the first attempt timed out, diagnose without writing any service data:
docker compose -f deploy/compose/docker-compose.yml exec -T postgres psql -U postgres -d job_posting <<'SQL'
SELECT job_id,employer_id,status,title FROM jobs
WHERE title LIKE 'JT76 real PublishJob %' ORDER BY created_at DESC LIMIT 5;
SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1;
SELECT to_regclass('public.outbox_events');
SQL
docker compose -f deploy/compose/docker-compose.yml up -d --build --no-deps job-posting
docker compose -f deploy/compose/docker-compose.yml logs --tail 6 job-posting
```

Job Posting startup reported `OK 00003_events.sql` and database version 3.
No other team's source, configuration, contracts or shared code was edited.
No SQL updates, synthetic verification events, manufactured identity headers or
locally signed JWTs were used.

### Successful public API flow

The repeat run used fresh synthetic accounts and:

1. Logged in as the configured seeded admin using Auth's real Login API. The
   existing credentials were captured in memory from the auth container and
   were never printed, written to evidence or passed as command-line arguments.
2. Registered and logged in an employer and applicant through Auth.
3. Read public category/tag/location lists, confirming seeded category
   `10000000-0000-4000-8000-000000000001`, tag
   `20000000-0000-4000-8000-000000000001`, and location `Thimphu`.
4. Created the employer profile and submitted a verification request. The
   authenticated admin called ApproveVerificationRequest. Polling
   GetMyCompanyProfile confirmed `VERIFICATION_STATUS_VERIFIED` after the real
   company-verification/employer event flow.
5. Created the applicant subscription with category, tag and location filters.
6. Called CreateJob and asserted `JOB_STATUS_DRAFT`.
7. Bound a temporary, exclusive AMQP observation queue to `job.published` and
   `job_alert.matched` before publishing. The observer has no publisher code.
8. Called `POST /v1/jobs/{job_id}/publish`, which invokes the real PublishJob RPC
   through grpc-gateway, and asserted `JOB_STATUS_PUBLISHED` and employer ownership.
9. Observed both typed envelopes with correct routing keys/producers and expected
   job, employer and applicant IDs.
10. Read PostgreSQL evidence: Job Posting's observed source event was confirmed;
    Job Alert had exactly one processed-event claim, expected subscription match,
    and confirmed outbox event for the target applicant/job.

The observed source timestamp `2026-11-16T18:19:20.527329Z` is
**17 November 2026, 00:19:20 Asia/Thimphu**.

Sanitized successful-run evidence (no passwords or access tokens):

```json
{
  "result": "PASS",
  "run_id": "cea991684682",
  "employer_id": "e5eb95a3-c529-4bbb-9321-ee5f796f845d",
  "applicant_id": "ecf749b9-5a40-4d1a-aa16-1c4d9bb8b70e",
  "verification_request_id": "ef19dfdd-8f9f-40d8-9e44-7dcc7848c694",
  "reviewed_by": "0b13c9be-7e47-444b-8ce7-3489c46060ef",
  "employer_status": "VERIFICATION_STATUS_VERIFIED",
  "alert_id": "5c14500b-0dec-4e87-9fd1-1ad1cfc4031f",
  "job_id": "ff714210-f308-4df9-b543-69065c6cfe55",
  "draft_status": "JOB_STATUS_DRAFT",
  "published_status": "JOB_STATUS_PUBLISHED",
  "events": {
    "job.published": {
      "event_id": "ed4bffae-df2f-4df9-a146-b2896089f4c7",
      "type": "job.published",
      "occurred_at": "2026-11-16T18:19:20.527329Z",
      "producer": "job-posting",
      "data": {
        "@type": "type.googleapis.com/jobtrack.events.v1.JobPublished",
        "job_id": "ff714210-f308-4df9-b543-69065c6cfe55",
        "employer_id": "e5eb95a3-c529-4bbb-9321-ee5f796f845d",
        "title": "JT76 real PublishJob cea991684682",
        "category_id": "10000000-0000-4000-8000-000000000001",
        "tag_ids": [
          "20000000-0000-4000-8000-000000000001"
        ],
        "location": "Thimphu",
        "deadline": "2026-11-18T18:19:20.407156Z"
      }
    },
    "job_alert.matched": {
      "event_id": "a4504a52-d2f3-4cce-b3f4-0c20cf26591c",
      "type": "job_alert.matched",
      "occurred_at": "2026-11-16T18:19:21.288293552Z",
      "producer": "job-alert",
      "data": {
        "@type": "type.googleapis.com/jobtrack.events.v1.JobAlertMatched",
        "job_id": "ff714210-f308-4df9-b543-69065c6cfe55",
        "applicant_id": "ecf749b9-5a40-4d1a-aa16-1c4d9bb8b70e"
      }
    }
  },
  "persisted": {
    "processed": 1,
    "matches": 1,
    "confirmed": 1
  }
}
```


Additional source-outbox confirmation command and result:

```sh
docker compose -f deploy/compose/docker-compose.yml exec -T postgres psql -U postgres -d job_posting <<'SQL'
SELECT event_id,routing_key,published_at IS NOT NULL AS confirmed,
 payload::jsonb->'data'->>'job_id' AS job_id
FROM outbox_events WHERE event_id='ed4bffae-df2f-4df9-a146-b2896089f4c7';
SQL
```

Result: one `job.published` row, `confirmed=t`, job
`ff714210-f308-4df9-b543-69065c6cfe55`. Job Alert's SQL assertions are included
in the runnable harness below. The three target counts were each 1. Other
pre-existing matching subscriptions may produce their own events; no claim is
made that the target event was the only notification for this job.

### Exact harness commands

The existing `services/job-alert/scripts/smoke.go` was reviewed but not modified
or run for this scenario: it directly publishes its input event and therefore
cannot prove PublishJob. The temporary harness below reuses its observation
approach, with real gateway calls replacing all direct event publication.

Save the two temporary files using the following blocks. They use the Python
standard library and Go dependencies already in go.mod; no dependency was added.
The Python harness retains synthetic records, including its subscription and
published job. It creates no messages in RabbitMQ itself.

<details>
<summary>Temporary read-only AMQP observer</summary>

```sh
cat > /private/tmp/jt76-observe.go <<'GO'
package main

import (
	"context"
	"fmt"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "observation failed")
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return err
	}
	for _, key := range []string{"job.published", "job_alert.matched"} {
		if err = ch.QueueBind(q.Name, key, "jobtrack.events", false, nil); err != nil {
			return err
		}
	}
	ds, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	if err != nil {
		return err
	}
	fmt.Println("READY")
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-ds:
			if !ok {
				return fmt.Errorf("consumer closed")
			}
			e, err := events.UnmarshalEnvelope(d.Body)
			if err != nil {
				return err
			}
			job := ""
			switch e.Type {
			case "job.published":
				var j eventsv1.JobPublished
				if err = e.Data.UnmarshalTo(&j); err != nil {
					return err
				}
				job = j.JobId
			case "job_alert.matched":
				var j eventsv1.JobAlertMatched
				if err = e.Data.UnmarshalTo(&j); err != nil {
					return err
				}
				if j.ApplicantId != os.Getenv("JT76_APPLICANT_ID") {
					continue
				}
				job = j.JobId
			}
			if job != os.Getenv("JT76_JOB_ID") {
				continue
			}
			if e.Type != d.RoutingKey {
				return fmt.Errorf("routing mismatch")
			}
			raw, err := events.MarshalEnvelope(e)
			if err != nil {
				return err
			}
			fmt.Println(string(raw))
			seen[e.Type] = true
		}
	}
	return nil
}
GO
```

</details>

<details>
<summary>Temporary public API harness, including persisted-state assertions</summary>

```sh
cat > /private/tmp/jt76-public-e2e.py <<'PYTHON'
import datetime, json, secrets, subprocess, sys, time, urllib.request, urllib.error, uuid
COMPOSE = ["docker", "compose", "-f", "deploy/compose/docker-compose.yml"]
BASE = "http://localhost:8000"
stage = "initialization"
def api(method, path, body=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(BASE + path, data=None if body is None else json.dumps(body).encode(),
                                 headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=15) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise RuntimeError(f"{method} {path}: HTTP {error.code}") from None
def login(email, password):
    return api("POST", "/v1/auth/login", {"email": email, "password": password})
def db(sql):
    raw = subprocess.check_output(COMPOSE + ["exec", "-T", "postgres", "psql", "-U", "postgres",
                                            "-d", "job_alert", "-At", "-c", sql], text=True)
    return json.loads(raw)
try:
    stage = "legitimate admin authentication"
    credentials = subprocess.check_output(COMPOSE + ["exec", "-T", "auth", "printenv",
                                                     "ADMIN_EMAIL", "ADMIN_PASSWORD"], text=True).splitlines()
    if len(credentials) != 2:
        raise RuntimeError("configured admin credentials unavailable")
    admin = login(*credentials)
    if admin["user"]["role"] != "ROLE_ADMIN":
        raise RuntimeError("login did not return an admin")
    admin_token = admin["accessToken"]
    del credentials, admin
    run_id = uuid.uuid4().hex[:12]
    accounts = {}
    stage = "public synthetic account registration"
    for role in ("EMPLOYER", "APPLICANT"):
        password = secrets.token_urlsafe(24)
        email = f"jt76-{run_id}-{role.lower()}@example.test"
        api("POST", "/v1/auth/register", {"email": email, "password": password,
            "fullName": "JT76 synthetic " + role.lower(), "role": "ROLE_" + role})
        accounts[role] = login(email, password)
    employer = accounts["EMPLOYER"]["user"]["userId"]
    applicant = accounts["APPLICANT"]["user"]["userId"]
    employer_token = accounts["EMPLOYER"]["accessToken"]
    applicant_token = accounts["APPLICANT"]["accessToken"]
    stage = "seeded reference data"
    category = "10000000-0000-4000-8000-000000000001"
    tag = "20000000-0000-4000-8000-000000000001"
    categories = api("GET", "/v1/categories?pagination.page_size=100", token=applicant_token)["categories"]
    tags = api("GET", "/v1/tags?pagination.page_size=100", token=applicant_token)["tags"]
    locations = api("GET", "/v1/locations?pagination.page_size=100", token=applicant_token)["locations"]
    assert any(c["categoryId"] == category for c in categories)
    assert any(t["tagId"] == tag for t in tags)
    assert any(loc["name"] == "Thimphu" for loc in locations)
    stage = "public employer profile and verification request"
    api("POST", "/v1/employer/profile", {"companyName": "JT76 synthetic " + run_id,
        "description": "Synthetic end-to-end verification", "website": "https://example.test",
        "location": "Thimphu"}, employer_token)
    verification = api("POST", "/v1/verification/requests", {"registrationNumber": "JT76-" + run_id,
        "notes": "Synthetic JT-76 end-to-end verification"}, employer_token)["request"]
    request_id = verification["requestId"]
    stage = "public admin approval"
    approved = api("POST", f"/v1/verification/requests/{request_id}/approve", {}, admin_token)["request"]
    assert approved["status"] == "REQUEST_STATUS_APPROVED"
    stage = "employer verification event propagation"
    until = time.monotonic() + 30
    while True:
        profile = api("GET", "/v1/employer/profile", token=employer_token)["profile"]
        if profile["verificationStatus"] == "VERIFICATION_STATUS_VERIFIED":
            break
        if time.monotonic() > until:
            raise RuntimeError("approved employer did not become VERIFIED within 30s")
        time.sleep(0.25)
    stage = "public applicant subscription"
    alert = api("POST", "/v1/job-alerts", {"categoryIds": [category], "tagIds": [tag],
        "location": "Thimphu"}, applicant_token)["jobAlert"]
    alert_id = alert["alertId"]
    stage = "public CreateJob"
    deadline = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=2)).isoformat().replace("+00:00", "Z")
    job = api("POST", "/v1/jobs", {"title": "JT76 real PublishJob " + run_id,
        "description": "Synthetic job for JT-76 public API end-to-end verification",
        "categoryId": category, "tagIds": [tag], "location": "Thimphu", "deadline": deadline}, employer_token)["job"]
    job_id = job["jobId"]
    assert job["status"] == "JOB_STATUS_DRAFT"
    stage = "start read-only AMQP observer"
    observer = subprocess.Popen(COMPOSE + ["exec", "-T", "-e", "JT76_JOB_ID=" + job_id,
                                          "-e", "JT76_APPLICANT_ID=" + applicant, "job-alert", "/tmp/jt76-observe"],
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if observer.stdout.readline().strip() != "READY":
        raise RuntimeError("observer did not become ready")
    stage = "real PublishJob through gateway"
    published = api("POST", f"/v1/jobs/{job_id}/publish", {}, employer_token)["job"]
    assert published["status"] == "JOB_STATUS_PUBLISHED"
    assert published["employerId"] == employer
    stage = "observe job.published and job_alert.matched"
    raw, errors = observer.communicate(timeout=65)
    if observer.returncode != 0:
        raise RuntimeError("observer failed to receive both events")
    observed = {e["type"]: e for e in map(json.loads, raw.splitlines())}
    source, matched = observed["job.published"], observed["job_alert.matched"]
    assert source["producer"] == "job-posting"
    assert source["data"]["job_id"] == job_id and source["data"]["employer_id"] == employer
    assert source["data"]["category_id"] == category and tag in source["data"]["tag_ids"]
    assert source["data"]["location"] == "Thimphu"
    assert matched["producer"] == "job-alert"
    assert matched["data"]["job_id"] == job_id and matched["data"]["applicant_id"] == applicant
    stage = "persisted claim, match and confirmed outbox"
    sql = f"""SELECT json_build_object(
      'processed', (SELECT count(*) FROM processed_events WHERE event_id='{source["event_id"]}'),
      'matches', (SELECT count(*) FROM matched_subscriptions WHERE source_event_id='{source["event_id"]}'
                   AND alert_id='{alert_id}' AND job_id='{job_id}' AND applicant_id='{applicant}'),
      'confirmed', (SELECT count(*) FROM outbox_events WHERE event_id='{matched["event_id"]}'
                     AND published_at IS NOT NULL AND payload::jsonb->'data'->>'job_id'='{job_id}'
                     AND payload::jsonb->'data'->>'applicant_id'='{applicant}'))"""
    until = time.monotonic() + 10
    while True:
        persisted = db(sql)
        if persisted == {"processed": 1, "matches": 1, "confirmed": 1}:
            break
        if time.monotonic() > until:
            raise RuntimeError("persisted evidence did not reach expected counts")
        time.sleep(0.1)
    evidence = {"result": "PASS", "run_id": run_id, "employer_id": employer, "applicant_id": applicant,
                "verification_request_id": request_id, "reviewed_by": approved["reviewedBy"],
                "employer_status": profile["verificationStatus"], "alert_id": alert_id,
                "job_id": job_id, "draft_status": job["status"], "published_status": published["status"],
                "events": observed, "persisted": persisted}
    with open("/private/tmp/jt76-public-e2e-evidence.json", "w") as output:
        json.dump(evidence, output, indent=2)
    print(json.dumps(evidence, indent=2))
except Exception as error:
    print("BLOCKED at " + stage + ": " + str(error), file=sys.stderr)
    sys.exit(1)
PYTHON
```

</details>


Commands used to compile and run (arm64 Docker engine):

```sh
GOCACHE=/private/tmp/jobtrack-go-cache GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /private/tmp/jt76-observe /private/tmp/jt76-observe.go
docker compose -f deploy/compose/docker-compose.yml cp /private/tmp/jt76-observe job-alert:/tmp/jt76-observe
python3 /private/tmp/jt76-public-e2e.py
docker compose -f deploy/compose/docker-compose.yml ps job-alert job-posting employer company-verification gateway --format '{{.Service}} {{.State}} {{.Health}}'
```

Final health: Job Alert, Job Posting, Employer, Company Verification and Gateway
all running and healthy.

### Categoryless review and focused checks

Inspected the corrected handler and its unit/integration tests. An omitted
category is accepted, malformed nonempty categories are permanent errors, job
and employer UUIDs remain mandatory and every supplied tag is validated.
The PostgreSQL integration test goes through the handler and proves tag-only,
location-only and tag-plus-location matching, category-filter nonmatching,
duplicate idempotency and no claim for malformed categories. No remaining defect
was identified in this correction.

Commands and actual results for this session:

```sh
GOCACHE=/private/tmp/jobtrack-go-cache go test ./services/job-alert/... ./tests/contract
# PASS (cached).
GOCACHE=/private/tmp/jobtrack-go-cache go test -tags=integration ./services/job-alert/... ./tests/contract
# PASS (cached).
GOCACHE=/private/tmp/jobtrack-go-cache go test -count=1 -tags=integration ./services/job-alert/... ./tests/contract
# PASS, fresh execution of all focused unit/integration/contract tests.
GOCACHE=/private/tmp/jobtrack-go-cache go vet ./services/job-alert/... ./tests/contract
# PASS.
git diff --check
# PASS.
```

Limitations: the real PublishJob scenario used a categorized job with all three
subscription filters. Categoryless behavior was freshly verified by automated
tests, not a second Compose publication. Notification dispatch/email, full
repository CI, load tests and Kubernetes were not exercised. The first attempt's
published job predates the active Job Posting outbox migration and was not
backfilled or republished. All synthetic records and existing data remain.

Remaining action: review this evidence and the existing JT-76 diff. No remaining
end-to-end blocker was observed after refreshing the stale container. No commits,
pushes, PRs, Jira transitions or time logs were performed.
