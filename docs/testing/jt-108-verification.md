# JT-108 verification — 19 Nov 2026

Implemented on `JT-108-fix-offer-discovery`. Changes are uncommitted.
The repository-root `AGENTS.md` matched local `main` before this change.

## Acceptance criteria

| Criterion | Implementation and evidence |
|---|---|
| Additive contract | `GetOfferByApplicationRequest.application_id = 1`, `GetOfferByApplicationResponse.offer = 1`, unary `OfferService.GetOfferByApplication`, `GET /v1/applications/{application_id}/offer`. Existing RPCs and field numbers preserved. Contract descriptor tests cover all five RPCs, bindings and fields; breaking check against local main passed. |
| End-to-end backend | Added repository port, generated mock, application use case, indexed PostgreSQL lookup and gRPC handler. Existing `offers_application_id_key` unique constraint enforces lifetime uniqueness; no migration required. |
| Ownership and errors | Uses existing `callerFrom`, `domain.NewCaller`, `NormalizeID`, `Offer.CanView` and `toStatus`. Associated applicant/applicant-role and employer/employer-role succeed; unrelated users, mismatched roles and admins denied. Malformed identity is UNAUTHENTICATED, malformed application ID INVALID_ARGUMENT, missing offer NOT_FOUND. Unit/gRPC tests assert no offer on failure. No new service calls or event schemas. |
| Frontend discovery | Every application card links to its role's existing offers route with `?applicationId=…`. Query parameter auto-loads the offer, supports revisit and reopening the URL after reload/login, and rejects malformed IDs. Displays loading, absent-offer and safe server-error states with retry. Stale responses are ignored after query navigation. New offers have a View Offer link to the same reloadable lookup. |
| Compatibility | Offer-ID form, existing routes, button labels/selectors, MakeOffer and applicant Accept/Decline remain. Failed decisions retain the offer so users can retry. Existing Go regression tests and Cypress offer-ID/create/decision scenarios retained. |
| Tests | Application authorization matrix; gRPC validation/authorization/error matrix and generated HTTP binding test; real PostgreSQL lookup, final-status lookup, ownership through the app and closed-database error tests; eight Cypress discovery scenarios. |
| Decision log | One dated 19 Nov 2026 JT-108 row added to AGENTS.md documenting approval and participant semantics. |

## Commands and results

Commands run from the repository root unless marked `frontend/`.

| Command | Result |
|---|---|
| `buf lint` | Passed. |
| `buf generate` | Passed using the existing remote plugin configuration; generated Offer protobuf, gRPC and gateway files. Initial sandbox attempt could not reach buf.build; rerun with network access succeeded. A second successful generation was compared using SHA-256 for every `gen/go` file; all were unchanged. |
| `buf breaking --against '.git#ref=main'` | Passed. |
| `go generate ./services/offer/internal/ports` | Passed; generated repository mocks. Initial sandbox attempt lacked network/cache permissions; rerun with access succeeded. |
| `gofmt -w services/offer/internal/app/offers.go services/offer/internal/app/offers_test.go services/offer/internal/ports/offers.go services/offer/internal/adapters/grpc/offer.go services/offer/internal/adapters/grpc/offer_test.go services/offer/internal/adapters/postgres/offers.go services/offer/internal/adapters/postgres/offers_integration_test.go` | Completed. |
| `gofmt -w tests/contract/offer_contract_test.go` | Completed. |
| `go test ./services/offer/... ./gateway/... ./tests/contract/...` | Passed after updating the contract test's four-RPC expectation to the approved five RPCs. Initial run failed only those stale contract assertions. |
| `go test -tags=integration ./services/offer/...` | Passed with Docker and real PostgreSQL/RabbitMQ Testcontainers; PostgreSQL package completed in 28.646 seconds. |
| `go vet ./services/offer/... ./gateway/... ./tests/contract/...` | Passed. |
| `go test ./...` | Passed across the repository. |
| `go vet ./...` | Passed across the repository. |
| `npm run lint` (`frontend/`) | Passed. |
| `npm run build` (`frontend/`) | Passed. Vite reported a JS chunk above 500 kB (651.22 kB), a non-fatal bundle warning. |
| `npm run dev -- --host 127.0.0.1` (`frontend/`) | Started Vite for Cypress. Initial sandbox attempt could not bind port 5173; rerun with access succeeded. |
| `npm test` (`frontend/`) | Initial attempts failed at Cypress startup with MODULE_NOT_FOUND because inherited `ELECTRON_RUN_AS_NODE=1` forced Electron into Node mode. |
| `env -u ELECTRON_RUN_AS_NODE npm test` (`frontend/`) | Full run: 198/200 passed; two new reload checks failed because they assumed JWT persistence. All four unrelated specs passed (158 tests). |
| `env -u ELECTRON_RUN_AS_NODE npm test -- --spec ../tests/e2e/application-workflows.cy.js` (`frontend/`) | Passed 42/42, including all eight JT-108 scenarios, after correcting the reload checks to log in again. Combined with the four unchanged specs above, all 200 tests have passing evidence; the entire suite was not rerun after that test-only correction. |
| `git diff --check` | Passed. |

## Limits and review

No new dependencies, migrations, services or event contracts. Full running-stack Cypress (`npm run test:stack`), deployment and remote CI were not run. Frontend Cypress uses the repository's stubbed gateway pattern; the new HTTP binding is separately exercised in Go, and persistence/ownership against real PostgreSQL. Offer lifecycle eligibility remains outside JT-108 as recorded in AGENTS.md. Existing authentication stores JWTs only in memory: browser reload requires login again before reopening an offer URL. The initial new Cypress reload checks assumed persistent authentication; they were corrected to verify the existing login requirement and then reopen the saved URL.

Review and rerun the checks locally, then manually commit with JT-108, push, open a PR and move the Jira card to In Review. No branch switch, commit, push, PR or Jira mutation was performed.

## Changed files

- `AGENTS.md`
- `proto/jobtrack/offer/v1/offer.proto`
- `gen/go/jobtrack/offer/v1/offer.pb.go`
- `gen/go/jobtrack/offer/v1/offer_grpc.pb.go`
- `gen/go/jobtrack/offer/v1/offer.pb.gw.go`
- `services/offer/internal/ports/offers.go`
- `services/offer/internal/ports/mocks/offers_mock.go`
- `services/offer/internal/app/offers.go`
- `services/offer/internal/app/offers_test.go`
- `services/offer/internal/adapters/grpc/offer.go`
- `services/offer/internal/adapters/grpc/offer_test.go`
- `services/offer/internal/adapters/postgres/offers.go`
- `services/offer/internal/adapters/postgres/offers_integration_test.go`
- `frontend/src/features/offer/api.ts`
- `frontend/src/features/offer/OffersPage.tsx`
- `frontend/src/features/application/ApplicationsPage.tsx`
- `tests/contract/offer_contract_test.go`
- `tests/e2e/application-workflows.cy.js`
- `docs/testing/jt-108-verification.md` (this report)
