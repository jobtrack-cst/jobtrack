# JT-84 Sprint 3 SonarQube run and JMeter deadline-rush preparation test cases

Traceability: [JT-84](https://jobtrack-cst.atlassian.net/browse/JT-84) (parent JT-15). Owner: Kinley Tobgay Lhendrup (testing lead). Verification date: 19 November 2026.

## Scope

JT-84 covers the Sprint 3 SonarQube run, the deadline-rush JMeter scenario in `tests/load` and a low-user-count trial from a JMeter laptop separate from the Docker host, with the hardware limitations of that setup. The full 200-user run and its results report belong to JT-96.

- SonarQube: see [sonarqube-sprint-3.md](sonarqube-sprint-3.md). One bug was logged: JT-104 (Trivial).
- JMeter: [`tests/load/deadline-rush.jmx`](../../tests/load/deadline-rush.jmx), described in [`tests/load/README.md`](../../tests/load/README.md). Every request goes through the gateway: `POST /v1/auth/register`, `POST /v1/auth/login`, `POST /v1/resumes`, `GET /v1/resumes/{resume_id}` (polled until `RESUME_VALIDATION_STATUS_VALIDATED`), `GET /v1/jobs/{job_id}`, `GET /v1/resumes` and `POST /v1/applications`. Routes, request fields and enum values were checked against the `auth`, `resume`, `application`, `job_posting` and `common` protos on 19 November 2026.
- Each virtual user registers its own synthetic applicant (`jt84-<runId>-<n>@jobtrack.test`), so reruns never hit the `UNIQUE (applicant_id, job_id)` constraint in `application` (`ALREADY_EXISTS`).
- Out of scope: the 200-user run (JT-96) and pass thresholds for response time and error rate, which are still an open question in [test-plan.md](test-plan.md) Section 8. The figures below are trial results, not performance results.

## Test environment

| Item | Value |
|---|---|
| Docker host | Phuntsho's MacBook, Apple M3, 16 GiB RAM; Docker Desktop 8 CPUs and about 7.75 GiB RAM |
| Code on the host | `main` at `f55fecb`, services rebuilt with `docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build --force-recreate auth employer company-verification job-posting resume resume-validator resume-validator-forwarder application application-status gateway` |
| Load generator | Kinley's MacBook Air, Apple M1, 8 GiB RAM; Apache JMeter 5.6.3, OpenJDK 25.0.2; branch `JT-84-sonarqube-jmeter` at `9c718f2` |
| Network | Both laptops on the same personal hotspot; gateway at `http://172.20.10.2:8000`. Reachability checked with `GET /healthz` (HTTP 200) and `GET /v1/jobs/{job_id}` without a token (HTTP 401) |
| Test data | Verified employer and published job `0ff38554-7d19-4941-9036-7854f06ce23c` (deadline 23 November 2026, 21:44 Bhutan time), created with a local setup script through the gateway. Applicants are created by the scenario |
| Also running on the host | All other Compose containers (the 20 services, Prometheus, Grafana, Mailpit) and the k3d cluster `jobtrack-jt29` |
| Resource capture | `docker stats --no-stream` in a loop on the host, one snapshot about every 2 s, saved before and during each run |

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT84-01 | AC1: run SonarQube locally on `main` and save the report | Analyse `main` with the default Sonar way profiles | Not applicable (static analysis) | Report saved in `docs/testing` | Code quality, `docs/testing/sonarqube-sprint-3.md` | Pass |
| JT84-02 | AC1: log bugs in Jira with severity | Every SonarQube Bug-type issue is logged | Not applicable (static analysis) | One Jira Bug per defect, with a severity label | Code quality, Jira | Pass, JT-104 (Trivial) |
| JT84-03 | AC2: draft the 200-user deadline-rush scenario | Every route, request field, response path and enum value in the plan matches the protos | Equivalence partitioning | No mismatch | Load, `tests/load/deadline-rush.jmx` | Pass |
| JT84-04 | AC2 | The 200-user count is a property, with no password or token stored in the plan | Boundary values | `users` from `-Jusers`; password from a properties file outside the repository | Load, `tests/load/deadline-rush.jmx` | Pass |
| JT84-05 | AC3: low-user trial from a separate laptop | 1 user (`-Jusers=1 -JrampUp=1 -JsetupRampUp=1`) | Boundary values (lowest load) | 8 of 8 samples HTTP 200, 0 errors | Load, `tests/load` | Pass |
| JT84-06 | AC3 | 10 users (`-Jusers=10 -JrampUp=10`) | Equivalence partitioning (low concurrent load) | 80 of 80 samples HTTP 200, 0 errors | Load, `tests/load` | Pass |
| JT84-07 | AC3 | The synchronizing timer releases every `13 Submit application` together | State transition | All 10 submits start within a few milliseconds and each creates an application | Load, `tests/load` | Pass (all 10 within 1 ms) |
| JT84-08 | AC3 | Asynchronous resume validation completes before the rush | State transition (PENDING to VALIDATED) | Every resume `RESUME_VALIDATION_STATUS_VALIDATED` within `validationTries` polls | Load, `tests/load` | Pass (1 poll per user) |
| JT84-09 | AC3 | Docker statistics cover each run | Not applicable (measurement) | A snapshot at or before the first sample and at or after the last | Load, host capture files | Pass |
| JT84-10 | AC4: document hardware limitations | Limitations of the trial setup recorded | Not applicable (documentation) | Section below | Documentation, this file | Pass |

## Trial results

### 1 user (19 November 2026, 16:17:16 to 16:17:20)

| Label | Samples | Elapsed (ms) | Errors |
|---|---|---|---|
| 01 Register applicant | 1 | 176 | 0 |
| 02 Log in applicant | 1 | 62 | 0 |
| 03 Upload resume | 1 | 1116 | 0 |
| 04 Poll resume status | 1 | 15 | 0 |
| 05 Check resume validated | 1 | 11 | 0 |
| 11 View job | 1 | 25 | 0 |
| 12 List my resumes | 1 | 15 | 0 |
| 13 Submit application | 1 | 1131 | 0 |

`03` and `13` took about 1.1 s here but 9 to 52 ms in the 10-user run a few minutes later. This was the first traffic after the stack was rebuilt, so the difference is most likely warm-up (new connections and first use of each code path). It is not a bottleneck, but the 200-user run (JT-96) should start with a short warm-up.

### 10 users (19 November 2026, 16:26:17 to 16:26:46)

JMeter summary: 80 samples in 28 s, 2.8 per second, average 43 ms, minimum 9 ms, maximum 204 ms, 0 errors.

Preparation (not measured): `01` 80 to 204 ms, `02` 62 to 67 ms, `03` 9 to 14 ms; every resume was validated after one poll, about 1 s after upload.

Measured labels:

| Label | Samples | Median (ms) | Min (ms) | Max (ms) | Errors |
|---|---|---|---|---|---|
| 11 View job | 10 | 38 | 20 | 119 | 0 |
| 12 List my resumes | 10 | 12 | 9 | 15 | 0 |
| 13 Submit application | 10 | 43 | 34 | 52 | 0 |

### Docker statistics on the host

| Run | Snapshots in window | Stack peak CPU (of 800%) | Stack peak memory (of 7.75 GiB) | Busiest containers |
|---|---|---|---|---|
| 1 user, idle before (no timestamp in the file) | 1 | 81.1% | 3,590 MiB (about 45%) | k3d server 74.5% (2,714 MiB), `rabbitmq` 2.3% |
| 1 user, during | 3 | 56.7% | 3,778 MiB (about 48%) | `rabbitmq` 44.4%, k3d server 20.7% (2,705 MiB), `postgres` 6.0% |
| 10 users, idle before | 1 | 24.0% | 3,630 MiB (about 46%) | k3d server 18.7% (2,705 MiB), `grafana` 3.1% |
| 10 users, during | 15 | 80.0% | 3,675 MiB (about 46%) | k3d server 60.6% (2,706 MiB), `rabbitmq` 49.5%, `auth` 11.8%, `postgres` 9.8% |

Figures are per snapshot sums over all containers on the host; CPU is relative to one CPU, so 800% is the full Docker allocation. At 10 users the host used at most about a tenth of its CPU and under half its memory, so the trials did not approach the host's limits. The k3d cluster `jobtrack-jt29` was running at the same time and was the largest single user of memory and, in most snapshots, of CPU. Its CPU varied between 18.7% and 74.5% even with no test running, so its peaks are not caused by the trials (see Hardware limitations, point 3). Among the JobTrack containers, `rabbitmq`, `auth` and `postgres` rose the most under load.

## Hardware limitations

1. One host runs everything. All services, PostgreSQL, RabbitMQ, the resume validator and the gateway share Docker Desktop's 8 CPUs and about 7.75 GiB on a 16 GiB M3 laptop. A bottleneck in one container (for example PostgreSQL) slows every service, so results cannot be split per service the way separate machines would allow.
2. Docker Desktop runs containers in a virtual machine. The extra layer adds overhead that a Linux server would not have, and the 7.75 GiB allocation, not the 16 GiB of the laptop, is the real memory limit.
3. Other workloads share the host. Besides the services in the deadline-rush flow, the host was running the whole Compose stack (all 20 services, Prometheus, Grafana and Mailpit) and the k3d cluster `jobtrack-jt29`. The k3d server alone held about 2.7 GiB, about 35% of the Docker allocation, and its CPU varied between 18.7% and 74.5% on its own, even with no test running. It competes with JobTrack for the same CPUs at unpredictable moments. Results therefore depend on what else is running, and the 200-user run (JT-96) should record which containers and clusters were up. Capacity for the complete deployment is not yet proven (AGENTS.md Section 16).
4. The network is a personal phone hotspot. Bandwidth and latency depend on the phone and its signal and can change during a run, and the hotspot may limit throughput or connections. Response times include this Wi-Fi hop, which a wired network would not add.
5. One load generator. A single MacBook Air (Apple M1, 8 GiB RAM) runs JMeter. It has no fan, so it slows its CPU down under long, heavy load, and JMeter 5.6.3 starts with a 1 GB heap by default. At 200 threads, the laptop's CPU, memory and network could limit the load before the system does, so JT-96 should also record the JMeter laptop's CPU and memory during the run.
6. Coarse resource sampling. `docker stats --no-stream` takes about 2 s per snapshot, so short peaks between snapshots can be missed. Prometheus and Grafana (JT-93) would give continuous data.
7. Registration cost at 200 users. Each virtual user registers in the setUp group, and `auth` hashes every password with bcrypt. At 200 users this is a large CPU burst before the rush; raise `-JsetupRampUp` if it slows the host. It is not measured.
8. SonarQube competes for the same memory. It needs about 6 GB of Docker memory (see the SonarQube report), so it cannot run on the host together with the stack.
9. Each run leaves `users` applicants, resumes and applications in the database, and the trials ran against a test stack only. Repeated runs grow the tables, which can slow later runs a little.

## Verification

Commands run from the repository root on the load laptop, branch `JT-84-sonarqube-jmeter` at `9c718f2`, with a properties file outside the repository holding `host`, `jobId` and `password`:

    jmeter -n -t tests/load/deadline-rush.jmx -q ~/jt84-load.properties \
      -Jusers=1 -JrampUp=1 -JsetupRampUp=1 \
      -l /tmp/jt84-1user/results.jtl -j /tmp/jt84-1user/jmeter.log -e -o /tmp/jt84-1user/report

    jmeter -n -t tests/load/deadline-rush.jmx -q ~/jt84-load.properties \
      -Jusers=10 -JrampUp=10 \
      -l /tmp/jt84-10user/results.jtl -j /tmp/jt84-10user/jmeter.log -e -o /tmp/jt84-10user/report

On the Docker host, before and during each run:

    { date; docker stats --no-stream; } > /tmp/jt84-<n>user-before.txt
    while date && docker stats --no-stream; do :; done > /tmp/jt84-<n>user-during.txt

| Check | Result |
|---|---|
| 1-user trial | Pass, 8 of 8 samples, 0 errors |
| 10-user trial | Pass, 80 of 80 samples, 0 errors |
| Docker statistics cover both runs | Pass |
| CI on the pull request | Green: all 32 checks passed, including the required `CI passed` check, on commit `b590f1a` ([PR #83 checks](https://github.com/jobtrack-cst/jobtrack/pull/83/checks)) |

The JMeter results, HTML reports and Docker statistics files are kept outside the repository by Kinley and Phuntsho.

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| AC1: run SonarQube locally on `main`, save the report in `docs/testing`, log bugs in Jira with severity | Met | JT84-01, JT84-02 |
| AC2: draft a 200-concurrent-user deadline-rush JMeter scenario in `tests/load` | Met | JT84-03, JT84-04 |
| AC3: perform a low-user-count trial using a JMeter laptop separate from the Docker host | Met | JT84-05 to JT84-09 |
| AC4: document hardware limitations | Met | JT84-10 |
| AC5: submit changes through a PR titled with the Jira key and log time in Jira | Met | [PR #83](https://github.com/jobtrack-cst/jobtrack/pull/83), time logged in Jira JT-84 |
