# JobTrack: Sprint 1 Review and Retrospective

| Item | Detail |
|---|---|
| Jira task | JT-64 |
| Owner | Jigden Shakya (Scrum Master). Backup: Phuntsho Namgyel |
| Sprint | Sprint 1 (Jira sprint 35) |
| Sprint dates | 6 Nov 09:00 to 8 Nov 23:59 |
| Closed in Jira | 9 Nov 2026, 02:03 |
| Review and retrospective | Held asynchronously on 12 Nov 2026 by Jigden, with input from Phuntsho (Product Owner) on WhatsApp. Kinley and Sangay did not take part |
| Data source | Jira sprint and issue history, checked on 12 and 13 Nov 2026 |
| Related documents | `docs/pm/sprint-1-planning.md`, `docs/pm/risk-register.md` (version 1.3), AGENTS.md (Sections 14 and 15) |

---

## 1. Sprint goal and outcome

Goal: All 20 services and the gateway running in Docker Compose with their basic public operations and tests. Committed minimum: the Must tasks needed for the apply-for-a-job flow.

Outcome: partly met. 15 of 26 tasks were Done, including 8 of the 12 Must tasks. Auth, the gateway, employer, application, the internal-call helpers in `pkg` and the cluster spike were finished. Four Must tasks (JT-38 job-posting, JT-39 category-tag, JT-48 resume, JT-49 application-status) and seven Should tasks carried over to Sprint 2.

---

## 2. Results

| Measure | Tasks | Story points |
|---|---|---|
| Planned | 26 | 80 |
| Done in Sprint 1 | 15 | 49 |
| Carried over to Sprint 2 | 11 | 31 |
| Must (High): planned / done | 12 / 8 | 45 / 30 |
| Should (Medium): planned / done | 14 / 7 | 35 / 19 |

- Velocity (story points Done in the sprint): 49.
- Completion: 58 percent of tasks, 61 percent of story points.
- Velocity for later sprints is planned from this figure, not from the size of the backlog.

### By member

