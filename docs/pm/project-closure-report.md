# JobTrack: Project Closure Report

| Item | Detail |
|---|---|
| Project | JobTrack: A Job and Internship Application Portal |
| Jira task | JT-100 |
| Prepared by | Jigden Shakya (Scrum Master and project manager). Backup: Kinley Tobgay Lhendrup |
| Version | 1.0 |
| Date | 20 Nov 2026 |
| Project dates | 2 Nov 2026 (Sprint 0 start) to 20 Nov 2026 (Sprint 4 end, 23:59 Asia/Thimphu) |
| Product Owner review | Approved: Phuntsho Namgyel reviewed PR #101 on 20 Nov 2026, requested corrections and approved the corrected version on 20 Nov 2026 at 20:37 |
| Data dates | Jira data read on 20 Nov 2026 at 19:10 (JT-100 In Review at that time). Test results as reported on 18 and 19 Nov 2026 (Section 8). UAT on 19 Nov 2026, retests on 20 Nov 2026 |
| Related documents | `docs/pm/project-charter.md` (version 1.2), `docs/pm/sprint-4-review.md`, `docs/pm/earned-value-report.md`, `docs/pm/risk-register.md` (version 1.6), `docs/testing/test-summary-report.md`, `docs/testing/uat-script.md`, `docs/testing/jt-96-load-test-report.md`, AGENTS.md (Sections 14 to 16) |
| Format | Closure report following PMI PMBOK practice, adapted for Scrum |

---

## 1. Purpose

This report formally closes the JobTrack development phase. It compares the result with the objectives, scope and deliverables in the project charter, records what was and was not delivered, summarises schedule, cost, quality and risk, and lists the lessons learned and the open items handed over.

The project is the combined final project for WEB303 (Microservices and Serverless Applications), SWE302 (Software Testing and Quality Assurance) and SWE303 (Software Project Management) at the College of Science and Technology, Royal University of Bhutan.

---

## 2. Project summary

| Item | Planned (charter) | Actual |
|---|---|---|
| Method | Scrum: Sprint 0 plus four 3-day sprints | Sprint 0 plus four sprints; Sprint 2 extended by two days and Sprint 4 by one day |
| Start | 28 Oct 2026 (proposal plan) | 2 Nov 2026 (moved on 1 Nov because of other coursework) |
| Completion | 17 Nov 2026 (5 Nov baseline) | 20 Nov 2026 (revised to 19 Nov on 11 Nov, extended to 20 Nov on 19 Nov) |
| Team | 4 members in two pairs | 4 members; work reassigned within pairs on 9 Nov and 17 Nov, ownership unchanged |
| Scope | 244 story points in Jira (Sprints 0 to 4) | 241 points Done at the snapshot; 244 (all planned work) when JT-100 (3 points, In Review) merges |
| Effort | No hours were planned; retrospective baseline 186.5 h at the Sprint 0 rate | 215.4 h logged in Jira |
| System | 20 microservices, gateway, 5 Knative functions, React frontend | All built; the 20 services and the gateway run in Docker Compose, the 5 functions run on Knative in k3d |
| Tools | Jira (space JT), GitHub, GitHub Actions | As planned |

---

## 3. Objectives against the charter

