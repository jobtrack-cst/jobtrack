# JT-102 employer company logo upload and display: test cases

Traceability: [JT-102](https://jobtrack-cst.atlassian.net/browse/JT-102) (Story, Should, 3 points, Sprint 4). Developer and employer owner: Jigden Shakya. Verification date: 18 November 2026.

## Scope

Employers can upload or replace one company logo (JPEG or PNG, 1 to 1,000,000 bytes). Any authenticated role can read it. `CompanyProfile` gets `has_logo`, and profile reads never load the image bytes. The frontend company page gets an upload section and a reusable `CompanyLogo` component that shows the logo or the company initials.

Changes: the approved additive `employer.proto` diff (`PutCompanyLogo`, `GetCompanyLogo`, `bool has_logo = 9`; Section 15, 18 Nov 2026, 17:16), migration `00004_create_company_logos.sql`, `domain/logo.go`, two `ProfileRepository` methods, `app/logos.go`, two gRPC handlers, the PostgreSQL adapter, and five frontend files in `frontend/src/features/employer/`. No new library, service, event or synchronous call.

Acceptance criteria used in the matrix:

- AC1: `employer.proto` additive only; `buf lint` and `buf breaking` pass; generated code committed
- AC2: JPEG or PNG only, 1 to 1,000,000 bytes, file signature checked (not just the declared type); every failure `INVALID_ARGUMENT`; the database enforces the same size and type limits
- AC3: only the owning employer can upload or replace (`PERMISSION_DENIED` otherwise); any authenticated role can read, like `GetCompanyProfile`; replacing never changes verification status
- AC4: profile reads that do not need the image never load its bytes
- AC5: goose migration; unit tests with boundary values (0 bytes, 1 byte, 1,000,000, 1,000,001, wrong signature, wrong type); integration test with Testcontainers PostgreSQL
- AC6: frontend upload on the company page with the same checks before sending; logo on the company page and job cards, initials as fallback; uses the JT-101 theme
- AC7: test cases in `docs/testing`

Rules:

- Checks run in this order: size, declared type, signature. JPEG starts with `FF D8 FF`, PNG with `89 50 4E 47 0D 0A 1A 0A`. `content_type` must match exactly (no trimming, lowercase only).
- A 1 byte file passes the size check but fails the signature check, so it is still `INVALID_ARGUMENT`. The smallest accepted content is the 3 byte JPEG signature.
- The role is checked before the input. The request has no `employer_id`, so the key is always the caller's own user ID.
- `PutCompanyLogo` returns `NOT_FOUND` without a profile; `GetCompanyLogo` returns `NOT_FOUND` when there is no profile or no logo.
- The logo lives in its own table `company_logos` (foreign key to `company_profiles`); profile queries read only `EXISTS (...)` for `has_logo`. The database also checks the signature.
- An upload sets the profile's `updated_at` (because `has_logo` changed) and never writes `verification_status` or `rejection_reason`.
- Gateway body limit (JT-54): the largest valid request is about 1.34 MB as base64 JSON, under 3 MiB.
- Frontend: same checks before sending, size checked before the file is read; fixed messages per gRPC code and HTTP 413; only validated JPEG or PNG data becomes an image source; one logo request per employer per session.

## Test matrix

Unit and integration paths are under `services/employer/internal/`.

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT102-01 | AC1 | `buf lint`, `buf breaking --against '.git#branch=main'`, `buf generate`, `go build ./...` | Contract | No lint or breaking errors; regenerated code builds | Contract, local and CI | Pass |
| JT102-02 | AC2, AC5 | Content of 0 bytes, nil, 1,000,000 bytes (JPEG and PNG), 1,000,001 bytes | Boundary value analysis | 0, nil and 1,000,001 rejected with `ErrInvalidLogoSize`; 1,000,000 accepted | Unit, `domain/logo_test.go` | Pass |
| JT102-03 | AC2, AC5 | 1 byte, 3 byte JPEG signature, 8 byte PNG signature, PNG cut to 7 bytes | Boundary value analysis | 1 byte and 7 byte PNG rejected with `ErrInvalidLogoContent`; 3 and 8 byte signatures accepted | Unit, same file | Pass |
| JT102-04 | AC2, AC5 | Declared type `image/gif`, `image/jpg`, `IMAGE/PNG`, `" image/png "`, empty | Equivalence partitioning | `ErrInvalidLogoType` | Unit, same file | Pass |
| JT102-05 | AC2, AC5 | PNG declared with JPEG bytes, JPEG declared with PNG bytes, PDF renamed to PNG, signature not at the start | Equivalence partitioning, error guessing | `ErrInvalidLogoContent` | Unit, same file | Pass |
| JT102-06 | AC2 | 1,000,001 bytes with type `image/gif` | Decision table | `ErrInvalidLogoSize` (size checked first) | Unit, same file | Pass |
| JT102-07 | AC3 | Employer uploads; repository called with the caller's own ID, the validated logo and the clock time | Equivalence partitioning | Profile returned with `HasLogo`, verification and reason unchanged | Unit, `app/logos_test.go` | Pass |
| JT102-08 | AC3 | Applicant and admin upload; applicant with invalid input | Equivalence partitioning | `ErrEmployerRoleRequired`; role checked before input; repository never called | Unit, same file | Pass |
| JT102-09 | AC3 | Another employer uploads | Equivalence partitioning | Repository called with that employer's own ID only | Unit, same file | Pass |
| JT102-10 | AC2 | Empty, oversized, GIF and wrong-signature content from an employer | Equivalence partitioning | Domain error returned; repository never called | Unit, same file | Pass |
| JT102-11 | AC3 | `GetLogo` as applicant, admin and another employer; uppercase ID; ID not a UUID; empty ID; no logo; repository error | Equivalence partitioning | Logo returned for every role; ID lowercased; `ErrInvalidEmployerID`; `ErrLogoNotFound` and repository errors passed through | Unit, same file | Pass |
| JT102-12 | AC3 | Both RPCs with no metadata, missing role, user ID not a UUID, unknown role | Equivalence partitioning | `UNAUTHENTICATED` | Unit, `adapters/grpc/logos_test.go` | Pass |
| JT102-13 | AC2, AC3 | `PutCompanyLogo` through the handler: owner, applicant, admin, role before input, nil request, 0 bytes, 1,000,001 bytes, wrong type, wrong signature, no profile | Equivalence partitioning, boundary value analysis | `OK` with `has_logo`, verification and `updated_at` set; `PERMISSION_DENIED`; `INVALID_ARGUMENT`; `NOT_FOUND` | Unit, same file | Pass |
| JT102-14 | AC3 | `GetCompanyLogo` through the handler: applicant, admin, owner, ID not a UUID, nil request, no logo | Equivalence partitioning | `OK` with the stored type and bytes; `INVALID_ARGUMENT`; `NOT_FOUND` | Unit, same file | Pass |
| JT102-15 | AC3 | Database failure on both RPCs (error text contains "secret") | Error guessing | `INTERNAL` with a fixed message; no driver detail | Unit, same file | Pass |
| JT102-16 | AC4 | `GetCompanyProfile` for a stored profile with a logo | Equivalence partitioning | `has_logo` true; the response message has no field for image bytes | Unit, same file | Pass |
| JT102-17 | AC2, AC3, AC4, AC5 | Rejected profile: no logo, upload PNG, read back, replace with JPEG, read back, then a profile update | State transition | `has_logo` false then true; `ErrLogoNotFound` before upload; bytes read back exactly; one row after replace; verification, reason and `created_at` unchanged; `updated_at` set to each upload time; profile update keeps `has_logo` | Integration, `adapters/postgres/logos_integration_test.go` | Pass |
| JT102-18 | AC3 | Upload and read for an employer without a profile | Equivalence partitioning | `ErrProfileNotFound` and `ErrLogoNotFound`; no logo row stored | Integration, same file | Pass |
| JT102-19 | AC2, AC5 | Direct inserts bypassing the domain: 0 bytes, 1 byte, 3 byte JPEG, 1,000,000, 1,000,001, GIF type, PNG with JPEG bytes, JPEG with PNG bytes | Boundary value analysis | Database accepts only the 3 byte JPEG and 1,000,000 byte PNG | Integration, same file | Pass |
| JT102-20 | AC5 | Logo row for an employer with no profile | Error guessing | Foreign key rejects the insert | Integration, same file | Pass |
| JT102-21 | AC5 | Migrations applied twice on start; all existing employer tests (profiles, verification consumer, wiring, server setup) | Regression | Migrations idempotent; all existing tests unchanged and passing | Unit and integration | Pass |
| JT102-22 | AC2, AC3, AC4 | Compose through the gateway with a new employer and applicant (20 checks, see Verification) | Manual end to end | All 20 checks pass | Manual, Compose (local helper, not committed) | Pass |
| JT102-23 | AC6 | Company page after creating a profile, before any upload | Equivalence partitioning | Logo section shows the initials `DL` and "Upload logo" | Manual, browser | Pass |
| JT102-24 | AC6 | Upload a real PNG, then log out and back in | State transition | "Logo added."; image shown; button "Replace logo"; "Last updated" changes; logo loads from the server after login | Manual, browser | Pass |
| JT102-25 | AC3, AC6 | Replace with another image | State transition | "Logo replaced." and the new image; verification still "Not verified" | Manual, browser | Pass |
| JT102-26 | AC2, AC6 | Choose `test.gif` (106 bytes), `fake.png` (PDF bytes, 108 bytes) and `big.png` (PNG signature, 1,000,008 bytes) | Equivalence partitioning, boundary value analysis | Matching fixed warning for each; no `logo` request in the Network tab; current logo unchanged | Manual, browser | Pass |
| JT102-27 | AC6 | Type in Description without saving, then upload | Error guessing | "Logo replaced."; one `PUT` (200) after the CORS preflight (204); typed text kept | Manual, browser | Pass |
| JT102-28 | AC6 | Logo on job cards with initials fallback | Not run | Job cards do not exist on `main` (JT-73 not merged). `CompanyLogo` takes `employer_id` and the company name and is ready to use | Not run | Blocked by JT-73 |
| JT102-29 | AC6 | Logo section uses the JT-101 theme | Not run | JT-101 not merged. The new components use plain MUI with no custom styling, so the theme applies once merged | Not run | Blocked by JT-101 |

## Verification

All commands ran on 18 November 2026 on Jigden's laptop (WSL Ubuntu, Docker Desktop), on branch `JT-102-company-logo` on top of `1c06303`.

    buf lint
    buf breaking --against '.git#branch=main'
    buf generate
    go build ./...
    gofmt -l services/employer
    go vet ./services/employer/...
    go generate ./services/employer/internal/ports/...
    go test -cover ./services/employer/...
    go test -tags=integration ./services/employer/...
    cd frontend && npm run lint && npm run build

| Check | Result |
|---|---|
| Contract | Pass, no `buf lint` or `buf breaking` output; regenerated `employer.pb.go`, `employer.pb.gw.go` and `employer_grpc.pb.go` committed |
| Formatting and static checks | Pass, no output |
| Unit tests | Pass, all packages `ok`. Coverage: domain 100.0%, app 100.0%, gRPC adapter 97.8%, cmd/server 82.8% (postgres is covered by the integration suite, Section 15, 11 Nov 2026) |
| Integration tests | Pass, all packages `ok` (postgres package 28.4 s, real PostgreSQL 18 container) |
| Compose | employer and gateway rebuilt from the branch; `00004_create_company_logos.sql` applied, database at version 4; consumer ready on `employer.verification` |
| End to end (JT102-22) | Pass, 20 of 20 checks through the gateway on port 8000 (employer `300ff3ef-d6b9-441f-87dc-2b31ff431d01`): upload before a profile 404; new profile without `has_logo`; logo before upload 404; real 1x1 PNG uploaded (200) with `has_logo` true and verification unchanged; applicant sees `has_logo` on the profile with no image bytes in the response; applicant reads the same PNG back; replaced with a JPEG; 1,000,000 bytes accepted; 1,000,001 bytes, 0 bytes, 1 byte, GIF and PNG with JPEG bytes each HTTP 400 with code 3; applicant upload 403; no token 401; unknown employer 404; ID not a UUID 400 |
| Frontend | `npm run lint` and `npm run build` pass; browser checks JT102-23 to JT102-27 pass (8 of 8). A page refresh logs the user out because the token is kept only in memory; this is existing frontend shell behaviour, so JT102-24 logs in again instead |
| CI on the pull request | Green on PR #71 |

## Not delivered in JT-102

JT102-28 and JT102-29 depend on JT-73 and JT-101. If either is not merged before the sprint ends, the job card logo and theme parts of AC6 are recorded as not delivered in the closure report (JT-100), following Section 14.
