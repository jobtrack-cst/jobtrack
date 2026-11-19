# JT-99 User acceptance test (UAT) script

Traceability: [JT-99](https://jobtrack-cst.atlassian.net/browse/JT-99). Owner: Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup. Session dates: <day month year>.

## Scope

UAT checks that each role can complete its real tasks through the React frontend on the full Docker Compose stack, as a user would, without developer tools. It complements the automated end-to-end test (JT-95, `tests/e2e/stack/main-workflow.cy.js`) and covers the same main workflow plus each role's other features.

Traceability rule: Jira has no Story issues, and no user stories are written in `docs` or the README. Each scenario is therefore traced to its feature Epic and to the task that built it. Epics: JT-8 Identity and Employers, JT-9 Jobs and Discovery, JT-10 Administration and Insights, JT-11 Applications and Resumes, JT-12 Hiring Workflow, JT-13 Notifications and Serverless, JT-14 Gateway and Frontend.

Out of scope: load (JT-96), k3d and Knative (JT-92), and Should tasks that are not merged when the session runs (JT-102 company logo, JT-103 profile photo). A scenario that needs an unmerged task is marked Not run with the task key.

Known gaps at the time of writing (19 Nov 2026):

- JT-72 (audit-log consumer) is To Do, so the audit log stays empty. UAT-AD-07 is Not run unless JT-72 is merged first.
- JT-70 (saved-jobs consumer for `job.closed`) is To Do, so a closed or removed job may still appear in saved jobs. UAT-AP-04 records this as a note, not a bug.

## Test environment

1. Update `main` and start the full stack, including the functions (resume-validator, notification-dispatcher and email-sender are needed for the main workflow):

        git checkout main && git pull
        docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build
        docker compose -f deploy/compose/docker-compose.yml ps

2. Start the frontend and open `http://localhost:5173`.
3. Admin login: `ADMIN_EMAIL` and `ADMIN_PASSWORD` from your local `.env` (never written in this document).
4. Emails: open the Mailpit web inbox at `http://localhost:8025`.
5. Use synthetic data only. Use a new email per session, for example `uat-employer-1911@example.com`.

Record the commit tested: `<git log --oneline -1>`.

## Session order

The roles depend on each other, so run the three sessions in one sitting in this order. Each tester runs only the rows for their own role.

1. Employer: UAT-EM-01 to UAT-EM-04 (register, company profile, request verification, publish refused while unverified)
2. Admin: UAT-AD-01 to UAT-AD-02 (approve the employer)
3. Employer: UAT-EM-05 to UAT-EM-06 (verified status, publish the job)
4. Applicant: UAT-AP-01 to UAT-AP-08 (find the job, upload resume, apply)
5. Employer: UAT-EM-07 to UAT-EM-11 (review, interview, message, offer)
6. Applicant: UAT-AP-09 to UAT-AP-12 (status, interview, accept the offer)
7. Admin: UAT-AD-03 to UAT-AD-08 (moderation, analytics, audit log)
8. All roles: UAT-X-01 to UAT-X-03 (access control)

## Scenarios

Result column: Pass, Fail (with the Jira Bug key), or Not run (with the reason).

### Employer

| ID | Requirement | Steps | Expected result | Result |
|---|---|---|---|---|
| UAT-EM-01 | JT-8, JT-30, JT-14 | Open `/register`, register as an employer, then log in at `/login` | Account created; after login the employer workspace `/employer` opens with its navigation | Pass |
| UAT-EM-02 | JT-8, JT-32, JT-88 | Open `/employer/company`, create a company profile with a name, description, website (`https://...`) and location; then edit the description | Profile saved and shown with status Unverified; the edit is shown after reload. An invalid website such as `example` is refused before sending | Pass. Invalid website refused; edit kept. Note: reloading the page logs the user out (JT-106) |
| UAT-EM-03 | JT-8, JT-33, JT-88 | Open `/employer/verification`, submit a request with a registration number and notes | Request listed as Pending. A second request while one is pending is refused with a clear message | Pass. Note: the page hides the second-request button instead of showing an error; one pending request is enforced |
| UAT-EM-04 | JT-9, JT-38, JT-57, JT-73 | Before the admin approves: open `/employer/jobs/new`, create a draft job and try to publish it | Draft saved; publishing is refused because the employer is not verified; the job stays Draft | Pass |
| UAT-EM-05 | JT-8, JT-56 | After the admin approves (UAT-AD-02), reload `/employer/company` and `/employer/notifications`; check Mailpit | Status Verified; an in-app notification and an email about the verification | Fail, JT-105. Status Verified, but no in-app notification and no Mailpit email |
| UAT-EM-06 | JT-9, JT-38, JT-73 | Open the draft at `/employer/jobs/:jobId`, publish it; check `/employer/jobs` | Job Published and listed in My jobs. Editing the title of the published job is saved | Pass |
| UAT-EM-07 | JT-11, JT-12, JT-78 | After the applicant applies (UAT-AP-07), open `/employer/applications` and `/employer/notifications` | The application is listed with links to Status, Interviews and Review; a notification about the new application | Fail, JT-105. Application listed with Status, Interviews and Review links, but no notification |
| UAT-EM-08 | JT-12, JT-51, JT-79, JT-81, JT-83 | Open `/employer/applications/:applicationId/review`, give score 4 and decision Shortlisted, confirm | Asked for confirmation; review saved and cannot be changed; status page shows Submitted, Under review, Shortlisted in history | Pass. Note: the employer cannot view the applicant's resume during review; known open question in AGENTS.md Section 16 (GetResume and DownloadResume are owner-only), not logged as a bug |
| UAT-EM-09 | JT-12, JT-52, JT-80, JT-81, JT-83 | Open `/employer/applications/:applicationId/interviews`, schedule an interview for tomorrow with details | Interview listed in local time; status becomes Interview. A time in the past is refused before sending | Pass. Past time refused; status changed to Interview |
| UAT-EM-10 | JT-12, messaging | Open `/employer/messages/:applicationId`, send a message to the applicant | Message shown in the thread; applicant can see it (UAT-AP-10) | Pass. Message shown in the thread with time and sender; applicant saw it and replied (AP-10) |
| UAT-EM-11 | JT-12, JT-45, JT-74, JT-81 | Open `/employer/offers/new/:applicationId`, enter offer terms, send; then try to make a second offer for the same application | Offer listed as Pending in `/employer/offers`; status becomes Offered; the second offer is refused | Pass. Offer made, status Offered, second offer refused (offer already exists). Note: the offer ID is shown only once after creation and cannot be found again (no list RPC) (JT-108) |

### Applicant

| ID | Requirement | Steps | Expected result | Result |
|---|---|---|---|---|
| UAT-AP-01 | JT-8, JT-30, JT-14 | Register as an applicant at `/register`, log in | Applicant workspace `/applicant` opens with its navigation | Pass |
| UAT-AP-02 | JT-9, JT-76 | Before the employer publishes: open `/applicant/job-alerts`, create an alert for the job's category or location | Alert listed. An alert with all filters empty is refused | Pass. Alert with location Sarpang created; with all filters Any the Create button is disabled. Alert before publishing not run: created after the job was published |
| UAT-AP-03 | JT-9, JT-69, JT-73 | After UAT-EM-06, open `/applicant/jobs`, search for the job, open `/applicant/jobs/:jobId` | Job found and its details shown; a draft job of the employer is not found | Pass. Job found in search; details open with View job |
| UAT-AP-04 | JT-9, JT-73, JT-70 | Save the job, open `/applicant/saved-jobs`, then remove it and save it again | Job listed in saved jobs; removing works. Note: closing a saved job is not reflected until JT-70 | Pass. Job saved and listed; Unsave available |
| UAT-AP-05 | JT-13, JT-71, JT-76 | Open `/applicant/notifications`; check Mailpit | A job alert notification for the published job | Not run. No alert existed when the job was published; notifications also fail because of the JT-105 |
| UAT-AP-06 | JT-11, JT-50, JT-83 | Open `/applicant/profile`, create a profile with summary, one education entry and three skills; edit it | Profile saved and shown after reload. A duplicate skill (same name, different case) is refused | Pass. Profile saved with summary, education and skills; a duplicate skill shows a warning and disables Save. Editing and saving again works as expected |
| UAT-AP-07 | JT-11, JT-48, JT-59, JT-83 | Open `/applicant/resumes`, upload a real PDF under 2 MB; wait and reload. Then try a `.txt` file and a PDF over 2 MB | The PDF goes from Pending to Validated; the other two are refused with a clear message | Fail, JT-107. Synthetic PDF Validated; .txt and over-2 MB files refused. A real 0.49 MB PDF that uses a cross-reference stream (Word style, 2 %%EOF) was Rejected: invalid PDF cross-reference count |
| UAT-AP-08 | JT-11, JT-58, JT-78 | Open `/applicant/apply`, apply to the job with the validated resume; open `/applicant/applications` | Application listed as Submitted; a notification and an email confirm it | Pass. Application listed (Submitted submission record); job page has Apply for this job with the job ID filled in. Notification part fails (JT-105) |
| UAT-AP-09 | JT-12, JT-49, JT-81, JT-83 | After UAT-EM-08 and UAT-EM-09, open `/applicant/applications/:applicationId/status` and `.../interviews` | History shows each step in order; the interview time is shown in local time | Pass. History in order from Submitted to Accepted; interview shown in local time |
| UAT-AP-10 | JT-12, messaging | Open `/applicant/messages/:applicationId`, read the employer's message and reply | Both messages shown, oldest first; the employer sees the reply | Pass. Both employer messages shown oldest first; reply sent |
| UAT-AP-11 | JT-12, JT-45, JT-81 | After UAT-EM-11, open `/applicant/offers`, accept the offer; try to decline it afterwards | Offer Accepted; status Accepted; declining afterwards is refused | Pass. Offer accepted (offer ID shared by the employer in Messages, JT-108); no decline option afterwards; status Accepted |
| UAT-AP-12 | JT-11, JT-58 | Try to apply to the same job again, and to a closed job | Both refused with a clear message | Pass. Second application refused: You have already applied for this job. Closed-job case not run: no closed job yet |

### Admin

| ID | Requirement | Steps | Expected result | Result |
|---|---|---|---|---|
| UAT-AD-01 | JT-8, JT-30 | Log in with the seeded admin account; try to register a new admin at `/register` | Admin workspace `/admin` opens; registration offers no admin role | Pass. Seeded admin login works; registration offers no admin role |
| UAT-AD-02 | JT-8, JT-33, JT-88 | Open `/admin/verification`, view the pending request and the company details, approve it | Asked for confirmation; request Approved and leaves the pending list | Pass. Request listed and removed from the pending list after approval |
| UAT-AD-03 | JT-8, JT-33, JT-88 | Have a second employer submit a request; reject it with a reason | Request Rejected; the employer sees the reason on `/employer/verification` | Pass. Request rejected with reason; employer sees Rejected and the reason |
| UAT-AD-04 | JT-10, JT-34, JT-88 | As the applicant, report a second published job at `/applicant/report-job/:jobId`; as admin open `/admin/moderation` and dismiss the report | Report listed as Open, then Dismissed | Pass. Report changed from Open to Dismissed. Note: the dismissed report was on Frontend Developer, not UAT job 2 |
| UAT-AD-05 | JT-10, JT-34, JT-87, JT-90, JT-69 | Report the second job again and remove it with a reason | Report Actioned; the job no longer appears in `/applicant/jobs` search and cannot be applied to | Pass. Job removed; open reports marked actioned; UAT job 2 no longer in applicant search. Note: before publishing, the draft job showed an empty details page to the applicant, which is correct (drafts are hidden, JT-38) |
| UAT-AD-06 | JT-10, JT-36, JT-85, JT-88 | Open `/admin/analytics`, choose today as From and To | Counts include the published job, the submitted application and the accepted offer | Pass. Today: 2 jobs published, 1 closed (after the AD-05 removal), 1 application submitted, 1 offer made and accepted, one of each status change |
| UAT-AD-07 | JT-10, JT-35, JT-72 | Open `/admin/audit-log`, filter by today | Entries for today's events, newest first. Not run if JT-72 is not merged | Not run. Audit log empty (0 entries): JT-72 audit-log consumer not merged; RabbitMQ has no audit-log queue. Known gap, not a bug |
| UAT-AD-08 | JT-13, JT-86 | Trigger the weekly summary: `docker compose -f deploy/compose/docker-compose.yml --profile functions exec weekly-summary wget -S -O- --post-data='' http://localhost:8080/`; check Mailpit | One summary email to the admin with the last 7 days' counts | Pass. One weekly summary email to admin@jobtrack.test in Mailpit |

### Access control (all roles)

| ID | Requirement | Steps | Expected result | Result |
|---|---|---|---|---|
| UAT-X-01 | JT-14, JT-31 | As the applicant, type `/employer/jobs/new` and `/admin/verification` in the address bar | Redirected to the applicant workspace; no employer or admin data shown | Pass. As the applicant, in-app navigation to /employer/jobs/new and /admin/verification redirected to /applicant. Also as the employer: /employer/jobs/new opened and /admin/verification redirected to /employer. Note: typing the URL reloads and logs out first (JT-106) |
| UAT-X-02 | JT-14, JT-31 | Log out, then open `/applicant/applications` | Sent to login | Pass. After logging out, the applications page sends to login |
| UAT-X-03 | JT-12, JT-49 | As a second applicant, open the first applicant's `/applicant/applications/:applicationId/status` | Refused with a clear message; no status shown | Pass. Second applicant got 403 Forbidden and the message: Only the applicant and the employer of this application can see its status. No history shown |

## Sessions

Each session is run by someone who did not build the features in that role's rows.

| Role | Tester | Did not build | Date | Commit | Scenarios run | Passed | Failed | Not run |
|---|---|---|---|---|---|---|---|---|
| Employer | Kinley Tobgay Lhendrup (EM-01 to EM-07), Phuntsho Namgyel (EM-08, EM-09), Jigden Shakya (EM-10, EM-11) | employer, company-verification, job-posting; review and interview tested by Phuntsho; messaging and offer tested by Jigden | 19 Nov 2026 | e85ab76 | 11 | 9 | 2 | 0 |
| Applicant | Jigden Shakya | applicant-profile, resume, application, application-status, offer, messaging, job-alert (Team A services), JT-73 job pages and JT-71 notifications | 19 Nov 2026 | e85ab76 | 15 (AP-01 to AP-12, X-01 to X-03) | 13 | 1 | 1 |
| Admin | Phuntsho Namgyel | company-verification, moderation, analytics, audit-log (Team B services) | 19 Nov 2026 | e85ab76 | 8 | 7 | 0 | 1 |

Session notes: code under test is main at c42d5ab (the branch adds only this document). Stack built one image at a time because a parallel build ran out of memory on the host.

## Bugs found

Each failure is logged as a Jira Bug with a severity label (`severity-critical`, `severity-major`, `severity-minor` or `severity-trivial`, see [test-plan.md](test-plan.md)), steps to reproduce, and assigned to the owner of the service in AGENTS.md Section 4.

| Bug | Scenario | Summary | Severity | Owner | Status |
|---|---|---|---|---|---|
| JT-105 | UAT-EM-05, UAT-EM-07, UAT-AP-08 | notification-dispatcher receives no events in Docker Compose (no forwarder or queue) | Major | Sangay Tenzin | To Do |
| JT-106 | UAT-EM-02, UAT-X-01, UAT-X-03 (notes) | Frontend logs the user out on every page reload | Minor | Phuntsho Namgyel | To Do |
| JT-107 | UAT-AP-07 | resume-validator rejects valid PDFs that use cross-reference streams | Major | Phuntsho Namgyel | To Do |
| JT-108 | UAT-EM-11, UAT-AP-11 (notes) | Offer ID shown only once and cannot be found again | Major | Phuntsho Namgyel | To Do |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence |
|---|---|---|
| AC1: UAT script with scenarios per role, traced, with expected results | Met | Scenarios: 34 scenarios for employer, applicant, admin and access, each traced to its Epic and implementing task, with expected results |
| AC2: At least one session per role on the full Compose stack, by a non-builder, results recorded | Met | Sessions: one session per role on 19 Nov 2026 on the full Compose stack (--profile functions), each by a tester who did not build those features; 34 run, 29 Pass, 3 Fail, 2 Not run |
| AC3: Every failure logged as a Jira Bug with severity and steps, assigned to the owner | Met | Bugs found: JT-105 to JT-108 logged as Jira Bugs with severity labels and steps to reproduce, assigned to the service owners and linked to JT-99 |
| AC4: UAT sign-off by the Product Owner recorded | Pending | Sign-off |

## Sign-off

| Product Owner | Decision | Date | Conditions |
|---|---|---|---|
| Phuntsho Namgyel | <Accepted / Accepted with conditions / Not accepted> | <date> | <open bugs agreed, if any> |
