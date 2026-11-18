# JT-73 frontend test cases

Scope: employer job posting, applicant search/details/bookmarks, and notifications
for all authenticated roles. Phuntsho takes over JT-73 with team approval.
No backend, gateway, contract or JT-101 styling changes are included.

## Routes and acceptance coverage

| Requirement | Pages and behavior |
|---|---|
| Employer create/edit/publish/close and own list | `/employer/jobs`, `/employer/jobs/new`, `/employer/jobs/:jobId`; create draft, replace editable fields, confirm publish/close, refresh current status; closed/removed jobs have no edit or lifecycle actions |
| Search and view jobs | `/applicant/jobs`, `/applicant/jobs/:jobId`; title/category/any tag/exact location filters combined with AND; soonest deadline first from SearchJobs; details and description from GetJob; Apply and Report links |
| Save/unsave/list | Save on search or open job details; unsave immediately after saving or from `/applicant/saved-jobs`; paginated bookmarks with View job |
| Notifications | `/<role>/notifications` for applicant, employer and admin; paginated list, unread-only filter/count, mark as read and refresh |
| Shared infrastructure | Existing gateway-only `api`, JWT handling, role workspaces, MUI, React Router, loaders, safe code-based errors and pager; existing public paginated category/tag/location lookup reused |
| Validation and failures | Job title 1–150, description 0–5000, location 0–100 Unicode characters after trim; no NUL; at most 20 tags; future local deadline sent as UTC. Search title up to 150 and location up to 100. Failed publish explains verification/draft/deadline requirements without showing server text |

The saved-jobs contract returns IDs and save times, with no job summaries. The
list displays those fields and loads full job details only when View job is
opened. Save buttons do not preload every bookmark: an existing bookmark gets a
safe ALREADY_EXISTS message with directions to Saved jobs. Closing/removing a
job propagates asynchronously to search and bookmarks; refresh after processing.
Search contains no descriptions, as documented in AGENTS.md; GetJob supplies them.
No related-notification URL is inferred because related IDs can denote different
resources. Notification text and job descriptions render as plain React text.

## Automated frontend cases

`tests/e2e/job-pages.cy.js` uses the existing MUI/label/nav/card selectors and
in-memory login through stubbed gateway responses. These check browser behavior
and contract-shaped requests, not live service integration or CORS.

| Case | Partition or transition | Expected result |
|---|---|---|
| Employer workflow | Own list → next/first page → create → edit → publish → close | Correct paths, methods and bodies; UTC deadline; owner identity never in form body; confirmations and final state restrictions |
| Job input boundaries | Blank, 150/151 title, 5000/5001 description, 100/101 location, past date; 20/21 tags | Invalid save blocked; Unicode counted by code point; valid limits accepted |
| Reference pagination | Category on second page; lookup unavailable then retry | Existing lookup follows tokens and populates form; save disabled until options load |
| Status and failures | Published edit; removed read-only; update 403/FAILED_PRECONDITION/INTERNAL; publish FAILED_PRECONDITION | Appropriate actions; fixed safe messages; entered fields retained after failed save |
| Search filters | Title/category/tag/location plus later-page search | Correct query parameters and pagination reset; full details and Apply/Report links |
| Bookmarks | Search save → saved list → view → unsave; details save → unsave | Correct applicant-only endpoints, confirmation of save, list refresh after removal; next-page and stale-bookmark refresh |
| Save failures | ALREADY_EXISTS, FAILED_PRECONDITION, UNAVAILABLE | Safe messages, usable controls, no raw server text |
| Eligibility | Closed or expired published job; missing job; invalid address ID | No Save/Apply on ineligible job; safe not-found/invalid-address messages |
| Empty search, clear and recovery | Filtered empty → clear; title 150/151 and location 100/101; network failure then refresh | Empty state, limits, safe network message and recovery; cleared inputs/query/token |
| Notifications | All three roles; unread → read; next page; permission failure | Authenticated requests, count/list refresh, read items have no action; body rendered as text |
| Authentication and isolation | Anonymous pages, wrong-role job pages, protected HTTP 401 | Login or own workspace redirect; session cleared on 401 |

