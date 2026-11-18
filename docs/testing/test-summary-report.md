# JobTrack test summary report

| Item | Value |
|---|---|
| Jira | [JT-97](https://jobtrack-cst.atlassian.net/browse/JT-97) (SWE302 test documentation) |
| Owner | Kinley Tobgay Lhendrup (testing and QA lead). Backup: Jigden Shakya |
| Final `main` | FINAL_MAIN_PLACEHOLDER |
| Plan | [test-plan.md](test-plan.md); test cases per task in `docs/testing/jt-<number>-*.md` |
| Data | Synthetic only |

## 1. Summary

| Level | Tool | Tests | Result | Where it ran |
|---|---|---|---|---|
| Unit | Go testing, testify, gomock | 1,190 test functions (6,249 cases with subtests) | All pass, 1 case skipped | CI and local |
| Integration | Testcontainers (PostgreSQL 18, RabbitMQ 4.1) | 163 test functions (594 cases) | All pass | CI and local |
| Contract | Event fixtures in `tests/contract`; `buf lint` and `buf breaking` in CI | 34 test functions (161 cases) | All pass | CI and local |
| GUI | Cypress, gateway stubbed with `cy.intercept` | 188 tests in 5 specs | All pass | CI and local |
| End to end | Cypress against the running Docker Compose stack, no stubs | 11 steps of the main workflow | All pass | Local (CI has no stack) |
| Load | Apache JMeter, deadline-rush scenario | LOAD_SUMMARY_PLACEHOLDER | LOAD_RESULT_PLACEHOLDER | Two laptops (JMeter apart from the host) |
| Code quality | SonarQube Community Build | Sprint 3 run: quality gate passed, 1 bug, 0 vulnerabilities | See Section 5 | Local |

Coverage target (AGENTS.md Section 10): at least 80 percent line coverage on `application` and `application-status`, measured with unit and integration tests together on production code, excluding generated gomock files, with the unit-only figure alongside (Section 15, JT-49).

| Service | Unit plus integration | Unit only | Statements | Target met |
|---|---|---|---|---|
| `application` | 94.4% | 75.0% | 496 | Yes |
| `application-status` | 91.4% | 69.0% | 746 | Yes |

Figures in this section are from `main` at `a9995f5` (18 Nov 2026). FINAL_RUN_NOTE_PLACEHOLDER

## 2. Go tests per component

Counted from `go test -json`: test functions, and in brackets all cases including table-driven subtests. Integration tests are those that run only with `-tags=integration`.

| Component | Unit | Integration | Failed |
|---|---|---|---|
| `gateway` | 32 (193) | - | 0 |
| `pkg` (shared packages) | 55 (293) | 7 (16) | 0 |
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
| `services/saved-jobs` | 21 (78) | 2 (2) | 0 |
| `services/service-template` (golden template) | 20 (93) | 2 (2) | 0 |
| `functions/deadline-reminder` | 26 (71) | - | 0 |
| `functions/email-sender` | 7 (40) | - | 0 |
| `functions/resume-validator` | 7 (88) | 2 (11) | 0 |
| `functions/weekly-summary` | 29 (84) | - | 0 |
| `deploy/knative/jt29` (JT-29 spike forwarder) | 1 (8) | - | 0 |
| **Total unit** | **1,190 (6,249)** | **163 (594)** | **0** |
| `tests/contract` (contract level, counted separately) | 34 (161) | - | 0 |

The 20th service in AGENTS.md Section 4, notification-dispatcher (JT-71), was not on `main` when these figures were taken. FINAL_COMPONENTS_PLACEHOLDER

## 3. Cypress

| Spec | Covers | Tests | Result |
|---|---|---|---|
| `tests/e2e/auth.cy.js` | Login, registration, routing, shared API client (JT-63) | 19 | Pass |
| `tests/e2e/application-workflows.cy.js` | Apply, applications, offers, messages, job alerts (JT-78) | 34 | Pass |
| `tests/e2e/applicant-hiring-pages.cy.js` | Profile, resumes, status, review, interviews (JT-83) | 47 | Pass |
| `tests/e2e/team-b-admin-pages.cy.js` | Company profile, verification, moderation, analytics, audit log (JT-88) | 55 | Pass |
| `tests/e2e/job-pages.cy.js` | Job pages, search, saved jobs, notifications (JT-73) | 33 | Pass |
| **Stubbed GUI total (`npm test`, runs in CI)** | | **188** | **Pass** |
| `tests/e2e/stack/main-workflow.cy.js` | Main workflow through the real UI and services (JT-95) | 11 | Pass, 40 s |

## 4. Load test

LOAD_SECTION_PLACEHOLDER

## 5. SonarQube

SONAR_SECTION_PLACEHOLDER

## 6. Defect log

All Jira issues of type Bug.

| Jira key | Summary | Severity | Status | Fix PR |
|---|---|---|---|---|
| DEFECTS_PLACEHOLDER | | | | |

## 7. How to reproduce the figures

From the repository root on the final `main`, with Docker running (stop the Compose stack first to free memory):

    go test -count=1 -json -coverprofile=/tmp/jt97/unit.cover ./... > /tmp/jt97/unit.json
    go test -count=1 -json -tags=integration -coverprofile=/tmp/jt97/all.cover ./... > /tmp/jt97/all.json

Counts are the test events with a final pass, fail or skip in the JSON output; integration tests are those present only in the second run. Coverage is computed from the profiles over `services/application` and `services/application-status`, excluding files under `mocks/`. Cypress: `npm test` and `npm run test:stack` in `frontend` (see `tests/e2e/README.md`). Load: `tests/load/README.md`.

## 8. Conclusion

CONCLUSION_PLACEHOLDER