| # | Objective | Success criteria (charter Section 2) | Result | Status |
|---|---|---|---|---|
| O1 | Establish the development foundation and contracts | Repo, `.proto` contracts for all 20 services, golden template, CI, Docker Compose and a proven cluster setup by 3 Nov | Contracts for all 20 services were agreed in PR #6 on 3 Nov; golden template, CI and Compose were set up in Sprint 0. The cluster spike (JT-29) was verified on 7 Nov, four days after Sprint 0 ended, and selected the forwarder fallback for RabbitMQ events | Met, with the cluster setup late |
| O2 | Build and deploy the full system | All 20 microservices, the gateway, 5 functions and the frontend running; the main workflow works end to end on the k3d cluster | The main workflow was verified end to end in Docker Compose by Cypress (JT-95) and UAT (JT-99), with notifications working after JT-105. On 18 Nov the Product Owner agreed a reduced k3d scope: all 5 functions run on Knative in k3d (JT-92) and the 20 services stay in Docker Compose, because capacity for all 20 services in k3d was not proven | Partly met: the workflow runs in Compose, not on k3d (agreed change) |
| O3 | Prove quality through systematic testing | At least 80 percent line coverage on `application` and `application-status`; contract checks in CI; Cypress end-to-end tests; JMeter load test with 200 users; results documented | Coverage (unit plus integration): `application` 94.4 percent, `application-status` 91.1 percent. `buf breaking` and 34 contract tests run in CI. 192 Cypress GUI tests run in CI, and the 11-step end-to-end workflow passes against the running stack (JT-95). The 200-user JMeter run had 0 errors (JT-96). In the JT-97 run on 19 Nov all 1,383 Go test functions passed. Results are in `docs/testing/test-summary-report.md` (JT-97, 19 Nov); see Section 8 for the dates of each result | Met |
| O4 | Manage the project with Scrum and PMBOK artefacts | Ceremonies held; Jira current with time logged; risk register reviewed every retrospective; earned value from Jira time logs; lessons learned at closure | Planning, reviews and retrospectives were recorded for Sprints 1 to 4 (the Sprint 3 review was held within Sprint 4 planning, the Sprint 4 review was written and reviewed in its pull request, and Sprint 0 has no review record). Every issue has hours logged. The risk register was reviewed at every review (versions 1.0 to 1.6). Earned value: `docs/pm/earned-value-report.md`, against a retrospective baseline because no hours were planned. Lessons learned: Section 11 | Partly met: some required ceremonies were not formally held or documented |

---

## 4. Exit criteria (charter Section 13)

| # | Exit criterion | Status |
|---|---|---|
| 1 | All in-scope services, functions, the gateway and the frontend are deployed and the main workflow works end to end | Met for the agreed deployment scope: the services in Docker Compose and the 5 functions on Knative in k3d; the main workflow passed in Cypress and UAT, and all UAT conditions were met |
| 2 | Objectives O1 to O4 are met, or any gap is documented in the closure report | Gaps documented in Sections 3 and 5 |
| 3 | All deliverables in charter Section 5 are in the repo and reviewed by the team | Not fully met: four planning documents were not produced (Section 5.3) |
| 4 | The final presentation is given and assessed by the module tutors | Not yet: dates not announced (milestone M7, risk R10) |
| 5 | The lessons-learned and closure report is completed | Met: this report, approved by the Product Owner in PR #101 on 20 Nov 2026 |

---

## 5. Scope

### 5.1 Delivered

- 20 microservices in Go, each with its own PostgreSQL database, hexagonal layout and gRPC contract (charter Section 4.1), running in Docker Compose.
- API gateway with JWT checks, CORS for the frontend and a 3 MiB body limit (JT-31, JT-53, JT-54).
- 5 Knative functions: notification-dispatcher, email-sender, weekly-summary, resume-validator and deadline-reminder, deployed to k3d with their triggers verified (JT-92).
- Domain events over RabbitMQ with transactional outboxes, idempotent consumers and a shared retry and dead-letter policy (JT-56, JT-61).
- React frontend for applicants, employers and admins (JT-78, JT-83, JT-88, JT-73), with a shared MUI theme (JT-101), company logos (JT-102), applicant profile photos (JT-103) and sessions kept across page reloads (JT-106).
- Audit-log consumes every domain event (JT-72), so the admin audit log page shows the event history.
- Docker Compose for local running, CI with GitHub Actions, Prometheus and Grafana in Compose (JT-93) and logging of unexpected gRPC errors (JT-66).
- Tests: unit, integration (Testcontainers), contract, Cypress GUI and end-to-end against the Compose stack, the JMeter load test and SonarQube runs.

### 5.2 Not delivered or changed

