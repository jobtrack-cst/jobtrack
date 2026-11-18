# JobTrack: Sprint 4 Planning Record

| Item | Detail |
|---|---|
| Jira task | JT-98 |
| Owner | Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup |
| Planning date | 17 Nov 2026 (plan agreed with Phuntsho Namgyel as Product Owner; JT-90, JT-102, JT-103, the deployment scope and the slip order confirmed by Phuntsho on 18 Nov 2026) |
| Sprint started | 17 Nov 2026, 09:00 (Jira sprint 38) |
| Sprint dates | 17 Nov 09:00 to 19 Nov 23:59 |
| Related documents | `docs/pm/sprint-3-review.md`, `docs/pm/sprint-3-planning.md`, `docs/pm/risk-register.md` (version 1.5), AGENTS.md (Sections 14 and 15) |

---

## 1. Sprint goal

End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze. Committed minimum: the Must tasks (39 points).

Sprint 4 is the last sprint. It ends on 19 Nov 2026, the planned project completion date. There is no Sprint 5 and no extension.

---

## 2. Scope rules

- Must tasks (Jira priority High) are the committed minimum and are never cut.
- Should tasks (Medium) are planned. Because this is the last sprint, an unfinished Should task does not move to another sprint: it is recorded as not delivered in the final review and closure report (JT-100).
- Slip order if time runs short: JT-66 first, then JT-102 and JT-103, then the other Should tasks.
- The 19 points carried over from Sprint 3 keep their size and priority. JT-73 moves to Phuntsho, and JT-72 and JT-70 move to Jigden; Sangay, as the owner, reviews those PRs. Ownership in AGENTS.md Sections 4 and 5 is unchanged.
- JT-101 (frontend polish) uses MUI theming only, with no new libraries.
- JT-102 and JT-103 (profile images): JPEG or PNG only, a 1 MB limit, file signature validation, ownership checks and additive protobuf changes. Each proto diff needs Phuntsho's explicit approval before implementation.
- Feature freeze: no new features after these tasks. Work found during the sprint goes to the backlog unless it blocks the sprint goal (AGENTS.md Section 12).
- No sub-tasks in Sprint 4. The acceptance criteria in each task are the checklist.
- The 11 Epics (JT-5 to JT-15) stay in the backlog and are not part of the sprint.

---

## 3. Sprint backlog

| Key | Summary | Owner | Priority | Points | Source |
|---|---|---|---|---|---|
| JT-66 | Decide and implement error logging for unexpected gRPC errors | Phuntsho | Should | 3 | Carried over |
| JT-67 | Align project charter and sprint calendar with the revised schedule | Jigden | Should | 1 | Carried over |
| JT-69 | Job-search: build the read model from job events | Sangay | Must | 2 | Carried over |
| JT-70 | Saved-jobs: consume job.closed | Jigden (reviewed by Sangay) | Should | 1 | Carried over, reassigned from Sangay |
| JT-71 | Implement notification-dispatcher Knative function | Sangay | Must | 5 | Carried over |
| JT-72 | Audit-log: consume all domain events | Jigden (reviewed by Sangay) | Should | 2 | Carried over, reassigned from Sangay |
| JT-73 | Frontend: employer job pages, job search, saved jobs and notifications | Phuntsho (reviewed by Sangay) | Must | 3 | Carried over, reassigned from Sangay |
| JT-84 | Sprint 3 SonarQube run and JMeter deadline-rush scenario preparation | Kinley | Should | 2 | Carried over |
| JT-90 | job-posting: removing a job through moderation does not publish job.closed (bug) | Jigden | Must | 2 | Moved from the backlog on 17 Nov |
| JT-91 | Saved-jobs: ListJobSavers pagination and internal interceptor | Sangay | Must | 2 | New |
| JT-92 | Knative: deploy all 5 functions to k3d and verify their triggers | Sangay | Must | 3 | New |
| JT-93 | Monitoring: Prometheus and Grafana in Docker Compose | Phuntsho | Must | 3 | New |
| JT-94 | Architecture and deployment documentation | Phuntsho | Must | 2 | New |
| JT-95 | Cypress: end-to-end main workflow against the running Compose stack | Kinley | Must | 5 | New |
| JT-96 | JMeter: run the deadline-rush load test and report results | Kinley | Must | 3 | New |
| JT-97 | Final test summary report: test counts, coverage, SonarQube and defects | Kinley | Must | 2 | New |
| JT-98 | Sprint 3 review, Sprint 4 planning record and AGENTS.md update | Jigden | Must | 2 | New |
| JT-99 | UAT: test script and sessions for applicant, employer and admin | Jigden | Must | 2 | New |
| JT-100 | Final sprint review and retrospective, earned value and project closure report | Jigden | Must | 3 | New |
| JT-101 | Frontend polish: MUI theme, shared layout and consistent page styling | Phuntsho | Should | 3 | New |
| JT-102 | Employer: company logo upload and display | Jigden | Should | 3 | New |
| JT-103 | Applicant-profile: profile photo upload and display | Kinley | Should | 3 | New |
| JT-104 | resume page: file name check uses a control character in a regular expression (SonarQube S6324) (bug) | Kinley | Should | Unestimated | Added by Kinley during the sprint |

