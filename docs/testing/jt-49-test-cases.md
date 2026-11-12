# JT-49 Application-status state machine test cases

Traceability: [JT-49](https://jobtrack-cst.atlassian.net/browse/JT-49) (parent JT-11). Owner: Kinley Tobgay Lhendrup. Verification date: 11 November 2026.

## Scope

The application status lifecycle from AGENTS.md Section 6, its history, and the two read-only public RPCs `GetApplicationStatus` and `ListApplicationStatusHistory`, using the existing `application_status.proto` contract without changes.

Transitions have no RPC. JT-49 implemented `app.Statuses.Start` and `app.Statuses.Transition`. JT-60 now consumes application submitted/withdrawn events and stores status events through a transactional outbox; see [JT-60 test cases](jt-60-test-cases.md).

Rules agreed for JT-49 (AGENTS.md Section 15):

- A new application starts at SUBMITTED with a first history entry whose `previous_status` is UNSPECIFIED.
- Allowed transitions are exactly the 15 in Section 6; every other change, including staying in the same status and leaving a final status, is `FAILED_PRECONDITION`.
- Each transition stores the new status and its history entry in one transaction, through a conditional update; of two concurrent transitions only one is stored.
- Status and history are readable only by the application's applicant (role applicant) and its employer (role employer). Admins are denied before input; others `PERMISSION_DENIED`; unknown `NOT_FOUND`. History is oldest first.
- Application-status has no free-text fields, so no length limits apply, and no `INTERNAL_TOKEN`.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT49-01 | AC1, AC3 | All 10 x 10 status pairs, including UNSPECIFIED, against `CanTransition` | State transition | Exactly the 15 Section 6 transitions allowed (written out independently in the test) | Unit, `internal/domain` | Pass |
| JT49-02 | AC1, AC3 | All 100 pairs through `Transition` | State transition | Allowed: new status, UTC `updated_at`, unchanged `created_at` and participants, exact history entry. Disallowed: `ErrInvalidTransition`, nothing returned | Unit, `internal/domain` | Pass |
| JT49-03 | AC1, AC3 | Four full lifecycles (accepted, declined, rejected after interview, withdrawn after offer) | State transition | History chains; no transition out of the final status | Unit, `internal/domain` | Pass |
| JT49-04 | AC1 | Disallowed transition maps to the Section 9 code | Equivalence partitioning | `FAILED_PRECONDITION` (also for a concurrent change) | Unit, `internal/adapters/grpc` | Pass |
| JT49-05 | AC2 | Start records SUBMITTED with history entry one; duplicate start | Equivalence partitioning | First entry UNSPECIFIED to SUBMITTED; duplicate `ALREADY_EXISTS` with no extra history | Unit, `internal/app`; integration | Pass |
| JT49-06 | AC2 | Full lifecycle stored in Postgres | State transition | 6 history entries, oldest first, chained, paged with the full total | Integration, `internal/adapters/postgres` | Pass |
| JT49-07 | AC2 | Stale writer and 4 simultaneous transitions | Concurrency | Stale write `ErrStatusChanged` with nothing stored; exactly one of four stored | Integration | Pass |
| JT49-08 | AC2 | Database guards: second first entry, non-SUBMITTED first entry, return to SUBMITTED, unknown application, unknown status | Equivalence partitioning | Refused; failed create leaves no row; driver details hidden | Integration; migration check | Pass |
| JT49-09 | AC2 | Applicant and owning employer read status and history | Authorization (ownership) | `OK` | Unit, `internal/app`; gRPC; wiring over real gRPC | Pass |
| JT49-10 | AC2 | Other applicant, other employer, applicant ID with employer role | Authorization (ownership) | `PERMISSION_DENIED`; history never read | Unit; gRPC; wiring | Pass |
| JT49-11 | AC2 | Admin on both RPCs, including with an invalid ID | Authorization (role) | `PERMISSION_DENIED` before any lookup | Unit; gRPC; wiring | Pass |
| JT49-12 | AC2 | Missing or malformed identity, unknown or wrong-case role | Equivalence partitioning | `UNAUTHENTICATED` on both RPCs | Unit, `internal/domain`; gRPC | Pass |
| JT49-13 | AC2 | Unknown application, invalid ID, nil request, bad pagination | Equivalence partitioning, boundary values | `NOT_FOUND`, `INVALID_ARGUMENT` | Unit; gRPC | Pass |
| JT49-14 | AC2 | Pagination size 0, 1, 100, 101, negative; token overflow and largest int | Boundary values | Default 20, cap 100, invalid `INVALID_ARGUMENT`; no `next_page_token` overflow (JT-65) | Unit, `internal/domain` | Pass |
| JT49-15 | AC2 | All 9 statuses and UNSPECIFIED map to the contract enum | Equivalence partitioning | Correct `ApplicationStatus` values | Unit, gRPC | Pass |
| JT49-16 | AC2 | Database error, canceled and expired context | Equivalence partitioning | `INTERNAL` "internal error", `CANCELED`, `DEADLINE_EXCEEDED` | Unit, gRPC | Pass |
| JT49-17 | AC9 | Migrations applied twice on a fresh database | Equivalence partitioning | Both runs succeed | Integration | Pass |
| JT49-18 | AC12 | `docker compose up application-status`, then `GET /healthz` | Equivalence partitioning | `HTTP/1.1 200 OK`; container `healthy` | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-49: AC1 to AC5 are the task criteria, AC6 to AC13 the shared Sprint 1 criteria.

## Verification

Commands run on branch `JT-49-application-status-state-machine`:

    go test -count=1 -cover ./services/application-status/...
    go test -count=1 -tags=integration -coverprofile=/tmp/as-int.out ./services/application-status/...
    go test -count=1 -coverprofile=/tmp/as-unit.out ./services/application-status/...
    docker compose -f deploy/compose/docker-compose.yml up -d --build application-status

| Check | Result |
|---|---|
| Unit tests | Pass |
| Integration tests (Testcontainers, `postgres:18-alpine`) | Pass, 9 subtests |
| Coverage, production code without generated mocks, unit + integration | **90.4%** (target 80%) |
| Coverage, production code without generated mocks, unit only | 73.9% (repository code is covered by integration tests) |
| Package coverage (unit) | `domain` 100%, `app` 100%, `adapters/grpc` 100%, `cmd/server` 83.2% |
| Docker Compose `/healthz` | `HTTP/1.1 200 OK` |
| CI on the pull request | Green, [CI run](https://github.com/jobtrack-cst/jobtrack/actions/runs/43066196140) |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| AC1: State machine exactly as in Section 6; any other transition `FAILED_PRECONDITION` | Met | JT49-01 to JT49-04 |
| AC2: Every transition stored in history; status and history readable by the applicant and the owning employer | Met | JT49-05 to JT49-16 |
| AC3: Tests cover every allowed and every disallowed transition | Met | JT49-01, JT49-02 (all 100 pairs) |
| AC4: At least 80 percent line coverage | Met | 90.4% unit + integration (Section 15 measure); 73.9% unit only |
| AC5: Event consumers and publishing `application.status_changed` are Sprint 2 | Not in scope | Marked in `app.Statuses.Transition` |
| AC6: Created with `scripts/new-service.sh`, hexagonal layout | Met | First commit on the branch |
| AC7: Public RPCs exactly as in the contract; no contract change | Met | No `proto/` changes in the PR |
| AC8: Role and ownership through `pkg/authctx`, Section 9 codes | Met | JT49-09 to JT49-16 |
| AC9: goose migrations on startup; free-text limits | Met | JT49-17; no free-text fields |
| AC10: Calls to other services as ports mocked with gomock | Met | No outgoing calls in JT-49; repository port mocked in `internal/ports/mocks` |
| AC11: Table-driven unit tests for domain and app; Testcontainers repository test | Met | All JT49 cases |
| AC12: Compose service block; `GET /healthz` returns 200 | Met | JT49-18 |
| AC13: PR with the Jira key, CI green, merged, time logged | Pending | This PR |
