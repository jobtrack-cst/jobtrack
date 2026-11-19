# JobTrack: Sprint 4 Review and Retrospective

| Item | Detail |
|---|---|
| Jira task | JT-100 |
| Owner | Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup |
| Sprint | Sprint 4 (Jira sprint 38), the last sprint |
| Sprint dates | 17 Nov 09:00 to 20 Nov 23:59 (extended by one day on 19 Nov; planned end 19 Nov 23:59, AGENTS.md Section 15) |
| Closed in Jira | TBD (20 Nov) |
| Review and retrospective | TBD (20 Nov): date, format and who attended |
| Data source | Jira sprint 38 and issue history; draft figures checked on 19 Nov 2026 at 20:56, final figures TBD (20 Nov) |
| Related documents | `docs/pm/sprint-3-review.md`, `docs/pm/sprint-4-planning.md`, `docs/pm/risk-register.md`, `docs/pm/project-closure-report.md`, `docs/testing/uat-script.md`, `docs/testing/test-summary-report.md`, AGENTS.md (Sections 14 and 15) |

---

## 1. Sprint goal and outcome

Goal: End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze. Committed minimum: the Must tasks (39 points).

Outcome: TBD (20 Nov). On 19 Nov at 20:56, 34 of 39 Must points were Done; the open Musts were JT-97 (final test summary report) and JT-100 (this closure work).

What was delivered by 19 Nov:

- The last function, notification-dispatcher (JT-71), and all 5 functions deployed to Knative in k3d with their triggers verified (JT-92).
- The job-search read model (JT-69), `ListJobSavers` pagination and internal interceptor (JT-91), and `job.closed` on moderation removal (JT-90).
- The frontend job pages, job search, saved jobs and notifications (JT-73), so all three roles can complete the main workflow in the UI.
- Prometheus and Grafana in Docker Compose (JT-93) and the architecture and deployment documentation (JT-94).
- Cypress end-to-end tests of the main workflow against the running Compose stack, replacing the `cy.intercept` stubs (JT-95).
- The JMeter deadline-rush load test with 200 concurrent users (JT-96, report in `docs/testing/jt-96-load-test-report.md`).
- UAT for applicant, employer and admin (JT-99): 34 scenarios, 29 Pass, 3 Fail, 2 Not run; the core workflow was verified end to end with no Critical defects (Section 4).

Not delivered: TBD (20 Nov). Following the agreed rule, every Should task still open at 20 Nov 23:59 is recorded in Section 3 as not delivered, not moved, because there is no Sprint 5.

---

## 2. Results

Draft on 19 Nov at 20:56. Final column TBD (20 Nov).

| Measure | Planned | Done on 19 Nov | Done at close |
|---|---|---|---|
| Tasks (estimated) | 22 | 14 | TBD |
| Story points | 57 | 37 | TBD |
| Must (High): tasks / points | 14 / 39 | 12 / 34 | TBD |
| Should (Medium): tasks / points | 8 / 18 | 2 / 3 | TBD |

- Velocity (story points Done in the sprint): TBD, compared with 43 in Sprint 3, 60 in Sprint 2 (over 5 days) and 49 in Sprint 1.
- Velocity per day: TBD over 4 days (17 to 20 Nov), compared with about 14 in Sprint 3, 12 in Sprint 2 and 16 in Sprint 1.
- R1 day 2 check (18 Nov, 17:00): 2 of 57 points Done or In Review, below the trigger of 29, so the trigger fired.
- Midday 19 Nov: 22 of 39 Must points Done, with JT-92, JT-99 and JT-100 open. This led to the one-day extension agreed by Phuntsho and Jigden (JT-67).
- Bugs added during the sprint without points: JT-104 (Done), JT-105 and JT-107 (conditions of the UAT sign-off, Section 4). They are not part of the 57 points.

### By member

By Jira assignee. Draft on 19 Nov at 20:56; final figures TBD (20 Nov).

| Member | Planned points | Done tasks | Done points | Open on 19 Nov |
|---|---|---|---|---|
| Phuntsho Namgyel | 14 | 3 of 5 (JT-73, JT-93, JT-94) | 8 | JT-66 (3), JT-101 (3); bug JT-107 |
| Kinley Tobgay Lhendrup | 15 | 3 of 5 (JT-84, JT-95, JT-96), plus bug JT-104 | 10 | JT-97 (2), JT-103 (3) |
| Jigden Shakya | 16 | 4 of 8 (JT-67, JT-90, JT-98, JT-99) | 7 | JT-70 (1), JT-72 (2), JT-100 (3), JT-102 (3, In Review) |
| Sangay Tenzin | 12 | 4 of 4 (JT-69, JT-71, JT-91, JT-92) | 12 | Bug JT-105 |
| Total | 57 | 14 of 22 | 37 | 8 tasks, 20 points |

---

## 3. Task outcomes

Owner is the Jira assignee. "Status on 19 Nov" is from Jira at 20:56. "Result at close" is TBD (20 Nov): Done, or Not delivered for an unfinished Should task.