22 estimated issues, 57 points, plus JT-104 (unestimated): 8 carried over from Sprint 3 (19 points), JT-90 from the backlog (2 points) and 13 new tasks (JT-91 to JT-103, 36 points). 14 Must issues (39 points) and 8 Should issues (18 points), plus JT-104.

---

## 4. Carryover from Sprint 3

| Key | Points | Priority | Sprint 3 owner | Sprint 4 owner | Reviewer |
|---|---|---|---|---|---|
| JT-71 | 5 | Must | Sangay | Sangay | |
| JT-73 | 3 | Must | Sangay | Phuntsho | Sangay |
| JT-69 | 2 | Must | Sangay | Sangay | |
| JT-72 | 2 | Should | Sangay | Jigden | Sangay |
| JT-70 | 1 | Should | Sangay | Jigden | Sangay |
| JT-66 | 3 | Should | Phuntsho | Phuntsho | |
| JT-84 | 2 | Should | Kinley | Kinley | |
| JT-67 | 1 | Should | Jigden | Jigden | |
| Total | 19 | 10 Must, 9 Should | | | |

Sangay held 13 of the 19 carried points. Moving JT-73, JT-72 and JT-70 (6 points) to other members lets Sangay focus on the notification-dispatcher (JT-71), which JT-92 needs. JT-72 is Jigden's own service (audit-log) and was Sangay's only as backup in Sprint 3.

---

## 5. Capacity and load

| Member | Must points | Should points | Total points |
|---|---|---|---|
| Jigden Shakya | 9 | 7 | 16 |
| Kinley Tobgay Lhendrup | 10 | 5 (plus JT-104, unestimated) | 15 |
| Phuntsho Namgyel | 8 | 6 | 14 |
| Sangay Tenzin | 12 | 0 | 12 |
| Total | 39 | 18 | 57 |

- Sprint 3 delivered 43 points in 3 days. The 39 Must points fit within that velocity; the 18 Should points are the work that can be dropped.
- Sangay has the lowest total but the most Must points (12), all on the deployment path (JT-71 then JT-92). His tasks are checked first at every stand-up (Sprint 3 review, Section 5).
- Jigden's JT-90 (2 points, Must) is already Done (PR #69, merged on 18 Nov).

---

## 6. Cross-service dependencies

"Done" means the dependency is already in `main`. A consumer whose producer is not ready builds against the contract fixtures first, as in Sprint 3.

| Key | Depends on | Needed by | Notes |
|---|---|---|---|
| JT-66 | None | None | Slips first |
| JT-67 | None | None | Documentation only |
| JT-69 | JT-68 (Done) | JT-73 (job search page) | In Progress |
| JT-70 | JT-68 and JT-90 (Done) | None | |
| JT-71 | JT-77, JT-82, JT-86 and the event producers (Done); notification-inbox and auth internal RPCs (Section 6) | JT-92, JT-95 (notifications in the main workflow), JT-99 | Critical path |
| JT-72 | All producers (fixtures until merged) | Audit log page from JT-88 (Done) shows data | Sangay reviews |
| JT-73 | JT-69 | JT-95, JT-99 | UAT needs JT-73 merged |
| JT-84 | None | JT-96 (JMeter scenario), JT-97 (SonarQube report) | In Progress |
| JT-90 | None | JT-70, JT-69 (`job.closed` on removal) | Done |
| JT-91 | None | JT-92 (deadline-reminder verification) | Closes the JT-82 open item in AGENTS.md Section 16 |
| JT-92 | JT-71; images named `dev.local/...` and Mailpit reachable in k3d (AGENTS.md Section 16) | JT-94 (deployment documentation), JT-100 | Scope: all 5 functions on Knative in k3d, the 20 services stay in Docker Compose |
| JT-93 | None | JT-94 | |
| JT-94 | JT-92, JT-93 | JT-100 | |
| JT-95 | The Compose stack; JT-73 for the job pages | JT-97 | Replaces the `cy.intercept` stubs |
| JT-96 | JT-84 | JT-97 | 200 concurrent users |
| JT-97 | JT-84, JT-95, JT-96 | JT-100 | |
| JT-98 | None | None | |
| JT-99 | JT-73; the main workflow running in Docker Compose | JT-100 | |
| JT-100 | JT-97, JT-99 and the final sprint status | None | Records unfinished Should tasks as not delivered |
| JT-101 | None | None | Due by the 18 Nov stand-up |
| JT-102 | Approved `employer.proto` diff (18 Nov, 17:16); Sangay confirms it in the PR | None | Slips after JT-66 |
| JT-103 | Phuntsho's approval of the `applicant_profile.proto` diff | None | Slips after JT-66 |
| JT-104 | None | JT-97 (SonarQube result) | |

