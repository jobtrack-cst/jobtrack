# JobTrack: Definition of Done, Working Agreement and Sprint Calendar

- Jira task: JT-25
- Owner: Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup
- Status: Agreed by the team (3 Nov 2026)
- Last updated: 3 Nov 2026 (review policy updated to match the JT-16 team decision)

This document records how the JobTrack team works together. If it conflicts with AGENTS.md, AGENTS.md wins and this file is corrected through a pull request.

---

## 1. Definition of Done

A task or story is Done only when all of the following are true:

1. All acceptance criteria in the Jira task are met.
2. Code follows the conventions in AGENTS.md and is formatted (`gofmt`, passes `go vet`).
3. Unit tests are written and pass. Integration tests are written where the task involves a database or events.
4. CI is green on the pull request (a required check once JT-28 is merged).
5. The work goes through a pull request. Review by the pair partner (and by the other team for contracts, shared `pkg/` or AGENTS.md) is encouraged but does not block merging.
6. The work is merged to `main` and runs in Docker Compose.
7. Related documentation is updated (AGENTS.md, `docs/` folder, test cases).
8. Time spent is logged in Jira.

For documentation-only tasks (for example PM documents), points 2, 3 and 6 apply only where relevant: the document must be in the agreed folder, submitted through a pull request and merged to `main`.

---

## 2. Working agreement

### Daily stand-up

- Length: 15 minutes.
- Format: physical or online (WhatsApp call), whichever suits the team that day.
- Time: flexible. Default time is 8:00 PM, confirmed in the WhatsApp group by 12:00 PM each day if it needs to move.
- Each member answers: what I finished, what I will do next, and what is blocking me.
- If a member cannot attend, they post the three answers in the WhatsApp group before the stand-up starts.
- The Scrum Master posts a short summary of blockers in the group after the stand-up.

### Communication

- Main channel: the JobTrack WhatsApp group.
- Technical decisions that affect architecture, contracts or tools are recorded in the AGENTS.md Decision log through a pull request, not only in chat.
- Task progress lives in Jira, not only in chat.

### Pull request reviews

- All changes to `main` go through a pull request. Direct pushes, force pushes and branch deletion on `main` are blocked.
- Reviews are encouraged but do not block merging (team decision, 3 Nov 2026). The owner may merge once CI is green.
- When the task owner posts a PR link in the WhatsApp group, the pair partner aims to review it within 4 hours on the same day. If it is posted late in the evening, the aim is by 12:00 PM the next day.
- Changes to `.proto` contracts, event schemas, shared `pkg/` code or AGENTS.md should be posted for a cross-team look before merging whenever possible.

### Time logging

- Log time in Jira (Time tracking field) before 11:59 PM on each day you work on a task.
- Logged hours are used for earned value calculations, so log real hours, not estimates.

### Jira board

- Move your own cards: To Do, In Progress, In Review (PR open), Done (PR merged and meets the Definition of Done).
- New work found during a sprint goes to the backlog and is discussed with the Product Owner, unless it blocks the sprint goal.

### Cluster host

- Phuntsho's laptop is the cluster host machine (k3d, Knative, RabbitMQ events).

---

## 3. Sprint calendar

| Sprint | Dates | Goal |
|---|---|---|
| Sprint 0 | Mon 2 Nov to Tue 3 Nov | Development foundation: repo, contracts for all 20 services, golden template, CI, local environment, proven cluster setup |
| Sprint 1 | Wed 4 Nov to Fri 6 Nov | All 20 services and the gateway running with basic operations |
| Sprint 2 | Sat 7 Nov to Mon 9 Nov | Core flow working end to end with events and the first functions |
| Sprint 3 | Tue 10 Nov to Thu 12 Nov | Full business logic, all 5 functions, frontend for all roles, integration tests |
| Sprint 4 | Fri 13 Nov to Sun 15 Nov | End-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze |
| Buffer | Mon 16 Nov to Tue 17 Nov | Only if needed |

Presentation date: not yet announced (likely to be postponed).

### Ceremonies

| Ceremony | When | Length |
|---|---|---|
| Sprint planning | First day of each sprint | 1 hour |
| Daily stand-up | Every sprint day (see Section 2) | 15 minutes |
| Sprint review and retrospective (combined) | Last day of each sprint | 1 hour |

- Sprint lengths stay fixed. If a sprint goal is met early, the team pulls the next items from the backlog.
- The risk register (`docs/pm/risk-register.md`, JT-26) is reviewed at every retrospective.

---

## 4. Review and sign-off

| Member | Role | Agreed (yes/no) |
|---|---|---|
| Phuntsho Namgyel | Product Owner and developer | Yes (3 Nov 2026, WhatsApp) |
| Kinley Tobgay Lhendrup | Developer | Yes (3 Nov 2026, WhatsApp) |
| Jigden Shakya | Scrum Master and developer | Yes (3 Nov 2026, WhatsApp) |
| Sangay Tenzin | Developer | Yes (3 Nov 2026, WhatsApp) |
