# JobTrack: Project Closure Report

| Item | Detail |
|---|---|
| Project | JobTrack: A Job and Internship Application Portal |
| Jira task | JT-100 |
| Prepared by | Jigden Shakya (Scrum Master and project manager). Backup: Kinley Tobgay Lhendrup |
| Version | 1.0 (draft) |
| Date | 20 Nov 2026 |
| Project dates | 2 Nov 2026 (Sprint 0 start) to 20 Nov 2026 (Sprint 4 end, 23:59 Asia/Thimphu) |
| Reviewed by | Phuntsho Namgyel (Product Owner): TBD (20 Nov) |
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
| System | 20 microservices, gateway, 5 Knative functions, React frontend | TBD (20 Nov): confirm all 20 services, the gateway, 5 functions and the frontend are on `main` |
| Tools | Jira (space JT), GitHub, GitHub Actions | As planned |

---

## 3. Objectives against the charter

| # | Objective | Success criteria (charter Section 2) | Result | Status |
|---|---|---|---|---|
| O1 | Establish the development foundation and contracts | Repo, `.proto` contracts for all 20 services, golden template, CI, Docker Compose and a proven cluster setup by 3 Nov | Contracts for all 20 services were agreed in PR #6 on 3 Nov; golden template, CI and Compose were set up in Sprint 0. The cluster spike (JT-29) was verified on 7 Nov, four days after Sprint 0 ended, and selected the forwarder fallback for RabbitMQ events | Met, with the cluster setup late |
| O2 | Build and deploy the full system | All 20 microservices, the gateway, 5 functions and the frontend running; the main workflow works end to end on the k3d cluster | The main workflow was verified end to end in Docker Compose by Cypress (JT-95) and UAT (JT-99). On 18 Nov the Product Owner agreed a reduced k3d scope: all 5 functions run on Knative in k3d (JT-92) and the 20 services stay in Docker Compose, because capacity for all 20 services in k3d was not proven. TBD (20 Nov): JT-105 (notifications in Compose) | Partly met: the workflow runs in Compose, not on k3d |
| O3 | Prove quality through systematic testing | At least 80 percent line coverage on `application` and `application-status`; contract checks in CI; Cypress end-to-end tests; JMeter load test with 200 users; results documented | Contract checks (`buf breaking` and event fixtures) run in CI. Cypress end-to-end tests run the main workflow against the Compose stack (JT-95). The 200-user JMeter run was completed and reported (JT-96). TBD (20 Nov, from JT-97): coverage figures and the test summary report | TBD |
| O4 | Manage the project with Scrum and PMBOK artefacts | Ceremonies held; Jira current with time logged; risk register reviewed every retrospective; earned value from Jira time logs; lessons learned at closure | Planning, reviews and retrospectives were recorded for Sprints 1 to 4 (the Sprint 3 review was held within Sprint 4 planning, and Sprint 0 has no review record). The risk register was reviewed at every review (review log, versions 1.0 to 1.6). Earned value: TBD (20 Nov). Lessons learned: Section 11 | TBD |

---

## 4. Exit criteria (charter Section 13)

| # | Exit criterion | Status |
|---|---|---|
| 1 | All in-scope services, functions, the gateway and the frontend are deployed and the main workflow works end to end | Partly met: deployed in Docker Compose with the 5 functions on Knative in k3d; the main workflow passed in Cypress and UAT. Conditions JT-105 and JT-107: TBD (20 Nov) |
| 2 | Objectives O1 to O4 are met, or any gap is documented in the closure report | Gaps documented in Sections 3 and 5 |
| 3 | All deliverables in charter Section 5 are in the repo and reviewed by the team | TBD (20 Nov): see Section 5.3 |
| 4 | The final presentation is given and assessed by the module tutors | Not yet: dates not announced (milestone M7, risk R10) |
| 5 | The lessons-learned and closure report is completed | This report, once reviewed by the Product Owner |

---

## 5. Scope

### 5.1 Delivered

- 20 microservices in Go, each with its own PostgreSQL database, hexagonal layout and gRPC contract (charter Section 4.1). TBD (20 Nov): confirm all 20 run in Docker Compose on the final `main`.
- API gateway with JWT checks, CORS for the frontend and a 3 MiB body limit (JT-31, JT-53, JT-54).
- 5 Knative functions: notification-dispatcher, email-sender, weekly-summary, resume-validator and deadline-reminder, deployed to k3d with their triggers verified (JT-92).
- Domain events over RabbitMQ with transactional outboxes, idempotent consumers and a shared retry and dead-letter policy (JT-56, JT-61).
- React frontend for applicants, employers and admins (JT-78, JT-83, JT-88, JT-73).
- Docker Compose for local running, CI with GitHub Actions, and Prometheus and Grafana in Compose (JT-93).
- Tests: unit, integration (Testcontainers), contract, Cypress end-to-end against the Compose stack, JMeter load test and SonarQube runs.