| Key | Summary | Owner | Priority | Points | Status on 19 Nov | Result at close |
|---|---|---|---|---|---|---|
| JT-66 | Decide and implement error logging for unexpected gRPC errors (carried over) | Phuntsho | Should | 3 | To Do | TBD |
| JT-67 | Align project charter and sprint calendar with the revised schedule (carried over) | Jigden | Should | 1 | Done | Done |
| JT-69 | Job-search: build the read model from job events (carried over) | Sangay | Must | 2 | Done | Done |
| JT-70 | Saved-jobs: consume job.closed (carried over) | Jigden | Should | 1 | To Do | TBD |
| JT-71 | Implement notification-dispatcher Knative function (carried over) | Sangay | Must | 5 | Done | Done (event delivery in Compose depends on bug JT-105) |
| JT-72 | Audit-log: consume all domain events (carried over) | Jigden | Should | 2 | To Do | TBD |
| JT-73 | Frontend: employer job pages, job search, saved jobs and notifications (carried over) | Phuntsho | Must | 3 | Done | Done |
| JT-84 | Sprint 3 SonarQube run and JMeter deadline-rush scenario preparation (carried over) | Kinley | Should | 2 | Done | Done |
| JT-90 | job-posting: removing a job through moderation does not publish job.closed (bug) | Jigden | Must | 2 | Done | Done |
| JT-91 | Saved-jobs: ListJobSavers pagination and internal interceptor | Sangay | Must | 2 | Done | Done |
| JT-92 | Knative: deploy all 5 functions to k3d and verify their triggers | Sangay | Must | 3 | Done | Done |
| JT-93 | Monitoring: Prometheus and Grafana in Docker Compose | Phuntsho | Must | 3 | Done | Done |
| JT-94 | Architecture and deployment documentation | Phuntsho | Must | 2 | Done | Done |
| JT-95 | Cypress: end-to-end main workflow against the running Compose stack | Kinley | Must | 5 | Done | Done |
| JT-96 | JMeter: run the deadline-rush load test and report results | Kinley | Must | 3 | Done | Done |
| JT-97 | Final test summary report: test counts, coverage, SonarQube and defects | Kinley | Must | 2 | In Progress | TBD |
| JT-98 | Sprint 3 review, Sprint 4 planning record and AGENTS.md update | Jigden | Must | 2 | Done | Done |
| JT-99 | UAT: test script and sessions for applicant, employer and admin | Jigden | Must | 2 | Done | Done |
| JT-100 | Final sprint review and retrospective, earned value and project closure report | Jigden | Must | 3 | In Progress | TBD |
| JT-101 | Frontend polish: MUI theme, shared layout and consistent page styling | Phuntsho | Should | 3 | To Do | TBD |
| JT-102 | Employer: company logo upload and display | Jigden | Should | 3 | In Review | TBD |
| JT-103 | Applicant-profile: profile photo upload and display | Kinley | Should | 3 | To Do | TBD |

### Bugs in the sprint (unestimated)

| Key | Summary | Owner | Severity | Status on 19 Nov | Result at close |
|---|---|---|---|---|---|
| JT-104 | resume page: file name check uses a control character in a regular expression (SonarQube S6324) | Kinley | TBD (from JT-97 defect log) | Done | Done |
| JT-105 | notification-dispatcher receives no events in Docker Compose (no forwarder or `notification-dispatcher.events` queue) | Sangay | Major | In Progress | TBD (retest EM-05, EM-07) |
| JT-107 | resume-validator rejects valid PDFs that use cross-reference streams | Phuntsho | Major | To Do | TBD (retest AP-07) |

### Not delivered

TBD (20 Nov). Every Should task not Done at 20 Nov 23:59, with its points. It is not moved to a later sprint, because there is no Sprint 5 (Section 15, 18 Nov 2026). Slip order as agreed: JT-66 first, then JT-102 and JT-103, then the other Should tasks; among Jigden's tasks, JT-72 before JT-70.

| Key | Summary | Owner | Points | Reason |
|---|---|---|---|---|
| TBD | | | | |

### Work outside the sprint

- JT-106 (Minor, login lost on page reload) and JT-108 (Major, offer ID shown only once) were found in UAT and are documented as known issues (Section 4). They were not added to the sprint.
- JT-109 (Low, applicant profile read-only view with an Edit button) was raised in UAT as a future improvement for after the project. TBD (20 Nov): confirm it is in the backlog and not counted in the sprint.

---

## 4. UAT and known issues