Critical paths:

- JT-71, then JT-92, then JT-94 (deployment)
- JT-69, then JT-73, then JT-95 and JT-99 (frontend, end-to-end test and UAT)
- JT-84, then JT-96, then JT-97 (load test and test report)
- JT-97 and JT-99, then JT-100 (closure), on 19 Nov

Jigden's order after JT-98: the JT-102 backend once Phuntsho approves the proto diff (18 Nov), then the JT-102 frontend and JT-99 UAT on the morning of 19 Nov, and JT-100 on the afternoon of 19 Nov. If JT-102 is not working by midday on 19 Nov, it stops and Jigden switches to JT-99 and JT-100 (both Must), because JT-102 must not delay Must tasks. If time is short, the agreed slip order applies: JT-66 first, then JT-102 and JT-103, then the other Should tasks. Among those other Should tasks, Jigden's JT-72 slips before JT-67 and JT-70.

---

## 7. Decisions made

Recorded in the AGENTS.md Decision log (Section 15), agreed with Phuntsho as Product Owner:

- Carryover reassignment: JT-73 to Phuntsho, JT-72 and JT-70 to Jigden; Sangay, as the owner, reviews those PRs. Ownership in Sections 4 and 5 is unchanged (17 Nov).
- No sub-tasks in Sprint 4; the acceptance criteria are the checklist (17 Nov).
- JT-101 frontend polish is added as a Should task, using MUI theming only, with no new libraries (17 Nov).
- JT-90 is moved from the backlog into Sprint 4 as a Must and assigned to Jigden as job-posting backup (17 Nov; approved by Phuntsho on 18 Nov).
- WEB303 deployment: all 5 functions run on Knative in k3d, and the 20 services stay in Docker Compose. JT-92 verifies the functions, their triggers, connectivity and scale to zero. Open action: confirm this against the official WEB303 assessment requirements (Phuntsho, 18 Nov).
- JT-102 and JT-103 (profile images) are approved as Should tasks after JT-66: JPEG or PNG, a 1 MB limit, file signature validation, ownership checks and additive protobuf changes. Each proto diff needs Phuntsho's explicit approval before implementation (Phuntsho, 18 Nov).
- Slip order: JT-66 first, then JT-102 and JT-103, then other Should tasks; never cut a Must. No Sprint 5 and no extension (Phuntsho, 18 Nov).

---

## 8. Risks

- R1 (schedule overrun): Sprint 3 missed its committed minimum, 19 points were carried over, and this sprint ends on the completion date with no buffer. The response is the slip order and the rule that unfinished Should tasks are not delivered. The trigger is checked at the 18 Nov stand-up (day 2) with the same rule as Sprint 3: less than half of the planned points (29 of 57) Done or In Review fires it.
- R6 (member unavailability or uneven workload): the carryovers were reassigned, and Sangay's Must tasks are checked at every stand-up.
- R2 (Knative and RabbitMQ): JT-92 deploys all 5 functions to k3d for the first time.
- R8 (load test): JT-84 and JT-96.
- See the risk register, version 1.5.

---

## 9. After Sprint 4

- There is no Sprint 5. Unfinished Should tasks are recorded as not delivered in the closure report (JT-100), not moved.
- Feature freeze at the end of Sprint 4 (19 Nov).
- The final presentation and submission dates are not yet announced (AGENTS.md Section 16).

---

## 10. Open items

- The `employer.proto` diff for JT-102 (`PutCompanyLogo`, `GetCompanyLogo` and `has_logo` on `CompanyProfile`) was sent to Phuntsho on 18 Nov at 16:53 and approved by him at 17:16 (AGENTS.md Section 15). Sangay confirms the exact diff for Team B in the JT-102 PR.
- Confirm the WEB303 deployment scope against the official assessment requirements (Phuntsho).
- JT-104 has no estimate yet (Sprint 2 action: estimate every task and bug before it enters a sprint).
