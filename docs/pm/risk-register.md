# JobTrack: Risk Register

| Item | Detail |
|---|---|
| Jira task | JT-26 (created), JT-37 (Sprint 1 update), JT-64 (Sprint 1 review and revised schedule), JT-89 (Sprint 2 review and Sprint 3 plan), JT-98 (Sprint 3 review and Sprint 4 plan), JT-100 (close-out at project closure) |
| Owner | Jigden Shakya (Scrum Master). Backup: Sangay Tenzin |
| Version | 1.6 (close-out at project closure) |
| Date | 20 Nov 2026 (1.0 on 2 Nov, 1.1 on 3 Nov, 1.2 on 6 Nov, 1.3 on 12 Nov, 1.4 on 14 Nov, 1.5 on 18 Nov) |
| Based on | Proposal risks R1 to R7 (26 Oct 2026), updated for the plan in AGENTS.md |
| Review | At every sprint retrospective (see Section 5) |

---

## 1. Scoring method

The register uses a probability and impact matrix, following PMI PMBOK practice.

Probability (P):

| Score | Level | Meaning |
|---|---|---|
| 1 | Very low | Unlikely to happen in this project |
| 2 | Low | Could happen but not expected |
| 3 | Medium | About as likely as not |
| 4 | High | Expected to happen at least once |
| 5 | Very high | Almost certain |

Impact (I):

| Score | Level | Meaning |
|---|---|---|
| 1 | Very low | Negligible, absorbed within a day |
| 2 | Low | Minor delay to one task |
| 3 | Medium | Delays a sprint goal or one service |
| 4 | High | Delays a milestone or a core workflow |
| 5 | Very high | Threatens the final submission or a required feature |

Score = P x I (1 to 25).

| Score | Rating | Action |
|---|---|---|
| 15 to 25 | High | Response actions active now; reviewed at every stand-up if triggered |
| 8 to 12 | Medium | Response actions planned; reviewed every retrospective |
| 1 to 6 | Low | Monitored; reviewed every retrospective |

Response strategies (threats): Avoid, Mitigate, Transfer, Accept, Escalate.

---

## 2. Risk register

