# JobTrack: Sprint 3 Review and Retrospective

| Item | Detail |
|---|---|
| Jira task | JT-98 |
| Owner | Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup |
| Sprint | Sprint 3 (Jira sprint 37) |
| Sprint dates | 14 Nov 09:00 to 16 Nov 23:59 (not extended, Section 15, 14 Nov 2026) |
| Closed in Jira | 16 Nov 2026, 22:29 UTC |
| Review and retrospective | Not held as a separate meeting; covered during Sprint 4 planning on 17 Nov 2026, led by Jigden |
| Data source | Jira sprint 37 and issue history, checked on 18 Nov 2026 |
| Related documents | `docs/pm/sprint-2-review.md`, `docs/pm/sprint-3-planning.md`, `docs/pm/sprint-4-planning.md`, `docs/pm/risk-register.md` (version 1.5), AGENTS.md (Sections 14 and 15) |

---

## 1. Sprint goal and outcome

Goal: Full business logic, all 5 functions, frontend for all roles, integration tests. Committed minimum: the Must tasks (44 points).

Outcome: not met. 17 of the 25 tasks in the sprint were Done (68 percent), 43 of 62 points. The committed minimum was missed: 13 of 16 Must tasks were Done, 34 of 44 Must points. The three unfinished Musts (JT-71 notification-dispatcher, JT-73 frontend job pages, JT-69 job-search read model) all belonged to Sangay, who held 13 of the 19 carried points.

What was delivered: job-posting, offer, messaging, job-alert, review, interview, application-status, analytics and moderation now publish and consume their domain events, three new functions were built (email-sender JT-77, deadline-reminder JT-82, weekly-summary JT-86), and the frontend pages for applicants, employers and admins in JT-78, JT-83 and JT-88 were finished. The gateway body size limit (JT-54) was done early, as agreed, so resume upload works through the gateway.

What was missing at sprint end:

- Only 4 of the 5 functions existed; the `notification-dispatcher` folder was empty.
- The frontend folders for job-posting, job-search, saved-jobs and notification-inbox were empty.
- The Cypress tests all used `cy.intercept` stubs, so no test ran against the real services.
- There was no JMeter scenario and no Prometheus or Grafana setup.

All 8 unfinished tasks (19 points) were carried into Sprint 4. Following the 14 Nov rule, the Should tasks moved to the top of Sprint 4; the three Musts were carried as well, because they are needed for the project goal.

---

## 2. Results

| Measure | Tasks | Story points |
|---|---|---|
| Planned | 25 | 62 |
| Done in Sprint 3 | 17 | 43 |
| Carried over to Sprint 4 | 8 | 19 |
| Must (High): planned / done | 16 / 13 | 44 / 34 |
| Should (Medium): planned / done | 9 / 4 | 18 / 9 |

- Velocity (story points Done in the sprint): 43, compared with 60 in Sprint 2 (over 5 days) and 49 in Sprint 1.
- Velocity per day: about 14 points, compared with about 12 in Sprint 2 and about 16 in Sprint 1.
- Completion: 17 of 25 tasks (68 percent) and 43 of 62 points (69 percent), counted against the tasks planned, including the carried tasks (Sprint 2 action).
- The Must completion of 34 of 44 points is the first time the committed minimum was not met. Sprint 1 and Sprint 2 both finished all of their Musts.
- Every task in Sprint 3 had an estimate before the sprint started, so the points figure now shows the unfinished work (Sprint 2 action).

### By member

By Jira assignee when the sprint closed. JT-70, JT-72 and JT-73 were reassigned for Sprint 4 after the sprint closed, so Jira now shows other assignees for them.

| Member | Planned points | Done tasks | Done points | Carried over |
|---|---|---|---|---|
| Phuntsho Namgyel | 16 | 5 of 6 (JT-74 to JT-78) | 13 | JT-66 (3) |
| Kinley Tobgay Lhendrup | 15 | 5 of 6 (JT-79 to JT-83) | 13 | JT-84 (2) |
| Jigden Shakya | 15 | 6 of 7 (JT-54, JT-85 to JT-89) | 14 | JT-67 (1) |
| Sangay Tenzin | 16 | 1 of 6 (JT-68) | 3 | JT-69 (2), JT-70 (1), JT-71 (5), JT-72 (2), JT-73 (3): 13 points |
| Total | 62 | 17 of 25 | 43 | 8 tasks, 19 points |

---

## 3. Task outcomes

Owner is the Jira assignee at sprint close. Status as of 18 Nov 2026.

