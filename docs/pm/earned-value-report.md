# JobTrack: Earned Value Report

| Item | Detail |
|---|---|
| Jira task | JT-100 |
| Prepared by | Jigden Shakya (Scrum Master and project manager). Backup: Kinley Tobgay Lhendrup |
| Version | 1.0 |
| Date | 20 Nov 2026 |
| Data source | Jira space JT (jobtrack-cst.atlassian.net): story points, sprint membership, status and logged time of all 94 issues, read on 20 Nov 2026 at 19:10 |
| Related documents | `docs/pm/project-closure-report.md`, `docs/pm/sprint-4-review.md`, `docs/pm/project-charter.md` (Section 8), `docs/pm/definition-of-done.md` (time logging) |

---

## 1. Purpose

The charter (Section 2, O4) asks for earned value calculated from Jira time logs. This report gives planned value (PV), earned value (EV), actual cost (AC), schedule and cost variance (SV, CV) and the schedule and cost performance indexes (SPI, CPI) at the end of every sprint from Sprint 0 to Sprint 4, with a short interpretation.

---

## 2. Method

The project uses agile earned value: value is measured in story points and cost in hours logged in Jira. There is no money budget (charter Section 8).

| Term | How it is measured |
|---|---|
| Scope (BAC) | Every estimated Jira issue in Sprints 0 to 4: 85 issues, 244 story points |
| PV | Cumulative story points of the work planned by the end of each sprint. Each issue counts once, in the sprint where it was first planned; carried-over work is not counted again |
| EV | Cumulative story points of the issues Done at the end of each sprint. JT-100 (3 points) was In Review at the snapshot, so it is not counted as Done; the figures it adds when its pull request merges are shown separately as projected |
| AC | Cumulative hours logged in Jira. Each issue's hours count in the last sprint it belonged to. Every carried-over issue has a single worklog dated in the sprint where it was finished (checked for all 21 carried issues), so this matches when the hours were logged. JT-53 and JT-65 (outside any sprint) count in Sprint 2 and JT-109 in Sprint 4, by their worklog dates |
| Unestimated work | Bugs without points and JT-109 add hours to AC but no points to EV, because they were not in the planned scope |

Cost baseline. The team never set planned hours: Jira has no original estimates, and the working agreement and sprint plans give only story points. The cost baseline is therefore a retrospective estimate, chosen on 20 Nov 2026 for this report and accepted by the Product Owner in the PR #101 review: the rate measured in Sprint 0, the first sprint, applied to all later work. It was not planned in advance, so CV and CPI show performance against the team's own first-sprint pace, not against an agreed budget.

- Planned rate = Sprint 0 hours / Sprint 0 points Done = 26.75 h / 35 points = 0.7642857 hours per point (shown as 0.7643)
- PV and EV in hours = story points x planned rate
- Budget at completion (BAC) = 244 points x planned rate = 186.5 hours

Rounding: every calculation uses the unrounded rate and the hours exactly as logged (in seconds in Jira). Only the displayed results are rounded: hours to one decimal place and indexes to two.

Formulas: SV = EV - PV, CV = EV - AC, SPI = EV / PV, CPI = EV / AC. Because the baseline comes from Sprint 0, Sprint 0 has a CPI of 1.00 by definition.

---

## 3. Per sprint (from Jira)

| Sprint | Dates | Committed points | First planned in this sprint | Points Done | Hours logged | Hours per point Done | Hours on unestimated work |
|---|---|---|---|---|---|---|---|
| Sprint 0 | 2 to 3 Nov | 46 | 46 | 35 | 26.8 | 0.76 | 0.0 |
| Sprint 1 | 6 to 8 Nov | 80 | 69 | 49 | 25.0 | 0.51 | 0.0 |
| Sprint 2 | 9 to 13 Nov | 60 | 29 | 60 | 51.5 | 0.86 | 2.7 |
| Sprint 3 | 14 to 16 Nov | 62 | 62 | 43 | 33.2 | 0.77 | 0.0 |
| Sprint 4 | 17 to 20 Nov | 57 | 38 | 54 | 79.0 | 1.46 | 12.6 |
| Total | 2 to 20 Nov | | 244 | 241 | 215.4 | 0.89 | 15.3 |

"Committed" counts every issue in the sprint, including carried-over work; "first planned" counts only new work and is the basis of PV.

---

## 4. Earned value at the end of each sprint (cumulative)

Story points:

| End of | PV (points) | EV (points) | SV (points) | SPI |
|---|---|---|---|---|
| Sprint 0 | 46 | 35 | -11 | 0.76 |
| Sprint 1 | 115 | 84 | -31 | 0.73 |
| Sprint 2 | 144 | 144 | 0 | 1.00 |
| Sprint 3 | 206 | 187 | -19 | 0.91 |
| Sprint 4 | 244 | 241 | -3 | 0.99 |

Hours (points x the unrounded planned rate):

