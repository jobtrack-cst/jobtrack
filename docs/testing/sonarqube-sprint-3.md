# SonarQube report, Sprint 3

Traceability: [JT-84](https://jobtrack-cst.atlassian.net/browse/JT-84) (parent JT-15). Owner: Kinley Tobgay Lhendrup (testing lead). Run on 17 November 2026. Covers AGENTS.md Section 10 (SonarQube once per sprint, report in `docs/testing`).

## How it was run

| Item | Value |
|---|---|
| Code | `main` at `b861368` (JT-76, #68) plus `sonar-project.properties`; analysed commit `5109238` |
| Server | SonarQube Community Build 26.8.0.126808 in Docker (`sonarqube:community`), on Kinley's MacBook Air |
| Scanner | SonarScanner CLI 8.4.0.16527 in Docker (`sonarsource/sonar-scanner-cli`, amd64 image under emulation) |
| Rules | Default "Sonar way" profiles for Go, TypeScript, JavaScript and Docker; default quality gate |
| Scope | `sonar-project.properties`: sources `cmd`, `functions`, `gateway`, `pkg`, `services`, `frontend/src`; tests, mocks, testdata and generated `.pb.go` excluded from sources |
| Coverage | Go unit tests only: `go test -coverprofile=coverage.out ./...`, 0 failing packages. No frontend coverage (Cypress does not produce any) |

Steps, from the repository root:

    docker run -d --name jt84-sonarqube -p 9000:9000 -e SONAR_SEARCH_JAVAADDITIONALOPTS="-Dnode.store.allow_mmap=false" sonarqube:community
    # create local project "jobtrack" in the UI and an analysis token; keep the token out of files and chat
    read -s SONAR_TOKEN && export SONAR_TOKEN
    go test -coverprofile=coverage.out ./...
    docker run --rm --network container:jt84-sonarqube -e SONAR_HOST_URL=http://localhost:9000 -e SONAR_TOKEN -v "$PWD:/usr/src" sonarsource/sonar-scanner-cli

Notes for the next run:

- Docker needs about 6 GB of memory. With 3.9 GB the TypeScript analysis failed ("bridge server is unresponsive"); at 5.9 GB it passed.
- Stop the Docker Compose stack first to free memory.
- Running the scanner inside the SonarQube container's network avoids `host.docker.internal`, which failed on the first attempt.
- A project analysis token cannot list security hotspots (HTTP 403 on `/api/hotspots/search`); the hotspot count is still available.

## Overall result

| Measure | Value |
|---|---|
| Quality gate | Passed. The default gate only checks issues in new code, and on a first analysis there is no new code, so this is not yet a meaningful signal |
| Lines of code | 30,159 |
| Security | 0 vulnerabilities, 0 security hotspots, rating A. Community Build has no taint analysis ("limited security analysis") |
| Reliability | 3 issues (1 bug), rating C, all in the frontend |
| Maintainability | 300 code smells, about 85 hours of estimated effort, rating A |
| Coverage | 67.8% line coverage on 17k coverable lines |
| Duplication | 34.2% of 37k lines |

By area:

| Area | Lines | Coverage | Duplication | Bugs | Code smells |
|---|---|---|---|---|---|
| `services` | 24,701 | 70.3% | 40.3% | 0 | 215 |
| `services/application` | 1,121 | 74.3% | 39.2% | 0 | 14 |
| `services/application-status` | 1,707 | 66.6% | 31.1% | 0 | 14 |
| `functions` | 2,115 | 72.3% | 11.3% | 0 | 11 |
| `gateway` | 542 | 84.5% | 2.6% | 0 | 1 |
| `pkg` | 951 | 71.1% | 0.0% | 0 | 14 |
| `frontend/src` | 1,782 | 0.0% | 0.0% | 1 | 40 |

## Findings

### F1. Reliability issues in the frontend (3)

| Rule | Type | Impact | Location | Assessment |
|---|---|---|---|---|
| `typescript:S6324` control character in a regular expression | Bug | Medium | `frontend/src/features/resume/upload.ts:11` | The `\u0000` in the file name check is deliberate: it rejects a NUL character, as the resume service does (JT-48). Behaviour is correct, but the intent is hidden in a regular expression. Logged as a Bug with severity Trivial |
| `typescript:S7758` prefer `String.fromCodePoint()` | Code smell | Low | `frontend/src/features/resume/upload.ts:25` | Converts bytes 0 to 255 to base64, where `fromCharCode` is correct. No defect |
| `typescript:S7781` prefer `String#replaceAll()` | Code smell | Low | `frontend/src/features/analytics/AnalyticsPage.tsx:21` | Style only. No defect |

The single bug sets the reliability rating to C. Fixing or accepting F1 raises it to A.

### F2. Coverage below the 80 percent target for `application` and `application-status`

SonarQube reports 74.3% and 66.6%. This is not the figure the Section 10 target is measured with: SonarQube counts every line in the folder, including `cmd/server` and the PostgreSQL and RabbitMQ adapters, which are tested by integration tests (build tag `integration`) that this run did not include. `go test -cover` per package, as in CI, reports per package instead. No bug logged. Proposed for the team: decide whether the 80 percent target applies to the SonarQube figure or to the per-package figure, and record it in Section 10.

### F3. Duplication of 34.2% (40.3% in `services`)

Most services follow the same hexagonal template, so `cmd/server` wiring, outbox relays, readiness checks and pagination code are very similar across services. This is a design consequence of the golden template rather than copy and paste within a service. No bug logged. Proposed for the team: consider moving more of the repeated wiring into `pkg` after the feature freeze.

### F4. Maintainability: 300 code smells

| Rule | Count | Where |
|---|---|---|
| `go:S3776` cognitive complexity above 15 | 237 | 214 in `_test.go` files (long table-driven tests), 23 in source |
| `go:S1192` repeated string literal | 21 | Mostly error messages such as "request is required" in gRPC and PostgreSQL adapters |
| `go:S4144` functions with identical bodies | 2 | Go source |
| TypeScript rules | 40 | All in `frontend/src`: 12 `S6759` (read-only props), 8 `S3358` (nested ternary), 4 `S7718`, 3 `S3776` complexity (`ProfilePage.tsx`, `api/client.ts`, `application/shared.tsx`), 3 `S6551` and others |

Highest cognitive complexity in source code:

| Function location | Complexity |
|---|---|
| `functions/resume-validator/internal/validator/pdf.go:47` | 45 |
| `functions/resume-validator/cmd/verify/main.go:31` | 42 |
| `services/application-status/internal/adapters/postgres/application_events.go:18` | 40 |
| `services/application-status/internal/adapters/postgres/hiring_events.go:21` | 26 |
| `services/resume/internal/adapters/postgres/validation.go:21` | 26 |
| `services/messaging/scripts/smoke.go:33` | 24 |

These are not defects and are not logged as bugs. They are candidates for refactoring after the feature freeze, starting with the three functions above 30.

### F5. Scope to review before the Sprint 4 run

- `services/*/scripts` (smoke scripts) are counted as source code; consider excluding them.
- Issues in `_test.go` files count towards maintainability; consider whether test complexity should be reported separately.

## Bugs logged in Jira

| Jira key | Summary | Severity | Owner |
|---|---|---|---|
| JT-104 | `resume page: file name check uses a control character in a regular expression (SonarQube S6324)` | Trivial | Kinley |

## Evidence

Overview screenshot of the analysis (quality gate Passed, Security A, Reliability C, Maintainability A, coverage 67.8%, duplication 34.2%) attached to JT-84 in Jira. The local helper used to export the figures through the SonarQube Web API is not committed.
