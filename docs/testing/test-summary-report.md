# JobTrack test summary report

| Item | Value |
|---|---|
| Jira | [JT-97](https://jobtrack-cst.atlassian.net/browse/JT-97) (SWE302 test documentation) |
| Owner | Kinley Tobgay Lhendrup (testing and QA lead). Backup: Jigden Shakya |
| Final `main` | `9908ad1` (19 Nov 2026, JT-96 merge, #89). CI on this commit: [run #234](https://github.com/jobtrack-cst/jobtrack/actions/runs/44547813658), success |
| Plan | [test-plan.md](test-plan.md); test cases per task in `docs/testing/jt-<number>-*.md` |
| Data | Synthetic only |

## 1. Summary

| Level | Tool | Tests | Result | Where it ran |
|---|---|---|---|---|
| Unit | Go testing, testify, gomock | 1,219 test functions (6,287 cases with subtests) | All pass | CI and local |
| Integration | Testcontainers (PostgreSQL 17 and 18, RabbitMQ 4.1) | 164 test functions (595 cases) | All pass, 1 skipped in code (Section 2) | CI and local |
| Contract | Event fixtures in `tests/contract`; `buf lint` and `buf breaking` in CI | 34 test functions (161 cases) | All pass | CI and local |
| GUI | Cypress, gateway stubbed with `cy.intercept` | 192 tests in 5 specs | All pass | CI and local |
| End to end | Cypress against the running Docker Compose stack, no stubs | 11 steps of the main workflow | All pass | Local (CI has no stack) |
| Load | Apache JMeter, deadline-rush scenario | 200 concurrent users, 2,003 requests | All pass, 0 errors | Two laptops (JMeter apart from the host) |
| Code quality | SonarQube Community Build | Final run: 0 bugs, 0 vulnerabilities; quality gate failed on new code | See Section 5 | Local |

Coverage target (AGENTS.md Section 10): at least 80 percent line coverage on `application` and `application-status`, measured with unit and integration tests together on production code, excluding generated gomock files, with the unit-only figure alongside (Section 15, JT-49).

| Service | Unit plus integration | Unit only | Statements | Target met |
|---|---|---|---|---|
| `application` | 94.4% | 74.6% | 500 | Yes |
| `application-status` | 91.1% | 69.2% | 751 | Yes |

All figures in this report were taken on 19 Nov 2026 from `main` at `9908ad1`; the only change on the JT-97 branch is documentation. Fixes merged after `9908ad1` (for example JT-105, #90) are not included in the counts; CI runs the same suites on each of them.

### Local environments

| Run | Machine | Versions |
|---|---|---|
| Go unit, integration and coverage; SonarQube; Cypress GUI | Kinley's MacBook Air, Apple M1, 8 GiB RAM, Docker Desktop | Go 1.27.3 darwin/arm64; Cypress 16.2.0, Electron 146, Node 20.20.2 |
| Cypress end to end; load test host | Phuntsho's laptop, Apple M3, 16 GiB RAM, Docker Desktop 8 CPUs and 7.75 GiB | Cypress 16.2.0, Electron 146, Node 26.13.0; Compose stack with the `functions` and `monitoring` profiles |
| Load generator | Kinley's MacBook Air | Apache JMeter 5.6.3, OpenJDK 25.0.2 |

## 2. Go tests per component

Counted from `go test -json`: test functions, and in brackets all cases including table-driven subtests. Integration tests are those that run only with `-tags=integration`.

| Component | Unit | Integration | Failed |
|---|---|---|---|
| `gateway` | 33 (194) | - | 0 |
| `pkg` (shared packages) | 60 (302) | 7 (16) | 0 |
| `services/analytics` | 62 (306) | 8 (31) | 0 |
| `services/applicant-profile` | 40 (225) | 3 (19) | 0 |
| `services/application` | 48 (222) | 4 (23) | 0 |
| `services/application-status` | 62 (575) | 8 (28) | 0 |
| `services/audit-log` | 41 (253) | 7 (35) | 0 |
| `services/auth` | 55 (273) | 4 (13) | 0 |
| `services/category-tag` | 47 (238) | 5 (22) | 0 |
| `services/company-verification` | 68 (323) | 12 (41) | 0 |
| `services/employer` | 65 (335) | 14 (38) | 0 |
| `services/interview` | 66 (305) | 7 (29) | 0 |
| `services/job-alert` | 51 (233) | 10 (21) | 0 |
| `services/job-posting` | 59 (374) | 5 (31) | 0 |
| `services/job-search` | 27 (123) | 7 (12) | 0 |
| `services/messaging` | 43 (193) | 8 (28) | 0 |
| `services/moderation` | 66 (333) | 16 (54) | 0 |
| `services/notification-inbox` | 20 (80) | 2 (2) | 0 |
| `services/offer` | 46 (243) | 12 (45) | 0 |
| `services/resume` | 67 (427) | 11 (62) | 0 |
| `services/review` | 59 (240) | 7 (29) | 0 |
| `services/saved-jobs` | 33 (91) | 3 (3) | 0 |
| `services/service-template` (golden template) | 20 (93) | 2 (2) | 0 |
| `functions/deadline-reminder` | 26 (71) | - | 0 |
| `functions/email-sender` | 7 (40) | - | 0 |
| `functions/notification-dispatcher` | 8 (8) | - | 0 |
| `functions/resume-validator` | 7 (88) | 2 (11) | 0 |
| `functions/weekly-summary` | 29 (84) | - | 0 |
| `deploy/compose/monitoring` (JT-93 configuration checks) | 3 (7) | - | 0 |
| `deploy/knative/jt29` (JT-29 spike forwarder) | 1 (8) | - | 0 |
| **Total** | **1,219 (6,287)** | **164 (595)** | **0** |
| `tests/contract` (contract level, counted separately) | 34 (161) | - | 0 |

- All 20 services in AGENTS.md Section 4 and all 5 functions in Section 5 have tests. `cmd/rabbitmq-forwarder` (the JT-61 forwarder) has no test files of its own; its logic is in `pkg/forwarder`, which is tested under `pkg`.
- One integration test is skipped in the code, not by a failure: `pkg/db` `TestMigrate_AppliesSQLAndReleasesResources` calls `t.Skip("Skipping due to goose global cache panic in concurrent runs")`. Every service's own migration integration tests still run.
- Since the draft of 18 Nov (`a9995f5`), notification-dispatcher (JT-71, JT-92), the monitoring configuration tests (JT-93) and the saved-jobs pagination and interceptor tests (JT-91) were added.

## 3. Cypress

| Spec | Covers | Tests | Result |
|---|---|---|---|
| `tests/e2e/auth.cy.js` | Login, registration, routing, shared API client (JT-63) | 19 | Pass |
| `tests/e2e/application-workflows.cy.js` | Apply, applications, offers, messages, job alerts (JT-78) | 34 | Pass |
| `tests/e2e/applicant-hiring-pages.cy.js` | Profile, resumes, status, review, interviews (JT-83) | 51 | Pass |
| `tests/e2e/team-b-admin-pages.cy.js` | Company profile, verification, moderation, analytics, audit log (JT-88) | 55 | Pass |
| `tests/e2e/job-pages.cy.js` | Job pages, search, saved jobs, notifications (JT-73) | 33 | Pass |
| **Stubbed GUI total (`npm test`, runs in CI)** | | **192** | **Pass, 3 min 13 s** |
| `tests/e2e/stack/main-workflow.cy.js` | Main workflow through the real UI and services (JT-95): employer registration to accepted offer and status history | 11 | Pass, 48 s |

The end-to-end spec ran on the Docker host against the stack built from `main`, with 7 screenshots of the key steps saved locally (not committed). User acceptance testing of the same workflow by role is recorded in [uat-script.md](uat-script.md) (JT-99).

## 4. Load test

Full report: [jt-96-load-test-report.md](jt-96-load-test-report.md) (JT-96, 19 Nov 2026).

| Item | Value |
|---|---|
| Scenario | Deadline rush: 200 synthetic applicants log in, search for the job, view it, list their resumes and apply at the same moment, all through the gateway |
| Requests | 2,003, 0 errors (0.00%), 28.2 requests per second over 71 s |
| Rush 95th percentile | Login 77 ms, search 12 ms, view 11 ms, list resumes 12 ms, submit 66 ms |
| Submit burst | All 200 applications created within about 77 ms of being released together |
| Docker host peak | 212% of 800% CPU (submit burst: `postgres`, `rabbitmq`, `application`), about 42% of 7.75 GiB memory |
| Defects | None found; no Jira Bug logged |

Pass thresholds for response time and error rate were not set (test plan Section 8), so the load test records measured values and the hardware used.

## 5. SonarQube

Final run on 19 Nov 2026, analysing `main` at `9908ad1` with the same setup as the Sprint 3 run ([sonarqube-sprint-3.md](sonarqube-sprint-3.md)): SonarQube Community Build in Docker and the SonarScanner CLI in Docker on Kinley's MacBook Air, default Sonar way profiles and quality gate, scope from `sonar-project.properties`. One difference: this run imported the unit plus integration coverage profile (`go test -tags=integration -coverprofile`), the same measure as the Section 10 target, where Sprint 3 imported unit tests only.

| Measure | Sprint 3 (17 Nov, JT-84) | Final (19 Nov) |
|---|---|---|
| Lines of code | 30,159 | 31,727 |
| Security | 0 vulnerabilities, 0 hotspots, A | 0 vulnerabilities, 0 hotspots, A |
| Reliability | 1 bug, C | 0 bugs, A |
| Maintainability | 300 code smells, about 85 hours, A | 325 code smells, about 88 hours, A |
| Coverage | 67.8% (unit only) | 79.4% on 17,780 coverable lines (unit plus integration; the frontend has none, see below) |
| Duplication | 34.2% | 32.7% of 38,934 lines |
| Quality gate | Passed (first analysis, no new code) | **Failed** on new code since 17 Nov (below) |

Overview screenshot: [sonarqube-final.png](jt-97-screenshots/sonarqube-final.png).

By area:

| Area | Lines of code | Coverage | Duplication | Bugs | Code smells |
|---|---|---|---|---|---|
| `services` | 25,243 | 84.7% | 39.7% | 0 | 220 |
| `services/application` | 1,128 | 93.5% | 39.3% | 0 | 14 |
| `services/application-status` | 1,715 | 88.1% | 31.3% | 0 | 14 |
| `functions` | 2,460 | 70.9% | 9.7% | 0 | 11 |
| `gateway` | 545 | 84.6% | 2.6% | 0 | 1 |
| `pkg` | 1,129 | 87.4% | 0.0% | 0 | 15 |
| `frontend/src` | 2,282 | 0.0% | 0.0% | 0 | 55 |

### Findings

1. **Reliability is now A.** The only bug from Sprint 3 (`typescript:S6324`) was fixed in JT-104 (#76), and the final run found no bugs or vulnerabilities.
2. **The quality gate failed on new code.** The default gate checks only code changed since the previous analysis (17 Nov). Three conditions failed: coverage on new code 58.9% (threshold 80%), duplication on new code 4.8% (threshold 3%), and 25 new issues (threshold 0). Overall bugs and vulnerabilities are 0, so the 25 new issues are code smells. The main causes:
   - New frontend pages (JT-73 job pages, JT-102 company logo) count as uncovered, because Cypress tests produce no coverage report for SonarQube. The frontend is tested by the 192 GUI tests and the end-to-end spec in Section 3.
   - New Go code added since 17 Nov follows the shared hexagonal template, which SonarQube counts as duplication, as in Sprint 3 F3.
   These are maintainability findings, not defects, so no Jira Bug was logged, the same rule as in Sprint 3. The coverage target that the project agreed on (Section 1) is met.
3. **Code smells** grew from 300 to 325. The largest rules are unchanged: `go:S3776` cognitive complexity (242, mostly long table-driven tests), `go:S1192` repeated string literals (22), and in the frontend `typescript:S6759` read-only props (17) and `typescript:S3358` nested ternaries (15).
4. SonarQube's per-area coverage for `application` (93.5%) and `application-status` (88.1%) uses its own line counting, so it differs slightly from the Go statement coverage in Section 1; both are above 80%.

Proposed for after the feature freeze (not in Sprint 4 scope): exclude `services/*/scripts` from the scan, report test-file complexity separately, and move repeated service wiring into `pkg` (Sprint 3 F3 to F5).

## 6. Defect log

All Jira issues of type Bug in the JT space, as at 19 Nov 2026, 21:45 Bhutan time. Severity labels follow [test-plan.md](test-plan.md) Section 7; JT-55, JT-65 and JT-90 had no label and were labelled during JT-97 by the tester.

| Jira key | Summary | Severity | Priority | Status | Fix PR |
|---|---|---|---|---|---|
| JT-55 | Compose smoke test uses hardcoded RabbitMQ host port instead of published port | Minor | Medium | Done | #48 |
| JT-65 | job-posting: huge `page_token` returns a negative `next_page_token` | Minor | Low | Done | #29 |
| JT-90 | job-posting: removing a job through moderation does not publish `job.closed` | Major | High | Done | #69 |
| JT-104 | resume page: file name check uses a control character in a regular expression (SonarQube S6324) | Trivial | Medium | Done | #76 |
| JT-105 | notification-dispatcher receives no events in Docker Compose (no forwarder or queue) | Major | High | In Progress | #90 (open at the time of writing) |
| JT-106 | Frontend logs the user out on every page reload (JWT kept only in memory) | Minor | Medium | To Do | None yet |
| JT-107 | resume-validator rejects valid PDFs that use cross-reference streams | Major | High | To Do | None yet |
| JT-108 | Offer ID is shown only once after making an offer and cannot be found again | Major | Medium | To Do | None yet |

Summary: 8 bugs, 4 fixed and 4 open. By severity: 0 critical, 4 major, 3 minor, 1 trivial. JT-105 to JT-108 were found in UAT (JT-99) on 19 Nov. No bug was found by the load test (JT-96) or the final SonarQube run. Open bugs at the end of Sprint 4 are listed in the closure report (JT-100).

## 7. How to reproduce the figures

From the repository root on `main`, with Docker running and at least about 6 GB of memory for Docker (stop the Compose stack first):

    docker pull postgres:17-alpine
    docker pull postgres:18-alpine
    docker pull rabbitmq:4.1-alpine
    go test -count=1 -json -coverprofile=/tmp/jt97/unit.cover ./... > /tmp/jt97/unit.json
    go test -count=1 -timeout 30m -json -tags=integration -coverprofile=/tmp/jt97/all.cover ./... > /tmp/jt97/all.json

Counts are the test events with a final pass, fail or skip in the JSON output, grouped by component (the first two path segments under the module); integration tests are those present only in the second run. Coverage is computed from the profiles over `services/application` and `services/application-status`, excluding files under `mocks/`. The helper that does this is kept outside the repository.

- Cypress: `npm test` and `npm run test:stack` in `frontend`, see `tests/e2e/README.md`.
- Load: `tests/load/README.md`.
- SonarQube: as in [sonarqube-sprint-3.md](sonarqube-sprint-3.md), with `cp /tmp/jt97/all.cover coverage.out` before the scanner.

## 8. Conclusion

On the final `main` every automated test passes: 1,383 Go test functions (unit and integration), 34 contract tests, 192 Cypress GUI tests in CI, the 11-step end-to-end workflow against the running stack, and the 200-user deadline-rush load test with no errors. `application` (94.4%) and `application-status` (91.1%) meet the 80 percent coverage target. SonarQube finds no bugs or vulnerabilities; its default quality gate fails on new code because of uncovered frontend pages and template duplication, which the team treats as maintainability work after the feature freeze. Four bugs found in UAT on the last day (JT-105 to JT-108, three of them major) were open when this report was written and are carried in the closure report.

System test exit criteria ([test-plan.md](test-plan.md) Section 6):

| Criterion | Status |
|---|---|
| The Cypress main workflow test passes | Met (Section 3) |
| The 200-user deadline-rush scenario has been run and its report saved | Met (Section 4) |
| SonarQube reports saved in `docs/testing` | Met for Sprint 3 (JT-84) and the final run (this report). SonarQube was set up in Sprint 3, so Sprints 1 and 2 have no report |
| Proposed: no Critical bug open | Met (0 critical) |
| Proposed: every open Major bug agreed with the Product Owner | Open: JT-105, JT-107 and JT-108 are for the Product Owner to decide in the closure report (JT-100) |
