# JT-78: Frontend application workflows

Date: 15 November 2026. Implementation base: `205e824`, branch
`JT-78-frontend-application-workflows`. All data must be synthetic.

## Scope and acceptance review

The checklist below traces the JT-78 requirements supplied with the task.
Cypress evidence uses intercepted responses; live observations and remaining
manual coverage are recorded separately below.

| Requirement | Implementation | Verification |
|---|---|---|
| Applicant applies using a validated resume | `/applicant/apply`, optional `?jobId=<UUID>` prefills the job ID; Load job reads the public Job Posting endpoint; only VALIDATED resumes are selectable | Cypress apply success, pending/rejected exclusion, closed job, passed deadline, no resumes, later resume page, duplicate submit prevention, business errors |
| Applicant lists and withdraws applications | `/applicant/applications`, paginated submission records, confirmation before withdrawal, refresh after success | Cypress list/withdraw/cancel, pagination; manual race cases below |
| Applicant views, accepts and declines offers | `/applicant/offers`, offer-ID lookup, optional `?offerId=<UUID>` prefill, terms/status display, final decision confirmation | Cypress both decisions and disabled final actions |
| Employer makes offers | `/employer/applications` links to `/employer/offers/new/:applicationId`; terms validated at 1–2000 Unicode characters | Cypress request shape, boundary, duplicate offer; returned offer ID displayed for sharing |
| Both participants read and send messages per application | Application cards link to `/applicant/messages/:applicationId` or `/employer/messages/:applicationId`; oldest-first pages, refresh, 1–2000 character composer | Cypress both roles, recipient/sender omitted from request, permission error, draft retained on failure |
| Applicant manages job alert subscriptions | `/applicant/job-alerts`; create/list/delete; all category/tag/location reference pages loaded; max 20 selected IDs per filter; exact location name up to 100 characters | Cypress location subscription/delete, location reference pagination and exact selected name, Any with category/tag filters, reference loading/failure/refresh, selected IDs, empty filters, duplicate and 10-subscription limit |
| Gateway, shared client, JWT, MUI, role restrictions | All feature API modules call existing `api`; no internal routes/credentials; nested role guards; JWT remains in memory | Existing auth regression suite plus feature role guards, unauthenticated deep links, protected 401, bearer header assertions |
| Clear gRPC/business failures | Shared client retains numeric gRPC code without displaying server message bodies; contextual safe messages distinguish conflicts and failed preconditions | Cypress closed/unvalidated eligibility, duplicate application/offer/alert, missing records, permissions, session expiry, retry |
| Existing tooling, tests and docs | No dependency, tool, contract, backend service or deployment changes | TypeScript lint/typecheck, production build, existing Cypress setup; this document |

## Contract constraints

Offer has `GetOffer` but no list or find-by-application RPC. The UI therefore
requires the offer ID supplied by the employer or another existing source. It
does not invent a list endpoint. An employer can copy the returned ID and send
it in the application's message thread. Offer-ID lookup is also available to
employers at `/employer/offers`.

Job discovery and resume upload pages belong to other frontend tasks. This task
accepts a job ID (or a prefilled link) and reads existing resume metadata; it does
not implement another owner's search/upload UI. Application's `withdrawn` field
is a submission record, not the separate Application Status lifecycle. The UI
labels it accordingly and relies on services for authorization and eligibility.
Offer lifecycle eligibility beyond current service rules is unresolved in
AGENTS.md; this frontend does not invent additional rules.

Alert editing has no RPC: delete and recreate, as the public contract requires.
Categories/tags are selected by public reference names/IDs. Location is a MUI
selection populated by all pages of the existing public `GET /v1/locations`
through the shared API client and reference-list helper. It submits the exact
returned location name, without trimming or changing case, rather than the
location ID. Any submits an empty location filter; at least one category, tag or
location filter is still required. Reference loading/errors and refresh apply to
all three lists. The existing 100-character location validation remains in place.

## Automated checks

From `/Users/phuntshonamgyel/Desktop/jobtrack`:

```sh
npm --prefix frontend run lint
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

In one terminal:

```sh
cd /Users/phuntshonamgyel/Desktop/jobtrack/frontend
npm run dev -- --host localhost
```

In another terminal:

```sh
cd /Users/phuntshonamgyel/Desktop/jobtrack/frontend
env -u ELECTRON_RUN_AS_NODE npm test
# Only JT-78, if needed:
env -u ELECTRON_RUN_AS_NODE npm test -- --spec ../tests/e2e/application-workflows.cy.js
```

The existing test runner is Cypress; no separate unit/component test runner is
installed. Tests exercise actual React pages and the shared client in the browser
with intercepted gateway responses. They assert contract URLs, JSON bodies,
nested pagination fields, JWT headers, state transitions, boundaries and errors.
They do **not** demonstrate real backend, event delivery or CORS integration.

## Live browser integration observations — 15 November 2026

These are the live browser results reported by the project member for this date,
recorded here without rerunning the browser session during this documentation
update. Verified observations are limited to the actions and responses listed;
page loads and empty lists do not verify workflows requiring existing records.

| Area | Verified observation |
|---|---|
| Frontend | Vite frontend at `http://localhost:5173` loaded. |
| Applicant authentication/navigation | Applicant registration, login and navigation worked. Logout redirected to login. |
| Applicant applications | Authenticated applications GET returned HTTP 200 with an empty list. No populated application workflow was exercised. |
| Apply | Apply page loaded; no validated resume was available and submission was disabled. No application was submitted. |
| Gateway CORS | Preflight from `http://localhost:5173` returned HTTP 204 with the expected allowed origin, headers and methods. This verifies the observed preflight, not every CORS scenario. |
| Reference-service recovery | Categories, tags and locations initially returned HTTP 503 because `category-tag` was absent. After it was started via Docker Compose, the service became healthy and the reference endpoints returned HTTP 200. |
| Location selection | The location dropdown displayed seeded location names. |
| Location-only alert | A Thimphu location-only job alert was created and deleted successfully, with visible confirmation and refreshed list. This does not verify alert matching or event delivery. |
| Category plus tag alert | Creation returned HTTP 503. The reported cause is that `job-alert` currently has no real `ResolveNames` client. That implementation belongs to JT-76, not JT-78; successful category/tag-filtered creation remains unverified. |
| Applicant offers | Offers page loaded; lookup of a synthetic unknown offer returned a safe HTTP 404 message. Viewing an existing offer or deciding one was not tested. |
| Employer authentication/navigation | Employer registration, login and navigation worked. |
| Employer applications | Authenticated applications GET returned HTTP 200 with an empty list. |
| Employer offers | Employer Offers page loaded. No offer was created. |

JWT and session state are held in memory. A full page reload logs the user out
by current design; login is required again. Authenticated cross-role manual route
testing was **not completed**. Cypress role restriction tests passed in the
previously reported **52-test suite**; they are separate evidence and do not
constitute live manual cross-role verification.

Full application submission, withdrawal, offer creation and accept/decline
decisions, messages round-trip and event delivery remain **unverified live**.
All manual scenarios or portions of scenarios not explicitly observed above
remain unverified, not passed. In particular, category/tag alert success is
blocked by the reported JT-76 dependency; location-only success does not close
that gap.

## Manual integration cases (expected results; not a pass record)

The table retains the planned steps and expected results. Only the limited
observations above have live evidence. For example, M02 has only the no-validated-
resume observation; M13 has location-only create/delete and a failed category
plus tag attempt; M16 has logout/session-design observations but no completed
authenticated cross-role testing; M17 has reference-service recovery only.
None of those partial observations marks the full manual case as passed.

Prerequisites: use the repository's existing local Compose setup, gateway at
`http://localhost:8000` allowing browser origin `http://localhost:5173`, and the
frontend above. Prepare synthetic applicant, employer and unrelated accounts,
an employer-owned published job with future deadline, a closed job, validated,
pending and rejected resumes. Producers/consumers must be running for downstream
application participant projections. Do not copy/log JWTs.