| ID | Description | Category | P | I | Score | Rating | Strategy | Response actions | Owner | Trigger | Status |
|---|---|---|---|---|---|---|---|---|---|---|---|
| R1 | Schedule overrun: 20 microservices, 5 functions, gateway and frontend in about two weeks of development (from proposal R1). Probability raised from 4 to 5 on 5 Nov because the buffer was removed (Option B dates: Sprint 4 ends on 17 Nov), so any slip hits the final date directly. On 11 Nov the schedule was revised (Sprint 2 9 to 13 Nov, Sprint 3 14 to 16, Sprint 4 17 to 19; planned completion 19 Nov) and there is still no buffer. Sprint 1 delivered 49 of 80 points (15 of 26 tasks). Sprint 2 met its goal: 21 of 23 tasks (91 percent) and all 60 estimated points Done, including all 12 Musts, but only with the two-day extension (5 days instead of 3); JT-53 and JT-54 (Should, unestimated) were carried into Sprint 3, where JT-53 was Done on 14 Nov. Sprint 3 (14 to 16 Nov) was planned at 62 points (44 Must, 18 Should), on purpose above the figure of about 50, with the Must points within it (agreed with the Product Owner on 14 Nov). Sprint 3 missed its committed minimum: 17 of 25 tasks and 43 of 62 points Done, 34 of 44 Must points; 8 tasks (19 points) were carried into Sprint 4, 13 of those points held by one member. Sprint 4 (17 to 19 Nov) is the last sprint, planned at 57 points (39 Must, 18 Should); on 19 Nov it was extended by one day to 20 Nov 23:59, the new completion date, and cannot be extended further | Schedule | 5 | 5 | 25 | High | Mitigate | Copy every service from the golden template; build the core flow first; fixed sprint goals and daily stand-ups; track progress in Jira; Must tasks are the committed minimum of each sprint and unfinished Should tasks move to the top of the next sprint; lower-priority services (for example Saved Jobs, Analytics, Weekly Summary) delivered with basic operations if time runs short, keeping the count at 20; no buffer remains; plan on measured velocity (49 points in Sprint 1, 60 points over 5 days in Sprint 2) instead of the full backlog; Sprint 2 followed the scope priorities agreed with the Product Owner on 12 Nov, and each task keeps its required acceptance criteria when scope is kept minimal; Sprint 3 is not extended: unfinished Should tasks move to the top of Sprint 4, the carryovers JT-66 and JT-67 first, while JT-54 is done early because JT-83 (Must) needs it; cross-service dependencies of every Sprint 3 task are recorded in `docs/pm/sprint-3-planning.md` so blocking producers start first; in Sprint 4 a Must task is never cut, the slip order is JT-66, then JT-102 and JT-103, then the other Should tasks, and an unfinished Should task is recorded as not delivered in the closure report (JT-100) because there is no Sprint 5 or further extension (Sprint 4 was extended by one day to 20 Nov on 19 Nov); the Sprint 3 carryovers were reassigned on 17 Nov and every Must task is checked at each stand-up; dependencies of every Sprint 4 task are recorded in `docs/pm/sprint-4-planning.md` | Jigden | A sprint goal is not met, or by day 2 of a sprint less than half the sprint's story points are Done or In Review (Sprint 4: fewer than 29 of 57 points on 18 Nov) | Closed 20 Nov: occurred (triggered in every sprint; see Section 6) |
| R2 | Knative and RabbitMQ setup takes longer than planned or the RabbitMQ to Knative connection does not work (from proposal R3 and R4). Probability lowered from 4 to 2 on 12 Nov: JT-29 proved k3d with Knative and the forwarder fallback, JT-61 wired the forwarder and JT-62 delivered the first function (resume-validator); the remaining risk is running all 5 functions on the cluster | Technical | 2 | 4 | 8 | Medium | Mitigate | Time-boxed cluster spike in Sprint 0 (JT-29) before development depends on it; try the Knative RabbitMQ source first, then the forwarder fallback; document the chosen approach and setup commands in `docs/deployment`; record the decision in the AGENTS.md Decision log | Sangay | A function deployed in k3d does not receive its events through the forwarder, or a scheduled function does not fire from its PingSource | Closed 20 Nov: occurred in part (see Section 6) |
| R3 | Cluster resources: a single laptop cannot run the full system (20 services, Postgres, RabbitMQ, Knative, monitoring) (from proposal R3) | Resource | 3 | 4 | 12 | Medium | Mitigate | One PostgreSQL server for all services; daily development in Docker Compose, cluster only for integration and deployment; functions scale to zero; set container resource limits in manifests; host machine has 16 GB RAM or more (Phuntsho's laptop) | Sangay | Pods restart or are evicted for memory, or host memory stays above 90 percent | Closed 20 Nov: occurred in part (see Section 6) |
| R4 | Integration failures between services from contract or event mismatches, especially across teams (from proposal R2) | Technical | 3 | 4 | 12 | Medium | Mitigate | Write all `.proto` and event contracts in Sprint 0 (JT-18, JT-21, JT-23); `buf lint` and `buf breaking` in CI; shared event fixtures in `tests/contract`; contract changes posted for a cross-team look before merging (encouraged, not merge-blocking); merge to `main` at least once a day; `main` must always run in Docker Compose; from the Sprint 1 retrospective (12 Nov): list each task's cross-service dependencies at sprint planning and verify every new event end to end in Docker Compose (published, consumed, effect stored) before moving the task to Done | Phuntsho | `buf breaking` fails in CI, a consumer test fails against a fixture, or the main flow breaks after a merge | Closed 20 Nov: occurred (see Section 6) |
| R5 | Inconsistent AI-generated code: members use different assistants (Claude, ChatGPT, Codex, Copilot), which may produce code in different styles, layouts or tools. Probability raised from 4 to 5 on 3 Nov because PR reviews no longer block merging | Quality | 5 | 3 | 15 | High | Mitigate | AGENTS.md as the single source of truth attached to every AI session; golden service template; CI runs `gofmt`, `go vet` and tests and becomes a required check once JT-28 is merged; pair review encouraged on every PR; Definition of Done requires following AGENTS.md; members must run and test generated code before opening a PR | Phuntsho | A PR adds a tool or framework not in AGENTS.md, does not follow the hexagonal layout, or fails CI repeatedly | Closed 20 Nov: not confirmed by the risk owner (see Section 6) |
| R6 | Member unavailability or uneven workload due to other coursework, illness or travel (from proposal R5). Probability raised from 3 to 4 on 12 Nov: the trigger fired at the start of Sprint 2, when Kinley and Sangay were not available; their carryovers were reassigned on 9 Nov and handed back when they returned (Kinley on the evening of 9 Nov, Sangay on 10 Nov). In Sprint 3 the planned load was even (15 to 16 points per member), but Done points ranged from 14 to 3: one member finished 1 of 6 tasks and held 13 of the 19 carried points. Part of that work was reassigned at Sprint 4 planning on 17 Nov (JT-73 to Phuntsho, JT-72 and JT-70 to Jigden) | Human resource | 4 | 4 | 16 | High | Mitigate | Every service and lead area has a named backup (pair partner); handoff notes in the agreed format; blockers raised at the daily stand-up; Scrum Master reviews workload at sprint planning and reassigns in Jira when needed; blockers raised the same day they appear, not at the end of the sprint; after any reassignment the Jira assignee is kept aligned with the agreed owner; from Sprint 4, a Must task that has not moved by the next stand-up is reassigned through the handover rule the same day (Sprint 3 retrospective) | Jigden | A member misses a stand-up without an update, or a task stays In Progress for more than one day with no commits | Closed 20 Nov: occurred (triggered 9 Nov; see Section 6) |
| R7 | Learning curve with Go, gRPC, Kubernetes, Knative and the testing tools slows early sprints (from proposal R4) | Technical | 3 | 3 | 9 | Medium | Mitigate | Golden template and sample tests (JT-19, JT-20) show the patterns once; pair on the first services; step-by-step AI guidance following AGENTS.md; reuse practical code from earlier module labs | Phuntsho | First services in Sprint 1 take more than twice their estimate | Closed 20 Nov: not shown (see Section 6) |
| R8 | Load test with 200 concurrent users cannot run reliably on one laptop, or results are skewed by the host | Technical | 3 | 3 | 9 | Medium | Mitigate | Prepare the JMeter deadline-rush scenario early in Sprint 3; run JMeter from a different laptop than the cluster host; document the hardware limits in the load test report | Kinley | A trial run fails or the host freezes before 200 users are reached | Closed 20 Nov: did not occur (see Section 6) |
| R9 | Exposure of personal data such as resumes, or secrets committed to the repo (from proposal R6) | Security | 1 | 4 | 4 | Low | Avoid | Synthetic data only; JWT and role checks on every request; resume type and size validation; no secrets committed, only `.env.example` files with placeholder values | Sangay | Real personal data or a real secret appears in the repo or a PR | Closed 20 Nov: did not occur (see Section 6) |
| R10 | Final presentation and submission dates are not yet announced and may change, affecting the closure plan | Schedule | 3 | 2 | 6 | Low | Accept | Keep the sprint plan fixed; feature freeze at the end of Sprint 4 (20 Nov; moved from 17 to 19 Nov on 11 Nov and to 20 Nov on 19 Nov); update milestone M7 in the charter when dates are announced | Jigden | Tutors announce or change the presentation date | Open after closure: dates still not announced (see Section 6) |

---

## 3. Closed or retired risks

| ID | Description | Reason closed | Date |
|---|---|---|---|
| P-R7 | Email or SMS providers unavailable or costly (proposal R7) | SMS Sender dropped and real email replaced by Mailpit, so there is no external provider dependency | 2 Nov 2026 |
| R1 to R9 | All risks in Section 2 except R10 | Project closed at the end of Sprint 4; each risk is marked occurred or not, with the response taken, in Section 6. The rows stay in Section 2 as the record | 20 Nov 2026 |

---

## 4. Summary

| Rating | Count | Risks |
|---|---|---|
| High | 3 | R1, R5, R6 |
| Medium | 5 | R2, R3, R4, R7, R8 |
| Low | 2 | R9, R10 |

After the Sprint 1 review (12 Nov): R1 stays at 25 with the revised schedule, still no buffer, so the agreed scope priorities and planning on measured velocity are the main responses. R6 is now High because its trigger fired at the start of Sprint 2. R5 is still covered by AGENTS.md, the golden template and the CI pipeline (JT-28). R2 dropped to Medium after JT-29, JT-61 and JT-62.

After the Sprint 2 review (14 Nov): R1 stays at 25. Sprint 2 met its goal with 21 of 23 tasks and 60 points, but only with the extension, and Sprint 3 is planned at 62 points with no buffer, so unfinished Should tasks move to Sprint 4 instead of extending the sprint. R6 stays at 16; the Sprint 3 load is even at 15 to 16 points per member. The other risks were reviewed with no change in score.

After the Sprint 3 review (18 Nov): R1 stays at 25, the highest score. Sprint 3 missed its committed minimum (34 of 44 Must points), 19 points were carried into Sprint 4, and Sprint 4 is the last sprint with no buffer and no extension, so Must tasks are protected by the slip order and unfinished Should tasks are recorded as not delivered. The Sprint 4 day 2 check fired the R1 trigger (2 of 57 points Done or In Review on 18 Nov). R6 stays at 16: the Done load in Sprint 3 ranged from 14 to 3 points, and part of the carried work was reassigned on 17 Nov. The other risks are not changed in this update. On 19 Nov Sprint 4 was extended by one day to 20 Nov 23:59 (JT-67, AGENTS.md Section 15); R1 stays at 25, there is still no buffer and no further extension.

At project closure (20 Nov, JT-100): R1 to R9 are closed and R10 stays open until the final presentation (milestone M7). Of the 10 risks, 3 occurred (R1, R4, R6), 2 occurred in part (R2, R3), 2 did not occur (R8, R9), 2 could not be confirmed (R5, R7) and 1 has not happened yet (R10). The two highest risks both occurred: the schedule overran in every sprint (R1) and member availability and workload were uneven (R6). See Section 6 for each risk.

---

## 5. Review process

- The Scrum Master reviews this register at every sprint retrospective with the team.
- For each open risk: check the trigger, update P, I and the score, and record whether the response actions worked.
- New risks can be raised by anyone at any time (stand-up or WhatsApp) and are added at the next review.
- When a trigger fires, the owner raises it at the next stand-up and the response actions are carried out.
- Closed risks move to Section 3 with the reason and date.

### Review log

| Date | Sprint | Changes |
|---|---|---|
| 2 Nov 2026 | Sprint 0 | Initial register created from proposal risks and Sprint 0 planning |
| 3 Nov 2026 | Sprint 0 | Team agreed PR reviews do not block merging. R5 probability raised to 5 (now High); R4 and R5 responses updated |
| 3 Nov 2026 | Sprint 0 | Register agreed by all four members; risk owners accepted their risks (WhatsApp) |
| 5 Nov 2026 | Sprint 1 planning | Option B sprint dates removed the 16 to 17 Nov buffer. R1 probability raised to 5 (score 25); R1 response updated (buffer removed, Must tasks as committed minimum). R10 feature freeze date moved to 17 Nov |
| 12 Nov 2026 | Sprint 1 review (JT-64) | Revised schedule from 11 Nov (Sprint 2 9 to 13, Sprint 3 14 to 16, Sprint 4 17 to 19 Nov; completion 19 Nov; no buffer). R1 stays at 25: description, response (measured velocity, scope priorities agreed with the Product Owner) and status updated. R2 probability lowered to 2 (score 8, Medium) and trigger updated. R4 response extended with retrospective actions. R6 probability raised to 4 (score 16, High) after its trigger fired on 9 Nov. R10 feature freeze moved to 19 Nov. R3, R5, R7, R8 and R9 reviewed, no change |
| 14 Nov 2026 | Sprint 2 review and Sprint 3 planning (JT-89) | R1 stays at 25: description, response, trigger and status updated with the Sprint 2 result (21 of 23 tasks, 60 points, goal met with the two-day extension; JT-53 and JT-54 carried into Sprint 3) and the Sprint 3 plan (62 points, 44 Must and 18 Should, above the figure of about 50; unfinished Should tasks move to the top of Sprint 4; JT-54 done early for JT-83, agreed with the Product Owner). Summary updated. R2 to R10 reviewed, no change in score |
| 18 Nov 2026 | Sprint 3 review and Sprint 4 planning (JT-98) | R1 stays at 25: description, response, trigger and status updated with the Sprint 3 result (17 of 25 tasks, 43 of 62 points, 34 of 44 Must points; goal not met; 8 tasks and 19 points carried into Sprint 4) and the Sprint 4 plan (57 points, 39 Must and 18 Should; last sprint, no extension; slip order agreed with the Product Owner; unfinished Should tasks recorded as not delivered). R6 stays at 16: description and response updated (uneven Done load in Sprint 3, carryover reassignment). Summary updated. Other risks not changed |
| 19 Nov 2026 | Sprint 4 extension (JT-67) | Sprint 4 extended by one day to 20 Nov 23:59 (AGENTS.md Section 15, 19 Nov 2026). R1 stays at 25: description, response and status updated; there is still no buffer and no further extension. R10 feature freeze moved to 20 Nov. Summary updated. Other risks not reviewed in this update |
| 20 Nov 2026 | Project closure (JT-100) | Every risk marked occurred or not, with the response taken and its result (Section 6). R1 to R9 closed; R10 stays open until the final presentation. Status column, Section 3 and summary updated |

---

## 6. Close-out at project closure

Checked on 20 Nov 2026 at the end of Sprint 4 against each risk's trigger. "Occurred" means the risk event happened, even if the response limited its effect. Final sprint figures are in `docs/pm/sprint-4-review.md` and `docs/pm/project-closure-report.md`.

| ID | Risk | Occurred | What happened | Response taken | Result |
|---|---|---|---|---|---|
| R1 | Schedule overrun | Yes, in every sprint | Development start moved from 28 Oct to 2 Nov. Sprint 1 delivered 49 of 80 points. Sprint 2 met its goal only with a two-day extension. Sprint 3 missed its committed minimum (34 of 44 Must points) and carried 19 points into Sprint 4. The Sprint 4 day 2 check fired (2 of 57 points on 18 Nov), and Sprint 4 was extended by one day to 20 Nov. Planned completion moved from 17 to 19 and then to 20 Nov | Must tasks as the committed minimum; scope priorities agreed with the Product Owner (12 Nov); planning on measured velocity; Should tasks slipping to the next sprint, then the Sprint 4 slip order; reassignment of the Sprint 3 carryovers (17 Nov); two schedule extensions agreed with the Product Owner | All 39 Must points of Sprint 4 were Done, and 16 of 18 Should points; only JT-72 (2 points) was not delivered. Project completion moved from 17 to 20 Nov. The core workflow was verified end to end in UAT (JT-99). Earned value at completion: see `docs/pm/earned-value-report.md` |
| R2 | Knative and RabbitMQ setup | In part | The native Knative RabbitMQSource could not reconcile in the JT-29 spike without extra topology CRDs. All 5 functions were later deployed to k3d and their triggers verified (JT-92). In Docker Compose, notification-dispatcher received no events because its forwarder and queue were missing (JT-105, found in UAT) | Time-boxed spike in Sprint 0 (JT-29); switch to the Go forwarder fallback; shared forwarder with the JT-61 retry and dead-letter policy; decisions recorded in AGENTS.md | Knative on k3d works for all 5 functions. JT-105 was fixed on 19 Nov (PR #90: forwarder and queue added to Compose), and UAT-EM-05 and UAT-EM-07 passed on retest on 20 Nov |
| R3 | Cluster resources on one laptop | In part | Capacity for all 20 services in k3d was never proven, so only the 5 functions run on Knative in k3d and the 20 services stay in Docker Compose (18 Nov). The full parallel Compose build ran out of memory on a laptop with about 7.75 GiB for Docker; building one image at a time took about 16 minutes. During the 200-user load test (JT-96) the Docker host used at most about a quarter of its CPU allocation and about 42 percent of its memory allocation. Whether pods restarted or were evicted for memory on the cluster host was not confirmed by the risk owner (Sangay) by 20 Nov | One PostgreSQL server; daily work in Compose; functions scale to zero; reduced k3d scope agreed with the Product Owner; images built one at a time | The agreed deployment scope ran on the 16 GiB host. Confirming this scope against the WEB303 requirements is an open action for Phuntsho |
| R4 | Integration failures between services | Yes | Moderation removals did not publish `job.closed` (JT-90, fixed in Sprint 4). Notification-dispatcher received no events in Compose (JT-105, fixed on 19 Nov). Whether `buf breaking` failed in CI on any PR was not confirmed by the risk owner (Phuntsho) by 20 Nov | Contracts for all 20 services in Sprint 0; `buf lint` and `buf breaking` in CI; shared event fixtures; consumers built against fixtures; end-to-end verification in Compose; real Cypress tests against the Compose stack (JT-95) | The main workflow passed end to end in Cypress (JT-95) and UAT (JT-99), and 34 contract tests pass on the final `main` (JT-97). The two gaps above were found late, in Sprint 3 and in UAT, and both were fixed |
| R5 | Inconsistent AI-generated code | Not confirmed | The risk owner (Phuntsho) did not confirm by 20 Nov whether any PR added a tool not in AGENTS.md, broke the hexagonal layout or failed CI repeatedly. The final SonarQube run (JT-97) found 0 bugs and 0 vulnerabilities, but its quality gate failed on new code: 25 new code smells and 4.8 percent duplication, mainly template duplication | AGENTS.md attached to every AI session; golden service template; CI with `gofmt`, `go vet` and tests; SonarQube runs (JT-84, JT-97) | No defect from inconsistent code was recorded; the duplication and code smells are maintainability work after the feature freeze |
| R6 | Member unavailability or uneven workload | Yes | Trigger fired on 9 Nov: Kinley and Sangay were not available at the start of Sprint 2. In Sprint 3 the Done load ranged from 14 to 3 points, and one member held 13 of the 19 carried points | Carryovers reassigned on 9 Nov and handed back; Sprint 3 carryovers reassigned on 17 Nov (JT-73 to Phuntsho, JT-72 and JT-70 to Jigden); Must tasks checked at every stand-up in Sprint 4 | In Sprint 4, Sangay finished all four of his Must tasks (12 points), including the deployment path JT-71 and JT-92. Of the reassigned Should tasks, JT-70 was delivered and JT-72 was not |
| R7 | Learning curve | Not shown | The trigger (first services taking more than twice their estimate) cannot be measured as written, because estimates were story points, not hours. Sprint 1 used 0.51 h per point Done, the lowest of any sprint (`docs/pm/earned-value-report.md`), so the services that were finished did not take unusually long; but Sprint 1 finished only 49 of 80 points, which is recorded under R1 | Golden template and sample tests (JT-19, JT-20); step-by-step AI guidance following AGENTS.md; pairing on the first services | No delay was attributed to the learning curve |
| R8 | Load test on one laptop | No | The 200-user deadline-rush run on the Docker Compose stack completed with 2,003 requests and 0 errors. The Docker host (Phuntsho's laptop, 16 GiB RAM, 7.75 GiB for Docker) used at most about a quarter of its CPU allocation and about 42 percent of its memory allocation (`docs/testing/jt-96-load-test-report.md`) | Scenario prepared in Sprint 3 and Sprint 4 (JT-84); JMeter run and reported (JT-96); hardware limits documented in the report | The 200-user deadline-rush run was completed and reported (JT-96) |
| R9 | Personal data or secret exposure | No | No real personal data or real secret was reported in the repo or a PR. The risk owner (Sangay) did not confirm by 20 Nov | Synthetic data only; JWT and role checks on every request; resume type, size and signature validation; `.env.example` placeholders; internal token never in the gateway or frontend | Avoided |
| R10 | Presentation and submission dates | Not yet | On 20 Nov the final presentation and submission dates are still not announced and are likely to be postponed | Sprint plan kept fixed; feature freeze at the end of Sprint 4 (20 Nov) | Stays open after closure. Milestone M7 in the charter is updated when the dates are announced |