UAT (JT-99, 19 Nov 2026, `docs/testing/uat-script.md`, PR #88): 34 scenarios, 29 Pass, 3 Fail, 2 Not run. The core workflow from employer registration to the accepted offer was verified end to end, and there were no Critical defects.

Not run:

- UAT-AP-05: no job alert existed before the job was published, so the alert match could not be observed.
- UAT-AD-07: the audit log stays empty until the audit-log consumer (JT-72) is merged.

Product Owner sign-off (Phuntsho, 19 Nov 2026): Accepted with conditions.

| Condition | Bug | Owner | Due | Retest | Result |
|---|---|---|---|---|---|
| Fix and retest | JT-105 (Major): notification-dispatcher gets no events in Compose | Sangay | 20 Nov 23:59 | UAT-EM-05, UAT-EM-07 (Jigden) | TBD |
| Fix and retest | JT-107 (Major): resume-validator rejects PDFs with cross-reference streams | Phuntsho | 20 Nov 23:59 | UAT-AP-07 (Jigden) | TBD |

TBD (20 Nov): any condition not met by the deadline is disclosed here and in the closure report.

Known issues accepted at closure:

| Bug | Severity | Issue | Workaround |
|---|---|---|---|
| JT-108 | Major | The offer ID is shown only once, right after the employer makes the offer | The employer copies the offer ID shown after making the offer and sends it to the applicant in Messages |
| JT-106 | Minor | The login is lost when the page is reloaded | Log in again after a reload |

Future improvements (not part of this project):

- JT-109 (Low, Kinley): show the saved applicant profile read-only with an Edit button. Backlog after the project.
- Employer access to applicant resumes during review (AGENTS.md Section 16 open question). Approved for backlog consideration only; no Jira issue yet. Needs Phuntsho's authorisation and a contract review.

---

## 5. What went well

- Every Sprint 3 carryover Must was finished: JT-69, JT-71 and JT-73. Sangay finished all four of his Must tasks (12 points), including the deployment critical path JT-71 then JT-92.
- All 5 functions now run on Knative in k3d with their triggers verified (JT-92), meeting the agreed WEB303 deployment scope.
- The Sprint 3 testing gap was closed: Cypress now runs the main workflow against the real Compose stack (JT-95), the 200-user JMeter run is reported (JT-96), and monitoring is in place (JT-93).
- UAT with the Product Owner verified the core workflow end to end with no Critical defects, and every defect found was logged in Jira with severity and steps to reproduce.
- The slip order agreed on 18 Nov was clear, so the Should tasks could be dropped without discussion while the Musts were protected.
- TBD (20 Nov): add points from the team retrospective.

---

## 6. What to change

These are the final retrospective points. Because there is no next sprint, they are carried into the lessons learned in `docs/pm/project-closure-report.md`.

- The day 2 check fired hard: only 2 of 57 points were Done or In Review on 18 Nov, and the sprint still needed a one-day extension. Most Sprint 4 tasks were testing and documentation that depend on finished features, so they could only start late.
- "Done" did not always mean working in Docker Compose. JT-71 was Done, but UAT found that notification-dispatcher receives no events in Compose (JT-105). The Sprint 3 action to verify each event end to end in Compose before Done was not applied to it.
- Bugs still entered the sprint without estimates (JT-104, JT-105, JT-107), so their work is not visible in the points.
- The full parallel Docker Compose build ran out of memory on a laptop with about 7.75 GiB for Docker. Building one image at a time worked but took about 16 minutes, which slowed testing and UAT.
- The databases still held test data from earlier tasks during UAT. Testing should start from a known, clean data set.
- TBD (20 Nov): add points from the team retrospective.

---

## 7. Follow-up of the Sprint 3 actions

| Sprint 3 action | Result |
|---|---|
| Reassign the carryovers: JT-73 to Phuntsho, JT-72 and JT-70 to Jigden, with Sangay reviewing | Done at Sprint 4 planning on 17 Nov. JT-73 was Done on 19 Nov; JT-70 and JT-72: TBD |
| Check every Must task at each stand-up and reassign through the handover rule the same day | TBD (20 Nov): confirm whether any Must was reassigned. All of Sangay's Musts were Done by 19 Nov evening; the remaining Musts were extended by one day instead |
| Follow the slip order: JT-66, then JT-102 and JT-103, then other Should tasks; never cut a Must | TBD (20 Nov) |
| Replace the stubbed Cypress tests with real end-to-end tests against Docker Compose (JT-95) | Done |
| Add Prometheus and Grafana to Docker Compose (JT-93) | Done |
| Record unfinished Should tasks as not delivered in the closure report (JT-100) | Done in Section 3 and in the closure report |
| Estimate every task before it is added to the sprint, including bugs | Not met: JT-104, JT-105 and JT-107 have no points |
| Verify each new event end to end in Docker Compose before moving its task to Done | Not met for JT-71 (found in UAT as JT-105) |

---

## 8. Risks

- R1 (schedule overrun): occurred. The day 2 trigger fired on 18 Nov and Sprint 4 needed a one-day extension. Final result TBD (20 Nov).
- R6 (uneven workload): the reassignment on 17 Nov worked for the Must path; Sangay finished all his Musts.
- R2 (Knative and RabbitMQ): all 5 functions were deployed and their triggers verified in k3d (JT-92). Event delivery to notification-dispatcher in Docker Compose is open as JT-105.
- R8 (load test): the 200-user run was completed and reported (JT-96).
- All risks are closed out in `docs/pm/risk-register.md`, version 1.6.

---

## 9. Open items at close

TBD (20 Nov): anything still open when the sprint closes, for example JT-105 or JT-107 if a condition is not met.