| Case | Steps | Expected result |
|---|---|---|
| JT-78-M01 apply success | Log in as applicant; Apply; enter published job UUID; Load job; select validated PDF; Submit application; open My applications | Returned application ID; submission button disabled after success; record appears in list |
| JT-78-M02 resume states | Inspect resume choices with VALIDATED, PENDING and REJECTED files; if none validated, refresh after validation event; test more than 20 resumes | Only VALIDATED files selectable; empty state explains validation; later pages reachable |
| JT-78-M03 eligibility race | Load open job and validated resume; close job or invalidate resume in backend before submitting | Clear closed/deadline/unvalidated eligibility message; no false success; refresh/retry available |
| JT-78-M04 closed/deadline | Load CLOSED job; repeat with PUBLISHED job whose deadline passed | Explanation shown; submission disabled |
| JT-78-M05 duplicates | Submit twice for the same job using separate tabs | One succeeds; other reports already applied without raw server input |
| JT-78-M06 withdraw | Open applications; Withdraw then Cancel; repeat and Confirm; refresh; attempt a concurrent withdrawal in another tab | Cancel makes no change; success becomes Withdrawn; second action reports latest-state conflict |
| JT-78-M07 employer offer | Log in as owning employer; Applications; Make offer; enter terms and submit; copy returned offer ID | Correct application prefilled; request has terms only; returned ID and pending terms/status visible |
| JT-78-M08 offer response | Applicant opens Offers and loads shared ID; Accept then Cancel; accept and confirm; repeat with another application's offer using Decline | Cancellation preserves pending status; confirmed decision final; no further response buttons |
| JT-78-M09 offer boundaries | Try whitespace-only terms, 1, 2000 and 2001 Unicode characters; make another offer for same application; race a decision in two tabs | Valid range 1–2000; duplicate offer explained; repeated/reversed decision rejected by service |
| JT-78-M10 message round trip | Applicant opens application Messages, sends text; employer opens same application's Messages and refreshes, replies; applicant refreshes | Both see messages oldest first with sender and timestamp; recipient derived by service |
| JT-78-M11 message boundaries | Try empty/whitespace, 1, 2000, 2001 characters; interrupt network during send | Invalid lengths disabled; valid lengths accepted; failure preserves draft and permits retry; confirm delivery before retrying an uncertain send |
| JT-78-M12 participant lag/access | Open thread before participant event arrives, then refresh after processing; try unrelated account application/offer IDs | Not-found message allows refresh; unrelated account gets permission error; no messages exposed |
| JT-78-M13 alert filters | Create category-only and tag-only filters with Any location, then select a returned location for location-only and combined filters; inspect saved names; delete with cancel then confirm | At least one filter required; combined filters use AND; exact location retained; deletion removes only confirmed subscription |
| JT-78-M14 alert boundaries | Choose 20 then 21 categories/tags if available; verify location choices come only from reference data and Any is available; create identical subscription; create an 11th | 21 IDs blocked; exact selected location name submitted; duplicates explained; service's 10-subscription limit explained |
| JT-78-M15 pagination | Prepare over 20 applications/messages/resumes or category/tag/location reference items; use Next page and First page; refresh | Correct records and stable ordering from service; no false empty state during loading; reference lists include later pages |
| JT-78-M16 route/session | Open feature deep links logged out; attempt applicant routes as employer/admin and employer routes as applicant; expire session or return protected 401 | Login redirect or own workspace; no forbidden controls; token/session cleared on 401 |
| JT-78-M17 infrastructure failure | Stop a needed backend service; load each page and retry after restoration | Error/loading states clear; no false success; refresh or submit retry available |

Run the commands locally and complete the remaining manual scenarios before
treating their service integration as verified. After review, the repository workflow is a Jira-keyed
commit, push, PR and moving JT-78 to In Review; none are performed by this task.

## Execution record

- `npm --prefix frontend run lint`: exit 0.
- `npm --prefix frontend run typecheck`: exit 0.
- `npm --prefix frontend run build`: exit 0; Vite emitted a non-fatal warning for
  the 560.19 kB minified main chunk (176.56 kB gzip). No dependency changes made.
- `npm run dev -- --host localhost` in `frontend`: started Vite on localhost:5173.
- `env -u ELECTRON_RUN_AS_NODE npm test` in `frontend`: exit 0, **50 passing**:
  31 JT-78 workflow tests and 19 existing authentication/client tests. No failed,
  skipped or pending tests. Cypress used its existing Electron headless browser
  and emitted its Electron deprecation notice.
- The first sandboxed server/test attempts failed (localhost bind `EPERM` and
  browser exit 134); rerunning the same existing tooling with approved sandbox
  escalation allowed the full suite to complete. No project configuration changed.
- At this original automated-check stage, live integration had not been verified.
  The dated live observations above update that status for the listed checks
  only. Producer/consumer delivery, cross-account integration and all unexecuted
  manual scenarios remain unverified. Offer discovery remains limited to ID lookup
  by the existing public contract.
- No branch changes, commits, pushes, PRs or Jira mutations performed.


## Final frontend correction pass

- Verified the requested display strings already read “Refresh the job”,
  “Withdraw this application?” and “one offer at a time”; no changes needed in
  those three files during this pass.
- Replaced the location free-text field with a MUI selection and an explicit Any
  option. Categories, tags and locations use the same paginated reference loader.
- Extended Cypress coverage for locations on later reference pages, exact name
  submission, resetting a selected location to Any with category/tag filters,
  and location-reference failure/loading/refresh recovery. Existing location
  subscription and limit cases now select a returned location.
