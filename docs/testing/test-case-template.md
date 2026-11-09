# Test case template

Copy this file to `docs/testing/jt-<number>-test-cases.md` for each Jira task, replace every `<...>` placeholder, and delete these instructions. See [test-plan.md](test-plan.md) for test levels, entry and exit criteria and the defect process.

---

# JT-<number> <service or feature> test cases

Traceability: [JT-<number>](https://jobtrack-cst.atlassian.net/browse/JT-<number>). Owner: <name>. Verification date: <day month year>.

## Scope

<What this task covers, which RPCs, events or pages are tested, and what is out of scope. Note any decisions or input limits from AGENTS.md Section 15.>

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT<number>-01 | AC1: <acceptance criterion, or story key> | <condition being tested> | Equivalence partitioning | <exact output or gRPC status code> | Unit, `internal/domain` | Pass |
| JT<number>-02 | AC1 | <boundary case> | Boundary values | <...> | Integration, `internal/adapters/postgres` | Pass |
| JT<number>-03 | AC2 | <transition case> | State transition | <...> | Unit, `internal/app` | Fail, JT-<bug key> |

Column rules:

- Requirement: the acceptance criterion number from the Jira task (AC1, AC2, ...) or the story key. Every acceptance criterion must appear at least once.
- Technique: equivalence partitioning, boundary values, state transition, or authorization (role and ownership).
- Level and location: unit, integration, contract, end to end or load, and the package or folder holding the automated test.
- Result: Pass, Fail (with the Jira Bug key), or Not run (with the reason).

## Verification

Commands run, with the commit they ran on:

    go test ./services/<service>/...
    go test -tags=integration ./services/<service>/...
    go test -cover ./services/<service>/...

| Check | Result |
|---|---|
| Unit tests | <pass or fail> |
| Integration tests | <pass or fail> |
| Coverage | <percent> |
| CI on the pull request | <green or red, with run link> |

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| AC1: <text> | Met | JT<number>-01, JT<number>-02 |
| AC2: <text> | Not met | JT<number>-03, JT-<bug key> |
