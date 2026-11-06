# JobTrack: Sprint 1 Planning Record

| Item | Detail |
|---|---|
| Jira task | JT-37 |
| Owner | Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup |
| Planning date | 5 Nov 2026 |
| Sprint started | 6 Nov 2026, 09:00 (Jira Sprint 1) |
| Sprint dates | 6 Nov 09:00 to 8 Nov 23:59 |
| Related documents | AGENTS.md (Sections 14 and 15), `docs/pm/risk-register.md` (version 1.2) |

---

## 1. Sprint goal

All 20 services and the gateway running in Docker Compose with their basic public operations and tests. Committed minimum: the Must tasks needed for the apply-for-a-job flow.

---

## 2. Scope rules

- Each service works on its own with public RPCs, authorisation, migrations, tests and Docker Compose.
- Cross-service calls are port interfaces mocked with gomock. Events and real cross-service clients move to Sprint 2.
- Must tasks (Jira priority High) are the committed minimum. Should tasks (Medium) are planned; unfinished ones move to the top of Sprint 2.
- No sub-tasks in Sprint 1. The acceptance criteria in each task are the checklist, including the handover rule.

---

## 3. Sprint backlog

| Key | Summary | Owner | Priority | Points |
|---|---|---|---|---|
| JT-20 | Golden template tests and testing conventions (carried over) | Kinley | Must | 3 |
| JT-22 | Draft test plan outline (carried over) | Kinley | Should | 3 |
| JT-29 | Cluster spike: k3d, Knative and RabbitMQ events (carried over) | Phuntsho | Must | 5 |
| JT-30 | Implement auth service basic operations | Jigden | Must | 5 |
| JT-31 | Implement API gateway with JWT verification | Jigden | Must | 5 |
| JT-32 | Implement employer service basic operations | Jigden | Must | 3 |
| JT-33 | Implement company-verification service basic operations | Jigden | Should | 3 |
| JT-34 | Implement moderation service basic operations | Jigden | Should | 3 |
| JT-35 | Implement audit-log service basic operations | Jigden | Should | 2 |
| JT-36 | Implement analytics service basic operations | Jigden | Should | 2 |
| JT-37 | Update sprint dates, decisions and risk register | Jigden | Must | 1 |
| JT-38 | Implement job-posting service basic operations | Sangay | Must | 5 |
| JT-39 | Implement category-tag service with seeded data | Sangay | Must | 2 |
| JT-40 | Implement job-search service basic operations | Sangay | Should | 3 |
| JT-41 | Implement saved-jobs service basic operations | Sangay | Should | 2 |
| JT-42 | Implement notification-inbox service basic operations | Sangay | Should | 2 |
| JT-43 | Implement application service basic operations | Phuntsho | Must | 5 |
| JT-44 | Add internal-call interceptor and headers to pkg | Phuntsho | Must | 3 |
| JT-45 | Implement offer service basic operations | Phuntsho | Should | 3 |
| JT-46 | Implement messaging service basic operations | Phuntsho | Should | 3 |
| JT-47 | Implement job-alert subscriptions | Phuntsho | Should | 3 |
| JT-48 | Implement resume upload and status | Kinley | Must | 3 |
| JT-49 | Implement application-status state machine | Kinley | Must | 5 |
| JT-50 | Implement applicant-profile service basic operations | Kinley | Should | 2 |
| JT-51 | Implement review service basic operations | Kinley | Should | 2 |
| JT-52 | Implement interview service basic operations | Kinley | Should | 2 |

26 tasks: 23 new (JT-30 to JT-52) and 3 carried over from Sprint 0 (JT-20, JT-22, JT-29).

---

## 4. Capacity and load

| Member | Must points | Should points | Total points |
|---|---|---|---|
| Jigden Shakya | 14 | 10 | 24 |
| Phuntsho Namgyel | 13 | 9 | 22 |
| Kinley Tobgay Lhendrup | 11 | 9 | 20 |
| Sangay Tenzin | 7 | 7 | 14 |
| Total | 45 | 35 | 80 |

Load is uneven. If Jigden falls behind, Sangay (pair partner and gateway backup) is the natural person to take over a task through the handover rule.

---

## 5. Decisions made

Recorded in the AGENTS.md Decision log (Section 15) on 5 Nov 2026:

- Option B sprint dates: Sprint 1 6 to 8 Nov, Sprint 2 9 to 11, Sprint 3 12 to 14, Sprint 4 15 to 17. No buffer.
- JT-29 (cluster spike) reassigned from Sangay to Phuntsho.
- Service ownership unchanged. Jigden keeps his six services and the gateway.
- `auth` hashes passwords with bcrypt from `golang.org/x/crypto`.
- Frontend shell moved to the Sprint 2 backlog.
- Sprint 1 services run on their own; cross-service calls are mocked ports; events and real clients move to Sprint 2.
- Must tasks are the committed Sprint 1 minimum; unfinished Should tasks move to the top of Sprint 2.
- No sub-tasks in Sprint 1.

---

## 6. Risks

- R1 (schedule overrun) probability raised from 4 to 5 (score 25, High) because the buffer was removed. See the risk register, version 1.2.
- Uneven load (Section 4) is covered by R6 (member unavailability or uneven workload).

---

## 7. Moved to Sprint 2

- Frontend shell
- Domain events (publishing and consuming through RabbitMQ)
- Real cross-service gRPC clients (replacing the mocked ports)
- Unfinished Should tasks from Sprint 1 (top of the Sprint 2 backlog)

---

## 8. Open items

- JT-29 still has the label `team-b` although Phuntsho (Team A) now owns it. Minor; can be changed in Jira.