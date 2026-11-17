# Load tests

JMeter plans for AGENTS.md Section 10 (Load: JMeter, `tests/load`, deadline-rush scenario, 200 concurrent users) and risk R8. Added in JT-84; the full 200-user run is JT-96.

## Deadline-rush scenario (`deadline-rush.jmx`)

Many applicants apply for the same job just before its deadline. Each virtual user is its own synthetic applicant.

| Phase | Thread group | Steps per user | Measured |
|---|---|---|---|
| Preparation | `Prepare applicants (not measured)` (setUp group) | `01` register, `02` log in, `03` upload a small synthetic PDF, `04` poll until the resume is validated (1 s apart, at most `validationTries` times), `05` check it is validated | No: ignore labels `01` to `05` in the results |
| Rush | `Deadline rush` | `11` view the job, `12` list my resumes, `13` submit an application. A synchronizing timer releases step `13` for all users at the same moment | Yes: labels `11` to `13` |

Every request goes through the gateway, as the frontend does. Assertions check HTTP 200, a validated resume and a created application. The plan stores no passwords or tokens: the password and target come from properties, and each user's token and resume ID are kept in memory between the two thread groups.

## Properties

| Property | Default | Meaning |
|---|---|---|
| `host` | `localhost` | Gateway host, for example the IP address of the machine running Docker Compose |
| `port` | `8000` | Gateway port |
| `protocol` | `http` | `http` or `https` |
| `jobId` | none (required) | A published job whose deadline is still ahead |
| `password` | none (required) | Password for the synthetic applicants, 8 to 72 bytes |
| `users` | `5` | Virtual users (applicants) |
| `rampUp` | `10` | Seconds to start the rush users |
| `setupRampUp` | `20` | Seconds to start the preparation users |
| `validationTries` | `30` | Polls (1 s apart) before a resume counts as not validated |
| `syncTimeoutMs` | `60000` | Longest wait for all users to reach step `13` |
| `runId` | current time | Makes the applicant emails unique per run (`jt84-<runId>-<n>@jobtrack.test`) |

Keep `password`, `host` and `jobId` in a properties file outside the repository, readable only by you, and pass it with `-q`. Do not put the password on the command line, where it stays in the shell history.

## How to run

Requirements: Apache JMeter 5.6.3 and Java 17 or later on the load machine (`brew install jmeter` on macOS). Per risk R8, run JMeter on a different laptop from the one hosting the system.

1. On the host, start the stack with Docker Compose (the services in the scenario: auth, job-posting, resume, resume-validator and its forwarder, application, application-status, gateway and their dependencies). The gateway listens on port 8000 on all interfaces.
2. As a verified employer, create and publish a job with a deadline at least an hour ahead, and note its ID.
3. On the load laptop, check the gateway is reachable: `curl -s -o /dev/null -w "%{http_code}\n" http://<host-ip>:8000/v1/jobs/<jobId>` (401 means reachable, because the request has no token).
4. Create the properties file, for example `~/jt84-load.properties` with `chmod 600`:

       host=<host-ip>
       jobId=<job id>
       password=<a test password, 8 or more characters>

5. Run without the GUI and write the results outside the repository. JMeter needs the parent folder of the report to exist:

       rm -rf /tmp/jt84-load && mkdir -p /tmp/jt84-load
       jmeter -n -t tests/load/deadline-rush.jmx -q ~/jt84-load.properties -Jusers=10 -JrampUp=10 \
         -l /tmp/jt84-load/results.jtl -e -o /tmp/jt84-load/report

   For the full scenario use `-Jusers=200`. Raise `-JsetupRampUp` if registration slows the host.

6. Open `/tmp/jt84-load/report/index.html`. Report the labels `11` to `13`: samples, error percentage, median, 90th and 95th percentile response times and throughput.

Each run leaves `users` synthetic applicants, resumes and applications in the target database. Run against a test stack only.

Pass thresholds for response time and error rate are not set yet (test plan, Section 8). Reports record the measured values and the hardware used.
