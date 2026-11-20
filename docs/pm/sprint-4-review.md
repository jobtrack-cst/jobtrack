# JobTrack: Sprint 4 Review and Retrospective

| Item | Detail |
|---|---|
| Jira task | JT-100 |
| Owner | Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup |
| Sprint | Sprint 4 (Jira sprint 38), the last sprint |
| Sprint dates | 17 Nov 09:00 to 20 Nov 23:59 (extended by one day on 19 Nov; planned end 19 Nov 23:59, AGENTS.md Section 15) |
| Closed in Jira | After the JT-100 pull request merges, before 20 Nov 23:59 |
| Review and retrospective | Not held as a separate meeting on the last day. Written by Jigden from Jira, UAT and the earlier retrospectives. Product Owner review: Phuntsho reviewed PR #101 on 20 Nov and requested corrections; approval pending |
| Data dates | Jira sprint 38 and issue history read on 20 Nov 2026 at 19:10, when JT-100 was In Review. UAT on 19 Nov 2026, retests on 20 Nov 2026. Test results: end-to-end (JT-95) completed on 18 Nov, load test (JT-96) and test summary (JT-97) on 19 Nov |
| Related documents | `docs/pm/sprint-3-review.md`, `docs/pm/sprint-4-planning.md`, `docs/pm/earned-value-report.md`, `docs/pm/risk-register.md` (version 1.6), `docs/pm/project-closure-report.md`, `docs/testing/uat-script.md`, `docs/testing/test-summary-report.md`, AGENTS.md (Sections 14 and 15) |

---

## 1. Sprint goal and outcome

Goal: End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze. Committed minimum: the Must tasks (39 points).

Outcome: met, pending the merge of JT-100. At the snapshot 21 of 22 tasks and 54 of 57 points were Done: 13 of the 14 Must tasks (36 of 39 points) and all 8 Should tasks (18 points). The open task is JT-100 (Must, 3 points), which includes this review; it is In Review in PR #101 and is Done when that merges. Projected at the merge: all 22 tasks and 57 points, the only sprint that finishes all of its planned work. The goal was met only with the one-day extension agreed on 19 Nov.

What was delivered:

- The last function, notification-dispatcher (JT-71), and all 5 functions deployed to Knative in k3d with their triggers verified (JT-92). Its Compose forwarder and queue were added by the JT-105 fix.
- The audit-log consumer for all domain events (JT-72), the job-search read model (JT-69), the saved-jobs `job.closed` consumer (JT-70), `ListJobSavers` pagination and the internal interceptor (JT-91), and `job.closed` on moderation removal (JT-90).
- The frontend job pages, job search, saved jobs and notifications (JT-73), the frontend polish with an MUI theme and shared layout (JT-101), the company logo on the company page and job cards (JT-102) and the applicant profile photo (JT-103).
- Logging of unexpected gRPC errors through `pkg/logging.GRPCError` (JT-66).
- Prometheus and Grafana in Docker Compose (JT-93), and the architecture and deployment documentation (JT-94).
- Cypress end-to-end tests of the main workflow against the running Compose stack (JT-95), the JMeter run with 200 concurrent users (JT-96) and the final test summary report (JT-97).
- UAT for applicant, employer and admin (JT-99), and fixes for all four bugs it found (JT-105 to JT-108).

