# JobTrack: Project Charter

| Item | Detail |
|---|---|
| Project | JobTrack: A Job and Internship Application Portal |
| Jira task | JT-24 |
| Prepared by | Jigden Shakya (Scrum Master and project manager) |
| Version | 1.1 (agreed by the team on 3 Nov 2026; awaiting tutor review) |
| Date | 3 Nov 2026 (1.0 on 2 Nov; 1.1 updates the review policy) |
| Based on | Approved project proposal (submitted 26 Oct 2026) and AGENTS.md (1 Nov 2026) |
| Format | Tailored project charter following PMI PMBOK practice, adapted for Scrum |

---

## 1. Purpose and justification

Employers offering jobs and internships, and the students looking for them, lack a single structured channel for publishing vacancies, applying and tracking progress. Applicants are unsure where their applications stand, employers screen manually, and placement coordinators cannot see outcomes.

JobTrack addresses this with a web portal where employers post vacancies, applicants apply with a validated resume and follow each application to an outcome, and coordinators verify employers and see results.

The project is also the combined final project for three modules at the College of Science and Technology, Royal University of Bhutan:

- WEB303 Microservices and Serverless Applications
- SWE302 Software Testing and Quality Assurance
- SWE303 Software Project Management

This charter formally starts the development phase and records what the team has agreed to deliver.

---

## 2. Objectives and success criteria

| # | Objective | Success criteria | Target |
|---|---|---|---|
| O1 | Establish the development foundation and contracts | Repo, `.proto` contracts for all 20 services, golden template, CI, Docker Compose and a proven cluster setup in place | End of Sprint 0 (3 Nov) |
| O2 | Build and deploy the full system | All 20 microservices, the gateway, 5 Knative functions and the React frontend running; the main workflow (post, apply, review, interview, offer, accept or decline, with notifications) works end to end on the k3d cluster | End of Sprint 4 (15 Nov) |
| O3 | Prove quality through systematic testing | At least 80 percent line coverage on `application` and `application-status`; contract checks in CI; Cypress end-to-end tests of the main workflow; JMeter load test with 200 concurrent users; results documented | End of Sprint 4 (15 Nov) |
| O4 | Manage the project with Scrum and PMBOK artefacts | Sprint ceremonies held; Jira kept current with time logged; risk register reviewed every retrospective; earned value calculated from Jira time logs; lessons-learned review at closure | Throughout, closure after presentation |

---

## 3. High-level requirements

- Three roles: applicant, employer and admin (placement coordinator), with role-based access checked on every request.
- Employers register, get verified by an admin, then post and manage jobs.
- Applicants build a profile, upload a PDF resume (max 2 MB) that is validated before use, search and save jobs, subscribe to job alerts and apply.
- Employers review, shortlist, interview and make offers; applicants accept or decline.
- Application status follows a fixed lifecycle, and invalid transitions are rejected.
- Every key step notifies the right people (in-app and test email through Mailpit).
- Admins verify employers, moderate job postings, view analytics and receive weekly summaries.
- Each microservice owns its own database; services communicate through gRPC and RabbitMQ events.
- All data is synthetic.

---

## 4. Scope statement

### 4.1 In scope: 20 microservices

| # | Service | Responsibility | Owner |
|---|---|---|---|
| 1 | Auth | Register, login, JWT, user lookup | Jigden |
| 2 | Employer | Company profiles, verified flag | Jigden |
| 3 | Company Verification | Verification requests and admin decisions | Jigden |
| 4 | Moderation | Job posting reports, admin takedowns | Jigden |
| 5 | Audit Log | Records key actions from all events | Jigden |
| 6 | Analytics | Outcome statistics from events | Jigden |
| 7 | Job Posting | Create, edit, publish and close vacancies | Sangay |
| 8 | Category and Tag | Categories, skill tags, locations | Sangay |
| 9 | Job Search | Search read model from job events | Sangay |
| 10 | Saved Jobs | Bookmarks | Sangay |
| 11 | Notification Inbox | Stores and lists in-app notifications | Sangay |
| 12 | Applicant Profile | Applicant details, education, skills | Kinley |
| 13 | Resume | Resume upload, storage and validation status | Kinley |
| 14 | Application Status | Status lifecycle and history | Kinley |
| 15 | Review | Employer scores and shortlist decisions | Kinley |
| 16 | Interview | Schedule and record interviews | Kinley |
| 17 | Application | Submit and withdraw applications, eligibility checks | Phuntsho |
| 18 | Offer | Offers, accept or decline | Phuntsho |
| 19 | Messaging | Message threads per application | Phuntsho |
| 20 | Job Alert | Applicant subscriptions, matching new jobs | Phuntsho |

### 4.2 In scope: 5 serverless functions (Knative)

