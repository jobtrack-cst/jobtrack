# JobTrack frontend redesign (JT-110 to JT-113)

Status: approved by Jigden with authority delegated by the team, 22 Nov 2026.
Scope: a new frontend UI built from scratch on the existing stack. No `.proto`,
service or gateway changes. Contract work lives in JT-114.

AGENTS.md stays the source of truth. This file describes the new UI only.

---

## 1. What is replaced and what is kept

Replaced (rewrite from scratch):

- `frontend/src/theme.ts`, `App.tsx`, every page and component under
  `frontend/src/components/` and `frontend/src/features/*/` that renders UI.
- `frontend/public/branding/` (old mark, valley and weave artwork).
- The stubbed Cypress specs in `tests/e2e/*.cy.js` (rewritten per phase).

Kept and reused (they encode agreed contracts and rules, not looks):

- `frontend/src/api/client.ts` (gateway client, JWT, 401 handling).
- `frontend/src/api/auth.ts` and the session rules of JT-106
  (`sessionStorage`, `/v1/auth/me` on reload, logout on 401).
- Every `frontend/src/features/*/api.ts` request function.
- Validation helpers with the agreed limits (JT-32, JT-33, JT-34, JT-48,
  JT-50, JT-51, JT-52 and the others listed in AGENTS.md Section 15).
- The rule that gateway errors show a fixed message per gRPC code (and HTTP
  413), never the server's text (JT-83, JT-88).

Stack is unchanged (AGENTS.md Section 7): React, TypeScript, Vite, MUI,
React Router. No new npm packages. Fonts and images are static files in
`frontend/public/`.

---

## 2. Visual system: Night in the Daylight glass layout

One dark theme for every role. Every page follows the approved "Daylight glass"
layout, in Night colours: glass surfaces float over the full-page background,
content sits on solid `surface` cards and panels. Calm, high contrast, modern.

### Colour tokens

| Token | Value | Use |
|---|---|---|
| `canvas` | `#151B2C` | Page background under the glow image |
| `glow` | `public/branding/background-night.svg` | Fixed full-page background image (indigo and teal glow), drawn with radial gradients, not an SVG blur filter, so it stays cheap to repaint |
| `glass` | `rgba(20,26,45,0.55)` + `backdrop-filter: blur(18px)` | Top bar, dialogs, the mobile drawer and the auth card only. Blur is costly to paint without a GPU (Cypress in WSL), so search bars and other glass panels use `rgba(20,26,45,0.72)` without blur, which looks the same over the smooth background |
| `glassBorder` | `rgba(255,255,255,0.12)` | Border of glass surfaces |
| `surface` | `#1E2640` | Cards, panels, list rows, inputs |
| `surfaceRaised` | `#26304F` | Hovered rows, menus |
| `border` | `#3A4566` | Card and input borders |
| `text` | `#FFFFFF` | Main text |
| `textSecondary` | `#B8C2DC` | Supporting text |
| `textMuted` | `#8E99B8` | Hints and metadata only (passes 4.5:1 on `surface`) |
| `accent` | `#8EA2FF` | Primary buttons, links, active nav, focus ring |
| `accentHover` | `#A9B8FF` | Hover of accent |
| `onAccent` | `#0E1A3B` | Text on accent (about 7.9:1) |
| `success` | `#5FD3A5` | Offered, accepted, validated, verified |
| `danger` | `#FF8A80` | Errors, rejected, withdraw and destructive actions |
| `urgentBg` / `urgentText` | `#FFE2D2` / `#8A2A0B` | Deadline tags within 7 days |
| `chipBg` / `chipText` | `#2B3560` / `#D6DDFF` | Tags, filters, neutral statuses |

Check contrast for every new pair: text 4.5:1, large text and UI parts 3:1.

### Type

- Plus Jakarta Sans, self-hosted latin `woff2` files in
  `frontend/public/fonts/` with the OFL licence file, loaded with
  `@font-face` in the theme (`font-display: swap`). No requests to Google at
  runtime.
- Top-level page title (Find jobs, Applications, Jobs, Applicants, admin
  pages, home): 64px, weight 800, letter spacing -0.05em, line height 1,
  `text`. Detail page title (a job, an application, the job editor): 40px,
  weight 800, with a back link above it. Both scale down on phones (40px and
  32px at 320px wide) so no word scrolls sideways.
