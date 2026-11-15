# JT-86 weekly-summary test cases

Traceability: [JT-86](https://jobtrack-cst.atlassian.net/browse/JT-86). Owner: Jigden Shakya. Verification date: 15 November 2026.

## Scope

The `weekly-summary` function: one `POST /` reads the last 7 days of statistics from `AnalyticsInternalService.GetInternalSummary`, pages through all admins from `AuthInternalService.ListUsersByRole`, and sends one plain-text email per admin through email-sender. The task also wires the JT-44 internal interceptor into auth, so that only `weekly-summary` can call `ListUsersByRole`. No `.proto` contract changes.

Rules (AGENTS.md Section 15, 15 Nov 2026, JT-86):

- Period: the rolling 7 days before the run, ending now in UTC, truncated to the microsecond.
- Admins: `ROLE_ADMIN`, page size 100, until `next_page_token` is empty. A repeated token or more than 100 pages fails the run. Non-admin users are skipped; duplicate emails (ignoring case) are sent once.
- All reads before any email; a failed read sends nothing. A failed email does not stop the others, but the run returns 503.
- Trigger: 204 when every email was accepted or there are no admins, 503 on any failure, 409 while a run is in progress, 405 for other methods, 413 above 64 KiB. Each run is limited to 4 minutes; each email-sender call to 30 s.
- Logs: counts, a fixed reason and the CloudEvent `Ce-Id` as `request_id`; never email addresses, the token or upstream error text.

Out of scope: retry policy on the PingSource, NetworkPolicies, Mailpit in k3d.

## Test matrix

AC1: triggered weekly by a PingSource in k3d, and by an HTTP call in Docker Compose. AC2: last 7 days of statistics and all admins, paging. AC3: `InternalUnaryClientInterceptor` with `INTERNAL_TOKEN`; analytics and auth allow the caller. AC4: one email per admin; a failure returns non-2xx. AC5: unit tests with mocked ports; summary appears in Mailpit.

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT86-01 | AC2 | Period for a run at 08:00 Bhutan time with nanoseconds | Boundary values | End = now in UTC truncated to the microsecond; start = end minus exactly 7 days | Unit, `internal/domain` (`TestPeriod_IsTheSevenDaysBeforeNowInUTC`) | Pass |
| JT86-02 | AC2 | Statistics request and mapping | Equivalence partitioning | Request carries start and end; all 7 counts and every status change mapped | Unit, `adapters/clients` (`TestAnalytics_Summary_MapsEveryCount`) | Pass |
| JT86-03 | AC2 | Email reports the requested period even if the response echoes other times | Equivalence partitioning | Subject and body use the requested period | Unit, `internal/app` (`TestRunner_Run_ReportsTheRequestedPeriod`) | Pass |
| JT86-04 | AC2 | Admins across 2 pages | State transition | Second call uses the returned token; stops on an empty token; 3 admins | Unit, `internal/app` (`TestRunner_Run_SendsOneEmailPerAdminAcrossPages`) | Pass |
| JT86-05 | AC2 | Admin request and response | Equivalence partitioning | `ROLE_ADMIN`, page size 100, token passed through; applicant in the response skipped; next token returned | Unit, `adapters/clients` (`TestAuth_ListAdmins_*`) | Pass |
| JT86-06 | AC2 | Duplicate (different case) and blank emails | Equivalence partitioning | One email to the first spelling; blanks skipped | Unit, `internal/app` (`TestRunner_Run_RemovesDuplicateAndBlankEmails`) | Pass |
| JT86-07 | AC2 | Repeated page token; 100 pages and a 101st token | Boundary values | `ErrAdmins`, no email sent | Unit, `internal/app` (`TestRunner_Run_ReadFailuresSendNothing`) | Pass |
| JT86-08 | AC2, AC4 | Analytics fails; auth fails on page 1 and on page 2; empty analytics response | Equivalence partitioning | Run fails, no email sent | Unit, `internal/app`, `adapters/clients` | Pass |
| JT86-09 | AC2 | No admins | Boundary values | 204, nothing sent, warning logged | Unit, `internal/app`, `adapters/httpapi` | Pass |
| JT86-10 | AC3 | Auth `ListUsersByRole` through a real gRPC server with the real interceptor | Authorization | `weekly-summary` with the right token OK; wrong token `UNAUTHENTICATED`; `notification-dispatcher` `PERMISSION_DENIED`; no token `UNAUTHENTICATED` | Unit, `services/auth/cmd/server` (`TestServe_InternalInterceptorGuardsAuthInternalService`) | Pass |
| JT86-11 | AC3 | Auth `GetUser` and public `Login` with the interceptor | Authorization | `GetUser` allowed for `notification-dispatcher`, denied for `weekly-summary`; `Login` reaches the handler without a token | Unit, `services/auth/cmd/server` | Pass |
| JT86-12 | AC3 | Auth `INTERNAL_TOKEN` missing, with a space, non-ASCII | Equivalence partitioning | Startup refused; message names `INTERNAL_TOKEN` without the value | Unit, `services/auth/cmd/server` (`TestLoadConfig`, `TestInternalServerOptions_RejectsInvalidTokens`, `TestRun_InvalidInternalTokenStopsBeforeStartup`) | Pass |
| JT86-13 | AC3 | Weekly-summary configuration: each variable missing or invalid; port 0, 1, 65535, 65536 | Boundary values | Valid values accepted; others refused without the value in the message | Unit, `cmd/server` (`TestLoadConfig`, `TestDialInternal_RejectsAnInvalidTokenWithoutLeakingIt`) | Pass |
| JT86-14 | AC4 | Email request to email-sender | Equivalence partitioning | `POST`, `application/json`, exactly `recipient`, `subject`, `body` | Unit, `adapters/emailsender` (`TestClient_Send_PostsTheContractJSON`) | Pass |
| JT86-15 | AC4 | email-sender answers 200, 302, 400, 413, 503; unreachable; cancelled | Equivalence partitioning | Only 204 is success; all others are errors; no redirect followed | Unit, `adapters/emailsender` | Pass |
| JT86-16 | AC4 | `EMAIL_SENDER_URL` relative, no host, other scheme, invalid host | Equivalence partitioning | Refused at startup | Unit, `adapters/emailsender` (`TestNew_ValidatesTheEndpoint`) | Pass |
| JT86-17 | AC4 | Second of 3 emails fails | Equivalence partitioning | Third still sent; result 2 sent, 1 failed; run fails | Unit, `internal/app` (`TestRunner_Run_SendFailureTriesTheRestAndFails`) | Pass |
| JT86-18 | AC4 | Trigger result to HTTP status; logs hide upstream details | Equivalence partitioning | 204 on success; 503 with a fixed reason for statistics, admins, send and unknown errors | Unit, `adapters/httpapi` (`TestHandler_ResultToStatus`) | Pass |
| JT86-19 | AC1 | GET and PUT; body of 64 KiB and 64 KiB + 1 | Boundary values | 405 with `Allow: POST`; at the limit 204; above it 413 without running | Unit, `adapters/httpapi` | Pass |
| JT86-20 | AC1, AC4 | Second trigger while a run is in progress | State transition | 409; the first run completes with 204 | Unit, `adapters/httpapi` (`TestHandler_SecondRunWhileRunningIsRejected`) | Pass |
| JT86-21 | AC1 | CloudEvent `Ce-Id` header; run deadline | Equivalence partitioning | Logged as `request_id`; run context has a 4 minute deadline | Unit, `adapters/httpapi` | Pass |
| JT86-22 | AC1 | Routes; server timeouts; start, stop and port in use | Equivalence partitioning | `POST /` triggers, `/healthz` and `/metrics` answer, other paths 404; write timeout above the run limit; clean stop; second start fails | Unit, `cmd/server` | Pass |
| JT86-23 | AC4 | Email content: subject across months and years; body with every count; no status changes; control characters, invalid UTF-8 and line breaks in a status name | Equivalence partitioning | Exact subject and body; `none` when empty; unsafe characters replaced with `?` | Unit, `internal/domain` | Pass |
| JT86-24 | AC1, AC5 | Docker Compose: trigger with `wget --post-data=''` | End to end | `HTTP/1.1 204 No Content`; log `"admins":1,"sent":1,"failed":0`; email "JobTrack weekly summary (8 Nov 2026 to 15 Nov 2026)" to `admin@jobtrack.test` in Mailpit with real counts (2 jobs published, 1 `UNDER_REVIEW`) | Docker Compose (manual, screenshot in PR #56) | Pass |
| JT86-25 | AC4 | Docker Compose with analytics stopped | End to end | `HTTP/1.1 503 Service Unavailable`; log `"reason":"statistics unavailable"`; no email | Docker Compose (manual) | Pass |
| JT86-26 | AC1 | PingSource in k3d triggers the Knative Service | End to end | Weekly run reaches `POST /` and emails admins | k3d | Not run: analytics, auth, email-sender and Mailpit are not deployed in the cluster yet (AGENTS.md Section 16) |

## Verification

Commands run on commit `703db76`:

    go test -cover ./functions/weekly-summary/...
    go test ./services/auth/...
    go test -tags=integration ./services/auth/...
    go vet ./...
    go test ./...

| Check | Result |
|---|---|
| Unit tests | Pass |
| Integration tests (auth) | Pass |
| Coverage | app 100%, domain 100%, adapters/clients 100%, adapters/httpapi 93.9%, adapters/emailsender 90.0%, cmd/server 78.7% |
| CI on the pull request | See PR #56 |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| AC1: Triggered weekly by a PingSource in k3d; in Docker Compose by an HTTP call | Partly met: Compose verified; PingSource manifest ready, k3d not run | JT86-19 to JT86-22, JT86-24, JT86-26 |
| AC2: Last 7 days through `GetInternalSummary`, admin emails through `ListUsersByRole`, paging through all admins | Met | JT86-01 to JT86-09 |
| AC3: `InternalUnaryClientInterceptor` with `INTERNAL_TOKEN`; analytics and auth allow the caller | Met | JT86-10 to JT86-13, JT86-24 |
| AC4: One summary email per admin through email-sender; a failure returns non-2xx | Met | JT86-14 to JT86-18, JT86-23, JT86-25 |
| AC5: Unit tests with mocked ports; summary in Mailpit in Docker Compose | Met | JT86-04, JT86-24 |