Not delivered: nothing is recorded as not delivered. JT-72 (audit-log consumer), last in the slip order, was started on the evening of 20 Nov while JT-100 was in review and merged at 19:07 (PR #102); it is the last task completed before this review. JT-100 itself is the final item and completes when PR #101 merges.

---

## 2. Results

| Measure | Tasks (snapshot) | Points (snapshot) | Tasks (projected, JT-100 merged) | Points (projected) |
|---|---|---|---|---|
| Planned | 22 | 57 | 22 | 57 |
| Done | 21 | 54 | 22 | 57 |
| In Review | 1 (JT-100) | 3 | 0 | 0 |
| Not delivered | 0 | 0 | 0 | 0 |
| Must (High): planned / done | 14 / 13 | 39 / 36 | 14 / 14 | 39 / 39 |
| Should (Medium): planned / done | 8 / 8 | 18 / 18 | 8 / 8 | 18 / 18 |

- Velocity (story points Done in the sprint): 54 at the snapshot (57 when JT-100 merges), compared with 43 in Sprint 3, 60 in Sprint 2 (over 5 days), 49 in Sprint 1 and 35 in Sprint 0.
- Velocity per day: about 14 over 4 days (17 to 20 Nov), about the same as Sprint 3.
- Completion: 21 of 22 tasks and 54 of 57 points at the snapshot; 22 of 22 and 57 of 57 projected when JT-100 merges.
- Hours logged in the sprint: see `docs/pm/earned-value-report.md` Section 3. Sprint 4 had the highest hours per point of any sprint, because it was mostly testing, UAT and documentation and included bug fixes without points.
- R1 day 2 check (18 Nov, 17:00): 2 of 57 points Done or In Review, below the trigger of 29, so the trigger fired. At midday on 19 Nov 22 of 39 Must points were Done, which led to the one-day extension agreed by Phuntsho and Jigden (JT-67).
- Work without points in the sprint: JT-104 (SonarQube finding) and the UAT bugs JT-105 to JT-108. Their hours count as actual cost but they add no points.

### By member

By Jira assignee on 20 Nov at 19:10.

| Member | Planned points | Done tasks | Done points | Not delivered | Bugs fixed (no points) |
|---|---|---|---|---|---|
| Phuntsho Namgyel | 14 | 5 of 5 (JT-66, JT-73, JT-93, JT-94, JT-101) | 14 | | JT-106, JT-107, JT-108 |
| Kinley Tobgay Lhendrup | 15 | 5 of 5 (JT-84, JT-95, JT-96, JT-97, JT-103) | 15 | | JT-104 |
| Jigden Shakya | 16 | 7 of 8 (JT-67, JT-70, JT-72, JT-90, JT-98, JT-99, JT-102); JT-100 (3) In Review | 13 (16 when JT-100 merges) | | |
| Sangay Tenzin | 12 | 4 of 4 (JT-69, JT-71, JT-91, JT-92) | 12 | | JT-105 |
| Total | 57 | 21 of 22 (JT-100 In Review) | 54 (57 projected) | 0 | 5 bugs |

---

## 3. Task outcomes

Owner is the Jira assignee. Status as of 20 Nov 2026, 19:10.

| Key | Summary | Owner | Priority | Points | Result |
|---|---|---|---|---|---|
| JT-66 | Decide and implement error logging for unexpected gRPC errors (carried over) | Phuntsho | Should | 3 | Done |
| JT-67 | Align project charter and sprint calendar with the revised schedule (carried over) | Jigden | Should | 1 | Done |
| JT-69 | Job-search: build the read model from job events (carried over) | Sangay | Must | 2 | Done |
| JT-70 | Saved-jobs: consume job.closed (carried over) | Jigden | Should | 1 | Done |
| JT-71 | Implement notification-dispatcher Knative function (carried over) | Sangay | Must | 5 | Done (Compose event delivery fixed in JT-105) |
| JT-72 | Audit-log: consume all domain events (carried over) | Jigden | Should | 2 | Done (PR #102, 20 Nov) |
| JT-73 | Frontend: employer job pages, job search, saved jobs and notifications (carried over) | Phuntsho | Must | 3 | Done |
| JT-84 | Sprint 3 SonarQube run and JMeter deadline-rush scenario preparation (carried over) | Kinley | Should | 2 | Done |
| JT-90 | job-posting: removing a job through moderation does not publish job.closed (bug) | Jigden | Must | 2 | Done |
| JT-91 | Saved-jobs: ListJobSavers pagination and internal interceptor | Sangay | Must | 2 | Done |
| JT-92 | Knative: deploy all 5 functions to k3d and verify their triggers | Sangay | Must | 3 | Done |
| JT-93 | Monitoring: Prometheus and Grafana in Docker Compose | Phuntsho | Must | 3 | Done |
| JT-94 | Architecture and deployment documentation | Phuntsho | Must | 2 | Done |
| JT-95 | Cypress: end-to-end main workflow against the running Compose stack | Kinley | Must | 5 | Done |
| JT-96 | JMeter: run the deadline-rush load test and report results | Kinley | Must | 3 | Done |
| JT-97 | Final test summary report: test counts, coverage, SonarQube and defects | Kinley | Must | 2 | Done |
| JT-98 | Sprint 3 review, Sprint 4 planning record and AGENTS.md update | Jigden | Must | 2 | Done |
| JT-99 | UAT: test script and sessions for applicant, employer and admin | Jigden | Must | 2 | Done |
| JT-100 | Final sprint review and retrospective, earned value and project closure report | Jigden | Must | 3 | In Review (PR #101); Done when it merges |
| JT-101 | Frontend polish: MUI theme, shared layout and consistent page styling | Phuntsho | Should | 3 | Done |
| JT-102 | Employer: company logo upload and display | Jigden | Should | 3 | Done (PR #71 and PR #100, including logos on job cards) |
| JT-103 | Applicant-profile: profile photo upload and display | Kinley | Should | 3 | Done |

### Bugs in the sprint (no points)

| Key | Summary | Owner | Severity | Found in | Result |
|---|---|---|---|---|---|
| JT-104 | resume page: file name check uses a control character in a regular expression (SonarQube S6324) | Kinley | Not from UAT (SonarQube rule S6324, Jira priority Medium) | SonarQube | Done |
| JT-105 | notification-dispatcher receives no events in Docker Compose (no forwarder or `notification-dispatcher.events` queue) | Sangay | Major | UAT | Done (PR #90); UAT-EM-05 and UAT-EM-07 passed on retest |
| JT-106 | Frontend logs the user out on every page reload (JWT kept only in memory) | Phuntsho | Minor | UAT | Done (session kept in `sessionStorage`, AGENTS.md Section 15, 19 Nov) |
| JT-107 | resume-validator rejects valid PDFs that use cross-reference streams | Phuntsho | Major | UAT | Done; UAT-AP-07 passed on retest |
| JT-108 | Offer ID is shown only once after making an offer and cannot be found again | Phuntsho | Major | UAT | Done (`GetOfferByApplication`, AGENTS.md Section 15, 19 Nov) |

### Not delivered

None. The slip order (JT-66 first, then JT-102 and JT-103, then the other Should tasks; among Jigden's, JT-72 before JT-67 and JT-70) was prepared but in the end no task had to be dropped. JT-100 is not dropped: it is In Review and completes when PR #101 merges.

### Work outside the sprint

- JT-109 (Low, applicant profile read-only view with an Edit button) was moved to the backlog on 19 Nov as an after-project improvement and was then done by Kinley the same evening. It is outside the sprint scope and has no points; its hours count as actual cost in the earned value report.

---

## 4. UAT and known issues

UAT (JT-99, sessions on 19 Nov 2026, `docs/testing/uat-script.md`, PR #88): 34 scenarios, 29 Pass, 3 Fail, 2 Not run. The core workflow from employer registration to the accepted offer was verified end to end, and there were no Critical defects.

Not run:

- UAT-AP-05: no job alert existed before the job was published, so the alert match could not be observed.
- UAT-AD-07: the audit log was empty because the audit-log consumer did not exist yet. JT-72 added it on 20 Nov and verified in Docker Compose that an admin sees a new event in the audit log; the UAT scenario itself was not rerun.

Product Owner sign-off (Phuntsho, 19 Nov 2026): Accepted with conditions. All conditions were met by 20 Nov 23:59.

| Condition | Bug | Owner | Due | Result |
|---|---|---|---|---|
| Fix and retest | JT-105 (Major): notification-dispatcher gets no events in Compose | Sangay | 20 Nov 23:59 | Fixed in PR #90 on 19 Nov; UAT-EM-05 and UAT-EM-07 passed on retest by Jigden on 20 Nov |
| Fix and retest | JT-107 (Major): resume-validator rejects PDFs with cross-reference streams | Phuntsho | 20 Nov 23:59 | Fixed on 19 Nov; UAT-AP-07 passed on retest by Jigden on 20 Nov |
| Accepted known issue, with a workaround | JT-108 (Major): offer ID shown only once | Phuntsho | | Fixed on 19 Nov instead (`GetOfferByApplication`), so the workaround is no longer needed |
| Documented as a Minor known issue | JT-106 (Minor): login lost on page reload | Phuntsho | | Fixed on 20 Nov (session kept in `sessionStorage` for the tab) |

Known issues at closure: none from UAT. The remaining limitations are listed in the closure report (Section 5.2) and AGENTS.md Section 16.

Future improvements (not part of this project):

- Employer access to applicant resumes during review (AGENTS.md Section 16 open question). Approved for backlog consideration only; no Jira issue yet. Needs Phuntsho's authorisation and a contract review.

---

## 5. What went well

- Every Must task except JT-100 (In Review) was finished, including all three Sprint 3 carryover Musts (JT-69, JT-71, JT-73). Sangay finished all four of his Must tasks, including the deployment path JT-71 then JT-92.
- All 8 Should tasks were delivered as well, so no points were dropped. Once JT-100 merges, Sprint 4 is the only sprint that finished all of its planned work.
- All 5 functions run on Knative in k3d with their triggers verified (JT-92), meeting the agreed WEB303 deployment scope.
- The Sprint 3 testing gap was closed: Cypress runs the main workflow against the real Compose stack (JT-95), the 200-user JMeter run finished with 0 errors (JT-96), monitoring is in place (JT-93), and `application` and `application-status` meet the 80 percent coverage target (JT-97).
- UAT with the Product Owner verified the core workflow, and all four bugs it found were fixed within about a day, including the two that were only expected to be documented.
- The slip order agreed on 18 Nov made clear which task would go first, so the team could finish the Must work and the closure documents before starting JT-72, the last Should task.

---

## 6. What to change

These are the final retrospective points. Because there is no next sprint, they are carried into the lessons learned in `docs/pm/project-closure-report.md`.

- The day 2 check fired hard: only 2 of 57 points were Done or In Review on 18 Nov, and the sprint still needed a one-day extension. Most Sprint 4 tasks were testing and documentation that depend on finished features, so they could only start late.
- "Done" did not always mean working in Docker Compose. JT-71 was Done, but UAT found that notification-dispatcher received no events in Compose (JT-105). The Sprint 3 action to verify each event end to end in Compose before Done was not applied to it.
- Bugs still entered the sprint without estimates (JT-104 to JT-108), so 11.9 hours of fixes are not visible in the sprint's points (see the earned value report).
- New work was added late: JT-102 and JT-103 were approved on 18 Nov as Should tasks, after the sprint had started. Both were delivered, but in the final sprint even approved Should work competes with the closure tasks.
- The full parallel Docker Compose build ran out of memory on a laptop with about 7.75 GiB for Docker. Building one image at a time worked but took about 16 minutes, which slowed testing and UAT.
- The databases still held test data from earlier tasks during UAT. Testing should start from a known, clean data set.

---

## 7. Follow-up of the Sprint 3 actions

| Sprint 3 action | Result |
|---|---|
| Reassign the carryovers: JT-73 to Phuntsho, JT-72 and JT-70 to Jigden, with Sangay reviewing | Done at Sprint 4 planning on 17 Nov. JT-73, JT-70 and JT-72 were all Done |
| Check every Must task at each stand-up and reassign through the handover rule the same day | All Must tasks were finished, so no further reassignment was needed; the remaining Musts were covered by the one-day extension |
| Follow the slip order: JT-66, then JT-102 and JT-103, then other Should tasks; never cut a Must | Followed: no Must was cut. JT-72, last among Jigden's Should tasks, was left until the Must work and JT-100 were in review, and was then finished on 20 Nov |
| Replace the stubbed Cypress tests with real end-to-end tests against Docker Compose (JT-95) | Done |
| Add Prometheus and Grafana to Docker Compose (JT-93) | Done |
| Record unfinished Should tasks as not delivered in the closure report (JT-100) | Not needed: every Should task was finished (Section 3) |
| Estimate every task before it is added to the sprint, including bugs | Not met: JT-104 to JT-108 have no points |
| Verify each new event end to end in Docker Compose before moving its task to Done | Not met for JT-71 (found in UAT as JT-105); met for JT-70, JT-72 and JT-90 |

---

## 8. Risks

- R1 (schedule overrun): occurred. The day 2 trigger fired on 18 Nov and Sprint 4 needed a one-day extension; with it, all planned work except JT-100 (In Review) was finished.
- R6 (uneven workload): the reassignment on 17 Nov worked; every Must task except JT-100 (In Review) was finished.
- R2 (Knative and RabbitMQ): all 5 functions were deployed and their triggers verified in k3d (JT-92), and the Compose forwarder for notification-dispatcher was added in JT-105.
- R8 (load test): the 200-user run finished with 0 errors without straining the host (JT-96).
- All risks are closed out in `docs/pm/risk-register.md`, version 1.6.

---

## 9. Open items at close

- JT-100 (this review) is In Review; the Product Owner's approval of PR #101 is pending.
- AGENTS.md Section 16 still says notification-dispatcher has no Compose forwarder; JT-105 (PR #90) added it, so that line should be removed in the next AGENTS.md update.
- Sprint 4 is closed in Jira after this pull request merges.
