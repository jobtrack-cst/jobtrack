# JT-51 Review service test cases

Traceability: [JT-51](https://jobtrack-cst.atlassian.net/browse/JT-51) (parent JT-12). Owner: Kinley Tobgay Lhendrup. Verification date: 12 November 2026.

## Scope

The two public `ReviewService` RPCs (`CompleteReview`, `GetReview`) using the existing `review.proto` contract without changes. Reviews are stored in the `review` database, one per application.

Ownership comes from a local `application_participants` table, the trusted projection of `application.submitted` (the Offer and Messaging design). In this task it is read through the `ParticipantRepository` port, mocked with gomock in unit tests; the Sprint 2 consumer fills it. Until then no application is known at runtime, so `CompleteReview` returns `NOT_FOUND`. Publishing `review.completed` is Sprint 2 and is marked in `app.Reviews.Complete`.

Rules agreed for JT-51 (AGENTS.md Section 15, approved by Phuntsho):

- `CompleteReview` requires role `employer` (checked before input), a UUID `application_id`, a score from 1 to 5 and a decision SHORTLISTED or REJECTED; then a known application (`NOT_FOUND` otherwise) and the stored employer (`PERMISSION_DENIED` otherwise). One review per application; a second is `ALREADY_EXISTS`. Scores do not decide shortlisting.
- `GetReview` is readable only by the owning employer; applicants, admins and other employers get `PERMISSION_DENIED`; no review yet is `NOT_FOUND`.
- No free-text fields, so no length limits; no internal RPCs and no `INTERNAL_TOKEN`.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT51-01 | AC1 | Owning employer completes a review | Equivalence partitioning | Stored with a new UUID, employer from metadata, UTC timestamps | Unit, `internal/app`; gRPC; wiring | Pass |
| JT51-02 | AC1 | Both decisions, independent of score (score 1 SHORTLISTED, score 5 REJECTED) | Equivalence partitioning | Accepted and mapped to the contract enum both ways | Unit, `internal/domain`; gRPC; integration | Pass |
| JT51-03 | AC1 | Another employer completes or reads a review for the application | Authorization (ownership) | `PERMISSION_DENIED`; nothing stored or read | Unit; gRPC; wiring | Pass |
| JT51-04 | AC1 | Applicant or admin calls either RPC | Authorization (role) | `PERMISSION_DENIED` before input or lookup | Unit; gRPC; wiring | Pass |
| JT51-05 | AC1 | Unknown or not yet processed application | Equivalence partitioning | `NOT_FOUND` | Unit; gRPC; integration; wiring | Pass |
| JT51-06 | AC1 | Second review for the same application; five simultaneous reviews | Equivalence partitioning, concurrency | `ALREADY_EXISTS`, first review kept; exactly one stored | Unit; gRPC; integration; wiring | Pass |
| JT51-07 | AC1 | Owning employer reads the review; no review yet | Equivalence partitioning | Review returned; `NOT_FOUND` | Unit; gRPC; integration; wiring | Pass |
| JT51-08 | AC2 | Score 0, 1, 3, 5, 6 and negative | Boundary values | 1, 3 and 5 accepted; 0, 6 and negative `INVALID_ARGUMENT` | Unit, `internal/domain` and `internal/app`; gRPC | Pass |
| JT51-09 | AC2 | Database refuses scores 0 and 6; stores 1 and 5 | Boundary values | Check constraint refuses; driver details hidden; nothing stored | Integration; migration check | Pass |
| JT51-10 | AC2 | Decision UNSPECIFIED or an unknown enum value | Equivalence partitioning | `INVALID_ARGUMENT` | Unit; gRPC | Pass |
| JT51-11 | AC6 | Missing or malformed identity, unknown or wrong-case role | Equivalence partitioning | `UNAUTHENTICATED` on both RPCs | Unit, `internal/domain`; gRPC | Pass |
| JT51-12 | AC6 | Invalid `application_id`, nil request | Equivalence partitioning | `INVALID_ARGUMENT` | Unit; gRPC | Pass |
| JT51-13 | AC6 | Database error, canceled and expired context | Equivalence partitioning | `INTERNAL` "internal error" without driver details; `CANCELED`; `DEADLINE_EXCEEDED` | gRPC; integration | Pass |
| JT51-14 | AC8 | Participants read through a port mocked with gomock | Equivalence partitioning | Every rule unit-tested without a database | Unit, `internal/app` | Pass |
| JT51-15 | AC7 | Migrations applied twice; review for an unknown application refused | Equivalence partitioning | Both runs succeed; foreign key refuses | Integration | Pass |
| JT51-16 | AC10 | `docker compose up review`, then `GET /healthz` | Equivalence partitioning | `HTTP/1.1 200 OK`; container `healthy` | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-51: AC1 to AC3 are the task criteria (AC3, publishing `review.completed`, is Sprint 2), AC4 to AC11 the shared Sprint 1 criteria (AC6 role and ownership through `pkg/authctx`, AC7 migrations and limits, AC8 ports mocked with gomock, AC10 Docker Compose and `/healthz`).

## Verification

Commands run on branch `JT-51-review-basic-operations`:

    go test -count=1 -cover ./services/review/...
    go test -count=1 -tags=integration -v -run TestReviews_Integration ./services/review/internal/adapters/postgres/
    docker compose -f deploy/compose/docker-compose.yml up -d --build review

| Check | Result |
|---|---|
| Unit tests | Pass |
| Integration tests (Testcontainers, `postgres:18-alpine`) | Pass, 9 subtests |
| Package coverage (unit) | `domain` 100%, `app` 100%, `adapters/grpc` 98.4%, `cmd/server` 81.2% |
| Docker Compose `/healthz` | `HTTP/1.1 200 OK` |
| CI on the pull request | Green, [CI run](https://github.com/jobtrack-cst/jobtrack/actions/runs/43255549109) |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Employer scores and shortlists applications for their own jobs | Met | JT51-01 to JT51-07 |
| Score range validated, with boundary value tests | Met | JT51-08 to JT51-10 |
| Publishing `review.completed` is Sprint 2 | Not in scope | Marked in `app.Reviews.Complete` |
| Created with `scripts/new-service.sh`, hexagonal layout | Met | First commit on the branch |
| Public RPCs exactly as in the contract; no contract change | Met | No `proto/` changes in the PR |
| Role and ownership through `pkg/authctx`, Section 9 codes | Met | JT51-03, JT51-04, JT51-11 to JT51-13 |
| goose migrations on startup; free-text limits | Met | JT51-15; no free-text fields |
| Calls to other services as ports mocked with gomock | Met | JT51-14; participants port mocked |
| Table-driven unit tests for domain and app; Testcontainers repository test | Met | All JT51 cases |
| Compose service block; `GET /healthz` returns 200 | Met | JT51-16 |
| PR with the Jira key, CI green, merged, time logged | Pending | This PR |
