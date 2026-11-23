# JT-114 design: applicant names and employer access to resume, profile and photo

Status: proposed by Jigden (auth owner) with authority delegated by the team,
23 Nov 2026. Owners of the changed services (Phuntsho: application; Kinley:
resume, applicant-profile) review the PRs when available. Record each part as a
row in AGENTS.md Section 15 in the PR that implements it.

AGENTS.md stays the source of truth. All contract changes below are additive:
`buf lint` and `buf breaking` must pass.

---

## 1. Goal

The owning employer of an application can see:

1. the applicant's name (employer pages and messages),
2. the resume used for that application,
3. the applicant's profile and photo.

Nobody else gains access. Applicants keep their existing owner-only RPCs.

Out of scope: names for applicants to see (they already see the company name
from the public `GetCompanyProfile`), editing anything, admin access.

---

## 2. Part A: applicant name on the application

### Flow

`SubmitApplication` already checks the job (job-posting) and the resume
(resume). After those checks pass, application calls
`AuthInternalService.GetUser` for the caller's user ID, stores `full_name` on
the application row, and returns it on `Application`.

- New internal synchronous call: `application` -> `auth`
  `AuthInternalService.GetUser`. Add it to the AGENTS.md Section 6 table.
- application uses `grpcclient.InternalUnaryClientInterceptor(INTERNAL_TOKEN,
  SERVICE_NAME)` with `SERVICE_NAME=application` (JT-44 pattern, as in JT-76).
- auth's internal allowlist for `GetUser` becomes `notification-dispatcher`
  and `application`.
- Fail closed: if auth is unavailable or returns an error, the submission is
  not stored and returns `UNAVAILABLE` (no application without a name).
- The name is copied once at submission. A later name change in auth does not
  change existing applications.

### Contract (application.proto)

```proto
message Application {
  // ... fields 1 to 7 unchanged
  // The applicant's full name from auth at submission time. Empty for
  // applications submitted before JT-114.
  string applicant_name = 8;
}
```

No change to `events.proto`: consumers do not need the name yet.

### Storage and configuration

- Migration: `ALTER TABLE applications ADD COLUMN applicant_name text NOT NULL
  DEFAULT ''` with a length CHECK of 0 to 200 characters. Old rows stay empty.
- Compose `application` block: add `AUTH_ADDR: auth:9090` and
  `INTERNAL_TOKEN`. Startup fails on a missing or invalid value (JT-44 rules).

### Who sees it

`GetApplication` and `ListApplications` already allow only the applicant and
the owning employer, so no new access rule is needed.

---

## 3. Part B: resume for the owning employer

### Projection (same pattern as offer, review and interview)

resume consumes `application.submitted` from the quorum queue
`resume.applications` (dead queue `resume.applications.dead`) through
`events.Consume`, with the JT-79 rules:

- producer must be `application`; `event_id`, `application_id`,
  `applicant_id`, `resume_id`, `employer_id` must be UUIDs (stored lowercase);
  anything else goes to the dead queue at once (`events.Permanent`);
- one transaction claims the `event_id` in `processed_events` and stores the
  row in `application_resumes (application_id PK, applicant_id, employer_id,
  resume_id, created_at)`; a repeated `event_id` changes nothing; the same
  participants under a new `event_id` are accepted; different participants for
  a stored application go to the dead queue without keeping the claim;
- database errors are retried under the JT-61 policy.

resume already has its own validation consumer (JT-59); the new consumer uses
the shared `events.Consume` and does not change JT-59.

### Contract (resume.proto)

```proto
service ResumeService {
  // ... existing RPCs unchanged

  // DownloadApplicationResume returns the resume submitted with one
  // application. Employer only, and only the application's owning employer.
  rpc DownloadApplicationResume(DownloadApplicationResumeRequest)
      returns (DownloadApplicationResumeResponse) {
    option (google.api.http) = {get: "/v1/applications/{application_id}/resume"};
  }
}

message DownloadApplicationResumeRequest {
  string application_id = 1;
}

message DownloadApplicationResumeResponse {
  string file_name = 1;
  string content_type = 2;
  bytes content = 3;
}
```