| # | Function | Trigger | Owner |
|---|---|---|---|
| 1 | Notification Dispatcher | Domain events | Sangay |
| 2 | Email Sender | HTTP call from other functions | Sangay |
| 3 | Weekly Summary | Weekly schedule | Jigden |
| 4 | Resume Validator | `resume.uploaded` event | Phuntsho |
| 5 | Deadline Reminder | Daily schedule | Kinley |

### 4.3 Also in scope

- API Gateway (grpc-gateway with JWT checks; not counted in the 20 services)
- React web frontend for all three roles
- Deployment on a k3d cluster with Kustomize manifests, plus Docker Compose for local development
- CI with GitHub Actions; monitoring with Prometheus and Grafana
- Automated tests: unit, integration, contract, end-to-end, load and code quality
- Project management documents (see Section 5)

### 4.4 Out of scope

- Payments
- Real SMS and real email providers (Mailpit is used instead)
- Automated resume parsing or machine-learning matching
- Mobile apps
- Production hosting
- Integration with national systems such as BLMIS
- Real personal data (all data is synthetic)

---

## 5. Key deliverables

| Area | Deliverables |
|---|---|
| Initiation and planning | Project proposal, this charter, Definition of Done and working agreement, sprint calendar, risk register, product backlog in Jira, stakeholder register, SRS, WBS, quality management plan |
| Design | Architecture and service catalogue (AGENTS.md), `.proto` and event contracts |
| Development | Source code for 20 services, gateway, 5 functions and frontend; Dockerfiles, Kubernetes and Knative manifests, CI pipeline |
| Testing | Test plan, test case document with traceability, coverage and SonarQube reports, load test report, defect log in Jira |
| Deployment | Running deployment on k3d, Grafana dashboards, deployment guide |
| Closure | Sprint reviews, earned value report, final presentation, lessons-learned and closure report |

---

## 6. Summary milestones

| ID | Milestone | Target date |
|---|---|---|
| M1 | Sprint 0 complete: repo, contracts for all 20 services, golden template, CI, local environment, proven cluster setup, charter, DoD, risk register | 3 Nov 2026 |
| M2 | Sprint 1 complete: all 20 services and the gateway running with basic operations | 6 Nov 2026 |
| M3 | Sprint 2 complete: core flow working end to end with events and the first functions | 9 Nov 2026 |
| M4 | Sprint 3 complete: full business logic, all 5 functions, frontend for all roles, integration tests | 12 Nov 2026 |
| M5 | Sprint 4 complete: end-to-end and load tests, UAT, bug fixes, monitoring, all documentation, feature freeze | 15 Nov 2026 |
| M6 | Buffer (only if needed) | 16 to 17 Nov 2026 |
| M7 | Final presentation, closure report and lessons learned | To be announced |

The full sprint calendar is in `docs/pm/definition-of-done.md`.

---

## 7. Overall project risk

The overall risk is high, mainly because of the compressed schedule (about two weeks of development for 20 services and 5 functions). The main risks are:

- Schedule overrun from the size of the system
- Knative and RabbitMQ setup on the cluster taking longer than planned
- Limited cluster resources on a single laptop
- Integration failures between services (contract or event mismatches)
- Inconsistent code generated by different AI assistants
- Member unavailability due to other coursework

Each risk is analysed with probability, impact, response and owner in the risk register (`docs/pm/risk-register.md`, JT-26), which is reviewed at every sprint retrospective.

---

## 8. Summary budget and resources

- Funding: none required. All tools are free or open source, and all work runs on the team's own laptops.
- People: four team members working part time alongside other coursework.
- Effort tracking: story points (1, 2, 3, 5, 8) for estimates; actual hours logged daily in Jira, used for earned value.
- Hardware: each member needs a laptop that runs Docker. Phuntsho's laptop hosts the k3d cluster (16 GB RAM or more).

---

## 9. Key stakeholders

| Stakeholder | Interest or role |
|---|---|
| Module tutors (Pema Namgay and Nima Yoezer) | Review and assess the project across WEB303, SWE302 and SWE303 |
| Project team (4 members) | Build, test and manage the project |
| Applicants (students and job seekers) | Main users: find jobs, apply and track applications |
| Employers | Post jobs, review applicants and make offers |
| Placement coordinator (admin) | Verifies employers, moderates postings, monitors outcomes |
| College of Science and Technology | Possible future user of the system |

---

## 10. Roles and authority

| Member | Scrum role | Charter role | Lead area |
|---|---|---|---|
| Jigden Shakya | Scrum Master and developer | Project manager | Scrum, Jira, PM documents |
| Phuntsho Namgyel | Product Owner and developer | Product authority | Architecture, contracts, service template, frontend shell |
| Kinley Tobgay Lhendrup | Developer | Team member | Testing and QA |
| Sangay Tenzin | Developer | Team member | DevOps |