### 5.2 Not delivered or changed

| Item | Charter reference | What happened | Decision |
|---|---|---|---|
| Main workflow on the k3d cluster | O2 | 20 services stay in Docker Compose; only the 5 functions run in k3d | Agreed with the Product Owner on 18 Nov (AGENTS.md Section 15); to be confirmed against the WEB303 requirements (Phuntsho) |
| Recording interview outcomes | Section 4.1, Interview | `interview.proto` has no outcome field or RPC; only scheduling and reading were built (JT-52) | Needs a contract change; not agreed (AGENTS.md Section 16) |
| Employer view of applicant resumes | Section 3, review step | `GetResume` and `DownloadResume` are owner only | Open question; backlog consideration only (Section 10) |
| NetworkPolicies for internal RPCs | AGENTS.md Section 6, layer 3 | Not written; layers 1 and 2 are in place | Open item (Section 10) |
| Unfinished Sprint 4 Should tasks | Sprint 4 plan | TBD (20 Nov): list from `docs/pm/sprint-4-review.md` Section 3, for example JT-66, JT-70, JT-72, JT-101, JT-102 and JT-103 if not Done | Recorded as not delivered; no Sprint 5 (Section 15, 18 Nov 2026) |

The out-of-scope items in charter Section 4.4 (payments, real SMS and email, resume parsing, mobile apps, production hosting, national systems, real personal data) stayed out of scope.

### 5.3 Deliverables (charter Section 5)

| Area | Deliverable | Status |
|---|---|---|
| Initiation and planning | Project proposal, charter, Definition of Done and working agreement, sprint calendar, risk register, product backlog in Jira | Done (`docs/pm`, Jira) |
| Initiation and planning | Stakeholder register, SRS, WBS, quality management plan | Not in the repo (checked 19 Nov). TBD (20 Nov): confirm with the team whether they were submitted separately for SWE303; if not, record them as not delivered in Section 5.2 |
| Design | Architecture and service catalogue (AGENTS.md), `.proto` and event contracts | Done |
| Development | Source code for 20 services, gateway, 5 functions and frontend; Dockerfiles, Kubernetes and Knative manifests, CI pipeline | TBD (20 Nov): confirm the Kustomize manifests in `deploy/k8s` |
| Testing | Test plan, test case document with traceability, coverage and SonarQube reports, load test report, defect log in Jira | Load test report Done (JT-96); the rest TBD (20 Nov, from JT-97) |
| Deployment | Running deployment on k3d, Grafana dashboards, deployment guide | Functions on k3d (JT-92), Grafana (JT-93), deployment documentation (JT-94) |
| Closure | Sprint reviews, earned value report, final presentation, lessons-learned and closure report | Sprint 1 to 4 reviews; earned value report TBD (20 Nov); closure report (this document); final presentation not yet scheduled |

---

## 6. Schedule

Baseline: the Option B dates agreed on 5 Nov 2026. Revised: the schedule agreed on 11 Nov 2026 (JT-64).

| Milestone | Baseline | Revised | Actual | Result |
|---|---|---|---|---|
| M1 Sprint 0 complete | 3 Nov | 3 Nov | 3 Nov | Met, except the cluster spike (7 Nov) |
| M2 Sprint 1 complete | 8 Nov | 8 Nov | 8 Nov | Goal partly met: 15 of 26 tasks, 49 of 80 points |
| M3 Sprint 2 complete | 11 Nov | 13 Nov | 13 Nov | Goal met with the two-day extension: 21 of 23 tasks, 60 points |
| M4 Sprint 3 complete | 14 Nov | 16 Nov | 16 Nov | Goal not met: 17 of 25 tasks, 43 of 62 points, 34 of 44 Must points |
| M5 Sprint 4 complete | 17 Nov | 19 Nov | 20 Nov | TBD (20 Nov) |
| M6 Project completion | 17 Nov | 19 Nov | 20 Nov | Three days later than the baseline |
| M7 Final presentation | To be announced | To be announced | Not yet | Open (R10) |

### Velocity

| Sprint | Days | Planned points | Done points | Points per day |
|---|---|---|---|---|
| Sprint 0 | 2 | TBD (Jira) | TBD (Jira) | TBD |
| Sprint 1 | 3 | 80 | 49 | about 16 |
| Sprint 2 | 5 | 60 | 60 | about 12 |
| Sprint 3 | 3 | 62 | 43 | about 14 |
| Sprint 4 | 4 | 57 | TBD | TBD |

Velocity and burndown charts for every sprint are in `docs/pm/earned-value-report.md`.

---

## 7. Cost and earned value

The project had no funding (charter Section 8). Cost is measured as the hours logged in Jira, and value as story points Done.

TBD (20 Nov): PV, EV, AC, SV, CV, SPI and CPI at closure, with a short interpretation. The full calculation, method and charts are in `docs/pm/earned-value-report.md`.

---

## 8. Quality