### Rules

| Case | Result |
|---|---|
| Role not employer | `PERMISSION_DENIED`, checked before input |
| `application_id` not a UUID | `INVALID_ARGUMENT` |
| Application unknown or not processed yet | `NOT_FOUND` |
| Caller is not the stored `employer_id` | `PERMISSION_DENIED` |
| Resume row no longer exists | `NOT_FOUND` |
| Otherwise | The stored resume's file name, type and bytes |

Access stays after a withdrawal: the employer keeps the record of who applied.

---

## 4. Part C: profile and photo for the owning employer

### Projection

applicant-profile consumes `application.submitted` from the quorum queue
`applicant-profile.applications` (dead queue
`applicant-profile.applications.dead`) with the same rules as Part B, storing
`application_participants (application_id PK, applicant_id, employer_id,
created_at)`.

### Contract (applicant_profile.proto)

```proto
service ApplicantProfileService {
  // ... existing RPCs unchanged

  // GetApplicantProfileForApplication returns the profile of the applicant of
  // one application. Employer only, and only the owning employer.
  rpc GetApplicantProfileForApplication(GetApplicantProfileForApplicationRequest)
      returns (GetApplicantProfileForApplicationResponse) {
    option (google.api.http) = {get: "/v1/applications/{application_id}/applicant-profile"};
  }

  // GetApplicantPhotoForApplication returns that applicant's photo.
  // Same access rule.
  rpc GetApplicantPhotoForApplication(GetApplicantPhotoForApplicationRequest)
      returns (GetApplicantPhotoForApplicationResponse) {
    option (google.api.http) = {get: "/v1/applications/{application_id}/applicant-photo"};
  }
}

message GetApplicantProfileForApplicationRequest {
  string application_id = 1;
}

message GetApplicantProfileForApplicationResponse {
  ApplicantProfile profile = 1;
}

message GetApplicantPhotoForApplicationRequest {
  string application_id = 1;
}

message GetApplicantPhotoForApplicationResponse {
  string content_type = 1;
  bytes content = 2;
}
```

### Rules

| Case | Result |
|---|---|
| Role not employer | `PERMISSION_DENIED`, checked before input |
| `application_id` not a UUID | `INVALID_ARGUMENT` |
| Application unknown or not processed yet | `NOT_FOUND` |
| Caller is not the stored `employer_id` | `PERMISSION_DENIED` |
| Applicant has no profile (or no photo, for the photo RPC) | `NOT_FOUND` |
| Otherwise | The current profile (or photo) |

The profile is read live, so the employer sees the applicant's current
details. Profile reads never include photo bytes (JT-103 rule kept).

---

## 5. Gateway, frontend and tests

- The new RPCs are in existing public services, so `buf generate` adds their
  routes; the gateway needs no code change. Commit regenerated `gen/go`.
- Frontend in this task: only request functions in the matching
  `features/*/api.ts` and the `Application.applicant_name` type. The employer
  pages that use them are JT-111; the Messages inbox is JT-113.
- Contract fixture: no change (`application.submitted` is unchanged).
- Each part: unit tests (table-driven, gomock ports) for every rule row above,
  Testcontainers integration tests for the new tables and consumers, and the
  auth allowlist test for the new caller.
- `buf lint`, `buf breaking`, `go test ./...`, `go test -tags=integration ./...`
  and CI pass for every PR.

---

## 6. Delivery

Three PRs under JT-114, each with its own AGENTS.md Section 15 row:

1. Part A: auth allowlist, application name, Compose settings, Section 6 row.
2. Part B: resume projection and `DownloadApplicationResume`.
3. Part C: applicant-profile projection and the two RPCs.

After a part is merged, rebuild the changed containers before testing:
`docker compose -f deploy/compose/docker-compose.yml build <service>`, then
`up -d --no-build <service>`. Existing applications have no projection rows
until a new `application.submitted` arrives; synthetic data, so no backfill.