| Key | Summary | Owner | Priority | Points | Sprint 3 result | Status now |
|---|---|---|---|---|---|---|
| JT-54 | Gateway: limit the request body size (carried over) | Jigden | Should | 2 | Done | Done |
| JT-66 | Decide and implement error logging for unexpected gRPC errors (carried over) | Phuntsho | Should | 3 | Carried over | Sprint 4, To Do |
| JT-67 | Align project charter and sprint calendar with the revised schedule (carried over) | Jigden | Should | 1 | Carried over | Sprint 4, To Do |
| JT-68 | Job-posting: publish job events through an outbox, consume moderation.job_removed | Sangay | Must | 3 | Done | Done |
| JT-69 | Job-search: build the read model from job events | Sangay | Must | 2 | Carried over | Sprint 4, In Progress |
| JT-70 | Saved-jobs: consume job.closed | Sangay | Should | 1 | Carried over | Sprint 4, To Do (now Jigden) |
| JT-71 | Implement notification-dispatcher Knative function | Sangay | Must | 5 | Carried over | Sprint 4, To Do |
| JT-72 | Audit-log: consume all domain events | Sangay | Should | 2 | Carried over | Sprint 4, To Do (now Jigden) |
| JT-73 | Frontend: employer job pages, job search, saved jobs and notifications | Sangay | Must | 3 | Carried over | Sprint 4, To Do (now Phuntsho) |
| JT-74 | Offer: consume application.submitted, publish offer events | Phuntsho | Must | 3 | Done | Done |
| JT-75 | Messaging: consume application.submitted, publish message.sent | Phuntsho | Should | 2 | Done | Done |
| JT-76 | Job-alert: consume job.published, real ResolveNames client, publish job_alert.matched | Phuntsho | Should | 3 | Done | Done |
| JT-77 | Implement email-sender Knative function | Phuntsho | Must | 2 | Done | Done |
| JT-78 | Frontend: apply for a job, my applications, offers, messages and job alerts | Phuntsho | Must | 3 | Done | Done |
| JT-79 | Review: consume application.submitted, publish review.completed | Kinley | Must | 2 | Done | Done |
| JT-80 | Interview: consume application.submitted, publish interview.scheduled | Kinley | Must | 2 | Done | Done |
| JT-81 | Application-status: consume review, interview and offer events | Kinley | Must | 3 | Done | Done |
| JT-82 | Implement deadline-reminder Knative function | Kinley | Must | 3 | Done | Done |
| JT-83 | Frontend: applicant profile, resume upload, application status, employer review and interview | Kinley | Must | 3 | Done | Done |
| JT-84 | Sprint 3 SonarQube run and JMeter deadline-rush scenario preparation | Kinley | Should | 2 | Carried over | Sprint 4, In Progress |
| JT-85 | Analytics: consume outcome events | Jigden | Must | 2 | Done | Done |
| JT-86 | Implement weekly-summary Knative function | Jigden | Must | 3 | Done | Done |
| JT-87 | Moderation: publish moderation.job_removed through an outbox | Jigden | Should | 2 | Done | Done |
| JT-88 | Frontend: company profile, verification request, admin verification, moderation, analytics and audit log | Jigden | Must | 3 | Done | Done |
| JT-89 | Sprint 2 review, Sprint 3 planning record, AGENTS.md and risk register update | Jigden | Must | 2 | Done | Done |

The 8 carried tasks were in the sprint from 14 Nov and were moved to Sprint 4 (Jira sprint 38) when Sprint 3 closed.

### Work outside the sprint

- No issue was resolved between 14 and 16 Nov outside sprint 37, apart from JT-53, which was Done on 14 Nov and is already explained in the Sprint 2 review.
- JT-90 (job-posting did not publish `job.closed` when moderation removed a job, bug) was found during Sprint 3, went to the backlog following the Section 12 rule, and was moved into Sprint 4 as a Must on 17 Nov.

---

## 4. What went well

- Phuntsho, Kinley and Jigden finished 13 to 14 points each and all of their Must tasks. The load at planning was balanced at 15 to 16 points per member, as the Sprint 2 action asked.
- Most of the event-driven business logic is in place: 9 services now publish or consume their domain events, with transactional outboxes, idempotent consumers and shared contract fixtures.
- Three of the four remaining functions were built, and weekly-summary was verified with a PingSource trigger in k3d (JT-86).
- Building consumers against the shared fixtures worked: analytics (JT-85) was built before its producers were ready, and the fixtures added in JT-85 were later used by job-posting and application-status.
- JT-54 was done early as agreed, so JT-83 could upload resumes through the gateway.
- Every task was estimated before it entered the sprint, and the Jira sprint matched the work done in it.
- The sprint was not extended, as agreed on 14 Nov.