From the repository root, using installed dependencies:

```sh
npm run lint --prefix frontend
npm run typecheck --prefix frontend
npm run build --prefix frontend
# Keep this running in a separate terminal:
npm run dev --prefix frontend -- --force
# Focused cases:
env -u ELECTRON_RUN_AS_NODE npm test --prefix frontend -- --spec ../tests/e2e/job-pages.cy.js
# All frontend regression cases:
env -u ELECTRON_RUN_AS_NODE npm test --prefix frontend
```

## Reproducible manual integration cases

Use the existing Compose setup in `frontend/README.md` and the repository
README. Open `http://localhost:5173` with the gateway at `http://localhost:8000`.
Use synthetic accounts: two employers (one verified, one unverified), an
applicant and the seeded admin. Keep tokens/passwords out of screenshots/logs.
The JWT is in memory; log in again after a full reload.

| ID | Steps | Expected result |
|---|---|---|
| JT73-M01 | Log in as unverified employer. My jobs → Create job. Enter title `QA intern`, description `Synthetic vacancy`, select seeded category/tags/location, choose tomorrow at 17:00 local. Create draft. | Draft appears in My jobs. Manage job preserves every field and the selected local deadline. Browser Network shows gateway-only requests with Bearer auth and UTC timestamp. |
| JT73-M02 | In draft, change title to blank, then 151 characters; description 5001; location 101; choose yesterday. Restore title 150, description 5000, location 100 and tomorrow. | Invalid values disable save with clear hints; exact valid boundaries enable save. Select 21 tags if seed data allows, then remove one. Only at most 20 allows save. |
| JT73-M03 | Publish draft as unverified employer and confirm. | Fixed message explains verified company/draft/future deadline; no raw response text. Cancel leaves status unchanged. Complete existing company verification flow through admin, wait for employer.verified, return and refresh, then publish and confirm. Status becomes Published. |
| JT73-M04 | Edit published title/description and save; refresh. Log in as the other employer and navigate directly to its manage URL. | Edits persist for owner. Other employer gets safe permission/not-found error and cannot mutate the job. |
| JT73-M05 | Applicant → Search jobs. Filter title, selected category, tag and exact location. View job. Follow Apply link, then return. | Published unexpired job is found after event processing. Description is complete. Apply opens existing application page with job ID filled in; existing eligibility checks still apply. |
| JT73-M06 | Save search result. Open Saved jobs → View job. Attempt to save again. Return to Saved jobs → Unsave job. | Bookmark appears once; duplicate gets safe guidance; unsave removes it. Save/unsave immediately from details also succeeds. |
| JT73-M07 | As employer close published job, cancel once, then confirm. Applicant refreshes search/bookmarks/details. Also test a published job whose deadline has passed. | Closed status cannot be reopened/edited. Event processing removes closed job from search/bookmarks. Closed or expired details offer no Save or Apply. Expired published job may remain bookmarked until closure. |
| JT73-M08 | With notifications from the existing workflow, open Notifications as applicant, employer and admin. Check Unread only, mark one read, return to all. | Only recipient's notifications appear; unread count/list update; marked item remains in all with Read and no action. Literal HTML-like notification text does not execute. |
| JT73-M09 | With over 20 own jobs, results, bookmarks or notifications, click Next page then First page; change filters from a later page. | Offset tokens move pages, first/last controls disable appropriately, filters reset token, empty pages show a clear message. |
| JT73-M10 | Log out and navigate to each new role path; log in under a different role and attempt employer/applicant paths. Allow session to expire or invalidate token server-side then load notifications. | Anonymous/expired session requires login. Wrong role returns to own workspace. Backend still enforces ownership. |
| JT73-M11 | Stop one relevant service using the normal local setup; refresh its page or retry a mutation. Restore it and retry. | Fixed safe failure with retry/refresh; no internal errors or secrets displayed. No successful mutation is reported on failure. |
| JT73-M12 | Admin removes a published job through existing Moderation; owner/applicant refresh after events. | Owner sees Removed without edit/publish/close. Applicant details are unavailable and search/bookmark records disappear after processing. |

