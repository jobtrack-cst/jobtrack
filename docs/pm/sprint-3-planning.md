# JobTrack: Sprint 3 Planning Record

| Item | Detail |
|---|---|
| Jira task | JT-89 |
| Owner | Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup |
| Planning date | 14 Nov 2026 (plan agreed with Phuntsho Namgyel as Product Owner) |
| Sprint started | 14 Nov 2026, 09:00 (Jira sprint 37) |
| Sprint dates | 14 Nov 09:00 to 16 Nov 23:59 |
| Related documents | `docs/pm/sprint-2-review.md`, `docs/pm/risk-register.md` (version 1.4), AGENTS.md (Sections 14 and 15) |

---

## 1. Sprint goal

Full business logic, all 5 functions, frontend for all roles, integration tests. Committed minimum: the Must tasks (44 points).

---

## 2. Scope rules

- Services publish and consume their remaining domain events, the four remaining functions are built, and each member builds the frontend pages for their own services.
- Must tasks (Jira priority High) are the committed minimum. Should tasks (Medium) are planned; unfinished ones move to the top of Sprint 4.
- The carryovers JT-54, JT-66 and JT-67 are Should tasks. They keep their size and count inside each owner's share. JT-66 and JT-67 slip first. JT-54 is the exception and is done early, because JT-83 (Must) needs it for resume upload through the gateway.
- A consumer whose producer is not ready builds and tests against the shared fixtures in `tests/contract/fixtures/<routing key>.json`, so no task waits for another to merge.
- No sub-tasks in Sprint 3. The acceptance criteria in each task are the checklist, including the handover rule.

---

## 3. Sprint backlog

| Key | Summary | Owner | Priority | Points |
|---|---|---|---|---|
| JT-54 | Gateway: limit the request body size (carried over) | Jigden | Should | 2 |
| JT-66 | Decide and implement error logging for unexpected gRPC errors (carried over) | Phuntsho | Should | 3 |
| JT-67 | Align project charter and sprint calendar with the revised schedule (carried over) | Jigden | Should | 1 |
| JT-68 | Job-posting: publish job events through an outbox, consume moderation.job_removed | Sangay | Must | 3 |
| JT-69 | Job-search: build the read model from job events | Sangay | Must | 2 |
| JT-70 | Saved-jobs: consume job.closed | Sangay | Should | 1 |
| JT-71 | Implement notification-dispatcher Knative function | Sangay | Must | 5 |
| JT-72 | Audit-log: consume all domain events | Sangay | Should | 2 |
| JT-73 | Frontend: employer job pages, job search, saved jobs and notifications | Sangay | Must | 3 |
| JT-74 | Offer: consume application.submitted, publish offer events | Phuntsho | Must | 3 |
| JT-75 | Messaging: consume application.submitted, publish message.sent | Phuntsho | Should | 2 |
| JT-76 | Job-alert: consume job.published, real ResolveNames client, publish job_alert.matched | Phuntsho | Should | 3 |
| JT-77 | Implement email-sender Knative function | Phuntsho | Must | 2 |
| JT-78 | Frontend: apply for a job, my applications, offers, messages and job alerts | Phuntsho | Must | 3 |
| JT-79 | Review: consume application.submitted, publish review.completed | Kinley | Must | 2 |
| JT-80 | Interview: consume application.submitted, publish interview.scheduled | Kinley | Must | 2 |
| JT-81 | Application-status: consume review, interview and offer events | Kinley | Must | 3 |
| JT-82 | Implement deadline-reminder Knative function | Kinley | Must | 3 |
| JT-83 | Frontend: applicant profile, resume upload, application status, employer review and interview | Kinley | Must | 3 |
| JT-84 | Sprint 3 SonarQube run and JMeter deadline-rush scenario preparation | Kinley | Should | 2 |
| JT-85 | Analytics: consume outcome events | Jigden | Must | 2 |
| JT-86 | Implement weekly-summary Knative function | Jigden | Must | 3 |
| JT-87 | Moderation: publish moderation.job_removed through an outbox | Jigden | Should | 2 |
| JT-88 | Frontend: company profile, verification request, admin verification, moderation, analytics and audit log | Jigden | Must | 3 |
| JT-89 | Sprint 2 review, Sprint 3 planning record, AGENTS.md and risk register update | Jigden | Must | 2 |

25 tasks, 62 points: 22 new (JT-68 to JT-89) and 3 carried over (JT-54 from Sprint 2; JT-66 and JT-67 found during Sprint 2). 16 Must tasks (44 points) and 9 Should tasks (18 points).

JT-53 (gateway CORS) was Done in Sprint 2 but had been added to Sprint 3 in Jira; it was removed from Sprint 3 on 14 Nov.

---

## 4. Capacity and load

| Member | Must points | Should points | Total points |
|---|---|---|---|
| Jigden Shakya | 10 | 5 | 15 |
| Kinley Tobgay Lhendrup | 13 | 2 | 15 |
| Phuntsho Namgyel | 8 | 8 | 16 |
| Sangay Tenzin | 13 | 3 | 16 |
| Total | 44 | 18 | 62 |

Planned on purpose at 62 points, above the figure of about 50 from the Sprint 1 retrospective (Option 1, agreed with Phuntsho as Product Owner on 14 Nov 2026). The Must points (44) stay within that figure. Sprint 2 delivered 60 points, but over 5 days. Unfinished Should tasks move to the top of Sprint 4; Sprint 3 is not extended.

Load is even (15 to 16 points each). Kinley and Sangay carry the most Must points (13 each), so their Must tasks are checked first at the day 2 R1 review (15 Nov).

---

## 5. Cross-service dependencies

