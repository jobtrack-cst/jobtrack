# JT-88 Team B frontend pages test cases

Traceability: [JT-88](https://jobtrack-cst.atlassian.net/browse/JT-88). Owner: Jigden Shakya (backup Sangay Tenzin). Verification date: 16 November 2026. All data is synthetic.

## Scope

Frontend pages under `frontend/src/features/employer`, `company-verification`, `moderation`, `analytics` and `audit-log`, using only the public RPCs of those services through the gateway and the shared client in `frontend/src/api/`. No contract, service, dependency or tool changes. Rules agreed for JT-88 are in AGENTS.md Section 15.

| Page | Route | Role | Public RPCs |
|---|---|---|---|
| Company profile | `/employer/company` | Employer | `GetMyCompanyProfile`, `CreateCompanyProfile`, `UpdateCompanyProfile` |
| Verification request | `/employer/verification` | Employer | `ListMyVerificationRequests`, `SubmitVerificationRequest` |
| Report a job | `/<role>/report-job`, `/<role>/report-job/:jobId` | All three | `ReportJob` |
| Verification queue | `/admin/verification` | Admin | `ListVerificationRequests`, `ApproveVerificationRequest`, `RejectVerificationRequest`, `GetCompanyProfile` |
| Job reports | `/admin/moderation` | Admin | `ListReports`, `DismissReport`, `RemoveJob` |
| Analytics | `/admin/analytics` | Admin | `GetSummary` |
| Audit log | `/admin/audit-log` | Admin | `ListAuditEntries` |

The JT-78 application cards link to Report job with the job ID filled in. Admins now have a workspace nav, and unknown `/admin/*` paths redirect to `/admin`.

## Test matrix

Cypress tests are in `tests/e2e/team-b-admin-pages.cy.js` and stub the gateway; every request is checked for the JWT from the shared client.

| Case ID | Requirement | Test case | Technique | Expected result | Level | Result |
|---|---|---|---|---|---|---|
| JT88-01 | AC1 | No company profile (404), fill all four fields with spaces around them, create | State transition | `POST /v1/employer/profile` with trimmed fields; "Company profile created"; chip Not verified; Request verification link; button becomes Save | Cypress | Pass |
| JT88-02 | AC1 | Rejected profile, change location, save | State transition | Rejected chip and reason shown; `PUT` with all four fields; "Company profile saved." | Cypress | Pass |
| JT88-03 | AC1 | Verified profile | State transition | Verified chip; no Request verification link | Cypress | Pass |
| JT88-04 | AC1 | 151 character name; name of only spaces; 2001 character description; website without scheme; `ftp://` website; 256 character website; 101 character location | Boundary value | Warning naming the rule (none for the spaces-only name); Save disabled; nothing sent | Cypress | Pass |
| JT88-05 | AC1 | 150 character name, 2000 character description, 255 character website, 100 character location | Boundary value | No warning; Save enabled | Cypress | Pass |
| JT88-06 | AC3 | Save answered 400 (code 3); load answered 503, then retry | Error guessing | Fixed message, never the server text; Try again reloads | Cypress | Pass |
| JT88-07 | AC1 | No requests; send a padded registration number and notes | State transition | `POST /v1/verification/requests` with trimmed values; list reloads; Pending card; form hidden; employer card never shows the admin ID | Cypress | Pass |
| JT88-08 | AC1 | Latest request rejected; latest request approved | State transition | Rejected: reason shown and "Send a new request" form. Approved: "Your company is verified", no form | Cypress | Pass |
| JT88-09 | AC1 | Registration number 51 and 50 characters; notes 1001 and 1000 characters | Boundary value | 51 and 1001 show a warning and disable Send; 50 and 1000 allowed | Cypress | Pass |
| JT88-10 | AC3 | Send answered 409 (6), 400 (9), 400 (3), 403 (7) | Error guessing | Pending request exists, already verified, rules, employers only; never the server text | Cypress | Pass |
| JT88-11 | AC1 | Admin queue; View company; Approve, cancel, approve again and confirm | State transition | List uses `status=REQUEST_STATUS_PENDING` and `pagination.page_size=20`; company name and location shown; nothing sent on cancel; `POST .../approve` with `{}`; list reloads | Cypress | Pass |
| JT88-12 | AC1 | Reject with reasons of 501, 500 and padded characters | Boundary value | 501 blocked with a warning; 500 allowed; `POST .../reject` with the trimmed reason | Cypress | Pass |
| JT88-13 | AC1 | Filter All; two pages | Equivalence partitioning | No `status` parameter; Next page uses token `20`; decided request shows "by admin" and no actions | Cypress | Pass |
| JT88-14 | AC3 | Approve answered 400 (9), 404 (5), 403 (7); View company answered 404 | Error guessing | Already decided, not found, admins only, no company profile yet; never the server text | Cypress | Pass |
| JT88-15 | AC1, AC4 | Verification path: employer Not verified, sends a request, admin approves, employer sees Verified | State transition | Each step as described; Request verification link gone; "Your company is verified" | Cypress | Pass |
| JT88-16 | AC1 | Applicant opens Report job from an application card | Equivalence partitioning | `/applicant/report-job/<jobId>` with the job ID filled in; `POST /v1/jobs/<jobId>/reports` with the trimmed reason; reason cleared | Cypress | Pass |
| JT88-17 | AC1 | Employer reports by ID: `not-a-uuid`, then the ID in capitals; reasons of 1001 and 1000 characters | Boundary value | Bad ID and 1001 blocked; ID sent in lowercase; 1000 sent | Cypress | Pass |
| JT88-18 | AC1, AC2 | Admin opens `/admin/report-job/not-a-uuid` | Error guessing | Warning about the address; Job ID field empty; admins can report | Cypress | Pass |
| JT88-19 | AC3 | Report answered 409 (6), 400 (9), 400 (3) | Error guessing | Open report exists, job already removed, rules; never the server text | Cypress | Pass |
| JT88-20 | AC1 | Admin reports list; Dismiss, cancel, dismiss again and confirm | State transition | List uses `status=REPORT_STATUS_OPEN`; nothing sent on cancel; `POST .../dismiss` with `{}`; list reloads | Cypress | Pass |
| JT88-21 | AC1 | Remove a reported job with reasons of 501, 500 and padded characters | Boundary value | Job ID filled in and locked; 501 blocked; `POST /v1/moderation/jobs/<jobId>/remove` with the trimmed reason | Cypress | Pass |
| JT88-22 | AC1 | Remove a job by ID when there are no reports | Equivalence partitioning | Removal sent with the typed ID and reason | Cypress | Pass |
| JT88-23 | AC1 | Filter Actioned | Equivalence partitioning | `status=REPORT_STATUS_ACTIONED`; Actioned chip; no actions | Cypress | Pass |
| JT88-24 | AC3 | Remove answered 400 (9), 400 (3), 403 (7); dismiss answered 400 (9) | Error guessing | Already removed, rules, admins only, no longer open; never the server text | Cypress | Pass |
| JT88-25 | AC1 | Analytics From 2026-11-01 To 2026-11-07 | Equivalence partitioning | `start_time` is local midnight of 1 Nov and `end_time` local midnight of 8 Nov, both in UTC; counts shown, a missing count as 0; status changes with readable names | Cypress | Pass |
| JT88-26 | AC1 | To one day before From; To equal to From | Boundary value | Before is blocked with a warning; equal is allowed (one day) | Cypress | Pass |
| JT88-27 | AC1, AC3 | Summary with no status changes; answered 400 (3) and 403 (7) | Error guessing | "No status changes in this period."; fixed messages | Cypress | Pass |
| JT88-28 | AC1 | Audit log, two pages | Equivalence partitioning | Data JSON shown exactly as received; total count; Next page | Cypress | Pass |
| JT88-29 | AC1 | Event type and local From and To filters, then Clear | Equivalence partitioning | `event_type`, `start_time` and `end_time` in UTC, page token reset; Clear sends no filters | Cypress | Pass |
| JT88-30 | AC1 | Event type `Application.Submitted`; From equal to To | Boundary value | Lowercase warning; From before To warning; Apply disabled | Cypress | Pass |
| JT88-31 | AC3 | Filters answered 400 (3) | Error guessing | Fixed message, never the server text | Cypress | Pass |
| JT88-32 | AC2 | Nav links and direct routes for applicant, employer and admin | Role-based access | Each role sees only its links; other roles' pages redirect to the user's own workspace; unknown `/admin/*` goes to `/admin` | Cypress | Pass |
| JT88-33 | AC1 to AC3 | Real flow in Chrome against Docker Compose (see below) | End to end | Every JT-88 step as expected; two event effects wait for JT-87 and JT-72 | Manual | Pass |
| JT88-34 | AC4 | JT-63, JT-78 and JT-83 Cypress suites after the nav, admin route and card changes | Regression | 19, 34 and 47 passing (see notes) | Cypress | Pass |

AC numbers follow the order of the acceptance criteria in JT-88: AC1 pages and flows, AC2 gateway only through the shared client, MUI and role-limited routes, AC3 gRPC errors as clear messages, AC4 test steps in `docs/testing` and Cypress for the verification path. AC5 and AC6 are the shared Sprint 3 criteria (Section 9 frontend with no new libraries; PR, CI, merge, time logged).

## Manual check against Docker Compose (JT88-33)

Backend started with `docker compose up -d --build auth employer company-verification moderation analytics audit-log job-posting gateway` in `deploy/compose`; frontend with `npm run dev`. The admin is the one seeded by auth from `.env`. A local helper (not committed) logged in as the employer through the gateway, created and published one job and read its status, without printing passwords or tokens.

| Step | Role | Action | Expected and observed |
|---|---|---|---|
| 1 | Employer | Register `jt88.employer@example.test` and log in | `/employer` with Company and Verification links |
| 2 | Employer | Company: website `druktech.bt` | "Website must be a full address"; Create disabled |
| 3 | Employer | Create with name, `https://druktech.example` and Thimphu | Created; Not verified |
| 4 | Employer | Verification: send `REG-JT88-1` | Request sent; Pending; form hidden |
| 5 | Admin | Verification; View company | Request listed; company name shown, Not verified |
| 6 | Admin | Reject with "Licence number not found" | Request rejected; list empty |
| 7 | Employer | Company | Rejected with the reason (`employer.verification_rejected` consumed by employer) |
| 8 | Employer | Verification: send `REG-JT88-2` | Rejected card with reason; new request Pending |
| 9 | Admin | Approve; filter All | Approved and rejected requests, each with the deciding admin |
| 10 | Employer | Company | Verified; no Request verification button (`employer.verified` consumed by employer) |
| 11 | Terminal | Helper creates and publishes a job | `Job ID: cfe7f8aa-a419-4349-a2cc-955c562167de`; publishing succeeded, which needs a verified employer (JT-57) |
| 12 | Employer | Report the job; report it again | Report sent; then "already have an open report" |
| 13 | Admin | Moderation (Open) | Report listed |
| 14 | Admin | Remove job with "Fee scam"; filter Actioned | Job removed; report Actioned with no actions |
| 14a | Admin | Remove the same job again | "This job has already been removed." (`FAILED_PRECONDITION`) |
| 15 | Terminal | Helper reads the job status | `JOB_STATUS_PUBLISHED`. Depends on JT-87: moderation does not publish `moderation.job_removed` yet. Evidence: no publish line in the moderation log; `job-posting.moderation` has 1 consumer and 0 messages; `job-posting.moderation.dead` has 0 messages |
| 16 | Employer | Report the removed job | "This job has already been removed, so it cannot be reported." |
| 17 | Admin | Analytics for today | Jobs published 1, Jobs closed 0 (the removal is not counted until JT-87) |
| 18 | Admin | Audit log with each event type filter, then Clear filters | "No entries on this page." Depends on JT-72 (audit-log consumer, To Do); the page loads and the service answers |
| 19 | Admin | Event type `Job.Closed` | Lowercase warning; Apply filters disabled |
| 20 | Employer | Type `http://localhost:5173/admin/analytics` in the address bar | Login page. Expected: typing an address reloads the app, and JT-63 keeps the session in memory only, so the reload logs out. Redirecting a logged-in user to their own workspace is covered by JT88-32 |

Screenshot of step 10 attached to JT-88 in Jira.

## Verification

Commands run on branch `JT-88-team-b-frontend-pages`, in `frontend`:

    npm ci
    npm run lint && npm run typecheck && npm run build
    npm run dev   (in a second terminal)
    npx cypress run --config-file cypress.config.cjs --spec ../tests/e2e/team-b-admin-pages.cy.js
    npx cypress run --config-file cypress.config.cjs --spec "../tests/e2e/auth.cy.js,../tests/e2e/application-workflows.cy.js,../tests/e2e/applicant-hiring-pages.cy.js"

| Check | Result |
|---|---|
| Lint, typecheck, production build | Pass (the existing over 500 kB chunk warning remains) |
| Cypress `team-b-admin-pages.cy.js` | 55 passing |
| Cypress `auth.cy.js`, `application-workflows.cy.js`, `applicant-hiring-pages.cy.js` | 19, 34 and 47 passing (see notes) |
| Docker Compose browser check | Pass for the JT-88 pages (JT88-33); steps 15, 17 and 18 wait for JT-87 and JT-72 |
| CI on the pull request | Pass: [run PASTE_RUN_NUMBER](https://github.com/jobtrack-cst/jobtrack/actions/runs/PASTE_RUN_NUMBER) on `e0ffca7` |

Notes:

- In the first regression run, the JT-78 test "loads validated resumes on a later page" timed out after 4 s waiting for a select option. JT-88 does not change `ApplyPage` or its resume select. Running `application-workflows.cy.js` again on the same branch passed 34 of 34, so it is recorded as a timing failure on this machine (WSL with Windows Node).
- Long text (150, 151, 500, 501, 1000, 1001, 2000 and 2001 characters) is set with the native value setter and one input event, as in the JT-78 and JT-83 tests.
- Cypress stubs the gateway, so it does not prove live connectivity, CORS or events; JT88-33 covers those.
- Report job needs the job ID. Until the job-search pages exist, applicants and employers reach it from the application cards or by typing the ID.

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Pages under the five feature folders: company profile create and edit, verification request, any user reports a job, admin approves or rejects requests, lists and dismisses reports and removes jobs, views the analytics summary for a date range and the audit log | Met | JT88-01 to JT88-03, JT88-07, JT88-08, JT88-11, JT88-12, JT88-16, JT88-18, JT88-20 to JT88-23, JT88-25, JT88-28, JT88-29, JT88-33 |
| Calls only the gateway through the shared client; MUI components; routes limited by role | Met | Bearer check on every Cypress request; JT88-32; no new dependencies |
| gRPC errors shown as clear messages | Met | JT88-06, JT88-10, JT88-14, JT88-19, JT88-24, JT88-27, JT88-31, step 14a |
| Test steps in `docs/testing`; Cypress for the verification path | Met | This document; JT88-07 to JT88-15 |
| Follows AGENTS.md Section 9 (Frontend); no new libraries | Met | `package.json` unchanged |
| PR with the Jira key, CI green, merged, time logged | Met on merge | PR #64, CI green, approved by Phuntsho; time logged in Jira |