- Results header: 24px, weight 800. Section title 22px 800. Card title 19px
  700. Body 16px 400; secondary 15px or 14px; field labels 14px 600 (13px 600
  in the search bar); smallest 13px. Nothing below 12.5px.
- Sentence case everywhere. No all-caps labels, no letter-spaced eyebrows.

### Shape, depth and motion

- Radius: 20px for cards and panels (dialogs too), 12px to 14px for controls
  and buttons, fully round for small pills (status and deadline). Tag chips
  use 8px. The top bar uses 18px, the search bar 20px and the auth card 24px.
- Avatars are circles everywhere (company logos, applicant photos, initials).
- Company and location are joined with a comma, never a middle dot:
  "Tobgay's Company, Trashigang".
- No shadows on cards (border only). Shadows only on menus and dialogs.
- Focus: 3px solid `accent` outline with 2px offset on every interactive
  element. Touch targets at least 44px.
- Motion only when it answers an action (dialog open, tab change, row
  expand), 150 to 200 ms, and none under `prefers-reduced-motion`.

### Logo

A rounded square in `accent` with a `#0E1A3B` "J" and a small white dot, in
`public/branding/jobtrack-mark.svg`, also the favicon. Wordmark "JobTrack" in
Plus Jakarta Sans 800: 32px mark and 20px wordmark in the top bar, 38px and
22px on the signed-out pages.

---

## 3. Layout shell

- Full-page Night background image behind everything.
- Top bar: a floating glass bar, not full width. Max width 1180px, centred,
  20px from the top (sticky), radius 18px, padding 10px 14px 10px 18px,
  `glass` background with blur and `glassBorder`.