The cases above are the manual test plan, not a record that every step passed.
The observations below establish only the explicitly recorded browser behavior;
stubbed Cypress checks do not establish live integration acceptance.

## Manually observed live results — 18 Nov 2026

These results record the team member's reported browser UI and Network observations from
18 Nov 2026. They were supplied for this documentation update; no live tests,
automated tests or Jira checks were newly executed for the update. A verified
observation below does not mark every step of the corresponding manual case
as passed.

| Case or area | Verified browser UI observation | Evidence limit |
|---|---|---|
| JT73-M01: draft creation and own-job list | Created a draft job and saw it in the employer's own-job listing. | Request headers, gateway-only routing and UTC request payload were not directly inspected. |
| JT73-M02: title | Blank title and 151 characters were invalid; 150 characters were valid. | Establishes the observed UI boundaries, not direct API validation or every Unicode/trim partition. |
| JT73-M02: description | 5001 characters were invalid; 5000 characters were valid. | Other unexecuted description partitions remain pending. |
| JT73-M02: location | 101 characters were invalid; 100 characters were valid. | Other unexecuted location partitions remain pending. |
| JT73-M02: deadline | A past deadline was invalid. | UTC serialization and other unexecuted time boundaries remain pending. |
| JT73-M02: tags | 21 selected tags were invalid; 20 were valid. | Direct API tag validation was not established. |
| Draft update | Updated the draft through the employer page. | No direct API mutation attempt was recorded. |
| JT73-M03: unverified publish | Publishing as an unverified employer was rejected with a safe error in the UI. | This observation does not establish all publish failure partitions or cancellation behavior. |
| JT73-M03: company verification setup | Created a test company and submitted a verification request; admin approval completed and the company showed verified status. | Records the observed setup workflow, not complete company-verification test coverage. |
| JT73-M03/M04: publish and published edit | Published successfully, edited the published job's description, and saw that description persist after Refresh Job. | Published-title editing was not established by this earlier observation; the later title-update scenario is recorded below. Other unexecuted edit partitions remain pending. |
| JT73-M04: second employer | A second employer was denied job management in the browser UI. | Owner mutation enforcement through direct API update/publish/close attempts was not tested. |
| JT73-M08: notifications | Applicant, employer and admin notification pages each displayed their empty state; applicant Unread only also displayed an empty state. | No nonempty notifications, mark-as-read transition, recipient isolation with data or notification pagination was established. |
| Saved jobs: initial observation | Applicant Saved jobs displayed its empty state. | This initial empty-state observation is retained; later bookmark operations are recorded below. |
| JT73-M10: admin route restrictions | Admin was denied employer jobs and applicant saved-jobs routes in the browser UI. | Other role/anonymous/session-expiry paths and direct API authorization remain pending. |

### Direct job details, bookmarks and closure — 18 Nov 2026

The following additional live UI observations were reported for the published
**QA intern** job using its known ID, independently of search indexing. These
extend the earlier evidence without establishing complete JT73-M05–M07
coverage. No live or automated tests were newly executed for this documentation
update.