| Item | Charter reference | What happened | Decision |
|---|---|---|---|
| Main workflow on the k3d cluster | O2 | 20 services stay in Docker Compose; only the 5 functions run in k3d | Agreed with the Product Owner on 18 Nov (AGENTS.md Section 15); to be confirmed against the WEB303 requirements (Phuntsho) |
| Recording interview outcomes | Section 4.1, Interview | `interview.proto` has no outcome field or RPC; only scheduling and reading were built (JT-52) | Needs a contract change; not agreed (AGENTS.md Section 16) |
| Employer view of applicant resumes | Section 3, review step | `GetResume` and `DownloadResume` are owner only | Open question; backlog consideration only (Section 10) |
| Kubernetes manifests for the 20 services | Section 5, development; AGENTS.md Section 8 | `deploy/k8s` has only the `service-template` base; no service has Kustomize manifests | Follows from the reduced k3d scope agreed on 18 Nov, where the services run only in Docker Compose |
| NetworkPolicies for internal RPCs | AGENTS.md Section 6, layer 3 | Not written, because the services do not run in k3d; layers 1 and 2 are in place | Open item (Section 10) |
| Stakeholder register, SRS, WBS and quality management plan | Section 5, initiation and planning | Not in the repository (checked on 19 Nov), and no team member reported having made them | Recorded as not delivered |

The out-of-scope items in charter Section 4.4 (payments, real SMS and email, resume parsing, mobile apps, production hosting, national systems, real personal data) stayed out of scope.

### 5.3 Deliverables (charter Section 5)

| Area | Deliverable | Status |
|---|---|---|
| Initiation and planning | Project proposal, charter, Definition of Done and working agreement, sprint calendar, risk register, product backlog in Jira | Done (`docs/pm`, Jira) |
| Initiation and planning | Stakeholder register, SRS, WBS, quality management plan | Not delivered (Section 5.2) |
| Design | Architecture and service catalogue (AGENTS.md), `.proto` and event contracts | Done |
| Development | Source code for 20 services, gateway, 5 functions and frontend; Dockerfiles, Kubernetes and Knative manifests, CI pipeline | Done for the source code, Dockerfiles, CI pipeline and Knative manifests (`deploy/knative`: all 5 functions and the forwarder). Kubernetes manifests in `deploy/k8s` exist only for `service-template`, so the 20 services have none (Section 5.2) |
| Testing | Test plan, test case document with traceability, coverage and SonarQube reports, load test report, defect log in Jira | Done (`docs/testing`: test plan, test cases per task, test summary report with coverage, SonarQube and the defect log, load test report; bugs in Jira) |
| Deployment | Running deployment on k3d, Grafana dashboards, deployment guide | Functions on k3d (JT-92), Grafana (JT-93), deployment documentation (JT-94) |
| Closure | Sprint reviews, earned value report, final presentation, lessons-learned and closure report | Sprint 1 to 4 reviews, earned value report and this closure report done; final presentation not yet scheduled |

---

## 6. Schedule

Baseline: the Option B dates agreed on 5 Nov 2026. Revised: the schedule agreed on 11 Nov 2026 (JT-64).

| Milestone | Baseline | Revised | Actual | Result |
|---|---|---|---|---|
| M1 Sprint 0 complete | 3 Nov | 3 Nov | 3 Nov | 35 of 46 points; the cluster spike was finished on 7 Nov |
| M2 Sprint 1 complete | 8 Nov | 8 Nov | 8 Nov | Goal partly met: 15 of 26 tasks, 49 of 80 points |
| M3 Sprint 2 complete | 11 Nov | 13 Nov | 13 Nov | Goal met with the two-day extension: 60 of 60 points |
| M4 Sprint 3 complete | 14 Nov | 16 Nov | 16 Nov | Goal not met: 17 of 25 tasks, 43 of 62 points, 34 of 44 Must points |
| M5 Sprint 4 complete | 17 Nov | 19 Nov | 20 Nov | Goal met with the one-day extension: at the snapshot 21 of 22 tasks and 54 of 57 points Done, with JT-100 (3 points) In Review; all 22 tasks and 57 points when it merges |
| M6 Project completion | 17 Nov | 19 Nov | 20 Nov | Three days later than the baseline |
| M7 Final presentation | To be announced | To be announced | Not yet | Open (R10) |

