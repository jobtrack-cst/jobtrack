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

1. Employer: UAT-EM-01 to UAT-EM-03 (register, company profile, request verification)
2. Admin: UAT-AD-01 to UAT-AD-02 (approve the employer)
3. Employer: UAT-EM-04 to UAT-EM-06 (publish a job)
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
| UAT-EM-01 | JT-8, JT-30, JT-14 | Open `/register`, register as an employer, then log in at `/login` | Account created; after login the employer workspace `/employer` opens with its navigation | |
| UAT-EM-02 | JT-8, JT-32, JT-88 | Open `/employer/company`, create a company profile with a name, description, website (`https://...`) and location; then edit the description | Profile saved and shown with status Unverified; the edit is shown after reload. An invalid website such as `example` is refused before sending | |
| UAT-EM-03 | JT-8, JT-33, JT-88 | Open `/employer/verification`, submit a request with a registration number and notes | Request listed as Pending. A second request while one is pending is refused with a clear message | |
| UAT-EM-04 | JT-9, JT-38, JT-57, JT-73 | Before the admin approves: open `/employer/jobs/new`, create a draft job and try to publish it | Draft saved; publishing is refused because the employer is not verified; the job stays Draft | |
| UAT-EM-05 | JT-8, JT-56 | After the admin approves (UAT-AD-02), reload `/employer/company` and `/employer/notifications`; check Mailpit | Status Verified; an in-app notification and an email about the verification | |
| UAT-EM-06 | JT-9, JT-38, JT-73 | Open the draft at `/employer/jobs/:jobId`, publish it; check `/employer/jobs` | Job Published and listed in My jobs. Editing the title of the published job is saved | |
| UAT-EM-07 | JT-11, JT-12, JT-78 | After the applicant applies (UAT-AP-07), open `/employer/applications` and `/employer/notifications` | The application is listed with links to Status, Interviews and Review; a notification about the new application | |
| UAT-EM-08 | JT-12, JT-51, JT-79, JT-81, JT-83 | Open `/employer/applications/:applicationId/review`, give score 4 and decision Shortlisted, confirm | Asked for confirmation; review saved and cannot be changed; status page shows Submitted, Under review, Shortlisted in history | |
| UAT-EM-09 | JT-12, JT-52, JT-80, JT-81, JT-83 | Open `/employer/applications/:applicationId/interviews`, schedule an interview for tomorrow with details | Interview listed in local time; status becomes Interview. A time in the past is refused before sending | |
| UAT-EM-10 | JT-12, messaging | Open `/employer/messages/:applicationId`, send a message to the applicant | Message shown in the thread; applicant can see it (UAT-AP-10) | |
| UAT-EM-11 | JT-12, JT-45, JT-74, JT-81 | Open `/employer/offers/new/:applicationId`, enter offer terms, send; then try to make a second offer for the same application | Offer listed as Pending in `/employer/offers`; status becomes Offered; the second offer is refused | |

### Applicant

| ID | Requirement | Steps | Expected result | Result |
|---|---|---|---|---|
| UAT-AP-01 | JT-8, JT-30, JT-14 | Register as an applicant at `/register`, log in | Applicant workspace `/applicant` opens with its navigation | |
| UAT-AP-02 | JT-9, JT-76 | Before the employer publishes: open `/applicant/job-alerts`, create an alert for the job's category or location | Alert listed. An alert with all filters empty is refused | |
| UAT-AP-03 | JT-9, JT-69, JT-73 | After UAT-EM-06, open `/applicant/jobs`, search for the job, open `/applicant/jobs/:jobId` | Job found and its details shown; a draft job of the employer is not found | |
| UAT-AP-04 | JT-9, JT-73, JT-70 | Save the job, open `/applicant/saved-jobs`, then remove it and save it again | Job listed in saved jobs; removing works. Note: closing a saved job is not reflected until JT-70 | |
| UAT-AP-05 | JT-13, JT-71, JT-76 | Open `/applicant/notifications`; check Mailpit | A job alert notification for the published job | |
| UAT-AP-06 | JT-11, JT-50, JT-83 | Open `/applicant/profile`, create a profile with summary, one education entry and three skills; edit it | Profile saved and shown after reload. A duplicate skill (same name, different case) is refused | |
| UAT-AP-07 | JT-11, JT-48, JT-59, JT-83 | Open `/applicant/resumes`, upload a real PDF under 2 MB; wait and reload. Then try a `.txt` file and a PDF over 2 MB | The PDF goes from Pending to Validated; the other two are refused with a clear message | |
| UAT-AP-08 | JT-11, JT-58, JT-78 | Open `/applicant/apply`, apply to the job with the validated resume; open `/applicant/applications` | Application listed as Submitted; a notification and an email confirm it | |
| UAT-AP-09 | JT-12, JT-49, JT-81, JT-83 | After UAT-EM-08 and UAT-EM-09, open `/applicant/applications/:applicationId/status` and `.../interviews` | History shows each step in order; the interview time is shown in local time | |
| UAT-AP-10 | JT-12, messaging | Open `/applicant/messages/:applicationId`, read the employer's message and reply | Both messages shown, oldest first; the employer sees the reply | |
| UAT-AP-11 | JT-12, JT-45, JT-81 | After UAT-EM-11, open `/applicant/offers`, accept the offer; try to decline it afterwards | Offer Accepted; status Accepted; declining afterwards is refused | |
| UAT-AP-12 | JT-11, JT-58 | Try to apply to the same job again, and to a closed job | Both refused with a clear message | |

