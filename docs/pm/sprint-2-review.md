# JobTrack: Sprint 2 Review and Retrospective

| Item | Detail |
|---|---|
| Jira task | JT-89 |
| Owner | Jigden Shakya (Scrum Master). Backup: Kinley Tobgay Lhendrup |
| Sprint | Sprint 2 (Jira sprint 36) |
| Sprint dates | 9 Nov 09:00 to 13 Nov 23:59 (extended from 11 Nov on 11 Nov 2026, JT-64) |
| Closed in Jira | 13 Nov 2026, 23:29 |
| Review and retrospective | Held asynchronously on 13 Nov 2026 by Jigden on WhatsApp; the review was shared with all four members |
| Data source | Jira sprint and issue history, checked on 14 Nov 2026 |
| Related documents | `docs/pm/sprint-1-review.md`, `docs/pm/sprint-3-planning.md`, `docs/pm/risk-register.md` (version 1.4), AGENTS.md (Sections 14 and 15) |

---

## 1. Sprint goal and outcome

Goal: Finish the Sprint 1 carryovers (Musts first: JT-38, JT-39, JT-48, JT-49) and get the core application flow working end to end with real gRPC calls, events and the first function. Committed minimum: the four Must carryovers.

Outcome: met, with the agreed two-day extension. All 21 tasks in the sprint were Done, including all 12 Must tasks and all 11 Sprint 1 carryovers. The apply-for-a-job flow now uses real clients and events: application calls job-posting and resume (JT-58), resume publishes `resume.uploaded` and consumes the validation results (JT-59), resume-validator runs through the forwarder (JT-61, JT-62), and application-status consumes application events and publishes status changes (JT-60). Employer verification events (JT-56), the real employer client in job-posting (JT-57) and the frontend shell (JT-63) were also finished. Only JT-54 (Should) was not finished and moved to Sprint 3.

---

## 2. Results

| Measure | Tasks | Story points |
|---|---|---|
| In the sprint at close | 21 | 60 |
| Done in Sprint 2 | 21 | 60 |
| Moved to Sprint 3 | 1 (JT-54) | 0 (unestimated in Sprint 2, now 2) |
| Must (High): planned / done | 12 / 12 | 41 / 41 |
| Should (Medium): planned / done | 9 / 9 | 19 / 19 |

- Velocity (story points Done in the sprint): 60, compared with 49 in Sprint 1.
- Completion: 100 percent of the tasks and points in the sprint at close.
- Sprint 2 ran for 5 days instead of 3 because of the extension, so its velocity per day (about 12 points) was lower than Sprint 1 (about 16 points). The figure of about 50 points for a 3-day sprint stays as the planning guide.
- JT-55 (bug) was Done without an estimate. JT-53 (gateway CORS) and JT-65 (bug) were Done during Sprint 2 but were not in the Jira sprint and have no points, so they are not counted above.

### By member

Done points by Jira assignee when the sprint closed.

| Member | Done tasks | Done points | Notes |
|---|---|---|---|
| Phuntsho Namgyel | 7 (JT-55, JT-58 to JT-63) | 22 | JT-55 had no points |
| Kinley Tobgay Lhendrup | 6 (JT-22, JT-48 to JT-52) | 17 | All six were his Sprint 1 carryovers |
| Jigden Shakya | 5 (JT-38, JT-39, JT-56, JT-57, JT-64) | 14 | Also Done outside the sprint: JT-53, JT-65. JT-54 moved to Sprint 3 |
| Sangay Tenzin | 3 (JT-40 to JT-42) | 7 | All three were his Sprint 1 carryovers |
| Total | 21 | 60 | |

---

## 3. Task outcomes

Owner is the Jira assignee at sprint close. Status as of 14 Nov 2026.