- In it: logo and wordmark, then the role's main navigation as pills (15px,
  weight 600, padding 10px 16px, radius 12px; 10px 12px between 900px and
  1200px so seven links fit). The active link is a filled pill (`#FFFFFF`
  background, `#0E1A3B` text); the others are `textSecondary`. On the right
  the account button: a 44px circle in `accent` with `onAccent` initials (an
  applicant's own photo when they have one, JT-103). Its menu shows the name
  and role and has Log out.
- Under 900px the navigation moves into a drawer opened by a Menu button
  (keep the JT-101 drawer behaviour: focus trap, Escape, focus return, 44px
  targets). The desktop links and the drawer are never rendered together.
- Signed-out pages (login, register) have no bar: only the logo, top left,
  aligned with their wider container (Section 5, Auth).
- Content: max width 1180px, centred, 32px side padding (16px on phones),
  32px gap between sections.
- Every page starts with a page header: title (Section 2 sizes), one plain
  sentence, and at most one primary action on the right.
- Skip link to `#main-content` stays.

### Navigation per role (fixes the menu flaws)

| Role | Items |
|---|---|
| Applicant | Home, Find jobs, Saved, Applications, Messages, Profile, Job alerts |
| Employer | Home, Jobs, Applicants, Messages, Company |
| Admin | Home, Verification, Moderation, Analytics, Audit log |

Messages is added to the applicant and employer menus (after Applications and
Applicants) in Phase 4 with the inbox; until then each application keeps its
own message link and there is no Messages item or placeholder page.

Removed from all menus: Apply, Offers, Report a job, Resumes (moved into
Profile), Verification for employers (moved into Company), Notifications
(see Section 7).

---

## 4. Shared components and page patterns

Build these once in `frontend/src/components/` and use them everywhere:

- `PageHeader` (top-level and detail variants), `GlassPanel`, `Card`,
  `EmptyState` (headline, one line, one action), `LoadingRows` (skeleton
  rows), `ErrorState` (fixed message plus Try again), `ConfirmDialog`.
- `StatusRail`: the five-step progress bar Submitted, Under review,
  Shortlisted, Interview, Offer, with the current step highlighted, and a
  final-state chip for Accepted, Declined, Rejected or Withdrawn. This is the
  signature element of the new design: use it on application cards and the
  application page.
- `StatusChip` (application, job, resume, verification, offer statuses with
  plain labels, never enum names).
- `DeadlineTag`: "Closes in 2 days" (`urgentBg` with `urgentText` within 7
  days), "Closes 9 Dec" otherwise (`chipBg` with `chipText`), "Closed" when
  past.
- `CompanyAvatar`: company logo from `GetCompanyLogo`, or initials on
  `chipBg` when there is none.
- `JobTitle` hook: `useJob(jobId)` with an in-memory cache shared across the
  app, so lists can show job titles from `GetJob` without repeated requests.
- `Pager`: "Showing 1 to 20 of 54" with Previous and Next, using
  `totalCount` and the Section 9 page tokens.
- `TextInput`: the field style of every form (below).
- `SearchBar` and `SearchField`, `ResultsHeader`, `CardGrid`, `JobCard`,
  `TagChip`, `ListPanel` and `ListRow`, `FormPanel` for the patterns below.

### Search bar (Find jobs, and every other search or filter bar)

- Glass bar, radius 20px, padding 8px. Each field shows a small label (13px,
  weight 600, `textSecondary`) above its value (17px, weight 600, `text`).
  One Search button, 60px high, radius 14px, `accent` with `onAccent` text.
  On phones the fields stack and the button takes the full width.
- Results header under it: for example "4 open roles in Software" (24px,
  weight 800) with "Soonest deadline first" (14px, `textSecondary`) on the
  right.

### Browse pages: card grid

Find jobs, saved jobs, my jobs, applications, applicants and the home page
sections. Find jobs uses the full job card below; Saved jobs, My applications
and Applicants use the small minimal card (Section 5).

- Grid `repeat(auto-fill, minmax(280px, 1fr))` (the full width on phones
  narrower than that), gap 16px. `auto-fill` keeps a single card at its
  normal size instead of stretching it across the row.
- Card: `surface`, `border`, radius 20px, padding 24px. Top row: company
  avatar (48px circle) and the deadline pill. Then the title (19px,
  weight 700), company and location (15px, `textSecondary`), tag chips (13px,
  weight 600, `chipBg` with `chipText`, radius 8px) and the actions: "View
  job" full width (`accent`, `onAccent` text, 44px, radius 12px) and "Save"
  (outline `border`).

### Small minimal cards

Same grid and card style, with less in each card: a circular avatar, the
title, one line of supporting text, one chip (deadline or status) and one
View button. Used for Saved jobs, My applications (Phase 2) and Applicants
(Phase 3).

### Dense lists: rows in one panel

Verification queue, reports, audit log and the employer home's newest
applicants: rows inside one `surface` panel with radius 20px and `border`
dividers, under a results header.

### Forms

- Forms sit in a `surface` panel with radius 20px and padding 32px.
- Fields as on the login page: label above the input (14px, weight 600,
  `text`), input 48px high, radius 12px, `surface` background, `border`
  border, `text` value, `textMuted` placeholder, 3px `accent` focus outline.

Rule for every page: never show a UUID to the user. Users never type an ID.

Exceptions, admin pages only:

- `/admin/audit-log` shows each entry's event ID and data unchanged (JT-35), in
  an expandable monospace block that is closed by default.
- `/admin/moderation` keeps Remove job by ID (Section 5).

---

## 5. Pages and the flaws they fix

### Auth (both roles)

Same layout as the approved "Daylight glass" login, in Night colours. No CST
or college text anywhere.

- Full-page Night background image behind everything.
- One container for the whole page: max width 1440px, centred, side padding
  `clamp(24px, 5vw, 96px)`. Top left, aligned with that container: the 38px
  logo mark and the "JobTrack" wordmark (22px, weight 800); no top bar.
- Below, a row with two columns, both vertically centred in the space under
  the logo, that wrap on small screens. Column gap `clamp(32px, 5vw, 64px)`,
  so the headline column keeps room for its two lines at the 1440px cap:
  - Left: the headline "Jobs and internships, without the guesswork."
    (`clamp(44px, 4.6vw, 68px)`, capped at 11vw on phones under 400px so no
    word scrolls sideways; weight 800, letter spacing -0.045em, line height
    1.04, `#FFFFFF`, balanced line breaks). Under it: "Search open roles,
    apply with a checked resume and follow every application from submitted
    to offer." (18px, line height 1.6, `textSecondary`, max width 44ch).
  - Right: a glass form card, 440px wide (full width on phones), radius
    24px, padding 40px (24px on phones), `glass` background with blur,
    `glassBorder`.
- Checked at 320, 390, 768, 1280, 1440 and 1920px wide.
- On phones the headline comes first and the card goes full width below it.
- `/login` card: "Log in" (32px, weight 800, the page's `h1`), "Welcome
  back." (15px, `textSecondary`), Email and Password, a full-width "Log in"
  button (52px high, radius 14px, `accent` with `onAccent` text), then "New
  here? Create an account" with the link in `accent`.
- `/register` card: "Create account" and "Create your JobTrack account.",
  then Full name, Email, Password, Confirm password and Account type
  (Applicant or Employer), a full-width "Create account" button and "Already
  have an account? Log in". The form is not sent when the passwords differ
  ("Passwords do not match."); the confirm value is never sent to the API.
  Keep the existing password length rule.
- Fields use the Section 4 form field style.

### Applicant

| Route | Content | Fixes |
|---|---|---|
| `/applicant` | Home: active applications with `StatusRail`, saved jobs closing soon, profile completeness hint (no profile, no validated resume) | A useful start page instead of marketing copy |
| `/applicant/jobs` | Find jobs: search bar on top (title, category, location, Search), tag filters as one row of chips, results list with `CompanyAvatar`, title, company, location, tags, `DeadlineTag`, Save | Awkward search layout |
| `/applicant/jobs/:jobId` | Job details: title, company, location, deadline, description, tags. Actions: Apply (dialog to choose a validated resume, with a link to upload one if none), Save or Unsave, Report this job (dialog with reason) | Apply and Report needed a typed job ID |
| `/applicant/saved` | Saved jobs as small minimal cards in the card grid: company logo circle, job title, "Company, Location", `DeadlineTag`, one View button that opens job details | Saved cards showed only an ID |
| `/applicant/applications` | My applications: tabs Active and Past, small minimal cards in the card grid, one card per job (the newest application); withdrawn and final ones under Past. Card: company logo circle, job title, "Company, Location", `StatusChip`, one View button | Job IDs on cards, duplicate withdrawn and new cards |
| `/applicant/applications/:applicationId` | One application page (detail header with a back link): job details, `StatusRail` and full status history, interviews, the offer (terms, Accept and Decline with confirmation) loaded with `GetOfferByApplication`, Withdraw (only in non-final states), and the conversation with the employer embedded at the bottom (thread and composer, Section 6 rules) | Offer ID lookups, five separate pages per application |
| `/applicant/messages` and `/applicant/messages/:applicationId` | Messages inbox (Section 6, Phase 4) | Unorganised chats |
| `/applicant/profile` | Tabs Profile, Resumes. Profile read-only with Edit (JT-109 behaviour). Photo upload shown only after the profile is saved, with the hint "Save your profile to add a photo". Resumes: upload PDF, status chip, View PDF (opens `DownloadResume` as a blob) | Could not view resumes, confusing photo error |
| `/applicant/job-alerts` | Job alert subscriptions (existing JT-76 rules) | Restyle only |

### Employer

| Route | Content | Fixes |
|---|---|---|
| `/employer` | Home: verification state with next step, open jobs with applicant counts, newest applicants | Useful start page |
| `/employer/jobs` | Tabs Active, Drafts, Closed (from the `ListMyJobs` status filter). Closed and removed jobs open read-only, with no Manage or Edit button | Closed jobs with a Manage button that failed |
| `/employer/jobs/new`, `/employer/jobs/:jobId` | Job editor for drafts and published jobs; Publish and Close with confirmation; read-only view for closed and removed jobs | Same |
| `/employer/applicants` | Job filter (dropdown of the employer's jobs, using the `ListApplications` `job_id` filter) and small minimal cards in the card grid: applicant photo circle, name, job title, applied date, `StatusChip`, one View button. Until JT-114: initials, "Applicant" and the applied date | Job IDs, no filter by job, applicant IDs |
| `/employer/applicants/:applicationId` | One application page (detail header with a back link): the applicant's profile, photo and resume (JT-114), job summary, `StatusRail` and history, Review (only if not reviewed), Schedule interview (only when Shortlisted or Interview), Make offer (only when Interview and no offer exists), offer status, and the conversation with the applicant embedded at the bottom. Until JT-114 the page shows initials and "Applicant", and no profile or resume | Actions shown at every stage, offer ID lookups |
| `/employer/messages` | Messages inbox (Section 6, Phase 4) with a job filter | Unorganised chats |
| `/employer/company` | Company profile read-only with Edit, Save and Cancel; logo upload; verification status, rejection reason and the request form on the same page | Accidental edits, separate verification page |

Employers have no Report a job page: they only see and manage their own jobs.

### Admin

| Route | Content |
|---|---|
| `/admin` | Home: pending verification count, open reports, links |
| `/admin/verification` | Verification queue (existing JT-33 and JT-88 rules) |
| `/admin/moderation` | Reports with Dismiss and Remove job; Remove job by ID stays here for admins only |
| `/admin/analytics` | Counts for a date range (JT-36 rules) |
| `/admin/audit-log` | Audit entries with filters (JT-35 rules) |

---

## 6. Messages inbox (both roles, Phase 4)

A chat-style inbox, like WhatsApp.

- Left column: a list of small conversation rows, one per application (from
  `ListApplications`), newest activity first. Each row: a circular avatar
  (the company logo for applicants; the applicant's photo for employers
  after JT-114; otherwise initials), the other side's name (the company for
  applicants; the applicant's name after JT-114, "Applicant" until then),
  the job title, the last message preview and its time (from
  `ListMessages`). Employers can filter by job.
- Right column: the thread as message bubbles (own messages on the right in
  `accent` with `onAccent` text, the other side's on the left on `surface`)
  with small circular avatars, oldest at the top and newest visible without
  paging; composer at the bottom (1 to 2000 characters, existing rule).
- On phones the list and the thread are separate screens.
- Messages show "You" and the other side's role until JT-114 adds names.
- The same thread and composer are embedded at the bottom of each
  application page (Phases 2 and 3).

---

## 7. Known limits shown honestly

- Notifications: notification-dispatcher is not wired (AGENTS.md Section 16),
  so the inbox stays empty. The Notifications item is removed from menus
  until delivery works.
- Applicant names, photos, profiles and resumes for employers need JT-114.
  Until then the UI never invents them: it shows initials, "Applicant" and
  the applied date.
- Offer eligibility is not enforced by the offer service (Section 16), so the
  UI only shows Make offer at the Interview stage.

---

## 8. Lifecycle rules for actions

| Status | Applicant can | Employer can |
|---|---|---|
| Submitted | Withdraw, message | Review, message |
| Under review | Withdraw, message | Review (if not reviewed), message |
| Shortlisted | Withdraw, message | Schedule interview, message |
| Interview | Withdraw, message | Schedule another interview, Make offer (if none), message |
| Offered | Accept, Decline, Withdraw, message | Message |
| Accepted, Declined, Rejected, Withdrawn | View only | View only |

Status comes from `GetApplicationStatus`; refresh it after every action,
because events update it with a short delay.

---

## 9. Testing

- Each phase rewrites the stubbed Cypress specs for the pages it replaces,
  keeping the existing coverage of rules: validation boundaries, fixed error
  messages, role-limited routes, pagination, session expiry, drawer and focus
  behaviour.
- `tests/e2e/stack/main-workflow.cy.js` is updated to the new pages,
  including Confirm password.
- `npm run lint`, `npm run build` and `npm test` pass before each PR; CI is
  the merge gate (AGENTS.md Section 11).
- Run stubbed specs with the Compose gateway stopped, so unstubbed requests
  cannot sign the test user out with a real 401.

---

## 10. Phases (one PR each)

| Phase | Jira | Delivers |
|---|---|---|
| 1 | JT-110 | Night theme, fonts, background, logo, layout shell and navigation, shared components, auth pages with Confirm password, home pages, admin pages, AGENTS.md Section 15 row |
| 2 | JT-112 | All applicant pages, including the one application page with the embedded conversation |
| 3 | JT-111 | All employer pages, including the applicant cards and the application page |
| 4 | JT-113 | Chat-style messages inbox for both roles, and the Messages menu item |

Order: Phase 1, Phase 2, then JT-114 (the contract work for applicant names,
photos, profiles and resumes for employers), then Phase 3, then Phase 4.
Phases 3 and 4 show names and photos, so they follow JT-114; Phase 2 needs
nothing from it.

Between phases the app must still build and run. Pages not rebuilt yet keep
their routes and only get the changes needed to compile with the new theme
and shared components; their full rebuild comes in their own phase.