---

## 5. What to change

- Watch the critical path every day, not only on day 2. Sangay held the only remaining function (JT-71, 5 points), the job-search read model and four frontend areas. One member's delay left a Must function and four frontend folders empty. The delay was seen at the day 2 check on 15 Nov, but no work was reassigned until 17 Nov.
- Do not let one member hold several Must tasks that others depend on. JT-73 depended on JT-69, and the deployment work in Sprint 4 (JT-92) depends on JT-71.
- Reassign earlier. The handover rule should be used as soon as a Must task has not moved by the day 2 stand-up, instead of at the next planning.
- Test against the real system. All Cypress tests used `cy.intercept` stubs, so the end-to-end flow was never tested through the real gateway and services. Sprint 4 replaces them with real tests against Docker Compose (JT-95).
- Start testing and monitoring tools earlier. JMeter, Prometheus and Grafana were not started in Sprint 3, so all of them fall into the final sprint.
- Sprint 4 is the last sprint, so unfinished Should work can no longer move to a later sprint. It is recorded as not delivered.

---

## 6. Action items

| Action | Owner | When |
|---|---|---|
| Reassign the carryovers: JT-73 to Phuntsho, JT-72 and JT-70 to Jigden, with Sangay reviewing; keep ownership in Sections 4 and 5 unchanged | Jigden with Phuntsho (Product Owner) | Sprint 4 planning, 17 Nov (done) |
| Check the progress of every Must task at each stand-up, starting with JT-71 and JT-92, and reassign through the handover rule the same day if a Must has not moved | Jigden | Daily, 17 to 19 Nov |
| Follow the agreed slip order: JT-66 first, then JT-102 and JT-103, then other Should tasks; never cut a Must | Jigden with Phuntsho | Sprint 4 |
| Replace the stubbed Cypress tests with real end-to-end tests against Docker Compose | Kinley (JT-95) | Sprint 4 |
| Add Prometheus and Grafana to Docker Compose | Phuntsho (JT-93) | Sprint 4 |
| Record unfinished Should tasks as not delivered in the closure report instead of moving them | Jigden (JT-100) | 19 Nov |
| Estimate every task before it is added to the sprint, including bugs | Jigden and the task owner | Sprint 4 |
| Verify each new event end to end in Docker Compose before moving its task to Done | Each task owner | Sprint 4 |

### Follow-up of the Sprint 2 actions

| Sprint 2 action | Result |
|---|---|
| Record cross-service dependencies for every Sprint 3 task in the planning record | Done in `docs/pm/sprint-3-planning.md` |
| Estimate every task and bug before it is added to a sprint | Done for all 25 Sprint 3 tasks. In Sprint 4, JT-104 (bug, added by Kinley) has no points yet |
| Report sprint completion as tasks and points Done out of those planned, including carried tasks | Done in this review (17 of 25 tasks, 43 of 62 points) |
| Keep the Jira sprint matching the work done in it | Done for Sprint 3; no work was done outside the sprint |
| Check the R1 trigger on day 2 of the sprint and reassign if needed | Checked on 15 Nov; the tasks held by Sangay were behind, but nothing was reassigned until Sprint 4 planning on 17 Nov. This is the main lesson of Sprint 3 (Section 5) |
| Let unfinished Should tasks, carryovers first, slip to the top of Sprint 4 instead of extending Sprint 3 | Done; Sprint 3 was not extended. Three Must tasks also slipped, which the rule did not plan for |
| Verify each new event end to end in Docker Compose before moving its task to Done | Done for the merged events (for example JT-87); continues in Sprint 4 |
| Raise blockers at the stand-up the same day | Ongoing |

---

## 7. Risks

- R1 (schedule overrun): Sprint 3 missed its committed minimum (34 of 44 Must points), and 19 points were carried into the last sprint, which has no buffer and cannot be extended. See the risk register for the updated score and response.
- R6 (member unavailability or uneven workload): the planned load was balanced, but the Done load ranged from 14 points (Jigden) to 3 points (Sangay). The carryovers were reassigned on 17 Nov.
- See the risk register, version 1.5.

---

## 8. Open items

- JT-71 (notification-dispatcher) is the only missing function, and JT-92 (deploy all 5 functions to k3d) depends on it.
- JT-73 (frontend job pages) is needed before UAT (JT-99) can cover the full flow.
- JT-104 (SonarQube S6324 bug in the resume page regex) was added to Sprint 4 by Kinley without points.
