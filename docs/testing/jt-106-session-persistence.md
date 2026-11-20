# JT-106 session persistence verification

20 Nov 2026. Automated cases live in `tests/e2e/session-persistence.cy.js`
and `tests/e2e/auth.cy.js`, using the existing gateway-stubbed Cypress setup.

| Cases | Expected result |
|---|---|
| Applicant, employer and admin login, reload and direct deep link | Only token and absolute expiry are saved after `/v1/auth/me`; the same expiry survives reload; role comes from `/me`, even when login reports a different role |
| Delayed restoration | Loading status, no protected content or premature redirect; original path and query survive |
| Logout, one-hour expiry, protected 401 | Memory and sessionStorage cleared; reload requires login |
| Invalid JSON, null, arrays, missing/empty/malformed token, invalid/expired/excessive expiry, identity alone | Discard storage before calling `/me` |
| Restore 401, 403, 500 or unsupported identity | Discard stored credentials; do not expose a session |
| Storage disabled | Login and logout still function in memory |
| Login rejected by `/me`, expiry during `/me` | No authenticated session is exposed or persisted |
| Logout during login or `/me` | Late success cannot restore the old session |
| Replacement login while older `/me` returns 200, 401 or 500 | New session remains, including when both logins return the same token |
| Existing API error cases | Fixed error messages retained; public 401 and non-401 errors do not expire an active session |

From `frontend/`, run `npm run lint`, `npm run build`, and `npm test` with
`npm run dev` in another terminal. If the environment sets
`ELECTRON_RUN_AS_NODE=1`, use `env -u ELECTRON_RUN_AS_NODE npm test`.
For a configured running Compose stack, run `npm run test:stack` as described
in `tests/e2e/README.md`.

Manual UI checks remaining: in the intended browser, sign in with each role,
reload a protected page, open a deep link in the same tab, then log out and reload.
Check an idle/background tab after the one-hour expiry and check tab closure with
the browser's session-restore settings. Browser storage failures intentionally
fall back to memory and cannot preserve login across reloads.

Verification results (completed 20 Nov 2026): frontend lint and build passed;
build emitted a bundle-size warning. The full stubbed suite passed 244/244 tests,
including all 28 JT-106 cases, and the running Compose stack suite passed 11/11.
Cypress was run with `ELECTRON_RUN_AS_NODE` unset. The stack run created only the
spec's synthetic accounts and workflow data. Manual browser checks above remain.