| Member | Planned points | Done tasks | Done points | Carried over |
|---|---|---|---|---|
| Jigden Shakya | 24 | 8 of 8 | 24 | None (JT-38 and JT-39 were Sangay's and reassigned in Sprint 2) |
| Phuntsho Namgyel | 22 | 6 of 6 | 22 | None |
| Kinley Tobgay Lhendrup | 20 | 1 of 7 | 3 | JT-22, JT-48 to JT-52 (17 points) |
| Sangay Tenzin | 14 | 0 of 5 | 0 | JT-38 to JT-42 (14 points) |
| Total | 80 | 15 of 26 | 49 | 11 tasks, 31 points |

---

## 3. Task outcomes

Owner is the Sprint 1 planned owner. Status as of 13 Nov 2026, 17:30.

| Key | Summary | Owner | Priority | Points | Sprint 1 result | Status now |
|---|---|---|---|---|---|---|
| JT-20 | Golden template tests and testing conventions | Kinley | Must | 3 | Done | Done |
| JT-22 | Draft test plan outline | Kinley | Should | 3 | Carried over | Done |
| JT-29 | Cluster spike: k3d, Knative and RabbitMQ events | Phuntsho | Must | 5 | Done | Done |
| JT-30 | Implement auth service basic operations | Jigden | Must | 5 | Done | Done |
| JT-31 | Implement API gateway with JWT verification | Jigden | Must | 5 | Done | Done |
| JT-32 | Implement employer service basic operations | Jigden | Must | 3 | Done | Done |
| JT-33 | Implement company-verification service basic operations | Jigden | Should | 3 | Done | Done |
| JT-34 | Implement moderation service basic operations | Jigden | Should | 3 | Done | Done |
| JT-35 | Implement audit-log service basic operations | Jigden | Should | 2 | Done | Done |
| JT-36 | Implement analytics service basic operations | Jigden | Should | 2 | Done | Done |
| JT-37 | Update sprint dates, decisions and risk register | Jigden | Must | 1 | Done | Done |
| JT-38 | Implement job-posting service basic operations | Sangay | Must | 5 | Carried over | Done (by Jigden) |
| JT-39 | Implement category-tag service with seeded data | Sangay | Must | 2 | Carried over | Done (by Jigden) |
| JT-40 | Implement job-search service basic operations | Sangay | Should | 3 | Carried over | Done |
| JT-41 | Implement saved-jobs service basic operations | Sangay | Should | 2 | Carried over | Done |
| JT-42 | Implement notification-inbox service basic operations | Sangay | Should | 2 | Carried over | Done |
| JT-43 | Implement application service basic operations | Phuntsho | Must | 5 | Done | Done |
| JT-44 | Add internal-call interceptor and headers to pkg | Phuntsho | Must | 3 | Done | Done |
| JT-45 | Implement offer service basic operations | Phuntsho | Should | 3 | Done | Done |
| JT-46 | Implement messaging service basic operations | Phuntsho | Should | 3 | Done | Done |
| JT-47 | Implement job-alert subscriptions | Phuntsho | Should | 3 | Done | Done |
| JT-48 | Implement resume upload and status | Kinley | Must | 3 | Carried over | Done |
| JT-49 | Implement application-status state machine | Kinley | Must | 5 | Carried over | Done |
| JT-50 | Implement applicant-profile service basic operations | Kinley | Should | 2 | Carried over | Done |
| JT-51 | Implement review service basic operations | Kinley | Should | 2 | Carried over | Done |
| JT-52 | Implement interview service basic operations | Kinley | Should | 2 | Carried over | Done |

All four Must carryovers were Done by 12 Nov, and all 11 carryovers were Done by 13 Nov.

### What happened to the carryovers

On 9 Nov Kinley and Sangay were not available at the start of Sprint 2, so the Scrum Master reassigned their carryovers: Kinley's to Phuntsho and Jigden, Sangay's to Jigden. Kinley took his tasks back on the evening of 9 Nov and completed them himself. Sangay's JT-40 to JT-42 were handed back to him on 10 Nov. Jigden kept JT-38 and JT-39; JT-39 also moved category-tag ownership to Jigden in AGENTS.md (Section 15, 12 Nov 2026).

---

## 4. What went well

- 15 of 26 tasks and 8 of 12 Musts were Done, covering auth, the gateway, employer, application and the internal-call helpers that the apply-for-a-job flow needs.
- The golden template, the Sprint 0 contracts and AGENTS.md kept every service in the same hexagonal layout, so services built by different members fit together.
- Each task recorded its decisions and boundary values in the AGENTS.md Decision log, which gives the testers clear values to test against.
- The cluster spike (JT-29) finished, and the RabbitMQ-to-Knative forwarder approach was chosen early, before Sprint 2 depended on it.
- All four Must carryovers (JT-38, JT-39, JT-48, JT-49) were Done in Sprint 2 by 12 Nov.

---

## 5. What to change

- Plan on measured velocity. Sprint 1 was planned at 80 points and delivered 49. Later sprints are planned at about 50 points with four members, and less when someone is unavailable.
- Identify cross-service dependencies earlier. At sprint planning, list each task's producer, consumer and client dependencies so that dependent work (for example JT-60 on JT-58) is ordered from the start.
- Verify events end to end. Mocked ports and unit tests are not enough once events are in place; each new event is checked in Docker Compose from publish to stored effect.
- Raise blockers sooner. Load and availability were uneven (Kinley 1 of 7 tasks, Sangay 0 of 5). Blockers are raised at the stand-up the same day, and the Scrum Master reassigns on day 2 of a sprint when the R1 trigger fires, not at the end.
- Keep Jira assignments aligned with agreed ownership. When work is reassigned or handed back, the Jira assignee is updated at the same time.

---

## 6. Action items

| Action | Owner | When |
|---|---|---|
| Plan Sprint 3 at about 50 points, Musts and carryovers first | Jigden | Sprint 3 planning, 14 Nov |
| List cross-service dependencies for every Sprint 3 task at planning | Jigden with each task owner | 14 Nov |
| Verify each new event end to end in Docker Compose before moving its task to Done | Each task owner | Sprints 2 to 4 |
| Check the R1 trigger on day 2 of each sprint and reassign through the handover rule if needed | Jigden | Day 2 of Sprints 3 and 4 |
| Raise blockers at the stand-up the same day | All members | Ongoing |
| Update the Jira assignee whenever work is reassigned or handed back | Jigden and the member taking the task | Ongoing |

---

## 7. Revised sprint schedule

Decided by Jigden as Scrum Master on 11 Nov 2026 and approved by Phuntsho as Product Owner on 12 Nov 2026. Recorded in AGENTS.md Section 15 (11 Nov 2026).

| Sprint | Previous dates | Revised dates | Jira |
|---|---|---|---|
| Sprint 2 | 9 to 11 Nov | 9 to 13 Nov | Sprint 36, active, ends 13 Nov 23:59 |
| Sprint 3 | 12 to 14 Nov | 14 to 16 Nov | Sprint 37, planned, 14 Nov 09:00 to 16 Nov 23:59 |
| Sprint 4 | 15 to 17 Nov | 17 to 19 Nov | Sprint 38, planned, 17 Nov 09:00 to 19 Nov 23:59 |

Planned project completion: 19 Nov 2026 (previously 17 Nov). There is still no buffer.

Reason: Sprint 1 finished 15 of 26 tasks (49 of 80 points), so 11 tasks (31 points) were carried into Sprint 2 on top of its event and function work, and Kinley and Sangay were not available at the start of Sprint 2.

Roles and service ownership stay as documented in AGENTS.md Sections 2, 4 and 5.

---

## 8. Scope priorities if time runs short

Agreed with Phuntsho as Product Owner on 12 Nov 2026 (WhatsApp):

1. The end-to-end apply-for-a-job flow: JT-58 first, then JT-60, which depends on it. JT-62 (resume-validator) was already Done.
2. The remaining basic service operations: JT-42 and JT-52.
3. JT-53 before JT-63, because the frontend depends on the gateway configuration.
4. JT-54 and JT-55, ordered by whether they block integration or testing.

If time becomes tight, lower-priority scope is kept minimal, but every task keeps its required acceptance criteria.

Status on 13 Nov: priorities 1 and 2 are Done (JT-58, JT-60, JT-62, JT-42, JT-52). JT-53, JT-54, JT-55 and JT-63 are still open.

---

## 9. Risks

- R1 (schedule overrun) stays at 25: the revised schedule still has no buffer. JT-58 and JT-60 (8 points of Sprint 2 Musts) were To Do on 12 Nov and treated as a carryover risk; both were Done on 13 Nov, so the Sprint 2 committed minimum is met. Any of JT-53, JT-54, JT-55 and JT-63 (Should) not Done when Sprint 2 ends move to the top of Sprint 3.
- R6 (member unavailability) raised to 16 (High) because its trigger fired on 9 Nov.
- R2 (Knative and RabbitMQ) lowered to 8 (Medium) after JT-29, JT-61 and JT-62.
- See the risk register, version 1.3.

---

## 10. Open items

- The project charter (objectives O2 and O3, milestones M2 to M6) and the sprint calendar in `docs/pm/definition-of-done.md` still show the original dates. They are outside JT-64; a follow-up task should align them with the revised schedule.
