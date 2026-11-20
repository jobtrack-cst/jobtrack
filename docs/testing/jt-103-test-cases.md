# JT-103 applicant profile photo upload and display: test cases

Traceability: [JT-103](https://jobtrack-cst.atlassian.net/browse/JT-103) (Story, Should, 3 points, Sprint 4). Developer and applicant-profile owner: Kinley Tobgay Lhendrup. Backup: Phuntsho Namgyel. Verification date: 19 November 2026.

## Scope

Applicants can upload or replace one profile photo (JPEG or PNG, 1 to 1,000,000 bytes) and read it back. Only the applicant sees their own photo. `ApplicantProfile` gets `has_photo`, and profile reads never load the image bytes. The frontend profile page gets an upload section, and the app bar shows the applicant's photo or initials.

Changes: the approved additive `applicant_profile.proto` diff (`PutMyProfilePhoto`, `GetMyProfilePhoto`, `bool has_photo = 6`; AGENTS.md Section 15, 19 Nov 2026, 22:12), the updated contract test, migration `00003_create_profile_photos.sql`, `domain/photo.go`, two `ProfileRepository` methods, `app/photos.go`, two gRPC handlers, the PostgreSQL adapter, and four frontend files in `frontend/src/features/applicant-profile/` plus `App.tsx`. No new library, service, event or synchronous call. The frontend reuses `toBase64` and `initialsOf` from the JT-102 logo code.

Acceptance criteria used in the matrix:

- AC1: `applicant_profile.proto` additive only (photo upload, photo read on the own profile); no renamed or removed fields; `buf lint` and `buf breaking` pass; generated code committed
- AC2: JPEG or PNG only, 1 to 1,000,000 bytes, file signature checked (not just the declared type); every failure `INVALID_ARGUMENT`; the database enforces the same size and type limits
- AC3: applicant only, own profile only, as in JT-50 (employers and admins `PERMISSION_DENIED`); no profile yet is `NOT_FOUND`
- AC4: profile reads that do not need the image never load its bytes
- AC5: goose migration; unit tests with boundary values (0 bytes, 1 byte, 1,000,000, 1,000,001, wrong signature, wrong type); integration test with Testcontainers PostgreSQL
- AC6: frontend upload on the profile page with the same checks before sending; photo shown on the profile page and the app bar, initials as fallback; uses the JT-101 theme
- AC7: test cases in `docs/testing`

Rules:

- Checks run in this order: size, declared type, signature. JPEG starts with `FF D8 FF`, PNG with `89 50 4E 47 0D 0A 1A 0A`. `content_type` must match exactly (no trimming, lowercase only).
- A 1 byte file passes the size check but fails the signature check, so it is still `INVALID_ARGUMENT`. The smallest accepted content is the 3 byte JPEG signature.
- The role is checked before the input. Neither request has an `applicant_id`, so the key is always the caller's own user ID.
- `PutMyProfilePhoto` returns `NOT_FOUND` without a profile; `GetMyProfilePhoto` returns `NOT_FOUND` when there is no profile or no photo.
- The photo lives in its own table `profile_photos` (foreign key to `applicant_profiles`); profile queries read only `EXISTS (...)` for `has_photo`. The database also checks the signature.
- An upload sets the profile's `updated_at` and changes nothing else on the profile. `UpdateMyProfile` keeps the photo.
- Gateway body limit (JT-54): the largest valid request is about 1.34 MB as base64 JSON, under 3 MiB.
- Frontend: same checks before sending, size checked before the file is read; fixed messages per gRPC code and HTTP 413, never the server's text; only validated JPEG or PNG data becomes an image source; one photo request per signed-in applicant, shared by the app bar and the profile page; after an upload both update without a new request. Employers and admins get no photo and no request.

## Test matrix

Unit and integration paths are under `services/applicant-profile/internal/`. Cypress cases are in `tests/e2e/profile-photo.cy.js` (gateway stubbed with `cy.intercept`).

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT103-01 | AC1 | `buf lint`, `buf breaking --against '.git#branch=main'`, `buf generate`, `go build ./...` | Contract | No lint or breaking errors; only the three `applicant_profile` generated files change; the build passes | Contract, local and CI | Pass |
| JT103-02 | AC1, AC3 | Contract test: five RPCs with exact verb, path and body; `has_photo` is field 6 and `bool`; four new messages with exact field numbers; photo requests carry no `applicant_id` | Contract | All checks pass | Contract, `tests/contract/applicant_profile_contract_test.go` | Pass |
| JT103-03 | AC2, AC5 | Content of 0 bytes, nil, 1,000,000 bytes (JPEG and PNG), 1,000,001 bytes; size checked before type | Boundary value analysis | 0, nil and 1,000,001 rejected with `ErrInvalidPhotoSize`; 1,000,000 accepted | Unit, `domain/photo_test.go` | Pass |
| JT103-04 | AC2, AC5 | 1 byte, 3 byte JPEG signature, 8 byte PNG signature, PNG cut to 7 bytes | Boundary value analysis | 1 byte and 7 byte PNG rejected with `ErrInvalidPhotoContent`; 3 and 8 byte signatures accepted | Unit, same file | Pass |
| JT103-05 | AC2, AC5 | Types `image/gif`, `image/jpg`, `IMAGE/PNG`, ` image/png `, empty | Equivalence partitioning | Each rejected with `ErrInvalidPhotoType` | Unit, same file | Pass |
| JT103-06 | AC2, AC5 | PNG declared with JPEG bytes, JPEG declared with PNG bytes, PDF renamed to PNG, signature not at the start | Equivalence partitioning | Each rejected with `ErrInvalidPhotoContent` | Unit, same file | Pass |
| JT103-07 | AC2 | Handler with nil request, 0 bytes, 1 byte, 1,000,001 bytes, wrong type, wrong signature | Equivalence partitioning | `INVALID_ARGUMENT`, nothing stored | Unit, `adapters/grpc/photos_test.go` | Pass |
| JT103-08 | AC2 | Repository given 0 bytes, 1,000,001 bytes, GIF, PNG with JPEG bytes, JPEG with PNG bytes, bypassing the domain | Boundary value analysis | Each rejected by the database `CHECK`s with the fixed message "put profile photo failed"; the previous photo is kept | Integration, `adapters/postgres/photos_integration_test.go` | Pass |
| JT103-09 | AC3 | Employer and admin call both RPCs; employer sends an invalid upload | Equivalence partitioning | `PERMISSION_DENIED` before any input check or storage | Unit, `app/photos_test.go`, `adapters/grpc/photos_test.go` | Pass |
| JT103-10 | AC3 | No metadata, missing role, user ID not a UUID, unknown role | Equivalence partitioning | `UNAUTHENTICATED` for both RPCs | Unit, `adapters/grpc/photos_test.go` | Pass |
| JT103-11 | AC3 | Upload without a profile; read without a photo | State transition | `NOT_FOUND` for both; nothing stored | Unit (app, gRPC) and integration | Pass |
| JT103-12 | AC3 | The repository is called with the caller's user ID from metadata | Equivalence partitioning | Photo stored for the caller only; a second applicant still has no photo | Unit (gRPC) and integration | Pass |
| JT103-13 | AC4 | `GetMyProfile` and profile updates report `has_photo` from `EXISTS`; only `GetPhoto` reads the bytes | Equivalence partitioning | `has_photo` true after upload, false before; profile responses carry no image bytes | Unit (gRPC) and integration | Pass |
| JT103-14 | AC2, AC4 | No photo, then a 1,000,000 byte PNG, then a 3 byte JPEG, then `UpdateMyProfile` | State transition | Upload sets `has_photo` and `updated_at` only (summary, education, skills and `created_at` unchanged); the JPEG replaces the PNG; the profile update keeps the photo | Integration, same file | Pass |
| JT103-15 | AC5 | Migrations applied twice on a fresh PostgreSQL 18 container | Not applicable (setup) | `00003_create_profile_photos.sql` applies and is safe to run again | Integration, `startProfileDB` | Pass |
| JT103-16 | AC6 | Applicant without a photo logs in and opens the profile page | Equivalence partitioning | Initials "PD" in the app bar and on the page; "Upload photo" offered | GUI, `profile-photo.cy.js` | Pass |
| JT103-17 | AC6 | Applicant with a photo logs in and opens the profile page | Equivalence partitioning | The same PNG in the app bar and on the page; one photo request shared by both; "Replace photo" offered | GUI, same spec | Pass |
| JT103-18 | AC6 | Upload a real 1x1 PNG | State transition | Request body `{contentType, content}` in base64; "Photo added."; photo shown on the page and in the app bar without a new request | GUI, same spec | Pass |
| JT103-19 | AC6 | Replace with a 3 byte JPEG | State transition | JPEG sent; "Photo replaced." | GUI, same spec | Pass |
| JT103-20 | AC6 | Choose a file of exactly 1,000,000 bytes | Boundary value analysis | Sent; decoded content is 1,000,000 bytes | GUI, same spec | Pass |
| JT103-21 | AC6 | Choose 0 bytes, 1,000,001 bytes, a GIF, a PNG name with JPEG bytes, a PDF renamed to JPEG | Boundary value analysis, equivalence partitioning | A fixed message before upload; no request sent | GUI, same spec (5 tests) | Pass |
| JT103-22 | AC6 | Gateway answers 400 (code 3), 404 (code 5), 403 (code 7), 413 | Equivalence partitioning | A fixed message for each, never the server's text; initials still shown | GUI, same spec (4 tests) | Pass |
| JT103-23 | AC6 | Applicant without a profile opens the profile page | State transition | Told to create the profile first; no upload control | GUI, same spec | Pass |
| JT103-24 | AC3, AC6 | Employer logs in | Equivalence partitioning | No avatar in the app bar and no photo request | GUI, same spec | Pass |
| JT103-25 | AC6 | Uses the JT-101 theme | Not applicable (review) | Photo styled by the shared theme | Review | Not delivered: JT-101 is not on `main`. The photo uses MUI's default `Avatar`, as the JT-102 logo does, and follows the theme once JT-101 merges |
| JT103-26 | Regression | Whole stubbed Cypress suite after the change | Not applicable (regression) | All specs pass | GUI, `npm test` | Pass, 208 of 208 in 6 specs |

## Verification

Run on 19 November 2026 on Kinley's MacBook Air (Apple M1, 8 GiB, Docker Desktop) and in CI, on branch `JT-103-applicant-profile-photo`, which includes `main` up to JT-107 (#92). Commands, from the repository root:

    buf lint
    buf breaking --against '.git#branch=main'
    buf generate
    go build ./...
    go generate ./services/applicant-profile/internal/ports/...
    gofmt -l services/applicant-profile tests/contract
    go vet ./services/applicant-profile/... ./tests/contract/...
    go test -count=1 ./...
    go test -count=1 -tags=integration ./services/applicant-profile/...
    cd frontend && npm run lint && npm run typecheck && npm run build
    npx cypress run --config-file cypress.config.cjs --spec ../tests/e2e/profile-photo.cy.js
    npm test

| Check | Result |
|---|---|
| Contract | Pass with `buf` 1.75.0, no `buf lint` or `buf breaking` output; regenerated `applicant_profile.pb.go`, `applicant_profile.pb.gw.go` and `applicant_profile_grpc.pb.go` committed; contract test updated and passing |
| Unit and integration tests | Pass in CI on the pull request (unit and coverage job, and the Testcontainers integration job) |
| Frontend | `npm run lint`, `npm run typecheck` and `npm run build` pass |
| Cypress | `profile-photo.cy.js` 16 of 16; whole stubbed suite 208 of 208 |
| CI on the pull request | Green |

Found and fixed during development:

- `tests/contract/applicant_profile_contract_test.go` pinned the old contract shape (three RPCs, five profile fields), so CI failed until it was updated to the approved diff.
- The in-memory `stubProfiles` in `cmd/server/register_test.go` needed the two new repository methods.
- The first Cypress run looked for the upload control as a `button`; MUI's `Button component="label"` renders a `label`, so 3 checks were corrected.

## Not verified

The photo RPCs were not exercised through the gateway against the running Docker Compose stack, unlike JT-102 (JT102-22). The handlers, repository and HTTP bindings are covered by the unit, integration, contract and stubbed GUI tests above; a live check needs applicant-profile and the gateway rebuilt from this branch.

JT103-25 (JT-101 theme) depends on JT-101. If it is not merged before the sprint ends, that part of AC6 is recorded as not delivered in the closure report (JT-100), following AGENTS.md Section 14.
