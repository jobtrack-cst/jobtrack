# JobTrack frontend (JT-63)

From the repository root:

```sh
cd frontend
npm ci
npm run dev
```

Open http://localhost:5173. The dev server requires that exact port. Start the
existing backend using the repository's Compose instructions first. Browser
requests go directly to http://localhost:8000, without a proxy or cookies.
The gateway must allow `CORS_ALLOWED_ORIGIN=http://localhost:5173` (the Compose
default). Use `localhost`, not `127.0.0.1`, for the browser origin.

Login uses `POST /v1/auth/login`, then confirms the account and role through
`GET /v1/auth/me`. Registration uses `POST /v1/auth/register` and returns to
login; only applicants and employers can register. JSON follows grpc-gateway's
camelCase field names and string enum names from `auth.proto`.

The JWT stays in memory. Reloading or closing the page requires login again.
Logout, the server's `expiresAt`, and protected HTTP 401 responses clear the
session. There is no refresh flow. Public login/registration failures stay on
the form, avoiding redirect loops. Client routing is a UI convenience; the
gateway and services remain responsible for authentication and authorization.
Feature directories reserve space for service owners; no service pages are
implemented beyond the role landing pages.

Validation:

```sh
npm run lint
npm run typecheck
npm run build
# Keep npm run dev running in another terminal:
npm test
```

`lint` uses TypeScript's unused-local, unused-parameter, and switch fallthrough
checks, alongside strict type checking. It introduces no separate lint tool.
Cypress tests live in `tests/e2e/auth.cy.js` and stub gateway responses; they do
not prove live backend connectivity or CORS. For a manual integration check,
register synthetic applicant and employer accounts, log in with each and the
seeded admin, verify each landing page, then log out. In browser Network tools,
verify an allowed preflight and a protected `/v1/auth/me` request; never copy or
log the token. A static host must serve `index.html` for client-side routes.

If an IDE terminal exports `ELECTRON_RUN_AS_NODE=1`, launch the tests with
`env -u ELECTRON_RUN_AS_NODE npm test`. After reinstalling dependencies, restart
Vite if its dependency cache is stale (`npm run dev -- --force`).
