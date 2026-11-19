# JT-109 applicant profile read-only view with an Edit button: test cases

Traceability: [JT-109](https://jobtrack-cst.atlassian.net/browse/JT-109) (Task, Low, backlog). Developer and applicant-profile owner: Kinley Tobgay Lhendrup. Verification date: 19 November 2026.

## Scope

UI enhancement suggested during UAT (JT-99, scenario UAT-AP-06). Not a bug and not a release blocker. Approved by Phuntsho as Product Owner on 19 Nov 2026 for the backlog as a low-priority improvement after the project, kept out of Sprint 4. The pull request is not merged during Sprint 4.

Change: `frontend/src/features/applicant-profile/ProfilePage.tsx` only. No change to `applicant-profile` RPCs, the JT-50 validation rules or the JT-103 photo rules. Existing MUI components only (AGENTS.md Section 9). No new library.

Acceptance points from the card:

- S1: after a profile exists, show it read-only (summary, education, skills)
- S2: an Edit button switches to the form; Save and Cancel return to the read-only view
- S3 (optional): disable Save while nothing has changed
- S4: frontend only; no change to the RPCs or the JT-50 rules; existing MUI components only

Rules:

- A saved profile opens read-only. Education is shown as "institution, qualification, start year to end year", with "ongoing" when the end year is 0. Skills are chips. Empty sections show "No summary.", "No education entries." and "No skills."
- A caller without a profile still gets the create form directly; after "Create profile" the page shows the read-only view with "Profile created.".
- Edit profile opens the form with the saved values. Save profile sends `PUT /v1/applicant/profile` and returns to the read-only view with "Profile updated.". Cancel restores the saved values and sends nothing.
- Save is disabled, with "No changes to save.", while the trimmed input equals the saved profile, so whitespace-only edits do not count as changes. The JT-50 checks still disable Save for invalid input.
- A rejected save keeps the form open with the fixed JT-83 message and never the server's text.
- The JT-103 photo section is shown in both modes.

## Test matrix

All cases are Cypress GUI tests in `tests/e2e/applicant-hiring-pages.cy.js` with the gateway stubbed by `cy.intercept`.

| Case ID | Requirement | Test case | Technique | Expected result | Result |
|---|---|---|---|---|---|
| JT109-01 | S1 | Saved profile with summary, one education entry and two skills | Equivalence partitioning | Summary text, "Synthetic College, BSc, 2020 to 2024" and two skill chips shown; no text inputs; no Save button; "Edit profile" shown | Pass |
| JT109-02 | S1 | Saved profile with no summary, education or skills | Boundary value analysis (empty lists) | "No summary.", "No education entries." and "No skills." | Pass |
| JT109-03 | S1, S2 | No profile, create one | State transition (none to saved) | Create form shown first; after "Create profile" the read-only view with "Profile created." and "Edit profile" | Pass |
| JT109-04 | S2 | Edit, clear the end year, Save | State transition (view to edit to view) | `PUT` with end year 0; read-only view shows "2020 to ongoing" and "Profile updated." | Pass |
| JT109-05 | S2 | Edit, change the summary, remove the education entry, Cancel | State transition (edit to view without saving) | Saved values shown again; Edit shows the saved summary; no `PUT` sent | Pass |
| JT109-06 | S3 | Edit without changes, type one character, delete it | Boundary value analysis (no change, one change, back to no change) | Save disabled with "No changes to save."; enabled after the change; disabled again after it is undone | Pass |
| JT109-07 | S2, S4 | Edit, change the summary, the gateway answers 400 | Equivalence partitioning | Fixed "The profile was not saved" message without the server text; Save stays enabled | Pass |
| JT109-08 | S4 | JT-50 limits in edit mode: start year 1949 and next year plus one, end year before start and more than 10 years ahead, empty and 151 character institution, duplicate skill | Boundary value analysis | Each blocks Save with its JT-50 message, as before JT-109 | Pass (7 tests) |
| JT109-09 | S4 | JT-50 boundaries in edit mode: 150 character institution, start year next year, end year 10 years ahead | Boundary value analysis | Save enabled | Pass |
| JT109-10 | S2 | Edit, remove the only education entry, Save | State transition | "No education entries." in the form; `PUT` with an empty education list | Pass |
| JT109-11 | S1 | Profile load fails, Try again | State transition (error to view) | Load error shown; after the retry the read-only view shows the saved summary | Pass |
| JT109-12 | S4 | JT-103 photo spec on the changed page | Not applicable (regression) | `profile-photo.cy.js` passes unchanged | Pass, 16 of 16 |
| JT109-13 | Regression | Whole stubbed Cypress suite | Not applicable (regression) | All specs pass | Pass, 212 of 212 in 6 specs |

JT109-04, JT109-07 to JT109-11 are the existing JT-83 profile tests, updated to open the form with Edit profile first or to check the read-only view after saving. JT109-01, JT109-02, JT109-05 and JT109-06 are new.

## Verification

Run on 19 November 2026 on Kinley's MacBook Air (Apple M1, 8 GiB), on branch `JT-109-profile-read-only-view` with `main` merged in (JT-103 #93 and JT-108 #95), from `frontend` with `npm run dev` running:

    npm run lint && npm run typecheck && npm run build
    npx cypress run --config-file cypress.config.cjs --spec ../tests/e2e/applicant-hiring-pages.cy.js,../tests/e2e/profile-photo.cy.js
    npm test

| Check | Result |
|---|---|
| Lint, typecheck, build | Pass, before and after merging `main` |
| `applicant-hiring-pages.cy.js` | 55 of 55 (51 before JT-109, 4 new) |
| `profile-photo.cy.js` | 16 of 16 |
| Whole stubbed suite | 212 of 212 in 6 specs, before merging JT-108 from `main`; CI runs it again on the pull request |

Not run: the stack end-to-end spec (`tests/e2e/stack/main-workflow.cy.js`) does not open the applicant profile page, so JT-109 does not affect it.