- UAT (JT-99, 19 Nov): 34 scenarios, 29 Pass, 3 Fail, 2 Not run. The core workflow was verified end to end with no Critical defects. Product Owner sign-off: Accepted with conditions (details in `docs/pm/sprint-4-review.md` Section 4).
- Conditions: JT-105 (Major, notification-dispatcher gets no events in Compose) and JT-107 (Major, resume-validator rejects PDFs with cross-reference streams), to be fixed and retested by 20 Nov 23:59. Result: TBD (20 Nov). Any condition not met is disclosed here.
- Load test (JT-96): the 200-user deadline-rush run, reported in `docs/testing/jt-96-load-test-report.md`.
- Test counts, coverage, SonarQube and the defect log: TBD (20 Nov), from `docs/testing/test-summary-report.md` (JT-97).

Known issues at closure:

| Bug | Severity | Issue | Workaround |
|---|---|---|---|
| JT-108 | Major | The offer ID is shown only once, right after the employer makes the offer | The employer copies the offer ID shown after making the offer and sends it to the applicant in Messages |
| JT-106 | Minor | The login is lost when the page is reloaded | Log in again after a reload |

---

## 9. Risks

All risks are closed out in `docs/pm/risk-register.md` Section 6. The two highest risks both occurred:

- R1 (schedule overrun): every sprint was affected, two extensions were needed and completion moved from 17 to 20 Nov. Must work was protected by the committed-minimum rule and the slip order.
- R6 (member unavailability and uneven workload): the trigger fired on 9 Nov, and in Sprint 3 the Done load ranged from 14 to 3 points. Work was reassigned within the pairs on 9 Nov and 17 Nov.

R2 and R3 occurred in part and led to the forwarder fallback and the reduced k3d scope. R10 stays open until the final presentation.

---

## 10. Open items and handover

| Item | Owner | Reference |
|---|---|---|
| Confirm the WEB303 deployment scope against the official assessment requirements | Phuntsho | AGENTS.md Section 16 |
| Final presentation and submission dates; update milestone M7 | Jigden | R10 |
| JT-105 and JT-107 if not fixed by 20 Nov 23:59 | Sangay, Phuntsho | Section 8 |
| Known issues JT-106 and JT-108 | Backlog | Section 8 |
| JT-109: applicant profile read-only view with an Edit button | Kinley, backlog after the project | `docs/pm/sprint-4-review.md` |
| Employer access to applicant resumes; needs Phuntsho's authorisation and a contract review; no Jira issue yet | Phuntsho | AGENTS.md Section 16 |
| Interview outcome recording (contract change) | Kinley with the team | AGENTS.md Section 16 |
| NetworkPolicies for internal RPCs in `deploy/k8s` | Sangay | AGENTS.md Section 6 |
| Other open questions in AGENTS.md Section 16 | Per item | AGENTS.md Section 16 |

---

## 11. Lessons learned

Collected from the Sprint 1 to 4 retrospectives. TBD (20 Nov): add the team's input from the final retrospective.

What worked:

- Writing all `.proto` and event contracts in Sprint 0 let both teams build in parallel, first with mocked ports and later against shared event fixtures.
- One source of truth (AGENTS.md with its decision log) kept four members and their AI assistants on the same architecture, conventions and decisions.
- The golden service template and sample tests gave every service the same layout and test style.
- Making Must tasks the committed minimum, with an agreed slip order, protected the core workflow when time ran short.
- Recording cross-service dependencies at planning showed the critical paths early.

What to do differently:

- Plan on measured velocity from the start. Sprint 1 was planned at 80 points and delivered 49.
- Keep a schedule buffer. With no buffer, every slip moved the completion date, and two extensions were needed.
- Do not let one member hold several Must tasks that others depend on, and reassign at the day 2 check instead of at the next planning.
- Test against the real system early. Stubbed Cypress tests in Sprint 3 hid integration gaps that were found late (JT-90 in Sprint 3, JT-105 in UAT).
- Include end-to-end event verification in Docker Compose in the Definition of Done, so "Done" means the feature works in the running stack.
- Estimate every task and bug before it enters a sprint, so the points show all the work.
- Start testing, monitoring and documentation tasks earlier, so they do not all fall into the final sprint.
- Plan for hardware limits: the full parallel Compose build ran out of memory with about 7.75 GiB for Docker; building one image at a time took about 16 minutes.
- Reset test data before UAT, so scenarios start from a known state.

---

## 12. Sign-off

| Name | Role | Reviewed or accepted | Date |
|---|---|---|---|
| Phuntsho Namgyel | Product Owner | TBD | |
| Jigden Shakya | Scrum Master and project manager | TBD | |
| Kinley Tobgay Lhendrup | Developer | TBD | |
| Sangay Tenzin | Developer | TBD | |
| Pema Namgay | Module tutor (reviewer) | | |
| Nima Yoezer | Module tutor (reviewer) | | |
