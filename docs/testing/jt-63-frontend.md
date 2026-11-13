# JT-63 frontend test cases

Automated cases: `tests/e2e/auth.cy.js`, run with `cd frontend && npm test`
while the Vite server is running on port 5173. Gateway responses are stubbed;
these are frontend behavior tests, not a live end-to-end backend test.

| Case | Partition or transition | Expected result |
|---|---|---|
| Unauthenticated routes | Applicant, employer, admin | Redirect to login without a loop |
| Role login | All three valid roles | Protected me request includes Bearer token; correct landing |
| Role isolation | Wrong role route for each role | Return to own landing |
| Logout/reload | Authenticated to anonymous | Login required |
| Invalid credentials | Public HTTP 401 | Inline error; form remains usable |
| Registration | Employer; two selectable roles | Exact contract fields; login required after success |
| Expiry | Valid session to expired | Login required; no refresh |
| Protected HTTP 401 | Authenticated to anonymous | Clear session and redirect to login |
| Public HTTP 401 | Anonymous login failure | No session invalidation callback |
| API failures | 400, 403, 409, 500 | Safe error, no session expiry or response body exposure |
| Transport failure | Network error | Usable error message |
| Invalid JSON | Successful status, malformed body | Usable error message |

Manual integration: follow `frontend/README.md` with the real gateway and Auth.
Verify browser CORS from http://localhost:5173 to http://localhost:8000 and a
real login for all three roles. Backend security is not replaced by UI routing.

## Local verification

Run on 13 Nov 2026 with Node 26.13.0 and npm 11.20.0 (CI uses Node 22):

| Command | Result |
|---|---|
| `npm ci --prefix frontend` | Passed; lockfile clean install |
| `npm run lint --prefix frontend` | Passed |
| `npm run typecheck --prefix frontend` | Passed |
| `npm run build --prefix frontend` | Passed |
| `env -u ELECTRON_RUN_AS_NODE npm test --prefix frontend` | 19 passed, 0 failed |
| `git diff --check` | Passed |

The IDE set `ELECTRON_RUN_AS_NODE=1`; removing it was necessary to launch the
Cypress browser locally. CI does not set this variable. The first browser run
used stale Vite dependency output after reinstalling packages and failed with
blank pages. Restarting Vite with `npm run dev --prefix frontend -- --force`
resolved it; the full suite then passed. No application assertions were disabled.
Live backend/CORS checks and the remote GitHub Actions run remain unrun.