| End of | PV (h) | EV (h) | AC (h) | SV (h) | CV (h) | SPI | CPI |
|---|---|---|---|---|---|---|---|
| Sprint 0 | 35.2 | 26.8 | 26.8 | -8.4 | 0 | 0.76 | 1.00 |
| Sprint 1 | 87.9 | 64.2 | 51.7 | -23.7 | +12.5 | 0.73 | 1.24 |
| Sprint 2 | 110.1 | 110.1 | 103.3 | 0 | +6.8 | 1.00 | 1.07 |
| Sprint 3 | 157.4 | 142.9 | 136.5 | -14.5 | +6.5 | 0.91 | 1.05 |
| Sprint 4 | 186.5 | 184.2 | 215.4 | -2.3 | -31.2 | 0.99 | 0.86 |

At completion. "Actual" is the state in Jira at the snapshot, with JT-100 In Review. "Projected" adds JT-100's 3 points as Done when its pull request merges, with no further hours:

| Measure | Actual (snapshot) | Projected (JT-100 merged) |
|---|---|---|
| BAC | 244 points, 186.5 h | 244 points, 186.5 h |
| EV | 241 points, 184.2 h | 244 points, 186.5 h |
| AC | 215.4 h | 215.4 h |
| SV | -3 points (-2.3 h) | 0 points (0 h) |
| CV | -31.2 h | -28.9 h |
| SPI | 0.99 | 1.00 |
| CPI | 0.86 | 0.87 |
| Variance at completion (BAC - AC) | -28.9 h | -28.9 h |
| Finish date | 20 Nov 2026: 3 days after the 5 Nov baseline (17 Nov) and 1 day after the revised plan of 11 Nov (19 Nov) | Same, if JT-100 merges before 20 Nov 23:59 |

---

## 5. Charts

Generated from the Jira figures in Sections 3 and 4:

![Velocity by sprint](charts/velocity.png)

![Cumulative PV, EV and AC](charts/earned-value.png)

Burndown charts, generated from Jira for every sprint: story points of the issues in the sprint and the date each issue was resolved. The right edge is the time the sprint was closed in Jira (Sprint 4 was still active on 20 Nov).

![Sprint 0 burndown](charts/sprint-0-burndown.png)

![Sprint 1 burndown](charts/sprint-1-burndown.png)

![Sprint 2 burndown](charts/sprint-2-burndown.png)

![Sprint 3 burndown](charts/sprint-3-burndown.png)

![Sprint 4 burndown](charts/sprint-4-burndown.png)

What the burndowns show: in Sprint 0 most tasks were moved to Done on 4 Nov, after the planned end and before the sprint was closed. Sprints 1 and 3 ended with 31 and 19 points left, which were carried into the next sprint. Sprint 2 still had 39 points left at its original end (11 Nov) and reached 0 only in its two extra days. In Sprint 4 only 6 of 57 points were Done by the evening of 18 Nov, and the line caught up with the guideline only on 20 Nov; the 3 points left at the right edge are JT-100, which is In Review and is Done when its pull request merges.

---

## 6. Interpretation

Schedule. The SPI was below 1 at the end of Sprint 0 (0.76), Sprint 1 (0.73) and Sprint 3 (0.91): each of those sprints finished less than it planned and carried work forward. Sprint 2 brought the SPI back to 1.00, but only with its two-day extension. At the snapshot the SPI is 0.99: every planned task except JT-100 is Done, the last of them JT-72 on the evening of 20 Nov. The SPI becomes 1.00 when JT-100 merges. A point-based SPI does not show lateness on its own: the scope was finished by adding three days to the schedule (two in Sprint 2, one in Sprint 4), so the project ended on 20 Nov instead of the 17 Nov baseline.

Cost. Up to Sprint 3 the CPI was above 1 (1.24, 1.07, 1.05): delivered points took fewer logged hours than the Sprint 0 rate. Sprint 4 used 79.0 h for 54 points (1.46 h per point, the highest of any sprint), of which 12.6 h went to bugs and JT-109, which have no points. Sprint 4 was mostly end-to-end and load testing, UAT and documentation. This brought the CPI down to 0.86 at the snapshot (0.87 when JT-100 merges): the project used 215.4 h against a baseline of 186.5 h, an overrun of 28.9 h (16 percent).

Together: the team delivered, or will deliver when JT-100 merges, all of the planned scope, but needed three extra days and, measured against the Sprint 0 rate, about 16 percent more effort than planned, most of it in the final sprint.

---

## 7. Limitations

- The cost baseline is the Sprint 0 rate, not a plan the team agreed in advance. A different baseline would change CV and CPI, but not PV, EV, SV or SPI in points.
- Hours are as logged in Jira. Most issues have one worklog entered when the task was finished, and nine issues have exactly 8 h in a single entry, so some hours are likely rounded rather than measured.
- AC is counted by sprint, not by day. Hours logged between two sprints count in the sprint the issue belonged to.
- Time logged on JT-100 after 20 Nov 2026 at 19:10 is not included (1.75 h logged at that time).
- The charts are drawn from Jira data rather than exported from Jira's report screens. They show each sprint's final scope from its first day, so work added during a sprint is not shown as a separate scope change.
- Story points are relative estimates by different members, so points per hour are not fully comparable between sprints.