Recorded per task, following the Sprint 1 retrospective action (R4). "Done" means the dependency is already in `main`. A consumer whose producer is not ready builds against the contract fixtures first and is verified end to end in Docker Compose once the producer is merged.

| Key | Depends on | Needed by | Notes |
|---|---|---|---|
| JT-54 | None | JT-83 (resume upload through the gateway) | Maximum body size agreed with Kinley (resume owner) |
| JT-66 | None | None | Applies to all services; design decided by Phuntsho as architecture lead |
| JT-67 | None | None | Documentation only |
| JT-68 | JT-87 for moderation removal (fixture until merged) | JT-69, JT-70, JT-76; job events also feed JT-72 and JT-85 | Moderation removal needs both JT-68 and JT-87 |
| JT-69 | JT-68 (`job.published`, `job.updated`, `job.closed`) | JT-73 (job search page) | |
| JT-70 | JT-68 (`job.closed`) | None | |
| JT-71 | JT-77 (email-sender); events from JT-60 and JT-56 (Done), JT-74, JT-75, JT-76, JT-80 and JT-82 | None | Calls notification-inbox and auth internal RPCs (Section 6); fixtures for producers not yet merged |
| JT-72 | All producers (fixtures until merged) | JT-88 (audit log page) | Owner of the service: Jigden (reviews the PR) |
| JT-73 | JT-63 and JT-53 (Done); JT-69 | None | |
| JT-74 | `application.submitted` from JT-58 (Done) | JT-81; offer events also feed JT-71, JT-72 and JT-85 | |
| JT-75 | `application.submitted` from JT-58 (Done) | JT-71 (`message.sent`) | |
| JT-76 | JT-68 (`job.published`); `ResolveNames` from JT-39 (Done) | JT-71 (`job_alert.matched`) | |
| JT-77 | None | JT-71, JT-86 | Owner of the function: Sangay (reviews the PR) |
| JT-78 | JT-63 and JT-53 (Done) | None | |
| JT-79 | `application.submitted` from JT-58 (Done) | JT-81 | |
| JT-80 | `application.submitted` from JT-58 (Done) | JT-81; `interview.scheduled` also feeds JT-71 | |
| JT-81 | JT-74, JT-79, JT-80 | None | |
| JT-82 | Job-posting `ListJobsClosingBetween` (JT-38, Done); saved-jobs `ListJobSavers` | JT-71 (`deadline.reminder`) | |
| JT-83 | JT-63 and JT-53 (Done); JT-54 | None | |
| JT-84 | None | None | Prepares the Sprint 4 load test (R8) |
| JT-85 | Job events from JT-68; application events from JT-58 and JT-60 (Done); offer events from JT-74 | JT-86; JT-88 (analytics page) | Fixtures for producers not yet merged |
| JT-86 | JT-85, JT-77 | None | Calls auth `ListUsersByRole` (Section 6) |
| JT-87 | None | JT-68 (moderation removal) | |
| JT-88 | JT-63 and JT-53 (Done) | None | Analytics and audit-log pages use JT-85 and JT-72 data |
| JT-89 | None | None | |

Order to start: the producers that block others (JT-68, JT-74, JT-77, JT-79, JT-80, JT-85, JT-87 and JT-54) go first on 14 Nov.

---

## 6. Decisions made

Recorded in the AGENTS.md Decision log (Section 15) on 14 Nov 2026, agreed with Phuntsho as Product Owner:

- Option 1: Sprint 3 is planned at 62 points (44 Must, 18 Should), above the figure of about 50 from the Sprint 1 retrospective. The Must points stay within it. Unfinished Should tasks move to the top of Sprint 4; Sprint 3 is not extended.
- The carryovers JT-54, JT-66 and JT-67 are Should tasks, keep their size and count inside each owner's share. JT-66 and JT-67 slip first.
- JT-54 is done early instead of slipping, because JT-83 (Must) needs it for resume upload through the gateway (proposed by Jigden as Scrum Master during JT-89, for Phuntsho as Product Owner to confirm in the JT-89 PR).
- JT-77 (email-sender) is assigned to Phuntsho as DevOps backup for this sprint; Sangay, as the function's owner, reviews the PR.
- JT-72 (audit-log consumer) is assigned to Sangay as Jigden's backup for this sprint; Jigden, as the service's owner, reviews the PR.
- Ownership in AGENTS.md Sections 4 and 5 is unchanged.
- JT-66 (error logging) is assigned to Phuntsho as architecture lead.
- Consumers build against the contract fixtures when the producer is not ready.
- No sub-tasks in Sprint 3.

---

## 7. Risks

- R1 (schedule overrun) stays at 25 (High). The schedule has no buffer, and Sprint 3 is planned above the figure of about 50. The trigger is checked on 15 Nov (day 2): less than half of the 62 points (31) Done or In Review fires it.
- R6 (member unavailability or uneven workload) stays at 16 (High). Load is now even, and each task has a named backup through the pairs.
- R2 (Knative and RabbitMQ) covers the four functions built in this sprint (JT-71, JT-77, JT-82, JT-86).
- R4 (integration failures) is addressed by the dependency table in Section 5, the contract fixtures and end-to-end checks in Docker Compose.
- R8 (load test) is addressed by JT-84.
- See the risk register, version 1.4.

---

## 8. Moved to Sprint 4

- Unfinished Should tasks from Sprint 3, JT-66 and JT-67 first (top of the Sprint 4 backlog)
- End-to-end and load tests, UAT, bug fixes, monitoring and all documentation, as planned in AGENTS.md Section 14
- Feature freeze at the end of Sprint 4 (19 Nov)

---

## 9. Open items

- None.