| Key | Summary | Owner | Priority | Points | Sprint 2 result | Status now |
|---|---|---|---|---|---|---|
| JT-22 | Draft test plan outline (carried over) | Kinley | Should | 3 | Done | Done |
| JT-38 | Implement job-posting service basic operations (carried over) | Jigden | Must | 5 | Done | Done |
| JT-39 | Implement category-tag service with seeded data (carried over) | Jigden | Must | 2 | Done | Done |
| JT-40 | Implement job-search service basic operations (carried over) | Sangay | Should | 3 | Done | Done |
| JT-41 | Implement saved-jobs service basic operations (carried over) | Sangay | Should | 2 | Done | Done |
| JT-42 | Implement notification-inbox service basic operations (carried over) | Sangay | Should | 2 | Done | Done |
| JT-48 | Implement resume upload and status (carried over) | Kinley | Must | 3 | Done | Done |
| JT-49 | Implement application-status state machine (carried over) | Kinley | Must | 5 | Done | Done |
| JT-50 | Implement applicant-profile service basic operations (carried over) | Kinley | Should | 2 | Done | Done |
| JT-51 | Implement review service basic operations (carried over) | Kinley | Should | 2 | Done | Done |
| JT-52 | Implement interview service basic operations (carried over) | Kinley | Should | 2 | Done | Done |
| JT-54 | Gateway: limit the request body size | Jigden | Should | Unestimated (now 2) | Not finished | Sprint 3, To Do |
| JT-55 | Compose smoke test uses hardcoded RabbitMQ host port (bug) | Phuntsho | Should | 0 | Done | Done |
| JT-56 | Publish and consume employer verification events | Jigden | Must | 3 | Done | Done |
| JT-57 | Job-posting: real employer client for the verified check | Jigden | Must | 2 | Done | Done |
| JT-58 | Application: real job-posting and resume clients, publish application events | Phuntsho | Must | 5 | Done | Done |
| JT-59 | Resume: publish resume.uploaded, consume validation results, protect GetResumeFile | Phuntsho | Must | 3 | Done | Done |
| JT-60 | Application-status: consume application events, publish status changes | Phuntsho | Must | 3 | Done | Done |
| JT-61 | Wire the Go RabbitMQ-to-CloudEvents forwarder for functions | Phuntsho | Must | 3 | Done | Done |
| JT-62 | Implement resume-validator Knative function | Phuntsho | Must | 5 | Done | Done |
| JT-63 | Create React frontend shell and shared API client | Phuntsho | Should | 3 | Done | Done |
| JT-64 | Sprint 1 review, sprint schedule update and risk register | Jigden | Must | 2 | Done | Done |

### Work outside the sprint

- JT-53 (gateway CORS for the frontend dev server, Jigden, Should, unestimated) was Done during Sprint 2 but was not in Jira sprint 36. It had been added to Sprint 3 by mistake and was removed from it on 14 Nov.
- JT-65 (job-posting: huge `page_token` returns a negative `next_page_token`, bug, Jigden, Low) was Done during Sprint 2 and was not in the sprint.
- JT-66 (error logging for unexpected gRPC errors) and JT-67 (align the project charter and sprint calendar with the revised schedule) were found during Sprint 2 and went to the backlog, following the Section 12 rule. Both are Should carryovers in Sprint 3.

---

## 4. What went well

- Every task in the sprint was Done, including all 12 Musts and all 11 Sprint 1 carryovers, so the committed minimum was met.
- The core apply-for-a-job flow now runs with real gRPC clients and events instead of mocked ports, and the first function (resume-validator) runs through the forwarder.
- Shared building blocks for Sprint 3 are in place: the shared consumer `events.Consume`, the transactional outbox pattern and the first contract fixtures (JT-56), the forwarder (JT-61), and the frontend shell with gateway CORS (JT-63, JT-53).
- The scope priorities agreed with the Product Owner on 12 Nov were followed: JT-58 before JT-60, and JT-53 before JT-63.
- Reassigning Kinley's and Sangay's carryovers on 9 Nov and handing them back when they returned kept all carryovers moving. Kinley and Sangay finished all of their own carryovers.
- New work found during the sprint (JT-66, JT-67) went to the backlog instead of growing the sprint.

