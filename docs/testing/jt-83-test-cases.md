# JT-83 Applicant and hiring frontend pages test cases

Traceability: [JT-83](https://jobtrack-cst.atlassian.net/browse/JT-83) (parent JT-14). Owner: Kinley Tobgay Lhendrup. Verification date: 15 November 2026. All data is synthetic.

## Scope

Frontend pages under `frontend/src/features/applicant-profile`, `resume`, `application-status`, `review` and `interview`, using only the public RPCs of those services through the gateway and the shared client in `frontend/src/api/`. No contract, service, dependency or tool changes. Rules agreed for JT-83 are in AGENTS.md Section 15 (approved by Phuntsho).

| Page | Route | Role | Public RPCs |
|---|---|---|---|
| Profile | `/applicant/profile` | Applicant | `GetMyProfile`, `CreateMyProfile`, `UpdateMyProfile` |
| Resumes | `/applicant/resumes` | Applicant | `ListMyResumes`, `UploadResume` |
| Status | `/applicant/applications/:id/status`, `/employer/applications/:id/status` | Both | `GetApplicationStatus`, `ListApplicationStatusHistory` |
| Review | `/employer/applications/:id/review` | Employer | `GetReview`, `CompleteReview` |
| Interviews | `/applicant/applications/:id/interviews`, `/employer/applications/:id/interviews` | Both (schedule: employer) | `ListInterviews`, `ScheduleInterview` |

The JT-78 application cards link to Status, Interviews and, for employers, Review.

## Test matrix

Cypress tests are in `tests/e2e/applicant-hiring-pages.cy.js` and stub the gateway; every request is checked for the JWT from the shared client.

| Case ID | Requirement | Test case | Technique | Expected result | Level | Result |
|---|---|---|---|---|---|---|
| JT83-01 | AC1 | No profile (404), fill summary, one education entry, two skills, create | State transition | `POST /v1/applicant/profile` with the exact body; "Profile created"; button becomes Save | Cypress | Pass |
| JT83-02 | AC1 | Existing profile, clear the end year, save | State transition | `PUT` with `endYear` 0 (ongoing); form shows it as empty | Cypress | Pass |
| JT83-03 | AC1 | Remove the only education entry and save | Equivalence partitioning | `PUT` with an empty education list | Cypress | Pass |
| JT83-04 | AC1 | Start year 1949 and next year + 1; end year before start and more than 10 years ahead; empty and 151 character institution; duplicate skill ignoring case | Boundary value | Warning naming the rule; Save disabled; nothing sent | Cypress | Pass |
| JT83-05 | AC1 | 150 character institution, start year next year, end year 10 years ahead | Boundary value | Save enabled | Cypress | Pass |
| JT83-06 | AC3 | Save answered with `INVALID_ARGUMENT`; load answered 503, then retry | Error guessing | Fixed message, never the server text; Try again reloads | Cypress | Pass |
| JT83-07 | AC1, AC4 | Valid PDF uploaded | Equivalence partitioning | Request has file name, `application/pdf` and the file as base64; list reloads from the first page and shows Pending validation | Cypress | Pass |
| JT83-08 | AC3, AC4 | Files of 2,000,000 and 2,000,001 bytes; empty file; not a PDF; PDF type without `%PDF-`; 256 character name | Boundary value | 2,000,000 is sent; the others show a clear message and send nothing | Cypress | Pass |
| JT83-09 | AC3 | Upload answered 400 (code 3), 413, 403 (code 7), 503 | Error guessing | "not accepted" with the rules, "too large", "Only applicants", generic retry message; never the server text | Cypress | Pass |
| JT83-10 | AC1, AC4 | Pending, validated and rejected resumes across two pages; refresh after validation | State transition | Status chips and rejection reason shown; paging uses `pagination.page_size=20`; refresh shows Validated | Cypress | Pass |
| JT83-11 | AC1 | Status page from the application card, two history pages | Equivalence partitioning | Current status and history oldest first with readable labels; Next page | Cypress | Pass |
| JT83-12 | AC3 | Status answered 404 and 403; application ID in the address not a UUID | Error guessing | One clear message each; no request for a bad ID | Cypress | Pass |
| JT83-13 | AC1 | No review yet: score 4, Shortlist, cancel the dialog, then confirm | State transition | Nothing sent on cancel; `POST .../reviews` with `{score: 4, decision: REVIEW_DECISION_SHORTLISTED}`; review shown read-only, form gone | Cypress | Pass |
| JT83-14 | AC1 | Review already exists | State transition | Shown read-only, no form | Cypress | Pass |
| JT83-15 | AC3 | Complete review answered 409, 404, 403, 400 | Error guessing | Already reviewed, not available yet, not your job, score and decision required | Cypress | Pass |
| JT83-16 | AC2 | Applicant card and direct `/applicant/.../review` | Role-based access | No Review link; route redirects to `/applicant` | Cypress | Pass |
| JT83-17 | AC1 | Applicant opens Interviews | Role-based access | List with details, no schedule form | Cypress | Pass |
| JT83-18 | AC1 | Employer schedules in 3 days at 10:00 local with padded details | Equivalence partitioning | `scheduledAt` is the UTC instant, details trimmed; list refreshed; form cleared | Cypress | Pass |
| JT83-19 | AC1 | Time in the past; more than 365 days ahead; details 1000 and 1001 characters | Boundary value | Past, over 365 days and 1001 blocked with a warning; 1000 allowed | Cypress | Pass |
| JT83-20 | AC3 | Schedule answered 409, 400, 403, 404 | Error guessing | Same time already booked, rules, not your job, not available yet; never the server text | Cypress | Pass |
| JT83-21 | AC1 to AC3 | Real flow in Chrome against Docker Compose (see below) | End to end | Every step as expected | Manual | Pass |
| JT83-22 | AC4 | JT-63 and JT-78 Cypress suites after the card links were added | Regression | 19 and 34 passing | Cypress | Pass |

AC numbers follow the order of the acceptance criteria in JT-83: AC1 pages and flows, AC2 gateway only through the shared client, MUI and role-limited routes, AC3 gRPC errors as clear messages, AC4 test steps in `docs/testing` and Cypress for the resume upload path. AC5 and AC6 are the shared Sprint 3 criteria (Section 9 frontend with no new libraries; PR, CI, merge, time logged).

## Manual check against Docker Compose (JT83-21)

Backend started with `docker compose --profile functions up -d` for auth, employer, company-verification, job-posting, resume, resume-validator and its forwarder, application, application-status, applicant-profile, review, interview and gateway; frontend with `npm run dev`. A local setup script (not committed) created a verified employer with a published job and an applicant through the gateway, without printing passwords or tokens.

| Step | Role | Action | Expected and observed |
|---|---|---|---|
| 1 | Applicant | Profile: create with summary, one education entry, two skills; log in again | Created; data still there |
| 2 | Applicant | Resumes: upload a `.txt` file, then a 2,000,001 byte PDF | "Only PDF files" and "larger than 2 MB"; neither listed |
| 3 | Applicant | Upload a valid PDF, refresh | Pending validation, then Validated (resume-validator) |
| 4 | Applicant | Apply for the job with that resume | Submitted |
| 5 | Applicant | Status from the card | Submitted, history None to Submitted |
| 6 | Employer | Review: score 4, Shortlist, confirm | Dialog shown; Review completed |
| 7 | Employer | Status | Shortlisted; history Submitted, Under review, Shortlisted |
| 8 | Employer | Interviews: schedule tomorrow 10:00; try a past time | Listed; past time blocked |
| 9 | Employer | Status | Interview |
| 10 | Applicant | Interviews and Status | Interview with details, no schedule form; Interview with 4 history rows (None to Submitted, Submitted to Under review, Under review to Shortlisted, Shortlisted to Interview) |

Application `78a1d91e-2edf-4297-ae5a-41cbc48b5b58`, job `222eefe7-df19-4832-8ada-9fd83d542706`. Screenshot of step 10 attached to JT-83 in Jira.

## Verification

Commands run on branch `JT-83-frontend-applicant-hiring`, in `frontend`:

    npm ci
    npm run lint && npm run typecheck && npm run build
    npm run dev   (in a second terminal)
    npm test

| Check | Result |
|---|---|
| Lint, typecheck, production build | Pass (the existing over 500 kB chunk warning remains) |
| Cypress `applicant-hiring-pages.cy.js` | 47 passing |
| Cypress `application-workflows.cy.js` and `auth.cy.js` | 34 and 19 passing |
| Docker Compose browser check | Pass (JT83-21) |
| CI on the pull request | CI_RUN_PLACEHOLDER |

Notes:

- Long text (150 and 151 characters, 1000 and 1001 characters) is set with the native value setter and one input event, as in the JT-78 offer terms test. Typing it key by key in Cypress trips React's nested update limit; typing 160 characters with real key events in Chrome works.
- Cypress stubs the gateway, so it does not prove live connectivity or CORS; JT83-21 covers that.
- The applicant cannot download a resume from this page; `DownloadResume` is not in the JT-83 criteria.

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Pages under the five feature folders: profile create and edit, resume upload and list with validation status, status and history, employer review and shortlist, interview scheduling, both sides list interviews | Met | JT83-01 to JT83-05, JT83-07, JT83-10, JT83-11, JT83-13, JT83-14, JT83-17, JT83-18, JT83-21 |
| Calls only the gateway through the shared client; MUI components; routes limited by role | Met | Bearer check on every Cypress request; JT83-16, JT83-17; no new dependencies |
| gRPC errors shown as clear messages (for example over 2 MB or not a PDF) | Met | JT83-06, JT83-08, JT83-09, JT83-12, JT83-15, JT83-20 |
| Test steps in `docs/testing`; Cypress for the resume upload path | Met | This document; JT83-07 to JT83-10 |
| Follows AGENTS.md Section 9 (Frontend); no new libraries | Met | `package.json` unchanged |
| PR with the Jira key, CI green, merged, time logged | Pending | This PR |
