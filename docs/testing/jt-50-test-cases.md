# JT-50 Applicant-profile service test cases

Traceability: [JT-50](https://jobtrack-cst.atlassian.net/browse/JT-50) (parent JT-11). Owner: Kinley Tobgay Lhendrup. Verification date: 12 November 2026.

## Scope

The three public `ApplicantProfileService` RPCs (`CreateMyProfile`, `GetMyProfile`, `UpdateMyProfile`) using the existing `applicant_profile.proto` contract without changes. A profile is stored in the `applicant_profile` database with its education entries and skills, in the order they were sent. The service publishes and consumes no events and has no internal RPCs.

Rules agreed for JT-50 (AGENTS.md Section 15):

- All three RPCs are applicant only and act on the caller's own profile; employers and admins get `PERMISSION_DENIED`, checked before input. A missing or malformed identity or an unknown role is `UNAUTHENTICATED`.
- A second create is `ALREADY_EXISTS`; get or update without a profile is `NOT_FOUND`. Update replaces all three fields (an empty list clears it) and keeps `created_at`.
- Limits, in characters after trimming: `summary` 0 to 2000 (line breaks allowed); up to 20 education entries; `institution` and `qualification` 1 to 150 on one line; `start_year` 1950 to next year; `end_year` 0 (ongoing) or `start_year` to 10 years ahead; up to 50 skills, each 1 to 50 on one line, no duplicates ignoring case; no invalid UTF-8 or other control characters. Every failure is `INVALID_ARGUMENT` and names the entry, for example `skills[2]`.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT50-01 | AC1 | Applicant creates a profile with education and skills | Equivalence partitioning | Stored trimmed, in order, owner from metadata, UTC timestamps | Unit, `internal/app`; gRPC; wiring | Pass |
| JT50-02 | AC1 | Stored profile round-trips every field in order, including ongoing study | Equivalence partitioning | `Get` returns exactly what was stored; `end_year` 0 kept | Integration, `internal/adapters/postgres` | Pass |
| JT50-03 | AC1 | Empty profile | Boundary values | Accepted; empty, non-nil lists | Unit; integration | Pass |
| JT50-04 | AC1 | Applicant gets own profile; before creating one | Equivalence partitioning | Profile returned; `NOT_FOUND` | Unit; gRPC; wiring | Pass |
| JT50-05 | AC1 | Update replaces every field and keeps `created_at`; empty update clears | Equivalence partitioning | Old entries removed; `created_at` unchanged | Unit; gRPC; integration; wiring | Pass |
| JT50-06 | AC1 | Second create; update without a profile | Equivalence partitioning | `ALREADY_EXISTS` with nothing changed; `NOT_FOUND` | Unit; gRPC; integration | Pass |
| JT50-07 | AC1 | Four simultaneous updates | Concurrency | Final profile is exactly one writer's content | Integration | Pass |
| JT50-08 | AC2 | Employer or admin calls each RPC | Authorization (role) | `PERMISSION_DENIED` before input or storage | Unit; gRPC; wiring | Pass |
| JT50-09 | AC2 | Every lookup and write uses the caller's ID; no request field names an applicant | Authorization (ownership) | Repository always called with the caller's own ID | Unit, `internal/app`; gRPC | Pass |
| JT50-10 | AC5 | Missing or malformed identity, unknown or wrong-case role | Equivalence partitioning | `UNAUTHENTICATED` on all three RPCs | Unit, `internal/domain`; gRPC | Pass |
| JT50-11 | AC6 | `summary` 2000 and 2001, multibyte, after trimming; line breaks; NUL; invalid UTF-8 | Boundary values | 2000 accepted, 2001 and bad text `INVALID_ARGUMENT`; database refuses 2001 | Unit; integration | Pass |
| JT50-12 | AC6 | 20 and 21 education entries | Boundary values | 20 accepted, 21 `INVALID_ARGUMENT`; database refuses the 21st | Unit; integration | Pass |
| JT50-13 | AC6 | `institution` and `qualification` 1, 150, 151, empty, whitespace, line break, multibyte | Boundary values | 1 and 150 accepted, others `INVALID_ARGUMENT`; database refuses 151 | Unit; integration | Pass |
| JT50-14 | AC6 | `start_year` 1949, 1950, 2027, 2028, 0 (current year 2026) | Boundary values | 1950 and 2027 accepted; database refuses before 1950 | Unit; integration | Pass |
| JT50-15 | AC6 | `end_year` 0, equal to start, before start, 2036, 2037, negative | Boundary values | 0, equal and 2036 accepted; database refuses end before start | Unit; integration | Pass |
| JT50-16 | AC6 | 50 and 51 skills; skill 50, 51, empty, whitespace, control character | Boundary values | Limits enforced; database refuses a 51-character skill | Unit; integration | Pass |
| JT50-17 | AC6 | Duplicate skills ignoring case and spaces, including multibyte case | Equivalence partitioning | `INVALID_ARGUMENT` naming the entry; database unique index refuses | Unit; gRPC; integration | Pass |
| JT50-18 | AC6 | A rejected write | Equivalence partitioning | The whole write rolls back; failed create leaves no profile | Integration | Pass |
| JT50-19 | AC5 | Database error, canceled and expired context | Equivalence partitioning | `INTERNAL` "internal error" without driver details; `CANCELED`; `DEADLINE_EXCEEDED` | gRPC; integration | Pass |
| JT50-20 | AC6 | Migrations applied twice on a fresh database | Equivalence partitioning | Both runs succeed | Integration | Pass |
| JT50-21 | AC9 | `docker compose up applicant-profile`, then `GET /healthz` | Equivalence partitioning | `HTTP/1.1 200 OK`; container `healthy` | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-50: AC1 and AC2 are the task criteria, AC3 to AC10 the shared Sprint 1 criteria (AC5 role and ownership through `pkg/authctx`, AC6 migrations and field limits, AC9 Docker Compose and `/healthz`).

## Verification

Commands run on branch `JT-50-applicant-profile-basic-operations`:

    go test -count=1 -cover ./services/applicant-profile/...
    go test -count=1 -tags=integration -v -run TestProfiles_Integration ./services/applicant-profile/internal/adapters/postgres/
    docker compose -f deploy/compose/docker-compose.yml up -d --build applicant-profile

| Check | Result |
|---|---|
| Unit tests | Pass |
| Integration tests (Testcontainers, `postgres:18-alpine`) | Pass, 9 subtests |
| Package coverage (unit) | `domain` 100%, `app` 100%, `adapters/grpc` 100%, `cmd/server` 83.3% |
| Docker Compose `/healthz` | `HTTP/1.1 200 OK` |
| CI on the pull request | Green, [CI run](https://github.com/jobtrack-cst/jobtrack/actions/runs/43216866313) |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Applicant creates, gets and updates their own profile, including education and skills | Met | JT50-01 to JT50-07 |
| Access rules for other roles follow the contract; only the owner can update | Met | JT50-08, JT50-09 |
| Created with `scripts/new-service.sh`, hexagonal layout | Met | First commit on the branch |
| Public RPCs exactly as in the contract; no contract change | Met | No `proto/` changes in the PR |
| Role and ownership through `pkg/authctx`, Section 9 codes | Met | JT50-08 to JT50-10, JT50-19 |
| goose migrations on startup; free-text limits set and noted in the PR | Met | JT50-11 to JT50-18, JT50-20 |
| Calls to other services as ports mocked with gomock | Met | No outgoing calls; repository and clock ports mocked in `internal/ports/mocks` |
| Table-driven unit tests for domain and app; Testcontainers repository test | Met | All JT50 cases |
| Compose service block; `GET /healthz` returns 200 | Met | JT50-21 |
| PR with the Jira key, CI green, merged, time logged | Pending | This PR |