- At the correction-pass automated-check stage, live integration had not been
  verified; intercepted browser tests do not establish live integration. Refer to
  the dated observations above for the subsequently reported live checks; event
  delivery and all unexecuted manual scenarios remain unverified.

Correction-pass verification:

- `npm --prefix frontend run lint`: exit 0.
- `npm --prefix frontend run typecheck`: exit 0.
- `npm --prefix frontend run build`: exit 0; main chunk 560.41 kB minified,
  176.62 kB gzip; existing Vite >500 kB warning remains.
- From `frontend`, `env -u ELECTRON_RUN_AS_NODE npm test`: exit 0; **52 passing**
  (33 JT-78 and 19 auth/client tests), 0 failures, skips or pending tests.
  Existing Cypress Electron deprecation notice remains.
- `git diff --check`: exit 0.
- Modified during this correction pass: `frontend/src/features/job-alert/api.ts`,
  `frontend/src/features/job-alert/JobAlertsPage.tsx`,
  `tests/e2e/application-workflows.cy.js` and this test document. Existing
  uncommitted implementation changes remain for review on the same branch.


## Documentation-only live-results update

Recorded the project member's 15 November 2026 live observations and the JT-76
`ResolveNames` dependency without changing source, contracts, services, other
documents, dependencies, Jira or Git state. Prior automated results and warnings
above are retained, not rerun or relabelled as live evidence. Changes are left
uncommitted for review.

## PR #57 Cypress readiness correction

Reported CI failure: workflow run `43700558456`, Frontend checks job
`132770534754`, commit `75ff5cb`: 52 tests, 49 passed and 3 failed. Location-only
create/delete and alert constraint 6/9 timed out finding options after the
`selectLocation` helper clicked the select. This is the supplied CI result,
not a claim that a subsequent GitHub run has passed.

Root cause reproduced locally with headless Cypress 16.2.0 / Electron 146,
`CI=1`, and the default 1000×660 viewport. A temporary diagnostic spec held the
location response until after an early click. It confirmed `aria-disabled=true`
and `aria-expanded=false` at the click; after releasing the response the select
became enabled but stayed closed, then the option lookup failed at 4000 ms just
as reported. MUI's non-native Select is a `div`, so Cypress permits clicking it
while `aria-disabled=true`; MUI removes the opening mouse handler while disabled.
The reference loader waits for categories, tags and locations together, so the
fixture response alone does not guarantee the control is ready. No evidence of
a viewport defect or incorrect application behavior was found.

The test-only fix locates the select within the Location-labelled field, waits
for its visible and ARIA-enabled state, confirms `aria-expanded=true`, and selects
an exact escaped option name within the visible listbox. It then confirms closure.
A regression case gates the reference response to verify the ignored early click
and subsequent successful selection with the helper. There are no arbitrary
sleeps, force clicks or timeout increases. The location-only create/delete request
assertions and alert constraint error assertions remain intact. Application code,
fixtures' intended data, dependencies and workflow configuration are unchanged.
The temporary diagnostic spec was removed after reproduction.

Local correction verification:

- `npm --prefix frontend run lint`: exit 0.
- `npm --prefix frontend run typecheck`: exit 0.
- `npm --prefix frontend run build`: exit 0; unchanged 560.41 kB minified main
  chunk (176.62 kB gzip); Vite's >500 kB warning remains.
- From `frontend`, `env -u ELECTRON_RUN_AS_NODE npm test`: exit 0,
  **53 passing** (34 workflow tests, including the added regression, plus all
  19 auth/client tests). The three CI-failing scenarios passed locally.
- `git diff --check`: exit 0.

The local runner is macOS with Node 26.13.0; GitHub's workflow uses Ubuntu and
Node 22. CI mode and viewport reproduction do not replicate that OS/runtime.
The existing Cypress Electron deprecation notice remains. No GitHub Actions
rerun has been triggered or verified; this correction must not be described as
CI green. Previously recorded live observations, the JT-76 dependency and all
unverified live/manual scenarios retain their existing status. No commits,
pushes, branch switches, PR mutations or Jira changes were performed.

Additional verification from `frontend`:
`env -u ELECTRON_RUN_AS_NODE CI=1 npm test -- --config viewportWidth=1000,viewportHeight=660`
completed with exit 0 and **53 passing**, 0 failures, skips or pending tests.
This rerun includes the final response-gated regression and all three originally
failing scenarios. It is local evidence, not a GitHub CI result.