Velocity (story points Done): 35, 49, 60, 43 and 54 in Sprints 0 to 4 (Sprint 4 becomes 57 when JT-100 merges). The velocity chart, the burndown charts for every sprint and the per-sprint figures are in `docs/pm/earned-value-report.md`.

---

## 7. Cost and earned value

The project had no funding (charter Section 8). Cost is measured as hours logged in Jira and value as story points Done. No planned hours were set, so the cost baseline is a retrospective estimate, not a plan made in advance: the Sprint 0 rate of 0.7642857 hours per point (shown as 0.7643). Calculations use unrounded values; only the results below are rounded (method in `docs/pm/earned-value-report.md` Section 2). "Actual" is Jira on 20 Nov at 19:10 with JT-100 In Review; "projected" adds JT-100's 3 points when it merges.

| Measure at completion | Actual (snapshot) | Projected (JT-100 merged) |
|---|---|---|
| BAC | 244 points, 186.5 h | 244 points, 186.5 h |
| PV | 244 points, 186.5 h | 244 points, 186.5 h |
| EV | 241 points, 184.2 h | 244 points, 186.5 h |
| AC | 215.4 h | 215.4 h |
| SV | -3 points (-2.3 h) | 0 points (0 h) |
| CV | -31.2 h | -28.9 h |
| SPI | 0.99 | 1.00 |
| CPI | 0.86 | 0.87 |

Interpretation: all of the planned scope except JT-100 is delivered (SPI 0.99, 1.00 when JT-100 merges), but only by adding three days to the schedule, which a point-based SPI does not show. The CPI stayed above 1 until Sprint 3 and fell to 0.86 in Sprint 4 (0.87 when JT-100 merges), which was mostly testing, UAT and documentation and included bug fixes without points: the project used about 16 percent more hours than the Sprint 0 rate predicts.

---

## 8. Quality

- Test summary (JT-97, completed on 19 Nov 2026 at 22:07): on `main` at that time, 1,383 Go test functions (unit and integration), 34 contract tests, 192 Cypress GUI tests in CI and the 11-step end-to-end workflow against the running stack all passed. Coverage: `application` 94.4 percent and `application-status` 91.1 percent, above the 80 percent target. Work merged after that run (JT-66, JT-70, JT-72, JT-101 to JT-103 and the UAT bug fixes) was tested in its own pull requests; no new full summary run was made.
- End-to-end workflow against the Compose stack (JT-95): completed on 18 Nov 2026.
- SonarQube (JT-97, run on 19 Nov 2026 on `main` at 9908ad1): 0 bugs and 0 vulnerabilities. The default quality gate failed on new code (coverage of new code, 4.8 percent duplication and 25 new code smells), mainly because frontend pages have no coverage report and because of template duplication; the team treats this as maintainability work after the feature freeze.
- Load test (JT-96, completed on 19 Nov 2026, run on `main` at 2d1abc5): 200 concurrent users, 2,003 requests, 0 errors; all 200 simultaneous applications were created within about 77 ms.
- UAT (JT-99, 19 Nov): 34 scenarios, 29 Pass, 3 Fail, 2 Not run; the core workflow was verified end to end with no Critical defects. Product Owner sign-off: Accepted with conditions.
- All conditions were met by 20 Nov: JT-105 and JT-107 (Major) were fixed on 19 Nov and UAT-EM-05, UAT-EM-07 and UAT-AP-07 passed on retest on 20 Nov; JT-108 (Major) and JT-106 (Minor), which were only to be documented, were fixed as well.
- Known issues at closure: no open bug from UAT. UAT-AD-07 (audit log) was not run during UAT because the audit-log consumer did not exist yet; JT-72 added it on 20 Nov and verified in Docker Compose that an event appears in the admin audit log, but the UAT scenario itself was not rerun.

---

## 9. Risks

All risks are closed out in `docs/pm/risk-register.md` Section 6:

