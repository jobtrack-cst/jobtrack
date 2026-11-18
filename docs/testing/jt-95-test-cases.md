# JT-95 Main workflow end-to-end test cases

Traceability: [JT-95](https://jobtrack-cst.atlassian.net/browse/JT-95). Owner: Kinley Tobgay Lhendrup (testing lead). Verification date: 18 November 2026. All data is synthetic.

## Scope

`tests/e2e/stack/main-workflow.cy.js` drives the main workflow through the real React UI against the full Docker Compose stack with the functions profile on. It uses no `cy.intercept` stubs: every request goes to the real gateway, services, RabbitMQ and resume-validator. It runs locally with `npm run test:stack` (`frontend/cypress.stack.config.cjs`); CI keeps running only the stubbed specs, because CI has no stack (AGENTS.md Section 15, JT-95).

Each step logs in through the UI as the right user. Services react to events, so the spec refreshes a page until the change is visible (at most about 30 seconds) and checks the application status between the hiring steps, because the status service accepts the transitions only in order. Accounts are new for each run (`jt95-*-<run>@jobtrack.test`), so the run is repeatable on the same stack.

## Test cases

| Case ID | Workflow step (traces to) | Actions through the UI | Expected result | Result |
|---|---|---|---|---|
| JT95-01 | Employer registers and creates a company profile (JT-63, JT-88) | Register as Employer, log in, Company: enter company name, Create profile | "Account created", employer workspace, "Company profile created" | Pass |
| JT95-02 | Employer requests verification (JT-88, JT-56) | Verification: registration number, Send request | "Request sent" | Pass |
| JT95-03 | Admin approves (JT-88, JT-56) | Log in as admin, Verification: find the request by registration number (paging if needed), Approve, Confirm approval | "Request approved" | Pass |
| JT95-04 | Employer is verified and posts a job (JT-56, JT-38, JT-73) | Company page shows Verified; My jobs, Create job: title, deadline in 3 days, Create draft; Publish job, Confirm publish | Verification status Verified; job page shows Draft, then "Job published" and Published | Pass |
| JT95-05 | Applicant uploads a resume that is validated (JT-48, JT-83) | Register as Applicant, Resumes: upload a synthetic PDF, Refresh resumes | "uploaded", then status Validated from resume-validator | Pass |
| JT95-06 | Applicant applies (JT-78) | Apply: Job ID, Load job, choose the validated resume, Submit application | "Application submitted. ID"; status page shows Submitted | Pass |
| JT95-07 | Employer reviews and shortlists (JT-79, JT-83) | Applications card, Review: score 4, Shortlist, Complete review, Confirm review | "Review completed"; status Shortlisted | Pass |
| JT95-08 | Employer schedules an interview (JT-80, JT-83) | Applications card, Interviews: date in 2 days at 10:00, details, Schedule interview | "Interview scheduled for", interview listed; status Interview | Pass |
| JT95-09 | Employer makes an offer (JT-78) | Applications card, Make offer: terms, Make offer | "Offer made" with the offer ID; status Offered | Pass |
| JT95-10 | Applicant accepts the offer (JT-78) | Offers: offer ID, Load offer, Accept offer, Confirm decision | Offer answered; Accept no longer offered | Pass |
| JT95-11 | Status history at the end (JT-81, JT-83) | Applications card, Status | Current status Accepted; history rows None to Submitted, Submitted to Under review, Under review to Shortlisted, Shortlisted to Interview, Interview to Offered, Offered to Accepted | Pass |

## Verification

Stack: `docker compose -f deploy/compose/docker-compose.yml --profile functions up -d` on `main` (all 29 containers up), frontend with `npm run dev`, admin login exported as `CYPRESS_ADMIN_EMAIL` and `CYPRESS_ADMIN_PASSWORD` from `deploy/compose/.env` without printing it.

| Check | Result |
|---|---|
| `npm run test:stack` (Cypress 16.2.0, Electron) | 11 of 11 passing in NN s |
| `npm test` (CI specs, stack folder excluded) | 4 specs, 155 of 155 passing |
| CI on the pull request | CI_RUN_PLACEHOLDER |

Screenshots of steps 01, 03, 04, 05, 07, 09 and 11 (`frontend/cypress/screenshots/stack/`, not committed) are attached to JT-95 in Jira.

Defects: none found in the application. Two failures during development were faults in the test itself (an application ID read before it was set, and a success message expected in an alert when the review page shows a card); both were fixed before this run.

Notes:

- Typed text is kept short because long key-by-key typing trips React's nested update limit inside Cypress (see JT-83).

## Acceptance criteria review

| Acceptance criterion | Status | Evidence |
|---|---|---|
| New spec in `tests/e2e` with no `cy.intercept`, run against the full Compose stack with the functions profile | Met | `tests/e2e/stack/main-workflow.cy.js`; verification above |
| Covers the main workflow from employer registration to the accepted offer, with the status history checked at the end | Met | JT95-01 to JT95-11 |
| Synthetic test data created by the test, so the run is repeatable | Met | Accounts unique per run; the passing run used a stack that already held data from an earlier run |
| How to run written in the `tests/e2e` README; results and screenshots saved for the test report | Met | `tests/e2e/README.md`; screenshots in Jira |
| Test cases in `docs/testing` traced to Jira; failures logged as Jira Bugs with severity | Met; no application failures to log | This document |
| Follows AGENTS.md; no new tools or contract changes | Met | Uses the existing Cypress 16; no `proto/` or `go.mod` changes |
| PR with the Jira key, CI green, merged, time logged | Pending | This PR |