### Admin

| ID | Requirement | Steps | Expected result | Result |
|---|---|---|---|---|
| UAT-AD-01 | JT-8, JT-30 | Log in with the seeded admin account; try to register a new admin at `/register` | Admin workspace `/admin` opens; registration offers no admin role | |
| UAT-AD-02 | JT-8, JT-33, JT-88 | Open `/admin/verification`, view the pending request and the company details, approve it | Asked for confirmation; request Approved and leaves the pending list | |
| UAT-AD-03 | JT-8, JT-33, JT-88 | Have a second employer submit a request; reject it with a reason | Request Rejected; the employer sees the reason on `/employer/verification` | |
| UAT-AD-04 | JT-10, JT-34, JT-88 | As the applicant, report a second published job at `/applicant/report-job/:jobId`; as admin open `/admin/moderation` and dismiss the report | Report listed as Open, then Dismissed | |
| UAT-AD-05 | JT-10, JT-34, JT-87, JT-90, JT-69 | Report the second job again and remove it with a reason | Report Actioned; the job no longer appears in `/applicant/jobs` search and cannot be applied to | |
| UAT-AD-06 | JT-10, JT-36, JT-85, JT-88 | Open `/admin/analytics`, choose today as From and To | Counts include the published job, the submitted application and the accepted offer | |
| UAT-AD-07 | JT-10, JT-35, JT-72 | Open `/admin/audit-log`, filter by today | Entries for today's events, newest first. Not run if JT-72 is not merged | |
| UAT-AD-08 | JT-13, JT-86 | Trigger the weekly summary: `docker compose -f deploy/compose/docker-compose.yml --profile functions exec weekly-summary wget -S -O- --post-data='' http://localhost:8080/`; check Mailpit | One summary email to the admin with the last 7 days' counts | |

### Access control (all roles)

| ID | Requirement | Steps | Expected result | Result |
|---|---|---|---|---|
| UAT-X-01 | JT-14, JT-31 | As the applicant, type `/employer/jobs/new` and `/admin/verification` in the address bar | Redirected to the applicant workspace; no employer or admin data shown | |
| UAT-X-02 | JT-14, JT-31 | Log out, then open `/applicant/applications` | Sent to login | |
| UAT-X-03 | JT-12, JT-49 | As a second applicant, open the first applicant's `/applicant/applications/:applicationId/status` | Refused with a clear message; no status shown | |

## Sessions

Each session is run by someone who did not build the features in that role's rows.

| Role | Tester | Did not build | Date | Commit | Scenarios run | Passed | Failed | Not run |
|---|---|---|---|---|---|---|---|---|
| Employer | <name> | <services or tasks> | <date> | <commit> | | | | |
| Applicant | <name> | <services or tasks> | <date> | <commit> | | | | |
| Admin | <name> | <services or tasks> | <date> | <commit> | | | | |

Session notes: <anything unexpected that is not a bug, for example slow event updates>.

## Bugs found

Each failure is logged as a Jira Bug with a severity label (`severity-critical`, `severity-major`, `severity-minor` or `severity-trivial`, see [test-plan.md](test-plan.md)), steps to reproduce, and assigned to the owner of the service in AGENTS.md Section 4.

| Bug | Scenario | Summary | Severity | Owner | Status |
|---|---|---|---|---|---|
| | | | | | |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence |
|---|---|---|
| AC1: UAT script with scenarios per role, traced, with expected results | | This document, Scenarios |
| AC2: At least one session per role on the full Compose stack, by a non-builder, results recorded | | Sessions |
| AC3: Every failure logged as a Jira Bug with severity and steps, assigned to the owner | | Bugs found |
| AC4: UAT sign-off by the Product Owner recorded | | Sign-off |

## Sign-off

| Product Owner | Decision | Date | Conditions |
|---|---|---|---|
| Phuntsho Namgyel | <Accepted / Accepted with conditions / Not accepted> | <date> | <open bugs agreed, if any> |