- Occurred: R1 (schedule overrun; two extensions, completion moved from 17 to 20 Nov), R4 (integration failures; JT-90 and JT-105, both fixed; confirmed by the risk owner in the PR #101 review) and R6 (member unavailability and uneven workload; work reassigned on 9 Nov and 17 Nov).
- Occurred in part: R2 (native Knative RabbitMQ source failed, forwarder fallback used) and R3 (k3d scope reduced to the functions; the Compose build ran out of memory when built in parallel).
- Did not occur: R8 (the load test ran without errors or host limits) and R9 (no real data or secret exposed).
- Not confirmed: R5 (insufficient evidence that inconsistent AI-generated code triggered the risk, per the risk owner in the PR #101 review) and R7.
- Still open: R10 (presentation dates).

---

## 10. Open items and handover

| Item | Owner | Reference |
|---|---|---|
| Confirm the WEB303 deployment scope against the official assessment requirements | Phuntsho | AGENTS.md Section 16 |
| Final presentation and submission dates; update milestone M7 | Jigden | R10 |
| Employer access to applicant resumes; needs Phuntsho's authorisation and a contract review; no Jira issue yet | Phuntsho | AGENTS.md Section 16 |
| Interview outcome recording (contract change) | Kinley with the team | AGENTS.md Section 16 |
| NetworkPolicies for internal RPCs in `deploy/k8s` | Sangay | AGENTS.md Section 6 |
| SonarQube quality gate findings on new code (duplication, code smells, frontend coverage) | Team | `docs/testing/test-summary-report.md` |
| Remove the outdated notification-dispatcher line in AGENTS.md Section 16 (fixed by JT-105) | Phuntsho | AGENTS.md Section 16 |
| Other open questions in AGENTS.md Section 16 | Per item | AGENTS.md Section 16 |

---

## 11. Lessons learned

Collected from the Sprint 1 to 4 retrospectives and the earned value results.

What worked:

- Writing all `.proto` and event contracts in Sprint 0 let both teams build in parallel, first with mocked ports and later against shared event fixtures.
- One source of truth (AGENTS.md with its decision log) kept four members and their AI assistants on the same architecture, conventions and decisions.
- The golden service template and sample tests gave every service the same layout and test style.
- Making Must tasks the committed minimum, with an agreed slip order, protected the core workflow when time ran short: every Must task in the project was finished except JT-100, which is In Review, and in the final sprint every Should task as well.
- Recording cross-service dependencies at planning showed the critical paths early.
- Testing against the real stack in the final sprint (Cypress end to end, JMeter, UAT) found the remaining integration bugs, and they were fixed within a day.

What to do differently:

- Plan on measured velocity from the start. Sprint 1 was planned at 80 points and delivered 49.
- Keep a schedule buffer. With no buffer, every slip moved the completion date, and two extensions were needed.
- Agree a cost baseline at the start. Without planned hours, the earned value report had to use the Sprint 0 rate as its baseline.
- Do not let one member hold several Must tasks that others depend on, and reassign at the day 2 check instead of at the next planning.
- Test against the real system early. Stubbed Cypress tests in Sprint 3 hid integration gaps that were found late (JT-90 in Sprint 3, JT-105 in UAT).
- Include end-to-end event verification in Docker Compose in the Definition of Done, so "Done" means the feature works in the running stack.
- Estimate every task and bug before it enters a sprint, so the points show all the work: Sprint 4 had the highest hours per point of the project, partly because of bug fixes without points.
- Start testing, monitoring and documentation tasks earlier, so they do not all fall into the final sprint.
- Produce every planning document named in the charter during initiation, or remove it from the charter through change control.
- Plan for hardware limits: the full parallel Compose build ran out of memory with about 7.75 GiB for Docker; building one image at a time took about 16 minutes.
- Reset test data before UAT, so scenarios start from a known state.
- Log time when it is spent, not as one entry at the end of a task, so actual cost reflects the real effort.

---

## 12. Sign-off

| Name | Role | Reviewed or accepted | Date |
|---|---|---|---|
| Phuntsho Namgyel | Product Owner | Approved in PR #101 | 20 Nov 2026 |
| Jigden Shakya | Scrum Master and project manager | Prepared | 20 Nov 2026 |
| Kinley Tobgay Lhendrup | Developer | | |
| Sangay Tenzin | Developer | | |
| Pema Namgay | Module tutor (reviewer) | | |
| Nima Yoezer | Module tutor (reviewer) | | |