| Case or area | Verified observation | Evidence limit |
|---|---|---|
| JT73-M05: direct published details | Applicant opened the job directly and saw the full updated description, location, deadline and Published status. | Search discovery, filtering and indexing were not established by direct navigation. |
| JT73-M06: Save and populated listing | Save succeeded. Saved Jobs listed one bookmark with the matching job ID and a saved timestamp. | Establishes a populated one-item listing, not pagination or closure cleanup. |
| JT73-M06: Unsave | Unsave succeeded and the saved-job count became zero. | Manual removal does not establish event-driven bookmark cleanup. |
| JT73-M06: save again and View Job | Saving again succeeded; opening the job through Saved Jobs → View Job succeeded. | Independent of the search workflow, which was blocked at that observation. |
| JT73-M06: duplicate Save | Duplicate Save returned HTTP 409 with safe already-saved guidance; the saved-job count remained one. | Other save/error partitions remain pending. |
| JT73-M05: Apply navigation | Apply opened `/applicant/apply` with the correct job ID prefilled. | No application was submitted; application submission and eligibility outcomes were not tested by this observation. |
| Report Job navigation | Report Job opened the report form with the correct job ID. | No report was submitted. |
| JT73-M07: employer close | Employer accepted the close confirmation and the job became Closed, with edit and lifecycle actions unavailable. | Close cancellation was not tested. The UI result does not establish event publication, delivery or consumption. |
| JT73-M07: direct closed details | Applicant direct job details showed Closed with no Save or Apply actions. | Establishes current detail-page behavior, not search removal or bookmark cleanup. |
| JT73-M07: bookmark after closure | After refresh, the closed job remained bookmarked. | Automatic cleanup did not pass; blocked on JT-70 as recorded below. |

### Admin Notifications Network observation — 18 Nov 2026

For the completed **Admin Notifications GET request only**, the browser Network
observations were:

| Item | Observed result |
|---|---|
| Request URL | `http://localhost:8000/v1/notifications` with pagination parameters |
| Response status | `200 OK` |
| Response Content-Type | `application/json` |
| Response CORS header | `Access-Control-Allow-Origin: http://localhost:5173` |
| Request Authorization | The user subsequently confirmed that a JWT token appeared after `Bearer` on the completed request; the token value is not recorded. |

The earlier screenshot showed provisional headers and `Bearer` alone. It is
**not proof that a token was sent**; the Authorization observation relies on
the user's subsequent confirmation about the completed request.

These observations establish the gateway URL, reported Bearer token presence,
JSON response and allowed-origin response header for this one Notifications
request. They do not establish those properties for other endpoints or roles,
any preflight request or its headers/status, or complete CORS coverage. UTC
payload serialization and the other pending checks remain unverified.

### JT-69: earlier blocker and subsequent live search verification