---

## 5. What to change

- Avoid further extensions. The 60 points needed 5 days. Sprint 3 and Sprint 4 have no buffer, and Sprint 4 ends on the planned completion date, so a slip in Sprint 3 cannot be absorbed by extending it again.
- Keep the Jira sprint matching the real work. JT-53 and JT-65 were Done during Sprint 2 but sat outside the sprint, and JT-53 was later added to Sprint 3 by mistake. Work done in a sprint should be in that sprint in Jira.
- Estimate every task, including bugs, before it enters a sprint. JT-54 and JT-55 were in Sprint 2 without points, and JT-53 and JT-65 had none either.
- Balance the load. Done points ranged from 22 (Phuntsho) to 7 (Sangay). Sprint 3 is planned at 15 to 16 points per member.
- Keep listing cross-service dependencies at planning. JT-60 depended on JT-58, and Sprint 3 has more producer and consumer pairs, so each task's dependencies are recorded in the Sprint 3 planning record.

---

## 6. Action items

| Action | Owner | When |
|---|---|---|
| Record cross-service dependencies for every Sprint 3 task in the planning record | Jigden with each task owner | Sprint 3 planning, 14 Nov |
| Estimate every task and bug before it is added to a sprint | Jigden and the task owner | Sprint 3 and 4 planning |
| Keep the Jira sprint matching the work done in it; add or remove issues as soon as scope changes | Jigden | Ongoing |
| Check the R1 trigger on day 2 of the sprint and reassign through the handover rule if needed | Jigden | 15 Nov (Sprint 3), 18 Nov (Sprint 4) |
| Let unfinished Should tasks, carryovers first, slip to the top of Sprint 4 instead of extending Sprint 3 | Jigden with Phuntsho (Product Owner) | End of Sprint 3, 16 Nov |
| Verify each new event end to end in Docker Compose before moving its task to Done | Each task owner | Sprints 3 and 4 |
| Raise blockers at the stand-up the same day | All members | Ongoing |

### Follow-up of the Sprint 1 actions

| Sprint 1 action | Result |
|---|---|
| Plan Sprint 3 at about 50 points, Musts and carryovers first | Sprint 3 is planned at 62 points on purpose (44 Must, 18 Should), agreed with the Product Owner on 14 Nov. The 44 Must points stay within the figure of about 50; see `docs/pm/sprint-3-planning.md` |
| List cross-service dependencies for every Sprint 3 task at planning | Done in `docs/pm/sprint-3-planning.md` |
| Verify each new event end to end in Docker Compose | Continues in Sprints 3 and 4 |
| Check the R1 trigger on day 2 of each sprint | Continues on 15 Nov and 18 Nov |
| Raise blockers at the stand-up the same day | Ongoing |
| Update the Jira assignee whenever work is reassigned or handed back | Done in Sprint 2 for the 9 and 10 Nov reassignments; ongoing |

---

## 7. Risks

- R1 (schedule overrun) stays at 25. Sprint 2 met its goal and delivered 60 points, but only with the two-day extension, and the schedule still has no buffer. Sprint 3 is planned at 62 points, above the figure of about 50; unfinished Should tasks move to the top of Sprint 4.
- R6 (member unavailability or uneven workload) stays at 16 (High). Its trigger fired on 9 Nov, and the Done load in Sprint 2 was uneven (22 to 7 points).
- See the risk register, version 1.4.

---

## 8. Open items

- JT-54 (gateway body size limit) is needed by JT-83 for resume upload through the gateway, so it is planned early in Sprint 3.
- JT-67 aligns the project charter and the sprint calendar in `docs/pm/definition-of-done.md` with the revised schedule (the open item from the Sprint 1 review).
