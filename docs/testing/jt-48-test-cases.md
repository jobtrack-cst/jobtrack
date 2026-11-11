# JT-48 Resume service test cases

Traceability: [JT-48](https://jobtrack-cst.atlassian.net/browse/JT-48) (parent JT-11). Owner: Kinley Tobgay Lhendrup. Verification date: 11 November 2026.

## Scope

All four public `ResumeService` RPCs (`UploadResume`, `GetResume`, `ListMyResumes`, `DownloadResume`) and the internal `ResumeInternalService.GetResumeFile` use the existing `resume.proto` contract without changes. Resumes are stored in the `resume` database as PENDING. `GetResumeFile` is protected by the JT-44 internal interceptor and allowed only for `resume-validator`.

Out of scope (Sprint 2, separate task): publishing `resume.uploaded` and consuming `resume.validated` and `resume.rejected`. Until then every stored resume stays PENDING; `app.Resumes.Upload` marks where the event is published.

Rules agreed for JT-48 (AGENTS.md Section 15):

- All four public RPCs are applicant only; employers and admins get `PERMISSION_DENIED`, checked before any input. A missing or malformed identity or an unknown role is `UNAUTHENTICATED`.
- `GetResume` and `DownloadResume` are owner only: another applicant gets `PERMISSION_DENIED` (the mapping `application` already uses for a wrong owner); an unknown resume is `NOT_FOUND`; a `resume_id` that is not a UUID is `INVALID_ARGUMENT`.
- Upload limits from the contract: `content` 1 to 2,000,000 bytes; `content_type` exactly `application/pdf`; `content` starts with `%PDF-` (deeper checks stay in `resume-validator`); `file_name` 1 to 255 characters after trimming, without `/`, `\`, NUL or invalid UTF-8. Every failure is `INVALID_ARGUMENT`. The database enforces the same size, type and length limits.
- `ListMyResumes` lists only the caller's resumes, newest first (`resume_id` breaks ties), without file content, using the shared pagination rules.
- `rejection_reason` has no user-entered limit; it is set only by the Sprint 2 `resume.rejected` consumer.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT48-01 | AC1 | Applicant uploads a valid PDF | Equivalence partitioning | Stored as PENDING with a new UUID, owner from metadata, trimmed name, size and UTC timestamps | Unit, `internal/app`; gRPC, `internal/adapters/grpc` | Pass |
| JT48-02 | AC1 | Stored resume round-trips every field and the binary content byte for byte | Equivalence partitioning | `GetByID` and `GetFile` return what was stored | Integration, `internal/adapters/postgres` | Pass |
| JT48-03 | AC2 | Content of 1,999,999, 2,000,000 and 2,000,001 bytes | Boundary values | First two accepted, 2,000,001 `INVALID_ARGUMENT` | Unit, `internal/domain`, `internal/app`, `internal/adapters/grpc` | Pass |
| JT48-04 | AC2 | Database stores 2,000,000 bytes and refuses 2,000,001 | Boundary values | Insert succeeds, then fails on `resumes_size_bytes_check` | Integration, `internal/adapters/postgres` | Pass |
| JT48-05 | AC2 | 2,000,000-byte upload over a real gRPC connection | Boundary values | `OK`; fits the default 4 MB gRPC message limit | Wiring, `cmd/server` | Pass |
| JT48-06 | AC2 | Empty content, not a PDF, signature not at start, lowercase `%pdf-` | Equivalence partitioning | `INVALID_ARGUMENT` | Unit, `internal/domain` | Pass |
| JT48-07 | AC2 | `content_type` empty, `text/plain`, `Application/PDF` | Equivalence partitioning | `INVALID_ARGUMENT` | Unit, `internal/domain`; integration (database check) | Pass |
| JT48-08 | AC9 | `file_name` 1, 255, 256 characters; 255 multibyte; trimmed; empty; whitespace; `/`, `\`, NUL, invalid UTF-8 | Boundary values | 1 and 255 accepted (also multibyte and after trimming), others `INVALID_ARGUMENT`; database refuses 256 and empty | Unit, `internal/domain`; integration | Pass |
| JT48-09 | AC3 | Applicant lists own resumes with a next page and with the default page size | Equivalence partitioning | Only the caller's resumes, `total_count` and `next_page_token` correct | Unit, `internal/app`; gRPC | Pass |
| JT48-10 | AC3 | List ordering, ties, paging past the end, applicant with no resumes | Boundary values | Newest first, `resume_id` descending on ties, empty list with total 0 | Integration, `internal/adapters/postgres` | Pass |
| JT48-11 | AC3 | Pagination: size 0, 1, 100, 101, negative; token 0, offset, `-1`, `+5`, letters, space, overflow, largest int | Boundary values | Default 20, cap 100, invalid values `INVALID_ARGUMENT`; no `next_page_token` overflow (JT-65) | Unit, `internal/domain`; gRPC | Pass |
| JT48-12 | AC3, AC8 | Another applicant calls `GetResume` or `DownloadResume` | Authorization (ownership) | `PERMISSION_DENIED`, nothing returned | Unit, `internal/app`; gRPC | Pass |
| JT48-13 | AC8 | Employer or admin calls each of the four public RPCs | Authorization (role) | `PERMISSION_DENIED` before any repository call | Unit, `internal/app`; gRPC; wiring | Pass |
| JT48-14 | AC8 | No metadata, missing ID or role, non-UUID ID, unknown or wrong-case role | Equivalence partitioning | `UNAUTHENTICATED` on all four RPCs | Unit, `internal/domain`; gRPC | Pass |
| JT48-15 | AC8 | Unknown resume, non-UUID `resume_id` | Equivalence partitioning | `NOT_FOUND`, `INVALID_ARGUMENT` | Unit, `internal/app`; gRPC; integration | Pass |
| JT48-16 | AC8 | Database error, canceled and expired context | Equivalence partitioning | `INTERNAL` "internal error" without driver details; `CANCELED`; `DEADLINE_EXCEEDED` | gRPC; integration (fixed messages) | Pass |
| JT48-17 | AC4 | Application's call: `GetResume` with forwarded applicant identity returns PENDING, VALIDATED or REJECTED with `rejection_reason` | Equivalence partitioning | Status and reason mapped to the contract enum | gRPC | Pass |
| JT48-18 | AC4 | `GetResumeFile` without user identity | Equivalence partitioning | File returned; invalid ID `INVALID_ARGUMENT`; unknown `NOT_FOUND` | Unit, `internal/app`; gRPC | Pass |
| JT48-19 | AC4 | `GetResumeFile` over real gRPC: resume-validator with right token, wrong token, `notification-dispatcher`, public client | Authorization (internal caller) | `OK`, `UNAUTHENTICATED`, `PERMISSION_DENIED`, `UNAUTHENTICATED` | Wiring, `cmd/server` | Pass |
| JT48-20 | AC4 | Missing or invalid `INTERNAL_TOKEN` (empty, space, tab, non-ASCII) | Equivalence partitioning | Startup refused before the database opens; error never contains the value | Wiring, `cmd/server` | Pass |
| JT48-21 | AC9 | Migrations applied twice on a fresh database | Equivalence partitioning | Both runs succeed | Integration, `internal/adapters/postgres` | Pass |
| JT48-22 | AC12 | `docker compose up resume`, then `GET /healthz` inside the container | Equivalence partitioning | `HTTP/1.1 200 OK`, body `ok`; container `healthy` | Docker Compose (manual) | Pass |

AC numbers follow the order of the acceptance criteria in JT-48: AC1 to AC5 are the task criteria, AC6 to AC13 the shared Sprint 1 criteria.

## Verification

Commands run on branch `JT-48-resume-upload-and-status`:

    go test -count=1 -cover ./services/resume/...
    go test -count=1 -tags=integration -v -run TestResumes_Integration ./services/resume/internal/adapters/postgres/
    docker compose -f deploy/compose/docker-compose.yml up -d resume
    docker compose -f deploy/compose/docker-compose.yml exec resume wget -S -qO- http://localhost:8080/healthz

| Check | Result |
|---|---|
| Unit tests | Pass |
| Integration tests (Testcontainers, `postgres:18-alpine`) | Pass, 13 subtests |
| Coverage | `domain` 100.0%, `app` 100.0%, `adapters/grpc` 98.8%, `cmd/server` 84.8% |
| Docker Compose `/healthz` | `HTTP/1.1 200 OK` |
| CI on the pull request | Green, [CI run](https://github.com/jobtrack-cst/jobtrack/actions/runs/43056397387) |

Local note: the first Compose start on a Mac with the repository on the Desktop needed Docker Desktop's Desktop-folder permission, and a RabbitMQ container created during a port clash had to be recreated (`up -d --force-recreate rabbitmq`) before `resume` could resolve `rabbitmq`. Neither is a code change.

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| AC1: Upload stored in the service database with status PENDING | Met | JT48-01, JT48-02 |
| AC2: Files over 2 MB (2,000,000 bytes, per the contract) rejected with `INVALID_ARGUMENT`, boundary tests | Met | JT48-03 to JT48-07 |
| AC3: Applicant lists own resumes and status; other users cannot read them | Met | JT48-09 to JT48-13 |
| AC4: RPCs used by application and resume-validator implemented | Met | JT48-17 to JT48-20 |
| AC5: Events are Sprint 2 | Not in scope | Marked in `app.Resumes.Upload` |
| AC6: Created with `scripts/new-service.sh`, hexagonal layout | Met | First commit on the branch |
| AC7: Public RPCs exactly as in the contract; no contract change | Met | No `proto/` changes in the PR |
| AC8: Role and ownership through `pkg/authctx`, Section 9 codes | Met | JT48-12 to JT48-16 |
| AC9: goose migrations on startup; free-text limits set and noted in the PR | Met | JT48-08, JT48-21 |
| AC10: Calls to other services as ports mocked with gomock | Met | No outgoing calls in JT-48; repository and clock ports mocked in `internal/ports/mocks` |
| AC11: Table-driven unit tests for domain and app; Testcontainers repository test | Met | All JT48 cases at unit and integration level |
| AC12: Compose service block; `GET /healthz` returns 200 | Met | JT48-22 |
| AC13: PR with the Jira key, CI green, merged, time logged | Pending | This PR |