The earlier observation is retained: after publishing QA intern, applicant
search stayed at **zero results**. At that time,
[JT-69](https://jobtrack-cst.atlassian.net/browse/JT-69) was reported as an
unmerged dependency, Jira status **In Progress**, owned by **Sangay**, and the
`main` job-search service lacked the job event consumer. The known-ID details
and bookmark checks above did not establish search integration at that stage.

Later on **18 November 2026**, after JT-69 was reported merged into `main`,
job-search was rebuilt via Docker Compose. Startup logs confirmed that the
event consumer was ready on queue **`job-search.jobs`**. A verified employer
created and published the following synthetic job:

| Field | Observed value |
|---|---|
| Title | `JT73 Search Test` |
| Job ID | `d4ad8da7-9ab4-4c70-b522-a21c4b59794e` |
| Category | Business and Finance |
| Tag | Accounting |
| Location | Bumthang |
| Deadline | 25 November 2026 at 5 PM local time |

| Live scenario (JT73-M05) | Observed result | Status |
|---|---|---|
| Applicant unfiltered Refresh Search after publication | Displayed one matching job, `JT73 Search Test`. | Passed |
| Combined title `JT73 Search Test`, category Business and Finance, tag Accounting and location Bumthang | Returned exactly one matching job. | Passed |
| Keep the same title, category and tag; change only location to Thimphu | Returned zero results, showed the correct empty state and disabled pagination. | Passed |

For these exact scenarios, **publish-to-search integration was verified after
the backend consumer merged**. The earlier JT-69 publish-to-search blocker is
therefore resolved for the observed environment and job. The observations
support these combined-filter positive and negative cases, not every search
partition or event path.

The merge, Compose rebuild, startup logs and browser results above are supplied
live evidence; no checkout, rebuild, new test execution or Jira query was
performed for this documentation update. At that stage, they did **not**
establish `job.updated` propagation or `job.closed` search removal; the
subsequent specific live scenarios below supply that evidence for this job.
Multi-page live pagination, notifications mark-as-read with real data and JT-70
bookmark cleanup remain pending or blocked; full JT-73 integration acceptance
is not established.

### Published update and closure in search — 18 Nov 2026 (PR #72)

Additional reported live UI verification used the same synthetic job,
`d4ad8da7-9ab4-4c70-b522-a21c4b59794e`. These results extend the earlier
publish-to-search and combined-filter observations without replacing them.

| Specific live scenario | Observed result | Status |
|---|---|---|
| JT73-M04/M05: published title edit and `job.updated` propagation | Employer changed `JT73 Search Test` to `JT73 Search Test Updated`, clicked Save Changes and received `Job updated`. The job remained Published with an unchanged deadline. Applicant cleared search filters, refreshed and saw exactly one result with the updated title. | Passed |
| JT73-M07: close and `job.closed` propagation to job-search | Employer closed the same job, received `Job closed`, saw Closed status and no editing actions. Applicant cleared filters and refreshed Search Jobs: `Matching jobs: 0`, the correct empty state and disabled pagination; the closed job was absent. | Passed |

These observations confirm **`job.updated` propagation** and **`job.closed`
removal from the job-search read model for this job**. They are supplied live
browser observations, not new executions by the assistant, event-payload
inspection or complete coverage of all consumer behavior. The unchanged
deadline is a UI observation, not an additional wire-precision check. Close
cancellation remains untested live.

Search removal does not establish saved-job cleanup: **JT-70 remains blocked**.
Notification mark-as-read and other notification checks requiring real data,
and multi-page live pagination, remain pending. No code, Git branch, Jira or
PR changes were made for this documentation update.

### Bookmark cleanup blocker — JT-70

After the employer closed QA intern successfully, the applicant refreshed Saved
Jobs and the closed job remained bookmarked. The reported investigation found
that `services/saved-jobs/cmd/server/main.go` does not start a `job.closed`
consumer. [JT-70 — Saved-jobs: consume job.closed](https://jobtrack-cst.atlassian.net/browse/JT-70)
is **To Do**, assigned to **Jigden**, as reported for 18 Nov 2026. This diagnosis
and Jira status are supplied evidence, not a new backend execution or Jira query
for this documentation update.

Automatic bookmark cleanup after closure (JT73-M07) remains **blocked on
JT-70** and is not recorded as passed. Publish-to-search and the specific
`job.updated` and `job.closed` search scenarios above are verified for the
synthetic search job; none establishes bookmark cleanup.
Once the saved-jobs consumer is available in the live environment, repeat the
bookmark cleanup check. Manual save/unsave success and direct Closed details
do not prove that the saved-jobs consumer ran.

### Untested and pending live checks

Mark-as-read and nonempty pagination (own jobs, search results, saved jobs and
notifications) are **untested for lack of sufficient data**. A one-bookmark
listing and a one-result search are now verified, but no multi-page live
pagination behavior or notification transition can be inferred from them, the
earlier empty states or disabled pagination in the zero-result search case.

All other live steps not explicitly verified above remain **pending**, including
unexecuted portions of JT73-M01–M12: request headers and CORS checks beyond the
single Admin Notifications request recorded above, all preflight checks, and UTC
payload serialization; direct API owner mutation enforcement; complete validation and
reference-data partitions; close cancellation and other unexecuted lifecycle
partitions, including expired-job behavior; application/report submission and
other unexecuted navigation/eligibility checks; save/error partitions beyond the
observed duplicate Save; nonempty notification
content and recipient isolation; remaining authentication/role/session-expiry
paths; service outage/recovery; and moderation-removal propagation. The exact
`job.updated` and `job.closed` search scenarios above passed for the specified
job; other unexecuted event-propagation partitions remain pending. Automatic
bookmark cleanup after closure remains blocked on JT-70. The observed close
and search removal do not cover close cancellation or bookmark cleanup.
Remote CI and deployment checks remain unrun. The partial UI observations do
not establish complete manual test-case coverage or full integration acceptance.

## Verification record

The following automated results are preserved as previously reported; no new
build, lint, typecheck or Cypress execution is claimed by this documentation
update.

Run on 18 Nov 2026 with Node 26.13.0, npm 11.20.0 and Cypress 16.2.0
(Electron 146 headless). Dependencies were already installed.

| Command | Result |
|---|---|
| `npm run lint --prefix frontend` | Passed |
| `npm run typecheck --prefix frontend` | Passed |
| `npm run build --prefix frontend` | Passed; Vite warns that the main minified chunk exceeds 500 kB |
| `env -u ELECTRON_RUN_AS_NODE npm test --prefix frontend` | Passed: 184 tests across five specs, including 29 JT-73 cases; no failed, pending or skipped cases |
| `git diff --check` | Passed; new files also checked individually with `git diff --no-index --check /dev/null <file>` |

The sandbox initially prevented binding the Vite port (`EPERM`) and launching
the Cypress browser (exit 134). Both ran with approved execution outside the
sandbox; `ELECTRON_RUN_AS_NODE` was removed as in JT-63. The first complete
focused run had 21 passing and four failing cases: autocomplete popups covered
controls, and empty Cypress DELETE stubs needed explicit JSON bodies. Those test
interactions/stubs were corrected without changing the shared client contract.
The final regression run includes all 29 JT-73 cases.

The manual results above supersede the earlier statement that live checks were
entirely unrun. Publish-to-search and the specific update/close propagation
scenarios above passed after JT-69 merged; full integration acceptance remains
blocked by JT-70 and the remaining
pending live checks. This update changes only this document;
no source, tests, Jira, branches, commits or PRs were altered. Before submitting,
complete the blocked/pending cases and run local checks, then commit with JT-73,
push, open a PR and move the Jira card to In Review when ready.


## P2 defect corrections — 18 Nov 2026

Job editing now preserves the exact original `job.deadline` string, including
seconds and fractional seconds, when the local deadline field is unchanged.
Validation uses that original value in this case. An explicitly changed local
deadline is still validated and converted to UTC; new-job behavior is unchanged.
Publish/close failures now appear as safe alerts inside the active dialog's
`DialogContent`, with retry and cancellation available. Opening a new
confirmation clears the previous transition error.

The Cypress regressions cover description-only edits of both draft and
published jobs with a deadline ending in `:45.123456Z`, exact request-body
preservation, changed-deadline UTC conversion and past-date rejection. Both
publish and close failures are checked for visible alerts within the dialog,
absence of an `aria-hidden=true` ancestor, no raw server message, cancellation,
clean reopening and successful retry. The existing reference-pagination test
now waits for Category to become enabled before opening its options.

Verification for these corrections (separate from the earlier results above):

| Command | Result |
|---|---|
| `npm run lint --prefix frontend` | Passed |
| `npm run typecheck --prefix frontend` | Passed |
| `npm run build --prefix frontend` | Passed; existing main-chunk size warning remains |
| `env -u ELECTRON_RUN_AS_NODE npm test --prefix frontend -- --spec ../tests/e2e/job-pages.cy.js` | Passed: 33 tests, zero failed/pending/skipped |
| `env -u ELECTRON_RUN_AS_NODE npm test --prefix frontend` | Passed: 188 tests across five specs, including 33 JT-73 cases; zero failed/pending/skipped |
| `git diff --check` | Passed; files changed by these corrections also checked individually with `git diff --no-index --check /dev/null <file>` |

The first focused run had 32 passing cases and one failure in the existing
reference-pagination case: Category was clicked while reference options were
still loading. Waiting for the control to be enabled resolved it; the complete
focused rerun passed. Vite was already running on port 5173, so the attempted
additional server exited with “Port 5173 is already in use”; checks used the
existing server. Cypress ran with approved execution outside the sandbox and
`ELECTRON_RUN_AS_NODE` unset.

These are gateway-stubbed browser checks, not new live-backend observations or
a manual screen-reader session. The live evidence and limitations recorded at
that stage are retained above. Subsequent publish-to-search verification
supersedes the earlier JT-69 blocker for those exact scenarios; JT-70 and the
other pending live checks remain.
