# JobTrack: Risk Register

| Item | Detail |
|---|---|
| Jira task | JT-26 (created), JT-37 (Sprint 1 update) |
| Owner | Jigden Shakya (Scrum Master). Backup: Sangay Tenzin |
| Version | 1.2 (Sprint 1 planning update) |
| Date | 6 Nov 2026 (1.0 on 2 Nov, 1.1 on 3 Nov) |
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
| R1 | Schedule overrun: 20 microservices, 5 functions, gateway and frontend in about two weeks of development (from proposal R1). Probability raised from 4 to 5 on 5 Nov because the buffer was removed (Option B dates: Sprint 4 ends on 17 Nov), so any slip hits the final date directly | Schedule | 5 | 5 | 25 | High | Mitigate | Copy every service from the golden template; build the core flow first; fixed sprint goals and daily stand-ups; track progress in Jira; Must tasks are the committed minimum of each sprint and unfinished Should tasks move to the top of the next sprint; lower-priority services (for example Saved Jobs, Analytics, Weekly Summary) delivered with basic operations if time runs short, keeping the count at 20; no buffer remains | Jigden | A sprint goal is not met, or by day 2 of a sprint less than half the sprint's story points are Done or In Review | Open |
| R2 | Knative and RabbitMQ setup takes longer than planned or the RabbitMQ to Knative connection does not work (from proposal R3 and R4) | Technical | 4 | 4 | 16 | High | Mitigate | Time-boxed cluster spike in Sprint 0 (JT-29) before development depends on it; try the Knative RabbitMQ source first, then the forwarder fallback; document the chosen approach and setup commands in `docs/deployment`; record the decision in the AGENTS.md Decision log | Sangay | JT-29 not Done by the end of Sprint 0 (3 Nov), or the hello-world function does not receive a RabbitMQ message | Open |
| R3 | Cluster resources: a single laptop cannot run the full system (20 services, Postgres, RabbitMQ, Knative, monitoring) (from proposal R3) | Resource | 3 | 4 | 12 | Medium | Mitigate | One PostgreSQL server for all services; daily development in Docker Compose, cluster only for integration and deployment; functions scale to zero; set container resource limits in manifests; host machine has 16 GB RAM or more (Phuntsho's laptop) | Sangay | Pods restart or are evicted for memory, or host memory stays above 90 percent | Open |
| R4 | Integration failures between services from contract or event mismatches, especially across teams (from proposal R2) | Technical | 3 | 4 | 12 | Medium | Mitigate | Write all `.proto` and event contracts in Sprint 0 (JT-18, JT-21, JT-23); `buf lint` and `buf breaking` in CI; shared event fixtures in `tests/contract`; contract changes posted for a cross-team look before merging (encouraged, not merge-blocking); merge to `main` at least once a day; `main` must always run in Docker Compose | Phuntsho | `buf breaking` fails in CI, a consumer test fails against a fixture, or the main flow breaks after a merge | Open |
| R5 | Inconsistent AI-generated code: members use different assistants (Claude, ChatGPT, Codex, Copilot), which may produce code in different styles, layouts or tools. Probability raised from 4 to 5 on 3 Nov because PR reviews no longer block merging | Quality | 5 | 3 | 15 | High | Mitigate | AGENTS.md as the single source of truth attached to every AI session; golden service template; CI runs `gofmt`, `go vet` and tests and becomes a required check once JT-28 is merged; pair review encouraged on every PR; Definition of Done requires following AGENTS.md; members must run and test generated code before opening a PR | Phuntsho | A PR adds a tool or framework not in AGENTS.md, does not follow the hexagonal layout, or fails CI repeatedly | Open |
| R6 | Member unavailability or uneven workload due to other coursework, illness or travel (from proposal R5) | Human resource | 3 | 4 | 12 | Medium | Mitigate | Every service and lead area has a named backup (pair partner); handoff notes in the agreed format; blockers raised at the daily stand-up; Scrum Master reviews workload at sprint planning and reassigns in Jira when needed | Jigden | A member misses a stand-up without an update, or a task stays In Progress for more than one day with no commits | Open |
| R7 | Learning curve with Go, gRPC, Kubernetes, Knative and the testing tools slows early sprints (from proposal R4) | Technical | 3 | 3 | 9 | Medium | Mitigate | Golden template and sample tests (JT-19, JT-20) show the patterns once; pair on the first services; step-by-step AI guidance following AGENTS.md; reuse practical code from earlier module labs | Phuntsho | First services in Sprint 1 take more than twice their estimate | Open |
| R8 | Load test with 200 concurrent users cannot run reliably on one laptop, or results are skewed by the host | Technical | 3 | 3 | 9 | Medium | Mitigate | Prepare the JMeter deadline-rush scenario early in Sprint 3; run JMeter from a different laptop than the cluster host; document the hardware limits in the load test report | Kinley | A trial run fails or the host freezes before 200 users are reached | Open |
| R9 | Exposure of personal data such as resumes, or secrets committed to the repo (from proposal R6) | Security | 1 | 4 | 4 | Low | Avoid | Synthetic data only; JWT and role checks on every request; resume type and size validation; no secrets committed, only `.env.example` files with placeholder values | Sangay | Real personal data or a real secret appears in the repo or a PR | Open |
| R10 | Final presentation and submission dates are not yet announced and may change, affecting the closure plan | Schedule | 3 | 2 | 6 | Low | Accept | Keep the sprint plan fixed; feature freeze at the end of Sprint 4 (17 Nov); update milestone M7 in the charter when dates are announced | Jigden | Tutors announce or change the presentation date | Open |

---

## 3. Closed or retired risks

| ID | Description | Reason closed | Date |
|---|---|---|---|
| P-R7 | Email or SMS providers unavailable or costly (proposal R7) | SMS Sender dropped and real email replaced by Mailpit, so there is no external provider dependency | 2 Nov 2026 |

---

## 4. Summary

| Rating | Count | Risks |
|---|---|---|
| High | 3 | R1, R2, R5 |
| Medium | 5 | R3, R4, R6, R7, R8 |
| Low | 2 | R9, R10 |

All three high risks are addressed in Sprint 0: R1 through fixed sprint goals and the golden template, R2 through the JT-29 cluster spike, and R5 through AGENTS.md, the golden template and the CI pipeline (JT-28).

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