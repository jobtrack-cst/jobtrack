# JT-96 deadline-rush load test report (200 concurrent users)

Traceability: [JT-96](https://jobtrack-cst.atlassian.net/browse/JT-96) (parent JT-15). Depends on [JT-84](jt-84-test-cases.md). Owner: Kinley Tobgay Lhendrup (testing lead). Backup: Jigden Shakya. Run date: 19 November 2026.

## Summary

The deadline-rush scenario (AGENTS.md Section 10) ran with 200 concurrent synthetic applicants against the Docker Compose stack, with every request going through the gateway. All 2,003 requests succeeded (0.00% errors). In the measured rush, login, search, view and list took 9 to 77 ms at the 95th percentile, and all 200 applications released together by the synchronizing timer were created within about 77 ms, with a 95th percentile of 66 ms. The Docker host used at most about a quarter of its CPU allocation and about 42% of its memory allocation. No failures or bottlenecks were found, so no Jira Bugs were logged.

Pass thresholds for response time and error rate are still not set ([test-plan.md](test-plan.md) Section 8), so this report records measured values rather than a pass or fail against targets.

## Scope

- Plan: [`tests/load/deadline-rush.jmx`](../../tests/load/deadline-rush.jmx), described in [`tests/load/README.md`](../../tests/load/README.md).
- JT-96 extended the JT-84 plan so that the measured rush covers the four steps in the acceptance criteria: login, search jobs, view a job and apply. The rush now runs `10 Log in` (`POST /v1/auth/login`), `11 Search jobs` (`GET /v1/search/jobs?query=<searchQuery>`, asserting the target job is in the results), `12 View job`, `13 List my resumes` and `14 Submit application`. A synchronizing timer releases step `14` for all users at the same moment. The new property `searchQuery` is required.
- Preparation (labels `01` to `05`: register, log in, upload a synthetic PDF, wait for validation) runs in the setUp thread group and is reported separately as not measured.
- Synthetic users and data only: each virtual user registers its own applicant `jt84-<runId>-<n>@jobtrack.test`. The password, host and job come from a properties file outside the repository.
- No contract, service or tool changes (Section 7 tools only: JMeter, Prometheus and Grafana).

## Test environment

| Item | Value |
|---|---|
| Docker host | Phuntsho's laptop, Apple M3, 16 GiB RAM; Docker Desktop 8 CPUs and 7.75 GiB RAM (8,319,705,088 bytes) |
| Code on the host | `main` at `2d1abc5`, started with `docker compose -f deploy/compose/docker-compose.yml --profile functions --profile monitoring up -d --build` |
| Containers running | 32 Compose containers (the 20 services, gateway, PostgreSQL, RabbitMQ, Mailpit, the 5 functions, the resume-validator forwarder, Prometheus, Grafana) plus the k3d cluster `jobtrack-jt29` (server and load balancer). `k3d cluster stop` failed with "No nodes found", so the cluster stayed up, as in the JT-84 trials |
| Load generator | Kinley's MacBook Air, Apple M1, 8 GiB RAM; Apache JMeter 5.6.3, OpenJDK 25.0.2; branch `JT-96-deadline-rush-run` |
| Network | Both laptops on the same personal hotspot; gateway at `http://172.20.10.2:8000`, checked with `GET /healthz` (HTTP 200) |
| Target job | `0ff38554-7d19-4941-9036-7854f06ce23c`, title `JT-83 browser check`, `published` in job-posting and open (`closed = f`) in the job-search read model, deadline 23 November 2026 21:44 Bhutan time; checked in both databases before the run |
| Run settings | `-Jusers=200 -JrampUp=10 -JsetupRampUp=60 -JvalidationTries=120`, other properties at their defaults |
| Host monitoring | `docker stats --no-stream` in a loop (one snapshot about every 2 s, 36 snapshots inside the run) and the Grafana dashboard **JobTrack Compose monitoring** (JT-93) |

`setupRampUp` was raised from 20 to 60 s and `validationTries` from 30 to 120 so that 200 bcrypt registrations and 200 queued resume validations could not fail the unmeasured preparation before the rush started.

## Runs

| Run | Time (Bhutan) | Users | Samples | Errors | Purpose |
|---|---|---|---|---|---|
| Smoke | 20:02:53 to 20:03:03 | 1 | 11 | 0 | Prove the extended plan (login and search steps, search assertion) works end to end |
| Warm-up | 20:04:19 to 20:04:47 | 10 | 102 | 0 | Warm the services after the rebuild (JT-84 recommendation) and give a 10-user baseline |
| Full | 20:19:49 to 20:21:03 | 200 | 2,003 | 0 | The JT-96 deadline-rush run |

The smoke run showed about 1.2 s on the first register, upload, search, view and submit requests, the same first-traffic warm-up effect recorded in JT-84. The warm-up run and the full run did not show it.

## Results: 200 users

Response times in milliseconds. `p90` and `p95` are the 90th and 95th percentiles from JMeter's HTML report (`statistics.json`).

### Measured rush (labels 10 to 14)

| Label | Samples | Errors | Average | Median | p90 | p95 | Max | Throughput (req/s) |
|---|---|---|---|---|---|---|---|---|
| 10 Log in | 200 | 0.00% | 71.2 | 70 | 74 | 77 | 93 | 20.00 |
| 11 Search jobs | 200 | 0.00% | 9.3 | 9 | 11 | 12 | 15 | 20.14 |
| 12 View job | 200 | 0.00% | 9.3 | 9 | 10 | 11 | 19 | 20.14 |
| 13 List my resumes | 200 | 0.00% | 9.4 | 9 | 11 | 12 | 18 | 20.14 |
| 14 Submit application | 200 | 0.00% | 56.0 | 58 | 65 | 66 | 68 | 2,597.40 |

- Labels `10` to `13` run at about 20 requests per second because the 200 rush users start over `rampUp` = 10 s.
- `14 Submit application` is the deadline rush itself. JMeter's throughput of 2,597 per second means all 200 submissions started and finished within about 77 ms (200 / 2,597.4 s). Every one created an application (assertion on `$.application.applicationId`).
- Login is the slowest step before the rush because `auth` checks the password with bcrypt, by design (AGENTS.md Section 15, 5 Nov 2026).

### Preparation (labels 01 to 05, not measured)

| Label | Samples | Errors | Average | Median | p90 | p95 | Max |
|---|---|---|---|---|---|---|---|
| 01 Register applicant | 200 | 0.00% | 81.5 | 82 | 88 | 90 | 123 |
| 02 Log in applicant | 200 | 0.00% | 62.4 | 62 | 66 | 68 | 88 |
| 03 Upload resume | 200 | 0.00% | 10.6 | 10 | 12 | 13 | 22 |
| 04 Poll resume status | 203 | 0.00% | 10.4 | 10 | 11 | 12 | 67 |
| 05 Check resume validated | 200 | 0.00% | 9.6 | 9 | 11 | 11 | 23 |

203 polls for 200 resumes means only 3 extra polls in total: almost every resume was validated at the first poll, about 1 s after upload, so the resume-validator kept up with 200 uploads over 60 s.

### Whole run

2,003 samples in 1 min 11 s, 28.19 requests per second overall, average 32.9 ms, median 11 ms, p90 74 ms, p95 82 ms, maximum 123 ms, 0 errors.

### Comparison with the 10-user warm-up

| Label | 10 users p95 | 200 users p95 | 10 users average | 200 users average |
|---|---|---|---|---|
| 10 Log in | 193 | 77 | 138.3 | 71.2 |
| 11 Search jobs | 13 | 12 | 10.6 | 9.3 |
| 12 View job | 12 | 11 | 9.9 | 9.3 |
| 13 List my resumes | 10 | 12 | 9.5 | 9.4 |
| 14 Submit application | 33 | 66 | 24.7 | 56.0 |

With twenty times the users, login, search, view and list did not get slower. Only the simultaneous submit grew, from about 25 to 56 ms on average, because 200 submissions arrived in the same instant instead of 10. It stayed far below the 2 s `pkg/grpcclient` timeout that `application` uses for its calls to job-posting and resume.

## Resource use on the Docker host

Sums of `docker stats` over all containers per snapshot. CPU is relative to one CPU, so 800% is the full Docker allocation.

| Phase | Time | Stack CPU (of 800%) | Busiest containers |
|---|---|---|---|
| Idle before | 20:07:15 | 18.8% | k3d server 15.5%; every JobTrack container under 1% |
| Preparation | 20:19:50 to 20:20:48 | about 45% to 115% | `auth` 33% to 45% (bcrypt registrations and logins), k3d server spikes up to 64% |
| Rush logins | 20:20:50 to 20:20:58 | about 96% to 125% | `auth` about 100% (one CPU), `gateway` 2% to 4% |
| Submit burst | 20:21:00 | 212.1% (peak, about 27% of the allocation) | `postgres` 37.8%, k3d server 70.2%, `rabbitmq` 24.0%, `application` 13.5%, `auth` 13% |
| After the run | 20:21:02 to 20:21:56 | about 20% to 100% | k3d server up to 91.3%; event consumers (`application-status`, `review`, `interview`, `offer`, `messaging`, `analytics`) and `mailpit` handling the 200 `application.submitted` events and notifications |

- Memory: the stack peaked at 3,330 MiB, about 42% of the 7.75 GiB allocation. JobTrack's own containers peaked at about 940 MiB; the k3d server held about 2.3 GiB of the rest. Idle before the run the stack used 3,173 MiB.
- Peak per container: `auth` 104.2% CPU and 18 MiB; `postgres` 37.8% and 209 MiB; `rabbitmq` 24.0% and 164 MiB; `application` 13.5% and 24 MiB.
- The k3d server's CPU varied between 3.9% and 91.3% and does not follow the load (its highest value came after the run), as in JT-84.

## Grafana (JT-93 monitoring)

Screenshots of the dashboard **JobTrack Compose monitoring** on the Docker host, time range "Last 30 minutes" (about 19:54 to 20:24), which contains all three runs.

| Panel | File | What it shows |
|---|---|---|
| Whole dashboard | [jt-96-dashboard.jpg](jt-96-screenshots/jt-96-dashboard.jpg) | All panels; scrape availability and readiness stay at 1 for every service throughout |
| Request rate by service | [jt-96-requests.jpg](jt-96-screenshots/jt-96-requests.jpg) | The full run at about 20:20 to 20:21, peaking at about 33 requests per second on the gateway; the warm-up at about 20:04 to 20:05 |
| Error rate by service | [jt-96-errors.jpg](jt-96-screenshots/jt-96-errors.jpg) | 0% for every service during all runs |
| Mean request latency by service | [jt-96-latency.jpg](jt-96-screenshots/jt-96-latency.jpg) | Under about 60 ms for every service during the full run; one 600 ms point at about 20:03 is the smoke run's first-traffic warm-up |

The request-rate panel also shows a spike of the same height at about 20:09 to 20:11. That was a separate test by a teammate on the same stack, not a JT-96 run: the smoke, warm-up and full runs above are the only JT-96 runs, and their JMeter results are the source of every figure here. The error-rate panel shows no errors during it either, and it ended about 8 minutes before the full run started.

The screenshots are photos of the host screen saved as JPEG, so small text is less sharp than a PNG export.

## Test matrix

| Case ID | Requirement | Test case | Technique | Expected result | Level and location | Result |
|---|---|---|---|---|---|---|
| JT96-01 | AC1: scenario covers login, search jobs, view a job, apply through the gateway | The rush runs labels `10` to `14` against the gateway only | Equivalence partitioning (main rush path) | All five measured steps present, all requests to `http://<host>:8000` | Load, `tests/load/deadline-rush.jmx` | Pass |
| JT96-02 | AC1 | Search finds the target job | Equivalence partitioning | `11 Search jobs` returns HTTP 200 and `jobId` is in `$.jobs[*].jobId` | Load, `tests/load` | Pass, 200 of 200 |
| JT96-03 | AC1 | 1-user smoke run of the extended plan | Boundary values (lowest load) | 11 of 11 samples pass | Load, `tests/load` | Pass |
| JT96-04 | AC1 | 10-user warm-up run | Equivalence partitioning (low load) | 0 errors | Load, `tests/load` | Pass, 102 of 102 |
| JT96-05 | AC1 | 200-user deadline rush (Section 10) | Boundary values (target load) | Every measured request HTTP 200 and every application created | Load, `tests/load` | Pass, 1,000 of 1,000 measured, 2,003 of 2,003 overall |
| JT96-06 | AC1 | All 200 submissions released together | State transition (waiting to released) | All 200 `14 Submit application` requests within one short burst | Load, `tests/load` | Pass, all within about 77 ms |
| JT96-07 | AC1 | Asynchronous resume validation keeps up with 200 uploads | State transition (PENDING to VALIDATED) | Every resume validated within `validationTries` | Load, `tests/load` | Pass, 203 polls for 200 resumes (3 extra in total) |
| JT96-08 | AC2: `.jmx` committed in `tests/load`, synthetic data only | Plan and README updated; no password, token or real data in the repository | Not applicable (review) | Password, host, job and search term only in a properties file outside the repository | Review, `tests/load` | Pass |
| JT96-09 | AC3: report with throughput, average, p90, p95, error rate, hardware | This report | Not applicable (documentation) | All listed figures present | Documentation, this file | Pass |
| JT96-10 | AC3: Grafana screenshots if monitoring is ready | JT-93 dashboard captured over the runs | Not applicable (measurement) | Request rate, error rate and latency screenshots | Documentation, `jt-96-screenshots` | Pass |
| JT96-11 | AC4: bottlenecks or failures logged as Jira Bugs with severity | Review results and host statistics for failures or saturation | Not applicable (analysis) | A Jira Bug for each failure or bottleneck | Jira | Pass, none found, so no Bug logged |

## Bottlenecks and failures

None found at 200 concurrent users:

- 0 failed requests and 0 failed assertions in any run.
- No container came near a limit. The highest CPU user was `auth` at about one CPU during the 10-second login burst, which is the bcrypt cost of 200 logins (AGENTS.md Section 15) and did not slow the other steps; `postgres` peaked at 37.8% during the submit burst.
- The gateway's 2 s backend timeout and 3 MiB body limit were not reached.

Observations for later work, not defects:

1. `auth` is the first service to use a full CPU. A larger rush (more users or a shorter `rampUp`) would queue logins in `auth` first.
2. The submit burst moves the load to `postgres` and `rabbitmq` (outbox and event fan-out to five consumers); this is where a larger test should look next.

## Limitations

The JT-84 hardware limitations still apply ([jt-84-test-cases.md](jt-84-test-cases.md), Hardware limitations). For this run in particular:

1. The load generator's CPU and memory were not recorded (the monitoring loop on the JMeter laptop was not running). The low and steady JMeter response times suggest the MacBook Air was not the limit, but this run cannot prove it.
2. The k3d cluster `jobtrack-jt29` stayed up and used up to 91% CPU on its own, independently of the load, on the same Docker allocation.
3. `docker stats` samples about every 2 s, so short peaks between snapshots can be missed. Grafana gives continuous request, error and latency data, but not CPU or memory per container.
4. One target job and one run per user count. The figures describe this machine and this network (a phone hotspot), not production capacity.
5. The run left 200 more synthetic applicants, resumes and applications (and their notifications in Mailpit) in the test stack, as expected.

Risk R8 ([risk-register.md](../pm/risk-register.md)): its trigger (a trial run fails or the host freezes before 200 users) did not fire. The risk owner can record this at the next risk review.

## Verification

On the load laptop, from the repository root on branch `JT-96-deadline-rush-run`, with `~/jt84-load.properties` (mode 600) holding `host`, `jobId`, `searchQuery` and `password`:

    jmeter -n -t tests/load/deadline-rush.jmx -q ~/jt84-load.properties \
      -Jusers=1 -JrampUp=1 -JsetupRampUp=1 \
      -l /tmp/jt96-smoke/results.jtl -j /tmp/jt96-smoke/jmeter.log -e -o /tmp/jt96-smoke/report

    jmeter -n -t tests/load/deadline-rush.jmx -q ~/jt84-load.properties \
      -Jusers=10 -JrampUp=10 \
      -l /tmp/jt96-warmup/results.jtl -j /tmp/jt96-warmup/jmeter.log -e -o /tmp/jt96-warmup/report

    jmeter -n -t tests/load/deadline-rush.jmx -q ~/jt84-load.properties \
      -Jusers=200 -JrampUp=10 -JsetupRampUp=60 -JvalidationTries=120 \
      -l /tmp/jt96-200/results.jtl -j /tmp/jt96-200/jmeter.log -e -o /tmp/jt96-200/report

Figures in this report come from `report/statistics.json` of each run, and the per-label error counts from `results.jtl`.

On the Docker host, before and during the full run:

    { date; docker stats --no-stream; } > /tmp/jt96-before.txt
    while date && docker stats --no-stream; do :; done > /tmp/jt96-during.txt

The JMeter results, HTML reports and Docker statistics files are kept outside the repository by Kinley and Phuntsho.

## Acceptance criteria review

| Acceptance criterion | Status | Evidence (case IDs) |
|---|---|---|
| Deadline-rush scenario (200 concurrent users) run against the Compose stack through the gateway: login, search jobs, view a job, apply | Met | JT96-01 to JT96-07 |
| `.jmx` plan committed in `tests/load`; synthetic users and data only | Met | JT96-08 |
| Report in `docs/testing` with throughput, response times (average, p90, p95), error rate, hardware and Grafana screenshots | Met | JT96-09, JT96-10 |
| Bottlenecks or failures logged as Jira Bugs with severity | Met (none found) | JT96-11 |
| Follows AGENTS.md; no new tools or contract changes | Met | Scope section |
| PR titled with the Jira key, merged to `main`, time logged in Jira | Pending | PR and Jira time log |