- Project manager (Jigden): runs sprint ceremonies, maintains Jira, the schedule and the risk register, removes blockers, and reports progress. Has authority to schedule meetings and track work, but not to change scope alone.
- Product Owner (Phuntsho): owns and prioritises the product backlog and accepts completed work against acceptance criteria.
- All members build services, write tests for their own services, and build the frontend pages for their services. Reviewing the pair partner's pull requests is encouraged but does not block merging (team decision, 3 Nov 2026).
- There is no external sponsor. The module tutors are the approving and reviewing authority.

---

## 11. Assumptions and constraints

Assumptions:

- All four members are available for the sprint dates in Section 6.
- Phuntsho's laptop can run the full k3d cluster with Knative and RabbitMQ.
- The JT-29 cluster spike confirms a working RabbitMQ to Knative connection.

Constraints:

- Fixed, short timeline (Sprint 0 to Sprint 4, 2 to 15 Nov, with a 2-day buffer).
- Minimum of 20 microservices, as required by the tutor.
- Only the tools listed in AGENTS.md Section 7 may be used unless the team agrees.
- No funding and no production hosting.

---

## 12. Change control

- The agreed scope (Section 4) is not expected to change.
- Any change to scope, a `.proto` contract, an event schema or the tech stack needs agreement from all four members, with the Product Owner deciding its priority in the backlog.
- New work found during a sprint goes to the backlog and is discussed with the Product Owner, unless it blocks the sprint goal.
- Agreed changes are recorded in the AGENTS.md Decision log through a pull request.

---

## 13. Approval requirements and exit criteria

The project is complete when:

1. All in-scope services, functions, the gateway and the frontend are deployed and the main workflow works end to end.
2. Objectives O1 to O4 are met, or any gap is documented in the closure report.
3. All deliverables in Section 5 are in the repo and reviewed by the team.
4. The final presentation is given and assessed by the module tutors.
5. The lessons-learned and closure report is completed.

---

## 14. Changes from proposal

| Area | Proposal (26 Oct) | Now (AGENTS.md, 1 Nov) | Reason |
|---|---|---|---|
| Service count | 14 microservices + 6 serverless functions = 20 | 20 microservices + 5 functions, gateway not counted | Safest reading of the tutor's "minimum 20 microservices" |
| Admin service | One Admin service | Split into Company Verification and Moderation | Real features to reach 20 microservices |
| New services | Not in proposal | Category and Tag, Messaging, Job Alert, Offer, Audit Log | Real features to reach 20 microservices |
| SMS Sender | Serverless stub | Dropped | No real value as a stub |
| Notification function | Notification Orchestrator | Notification Dispatcher | Renamed |
| Service ownership | 5 services per member | Reassigned by team (see Section 4) | Matches the new service list and team pairs |
| Method | Incremental phases with a weekly meeting | Scrum: Sprint 0 plus four 3-day sprints, daily stand-ups | Accelerated plan with regular feedback |
| Timeline | Weeks 7 to 15 of the semester | 2 to 17 Nov 2026 (start moved from 28 Oct) | Team was busy with other coursework |
| PM tools | GanttProject, draw.io, LibreOffice Calc, shared task board | Jira (space JT) and GitHub | Single tool for backlog, sprints and time tracking |
| Branching | Not specified | GitHub Flow | Long-lived branches add merge work |
| Code review | All members review one another's changes | PRs required for `main`; reviews encouraged but not merge-blocking; CI required once in place | Avoid blocking short sprints on reviewer availability (team decision, 3 Nov) |
| CI/CD | Department pipeline or Gitea Actions | GitHub Actions, images to GitHub Container Registry | Fits the GitHub repo |
| Contract testing | Pact | `buf breaking` in CI and shared event fixtures | Same purpose, less tooling friction |
| Kubernetes | kind or k3s on a shared server or VM | k3d on Phuntsho's laptop | No shared server available |
| Resume storage | Kubernetes volume | Resume service database (2 MB limit) | Removes storage setup work |
| Email | Stub providers and local SMTP server | Mailpit | Real test inbox, no external provider |
| Database | One PostgreSQL per service | One PostgreSQL server, separate database and user per service | Keeps isolation and saves memory |

---

## 15. Sign-off

| Name | Role | Agreed (yes/no) | Date |
|---|---|---|---|
| Jigden Shakya | Scrum Master and project manager | Yes (WhatsApp) | 3 Nov 2026 |
| Phuntsho Namgyel | Product Owner | Yes (WhatsApp) | 3 Nov 2026 |
| Kinley Tobgay Lhendrup | Developer | Yes (WhatsApp) | 3 Nov 2026 |
| Sangay Tenzin | Developer | Yes (WhatsApp) | 3 Nov 2026 |
| Pema Namgay | Module tutor (reviewer) | | |
| Nima Yoezer | Module tutor (reviewer) | | |
