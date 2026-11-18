# JT-94 documentation validation

Date: 19 November 2026 (Asia/Thimphu). Branch:
`JT-94-architecture-deployment-docs`. Initial `git status --short --branch`
showed this branch with a clean working tree. No branch switch, pull, staging,
commit, push or Jira mutation was performed. Only Markdown documentation changed;
local `.env`, code, contracts, manifests and dependencies were preserved.

## Changes and traceability

| File | JT-94 scope |
|---|---|
| [Architecture](../architecture/system-architecture.md) | Logical Mermaid diagram; 20 services with owners, database/users, publishing/consumption; five functions/triggers; main event flows and lifecycle; approved gRPC table; JWT boundary and three internal security layers with implementation caveats |
| [Run guide](../deployment/how-to-run.md) | End-to-end Compose setup, opt-in functions, frontend and operational checks; conditional k3d/Knative sequence and explicit deployment gates; JT-29/JT-61/email-sender links |
| [Root README](../../README.md) | Links both guides and replaces obsolete template-only deployment status |
| [Compose README](../../deploy/compose/README.md) | Links the canonical run guide; removes obsolete service-copy/setup claims and automatic destructive reset advice |
| This record | Checks actually run, their limits, and teammate review |

Source inspection covered root AGENTS.md (especially Sections 3–6 and 16), root
README, `docs/architecture/`, existing `docs/deployment/` guides, Compose file,
env example, database init/smoke scripts, monitoring instructions, Knative manifests
and JT-29 scripts, function/frontend READMEs, existing JT-95 test instructions and
CI configuration. Local secret values were not printed or copied into docs.

## Checks performed

| Check | Result and scope |
|---|---|
| `git diff --check` and full changed/new Markdown review | Pass; whitespace and documentation-only scope inspected, including untracked new files |
| Local link/path validation with Python standard library | Pass; 43 relative Markdown link destinations exist; referenced runnable command files exist. No external website checks required for the new local navigation |
| Inventory comparison with AGENTS.md and database init script | Pass; exact 20 service owner/responsibility/event rows, matching snake_case database/user names; five function/trigger rows and all ten approved synchronous-call rows retained |
| Fenced shell blocks via `bash -n` | Pass; 16 shell blocks, syntax only, including the conditional k3d variable checks. Does not run build/apply commands or establish prerequisite values |
| Mermaid source review | Pass for declared nodes/edge references, five-function scope and flow direction against AGENTS.md; no installed Mermaid renderer used. Teammate must inspect rendering in the PR preview |
| Four Compose `config --quiet` checks with `.env.example` | Pass for default, `functions`, `monitoring` and both profiles. Structural/interpolation check only; placeholders are not runtime credentials |
| Compose parsed inventory with example configuration | Pass; 20 service build definitions and gateway; five functions plus validator forwarder in the functions profile; no dispatcher forwarder, host gRPC mapping or gateway internal token |
| `kubectl kustomize deploy/knative/forwarder` | Pass; local render only, output kept outside repo |
| `kubectl kustomize deploy/knative/jt29` | Pass; local render only, output kept outside repo |
| `bash deploy/compose/smoke-test_test.sh` | Pass (three result groups); mocked CLI regressions, not a live infrastructure smoke run |
| Existing Compose `ps --format '{{.Service}} {{.State}} {{.Health}}'` | Pass; observed 29 running containers, including optional monitoring; notification-dispatcher, weekly-summary and deadline-reminder absent from this running stack. Services lacking healthchecks were not treated as healthy |
| `docker compose ... exec -T resume wget -qO- http://localhost:8080/healthz` | Pass, `ok`, against the existing container; no rebuild/restart |
| `docker compose ... exec -T rabbitmq rabbitmqctl list_queues name type messages_ready messages_unacknowledged consumers` | Pass; validator inbox/result source queues each had one consumer and zero ready/unacknowledged messages; their dead queues were empty. Snapshot only, not proof of a new event outcome |
| Gateway `GET /healthz`, unauthenticated `GET /v1/auth/me`, Mailpit `/api/v1/info` | Pass: `ok`, HTTP 401 and successful Mailpit response, respectively, using five-second curl limits |

The first unprivileged Docker status query was denied by the local socket sandbox;
the read-only status and Resume probes succeeded with approved elevated execution.
No daemon settings or resources were changed. No complete stack smoke run, image
build, npm/Go test suite, workflow mutation, Knative apply or live cluster test was
run for this documentation-only change. Existing prior results are linked and dated
in their own reports; they are not new JT-94 evidence. The repository has no
dedicated Markdown/link/Mermaid validation job in CI, so local checks used existing
Python/Bash/Compose/kubectl tools without adding dependencies or test code.

## Open deployment dependencies and teammate review

- Phuntsho: confirm the official WEB303 assessment permits the documented split
  (20 services in Compose, five functions in Knative). Confirm architecture tables,
  lifecycle and approved calls against AGENTS.md; inspect the rendered Mermaid.
- Sangay/DevOps: review the k3d context/namespace, private credential provisioning,
  persistent broker, Compose-to-k3d routing/DNS and return paths, Mailpit SMTP,
  local image resolution and pending NetworkPolicies. Same-namespace manifest
  endpoints do not establish connectivity to Compose.
- Function owners: supply the missing dispatcher Knative/forwarder wiring and
  validate the unchanged envelope and restart-safe deduplication; review the
  current Saved Jobs internal-call/pagination gaps. JT-94 does not repair these.
- Teammate following the guide: run fresh setup/build/runtime checks on the intended
  host, the existing main workflow and all function outcomes; JT-92 must capture
  actual trigger-to-outcome and scale-to-zero/reactivation evidence for all five.
  Review conditional image/apply examples against the agreed reachable endpoints
  before executing them. Recheck resource capacity under the intended workload.

Peer review, applicable green CI, PR and merge are not verified here. JT-94 is not
claimed Done. After local verification/review, the member should commit with
`JT-94`, push the branch, open a PR and move the card to In Review; these remain
manual follow-up steps under the user's restrictions for this session.
