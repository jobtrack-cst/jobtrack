# End-to-end tests (Cypress)

AGENTS.md Section 10: end to end and GUI tests with Cypress in `tests/e2e`. The Cypress project lives in `frontend` (Cypress 16 from `frontend/package.json`); run every command below from `frontend`.

| Folder | What it tests | Backend | Config | Command | In CI |
|---|---|---|---|---|---|
| `tests/e2e/*.cy.js` | Pages and requests one by one | Stubbed with `cy.intercept` | `cypress.config.cjs` | `npm test` | Yes |
| `tests/e2e/stack/*.cy.js` | The main workflow through the real UI and real services | The running Docker Compose stack, no stubs | `cypress.stack.config.cjs` | `npm run test:stack` | No (CI has no stack) |

Both need the frontend dev server: `npm run dev` in another terminal (http://localhost:5173).

## Main workflow against the stack (`stack/main-workflow.cy.js`, JT-95)

The spec runs the workflow in order, each step logging in through the UI as the right user:

1. Employer registers and creates a company profile.
2. Employer requests verification.
3. Admin approves the request.
4. Employer sees Verified, then creates and publishes a job on the My jobs pages.
5. Applicant registers and uploads a resume, which resume-validator validates.
6. Applicant applies.
7. Employer reviews and shortlists.
8. Employer schedules an interview.
9. Employer makes an offer.
10. Applicant accepts it.
11. The status history shows None, Submitted, Under review, Shortlisted, Interview, Offered and Accepted.

Services react to events, so the spec waits (refreshing the page, up to about 30 seconds per step) until each change is visible before the next step. All accounts and data are synthetic and unique per run (`jt95-*-<run>@jobtrack.test`), so the spec can run again on the same stack. Data from earlier runs stays in the database.

### How to run

1. Start the whole stack with the functions profile, from the repository root:

       docker compose -f deploy/compose/docker-compose.yml --profile functions up -d

2. Give Cypress the admin login from `deploy/compose/.env` without printing it, in the terminal that will run Cypress (from the repository root):

       export CYPRESS_ADMIN_EMAIL="$(grep '^ADMIN_EMAIL=' deploy/compose/.env | cut -d= -f2-)"
       export CYPRESS_ADMIN_PASSWORD="$(grep '^ADMIN_PASSWORD=' deploy/compose/.env | cut -d= -f2-)"

3. In `frontend`: `npm run dev` in one terminal, then `npm run test:stack` in the other. The admin login is read with `cy.env()` (Cypress 16), which keeps it out of the browser and the command log.

Screenshots of the key steps are saved in `frontend/cypress/screenshots/stack/` (not committed); attach them to the Jira issue for the test report. The password is typed with logging off, so it does not appear in the Cypress command log.
