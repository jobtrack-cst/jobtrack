# JT-82 Deadline-reminder test cases

Traceability: [JT-82](https://jobtrack-cst.atlassian.net/browse/JT-82). Owner: Kinley Tobgay Lhendrup. Verification date: 15 November 2026.

## Scope

The `deadline-reminder` Knative function finds jobs closing within 48 hours through `JobPostingInternalService.ListJobsClosingBetween`, finds who saved them through `SavedJobsInternalService.ListJobSavers`, and publishes one `deadline.reminder` per saver and job through `pkg/events`. It has no database. No contract or `go.mod` changes; a new Compose block in the `functions` profile and a Knative manifest.

Rules agreed for JT-82 (AGENTS.md Section 15, approved by Phuntsho):

- Window `[now, now + 48 h)` in UTC, truncated to the microsecond. Both lists use page size 100 and follow `next_page_token`; a repeated token or more than 100 pages fails that read.
- All jobs are read before anything is published; a failed job read publishes nothing. A failed saver read or publish does not stop the other jobs, but the run returns 503.
- A job or saver listed twice is reminded once; a saver ID that is not a UUID is skipped.
- `event_id` is a UUIDv5 of `job_id/applicant_id/deadline`, so a repeated run in the same window publishes the same `event_id` and consumers ignore it. A changed deadline gives a new `event_id`.
- Internal calls use `grpcclient.InternalUnaryClientInterceptor` with `INTERNAL_TOKEN` and caller `deadline-reminder`.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT82-01 | AC2 | Window for a run time with nanoseconds and a non-UTC zone | Boundary value | Start is the run time in UTC truncated to the microsecond; end is start plus 48 h | Unit, `internal/domain` | Pass |
| JT82-02 | AC2 | Jobs on two pages (one job listed on both); savers on two pages (one saver listed twice); a second job | Equivalence partitioning | Both lists paged; 4 reminders, each job and saver once | Unit, `internal/app` | Pass |
| JT82-03 | AC2 | No closing jobs | Equivalence partitioning | Nothing published; 204 | Unit, `internal/app` | Pass |
| JT82-04 | AC2 | Job read fails on the first or a later page; repeated page token; more than 100 pages | Fault injection, boundary value | `ErrJobs`; nothing published | Unit, `internal/app` | Pass |
| JT82-05 | AC2 | Saver read fails for one job; publish fails for one saver; saver ID not a UUID; uppercase saver ID | Fault injection, equivalence partitioning | Other jobs and savers still published; failed 2, skipped 1; uppercase published lowercase; run returns 503 | Unit, `internal/app`, `adapters/httpapi` | Pass |
| JT82-06 | AC2 | Clients map requests and responses | Equivalence partitioning | Window, page size 100 and token sent; missing or invalid deadline becomes zero time; errors passed up | Unit, `adapters/clients` | Pass |
| JT82-07 | AC3 | Internal client with an invalid token | Equivalence partitioning | Dial fails; the token is not in the error | Unit, `cmd/server` | Pass |
| JT82-08 | AC3 | Configuration: each variable missing or invalid, wrong `SERVICE_NAME`, broker URL without `amqp` or a host | Equivalence partitioning | Startup fails; no value in the error | Unit, `cmd/server` | Pass |
| JT82-09 | AC4 | Envelope for a reminder | Contract | Type `deadline.reminder`, producer `deadline-reminder`, `occurred_at` the run time, data matches; equals `tests/contract/fixtures/deadline.reminder.json` | Unit, `internal/app`; contract, `tests/contract` | Pass |
| JT82-10 | AC4 | `event_id` for the same job, applicant and deadline; for another applicant, job or deadline | Equivalence partitioning | Same input gives the same ID; any change gives a different ID | Unit, `internal/domain` | Pass |
| JT82-11 | AC4 | Today's run and tomorrow's run for the same reminder | State transition | Same `event_id` both times | Unit, `internal/app` | Pass |
| JT82-12 | AC4 | Publisher per run; broker unavailable | Fault injection | Opened and closed for each run; broker failure returns `ErrPublish` without the URL | Unit, `cmd/server` | Pass |
| JT82-13 | AC1 | `POST /` result to status; wrong method, body over 64 KiB, body at 64 KiB; second run while running; run deadline; `Ce-Id` logged | Equivalence partitioning, boundary value, concurrency | 204 or 503; 405, 413, accepted; 409; 4 minute limit; `request_id` in the log | Unit, `adapters/httpapi` | Pass |
| JT82-14 | AC1 | Routes, server timeouts, shutdown and bind failure | Equivalence partitioning | `/`, `/healthz`, `/metrics`; write timeout covers a full run; clean stop | Unit, `cmd/server` | Pass |
| JT82-15 | AC1, AC2, AC4, AC5 | Real flow: two jobs (closing in 30 h and in 5 days), two applicants each saving both, function triggered twice in Compose | End to end | Both runs 204; 4 messages for the 30 h job (2 savers x 2 runs) with 2 distinct `event_id` values; none for the 5 day job; producer, type and deadline match | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-82: AC1 trigger (daily PingSource in k3d, HTTP in Compose), AC2 closing jobs and savers with paging, AC3 internal interceptor and allowlists, AC4 one event per saver and job with no duplicates and a fixture, AC5 unit tests with mocked ports and Docker Compose.

## Verification

Commands run on branch `JT-82-deadline-reminder`:

    go test -count=1 -race -cover ./functions/deadline-reminder/... ./tests/contract/...
    docker compose -f deploy/compose/docker-compose.yml --profile functions up -d deadline-reminder
    python3 /tmp/jt82_e2e.py   (local script, not committed; synthetic accounts, no credentials printed)

| Check | Result |
|---|---|
| Unit and contract tests | Pass |
| Package coverage (unit) | `domain` 100%, `app` 97.5%, `adapters/clients` 100%, `adapters/httpapi` 93.9%, `cmd/server` 80.0% |
| Docker Compose end to end | Pass: job `132a2793-1e91-4c85-a334-e8936b9bfe82`, `event_id` values `23ed05c0-b8be-5cb6-9445-98bd3e220643` and `62490f4c-cae1-5732-8d3a-db2398822725` published by both runs; log "deadline reminders published" with jobs 1, reminders 2, published 2, failed 0, skipped 0 for each run |
| CI on the pull request | CI_RUN_PLACEHOLDER |

Known gaps (AGENTS.md Section 16):

- Saved-jobs `ListJobSavers` ignores `page_token` and returns no `next_page_token`, so paging past 100 savers is tested only with mocks (JT82-02). Saved-jobs does not wire the internal server interceptor, so the token is sent but not yet checked by saved-jobs. Both fixes are requested from Sangay.
- The daily PingSource run in k3d is not verified; only the manifest is provided.

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Daily PingSource in k3d and an HTTP trigger in Compose | Partly met: Compose verified, k3d manifest not run | JT82-13 to JT82-15; `deploy/knative/deadline-reminder.yaml` |
| Finds jobs closing within 48 hours and their savers, paging through both | Met (saver paging depends on the saved-jobs fix) | JT82-01 to JT82-06, JT82-15 |
| Internal interceptor with `INTERNAL_TOKEN`; job-posting and saved-jobs allow the caller | Met on the caller side; saved-jobs enforcement pending | JT82-07; AGENTS.md Section 6 |
| One `deadline.reminder` per saver and job; no duplicates on a repeated run; rule recorded; fixture added | Met | JT82-09 to JT82-12, JT82-15; AGENTS.md Section 15 |
| Unit tests with mocked ports; verified in Docker Compose | Met | JT82-01 to JT82-15 |
| Follows AGENTS.md Sections 5, 6, 9 and 10; no new tools or contract changes | Met | No `proto/` or `go.mod` changes |
| PR with the Jira key, CI green, merged, time logged | Pending | This PR |
