# JT-104 Resume file name check test cases

Traceability: [JT-104](https://jobtrack-cst.atlassian.net/browse/JT-104) (Bug, `severity-trivial`), found by the Sprint 3 SonarQube run (JT-84, finding F1). Owner: Kinley Tobgay Lhendrup. Verification date: 18 November 2026.

## Fix

`frontend/src/features/resume/upload.ts` checked the file name with the regular expression `/[/\\\u0000]/`, which SonarQube reports as `typescript:S6324` (control character in a regular expression). The check now uses a list of the characters the resume service rejects (`/`, `\` and NUL, as in `cleanFileName` in `services/resume/internal/domain/resume.go`) and `String.includes`. Behaviour is unchanged.

## Test cases

| Case ID | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|
| JT104-01 | File name of 255 characters | Boundary value | Accepted and sent unchanged | Cypress, `tests/e2e/applicant-hiring-pages.cy.js` | Pass |
| JT104-02 | File name of 256 characters (existing case) | Boundary value | Rejected before upload | Cypress | Pass |
| JT104-03 | File name with a slash | Equivalence partitioning | Rejected before upload | Cypress | Pass |
| JT104-04 | File name with a backslash | Equivalence partitioning | Rejected before upload | Cypress | Pass |
| JT104-05 | File name with a NUL character | Equivalence partitioning | Rejected before upload | Cypress | Pass |
| JT104-06 | The rest of the JT-83 resume tests | Regression | All pass | Cypress | Pass |

The same cases, plus a blank name, were also run on the new `checkResume` directly in Node before the change was sent: all gave the expected result.

## Verification

    npm run lint && npm run typecheck && npm run build
    npm test

| Check | Result |
|---|---|
| Lint, typecheck, build | Pass |
| Cypress (`npm test`) | Pass |
| CI on the pull request | CI_RUN_PLACEHOLDER |
| SonarQube | `typescript:S6324` no longer reported; checked in the final run (JT-97) |
