# JT-62 resume-validator test cases and execution evidence

Executed 12 November 2026 on `JT-62-resume-validator`. Initial
`git status --short --branch` showed only the branch line (clean); repository-root
AGENTS.md was read before implementation. Jira JT-62 was read without edits.
No branch switch, contract/service changes, dependencies, commit, push or PR.

## Cases and acceptance criteria

The Product Owner-approved criteria recorded in Jira JT-62 require valid uploaded
PDFs within the classic-cross-reference/direct-catalog subset to become VALIDATED,
plain non-PDF uploads to return INVALID_ARGUMENT synchronously under the existing
Resume contract, and stored spoofed, malformed or unsupported PDF content to
become REJECTED through resume-validator. On 12 November 2026 the Product Owner
approved rejecting unsupported layouts, including compressed object-stream
catalogs; general PDF-format compatibility is outside JT-62 scope. The evidence
below retains the actual tests and demonstrated scenarios. The previously pending
PDF-layout decision is now resolved. The scope clarification itself did not run
tests; the later catalog-lexing follow-up at the end records new test execution
without changing the existing deployment infrastructure.

| ID | Technique / check | Expected | Actual |
|---|---|---|---|
| JT62-01 | Valid synthetic PDF with real page, font, content stream and xref offsets | validated event | PASS within the approved classic-xref/direct-catalog subset, unit + Compose + k3d |
| JT62-02 | Boundary sizes 1,999,999 / 2,000,000 / 2,000,001 | accept / accept / reject | PASS, exact fixture lengths asserted |
| JT62-03 | Empty / one byte / wrong MIME / HTML MIME spoof | rejected event | PASS, unit |
| JT62-04 | `%PDF-` spoof; bad version; embedded header; truncated file; invalid startxref; appended executable | reject | PASS, table-driven unit |
| JT62-05 | Binary CloudEvent / shared uploaded fixture / typed output schema | validate metadata and schema | PASS, handler + existing contract suites |
| JT62-06 | Wrong method/path; malformed/oversized body; wrong MIME/spec/producer/data; missing source; mismatched event ID | reject before RPC or publication | PASS, table-driven unit |
| JT62-07 | Internal RPC unavailable; unconfirmed publish | 503, forwarder can retry | PASS, unit |
| JT62-08 | File applicant differs from event applicant | 422, no outcome published | PASS, unit |
| JT62-09 | Repeated input event | stable outcome event ID and logical timestamp | PASS, unit; Resume owns durable effect deduplication |
| JT62-10 | Real RabbitMQ -> Go forwarder -> real validator handler -> protected gRPC test server -> confirmed output | validated event | PASS, Testcontainers integration + race detector |
| JT62-11 | Real Compose upload -> outbox -> broker -> Go forwarder -> actual validator -> real Resume consumer | VALIDATED / REJECTED | PASS, final image after catalog lexer fix; source/outcome/Resume persistence IDs in final section |
| JT62-12 | Same genuine delivery in k3d with actual Knative Service | VALIDATED / REJECTED | PASS, same final image and binary; retained namespace/data; IDs in final section |
| JT62-13 | Plain non-PDF upload and stored spoofed/invalid PDF content | synchronous INVALID_ARGUMENT for plain non-PDF; asynchronous REJECTED for stored spoofed/invalid PDF | PASS under the Product Owner-approved clarification: synchronous non-PDF rejection and asynchronous spoofed-PDF rejection documented in Compose and k3d |
| JT62-14 | Dockerfile, Compose functions profile, Knative manifest | real function deployment | PASS, image build + both runtimes |
| JT62-15 | Shared Sprint 2 PR/CI/merge/time logging | project workflow | Not performed: explicitly excluded by request |
| JT62-16 | Bare-xref review counterexample; bad offsets/counts/entries/trailer/root | rejected event | PASS, new table-driven unit cases; real bare-xref rejection in Compose + k3d |
| JT62-17 | Catalog in a FlateDecode object stream with a type-2 xref-stream entry | reject unsupported layout under the approved subset | PASS, unit test returns UnsupportedLayout; final-image REJECTED confirmed through real Resume in both Compose and k3d |
| JT62-18 | Stalled TCP/AMQP handshake, channel-open, declare, confirm-select, publish acknowledgment, close acknowledgment | bounded return, retryable uncertainty, stable retry identity | PASS, unit + real RabbitMQ protocol-proxy integration + race |
| JT62-19 | Actual root /Type /Catalog entry; string/comment/unrelated/nested spoofs; name escapes and malformed lexical structure | genuine direct catalog accepted; catalog-like text rejected | PASS, new table-driven unit/handler cases and isolated real RabbitMQ/forwarder/gRPC integration; validator race suite |
| JT62-20 | Final-image delivery after catalog lexer fix, both Compose and k3d | valid PDF VALIDATED; prefix/catalog spoofs, malformed and unsupported PDFs REJECTED; plain non-PDF INVALID_ARGUMENT | PASS, 5 stored-file cases and synchronous non-PDF rejection per environment; all event IDs correlated and source/outcome persistence verified |

The format check is a signature gate, not a full PDF parser or malware scan.
The review fix validates a standalone classic xref table and its direct catalog;
xref streams, compressed catalogs and hybrid/incremental layouts needing
`/XRefStm` or `/Prev` resolution are explicitly unsupported. See the function
README and investigation below. No claim of complete PDF semantic validation or
general PDF-layout support is made.

## Actual automated commands and results

From repository root. Local sandbox restrictions initially blocked the default Go
cache, Docker socket, and test listeners. Checks were rerun with approved local
infrastructure access and a writable build cache. No secrets were printed.

```sh
gofmt -w functions/resume-validator
GOCACHE=/tmp/jobtrack-jt62-go-cache go test ./...
GOCACHE=/tmp/jobtrack-jt62-go-cache go vet ./...
GOCACHE=/tmp/jobtrack-jt62-go-cache go test -tags=integration ./...
GOCACHE=/tmp/jobtrack-jt62-go-cache go test -race -tags=integration ./functions/resume-validator/...
buf lint
buf breaking --against '.git#ref=HEAD'
git diff --check
```

All completed successfully. Full unit and integration runs each reported 167
package-result lines at the time of execution (before the final test-helper
package was added); logs were saved under `/tmp/jobtrack-jt62-unit.log` and
`/tmp/jobtrack-jt62-all-integration.log`. Final function/race results are in
`/tmp/jobtrack-jt62-final-function.log`. Existing Resume PostgreSQL/RabbitMQ and
forwarder integration suites passed. An initial JT-62 integration build used the
wrong topology helper name; it was corrected to
`forwarder.ResumeValidatorTopology().Declare(ch)` before successful reruns.
No remote CI result is claimed. Proto generation was not required: contracts and
generated sources are unchanged.

## Compose: genuine delivery

```sh
docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build resume-validator resume-validator-forwarder
GOCACHE=/tmp/jobtrack-jt62-go-cache CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -o /tmp/jobtrack-jt62-verify \
  ./functions/resume-validator/cmd/verify
docker compose -f deploy/compose/docker-compose.yml --profile functions run --rm --no-deps -v /tmp/jobtrack-jt62-verify:/jt62-verify:ro --entrypoint /jt62-verify resume-validator
docker compose -f deploy/compose/docker-compose.yml --profile functions logs --tail 25 resume-validator resume-validator-forwarder
docker compose -f deploy/compose/docker-compose.yml exec -T rabbitmq rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers
```

The verifier observes source and result envelopes from an exclusive broker queue,
then polls real `GetResume`. It does not fabricate domain events or edit storage.
The last run used a complete synthetic one-page PDF. On x86 hosts build the
verifier with `GOARCH=amd64` instead.

| Case | Source event_id | Result event_id | Resume ID | Final status |
|---|---|---|---|---|
| Valid | `71717932-0a42-4fed-b0b6-c367c4d967c2` | `53679c8e-709e-57c8-8a9c-0afda46fea04` | `2c18e7eb-9ce9-448b-ad2b-9b30f9d0d195` | VALIDATED |
| Spoof | `03da9b67-9267-4c48-9c74-9e2b8392b44a` | `17c5665f-a6a5-5543-abd3-25c148cc588a` | `10f7cc79-fdfb-4a64-adf1-db0d733e3d9c` | REJECTED |

An earlier smoke run also correlated source IDs
`71f8a6db-b09a-42fa-b981-37e21f026873` (valid) and
`7b665363-a0b6-4d72-8b28-4f371510725f` (spoof) across broker observer,
`event delivered` forwarder logs and `resume validation published` function logs.
At queue inspection, both source/validation queues had zero ready and
unacknowledged messages, one consumer each, and both dead queues were empty.
Plain `text/plain` upload returned INVALID_ARGUMENT in both runs.

## k3d: genuine Knative delivery

Cluster `jobtrack-jt29` already contained Knative, but only disposable JT-29 spike
workloads. A separate **jt62-verification** namespace was created for this test.
It contains an unchanged real Resume image, PostgreSQL 18, RabbitMQ 4.1, each with
a 1 GiB PVC for database/broker persistence, generated test credentials in Secrets,
the actual validator Knative Service, the Go forwarder, and verifier Jobs.
The JT-29 spike sink/broker were not used. The user's current context was not set
or changed; commands use a temporary kubeconfig explicitly.

Actual setup commands included:

```sh
k3d kubeconfig get jobtrack-jt29 > /tmp/jobtrack-jt62-kubeconfig
docker tag jobtrack-resume-validator-forwarder:latest jobtrack-rabbitmq-forwarder:local
k3d image import jobtrack-resume-validator:local jobtrack-resume:latest jobtrack-rabbitmq-forwarder:local jobtrack-jt62-verify:local -c jobtrack-jt29
python3 /tmp/jobtrack-jt62-infra.py
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification apply -f deploy/knative/resume-validator.yaml
python3 /tmp/jobtrack-jt62-patch-image.py
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification apply -k deploy/knative/forwarder
python3 /tmp/jobtrack-jt62-run-job-qualified.py
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification wait --for=condition=complete job/jt62-verify-qualified --timeout=45s
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs job/jt62-verify-qualified
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs deployment/resume-validator-forwarder --tail=8
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs -l serving.knative.dev/revision=resume-validator-00005 -c user-container --tail=8
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification exec deployment/rabbitmq -- rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers
```

The `/tmp` Python scripts were local setup helpers, not shipped production
infrastructure. `infra.py` applied the namespace, generated Secrets, PVCs, Services
and Deployments described above. It must **not** be rerun against retained broker
storage with new credentials. `patch-image.py` changed only
`/spec/template/spec/containers/0/image` via JSON Patch to the imported digest.
The verifier image was built from Alpine 3.22 with `/tmp/jobtrack-jt62-verify`
copied to `/verify`, entrypoint `/verify`, user 10001. The Job has backoffLimit 0,
restartPolicy Never, RESUME_ADDR=resume:9090, and RABBITMQ_URL from the
`rabbitmq-credentials` Secret. For reproduction, provision the approved
application namespace/Secrets first as described in the function README; run
`cmd/verify` on that network. Temporary machine-specific scripts are not a
portable deployment interface.

The validator image digest was
`sha256:7c98eedca64f01b1c4799df870b909510049e9039a83830aaa72fd08dc2a2dbd`.
Local Knative tag resolution initially contacted Docker Hub and failed. The
imported image was registered under its digest in node containerd, then the
Knative Service was JSON-patched to that digest (no registry push):

```sh
docker exec k3d-jobtrack-jt29-server-0 ctr -n k8s.io images tag docker.io/library/jobtrack-resume-validator:local docker.io/library/jobtrack-resume-validator@sha256:7c98eedca64f01b1c4799df870b909510049e9039a83830aaa72fd08dc2a2dbd
```

PostgreSQL's initial multi-platform archive import reported a missing digest
(despite k3d's success summary). Importing only arm64 and registering that manifest
resolved it; deployment startup was checked independently:

```sh
docker image save --platform linux/arm64 -o /tmp/jobtrack-jt62-postgres.tar postgres:18-alpine
k3d image import /tmp/jobtrack-jt62-postgres.tar -c jobtrack-jt29
docker exec k3d-jobtrack-jt29-server-0 ctr -n k8s.io images tag docker.io/library/postgres:18-alpine docker.io/library/postgres@sha256:b1c1673a890fba8fae71d37210b4fd9a0b5cd0df8bb994a6bcc01d9cf8e1670c
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification set image deployment/postgres postgres=docker.io/library/postgres@sha256:b1c1673a890fba8fae71d37210b4fd9a0b5cd0df8bb994a6bcc01d9cf8e1670c
```

The first real upload exposed the JT-61 manifest's short sink hostname returning
404 from Knative. A direct health request to `http://resume-validator/healthz`
returned 404; the namespace-qualified `.svc.cluster.local` hostname returned 200.
The committed forwarder manifest now obtains POD_NAMESPACE through fieldRef and
uses `http://resume-validator.$(POD_NAMESPACE).svc.cluster.local`.

That first event, `7c0be053-6f92-45d9-9754-d82312b20381`, exhausted attempts
1/2/3/4 at 14:00:41/42/44/48 UTC and remains in `resume-validator.events.dead`.
It was not deleted or automatically replayed; its resume remains PENDING.
The routing fix was applied, and fresh real uploads passed:

| Case | Source event_id | Result event_id | Resume ID | Final status |
|---|---|---|---|---|
| Valid | `e4095e93-e007-4543-957b-d44273f63cbe` | `f528e335-e131-5691-9649-2cc45e9fed53` | `6caa22c0-a56f-455d-9fdf-89103283b55f` | VALIDATED |
| Spoof | `1533a84e-c272-4981-a7dd-9b46366729f6` | `d51d6af6-7e81-55a6-a8f4-386cf50977a0` | `eceafc5c-c4ca-4102-a168-aa3e13904c68` | REJECTED |

The verifier Job completed. Function publication logs and forwarder delivery logs
both contain those source IDs at 14:01:57 and 14:01:58 UTC (20:01:57/58 Bhutan).
The validator's latest ready revision was `resume-validator-00005`; it also
activated through the namespace-qualified hostname after scaling to zero.
The plain non-PDF upload returned INVALID_ARGUMENT again.

Obsolete test revisions 00001..00004 were removed after evidence collection;
current workloads, test resume records, PVCs and the one diagnostic dead letter
were retained for inspection. No queues or volumes were deleted.

## Remaining limits

- Plain non-PDF uploads return INVALID_ARGUMENT before storage under the existing
  contract; stored prefix-spoofed PDFs become REJECTED asynchronously. These
  documented outcomes satisfy the clarified JT62-13 criterion without a bypass
  or contract change.
- The retained diagnostic DLQ event requires deliberate operator review/replay if
  its synthetic PENDING resume is to be completed; no automatic replay is provided.
- Structural/signature validation has the unsupported-layout limitation documented
  above. Rejection of unsupported layouts is approved; general PDF-format
  compatibility is outside JT-62 scope.
- Remote CI, peer review, commit/push/PR/merge and Jira workflow remain unperformed
  under the user's explicit instructions.

Final k3d queue inspection after successful fresh uploads: source queue
`resume-validator.events` and result queue `resume.validation` each had 0 ready,
0 unacknowledged and 1 consumer. `resume.validation.dead` had 0 messages;
`resume-validator.events.dead` retained exactly 1 diagnostic message.

## Final-review fixes: new execution on 12 November 2026

This section supersedes the earlier implementation's PDF/publisher descriptions,
while retaining its real historical results, IDs and diagnostic dead letter.
No contracts, shared APIs, dependencies, other services, Jira, branches or Git
publication state were changed in this follow-up.

### JT62-16: reproduced and rejected malformed xref

Before changing the implementation, the following command **failed as expected**
with `expected: false, actual: true` for `bare_xref_review_regression`:

```sh
GOCACHE=/tmp/jobtrack-jt62-go-cache go test ./functions/resume-validator/internal/validator -run TestValidate_SignaturesAndBoundaries/bare_xref -count=1
```

The exact input was
`%PDF-1.4\n1 0 obj\n/Type /Catalog\nendobj\nxref\nstartxref\n39\n%%EOF\n`.
It passed the old signature-token checks but had no xref entries/trailer and no
valid catalog dictionary. The new bounded checker requires subsection counts,
fixed-format xref entries, matching referenced object IDs/generations at their
actual byte offsets, a trailer dictionary with `/Size` and `/Root`, and a direct
catalog dictionary at that root reference. Table-driven cases also reject offset
mismatches, overflowing counts, missing entries/trailer, invalid trailer size,
wrong root generation and a root pointing to a non-catalog object.

It remains a structural/signature gate: it does not parse the full PDF grammar,
validate page graphs/content streams or scan for malware. The optimistic old
xref-stream branch was removed rather than declaring structurally unverified
streams valid. File size remains exactly 1..2,000,000 decoded bytes.

### JT62-17: compressed-catalog investigation and approved scope

No PDF parser is present in the approved dependency list. Standard-library zlib
can decode a single Flate stream, but safe general support also needs dictionary
and indirect-reference parsing, stream-length resolution, filter chains and
predictors, bounded decompression, xref `/W`/`/Index` decoding, type-2 entry lookup,
object-stream `/N` and `/First` indexing, and handling of earlier/hybrid revisions.
Adding those mechanisms would be a substantial PDF parser beyond a signature
check. Scanning arbitrary inflated bytes for `/Catalog` would repeat the spoofing
problem and would not prove that the catalog is the document's referenced root.

The investigation follows the primary implementer documentation:
[QPDF: Object and Cross-Reference Streams](https://qpdf.readthedocs.io/en/stable/object-streams.html).
It describes object-number/offset pairs, cross-reference-stream field widths,
type-2 entries and hybrid/previous-table resolution. QPDF was consulted as
documentation only; it was not installed or introduced as a runtime dependency.

`testpdf.CompressedCatalog()` constructs a PDF 1.5 layout with a catalog in a
FlateDecode object stream and a type-2 xref-stream entry. The test confirms the
catalog signature is absent from the raw file and that the current validator
returns `UnsupportedLayout`. This is a limitation test, **not positive parser
support or independent external-reader validation**. The first small fixture
used a stored DEFLATE block, leaving the signature visible; legal padding was
added to exercise actual compressed bytes before the successful rerun.

Approved restriction: only standalone classic xref tables with a root catalog
available as a direct dictionary are structurally checked. Cross-reference
streams, compressed object-stream catalogs and hybrid/incremental layouts
requiring `/XRefStm`/`/Prev` resolution fail closed and produce REJECTED, even when
another PDF reader could accept the file.

**Product Owner decision recorded in Jira JT-62, approved 12 Nov 2026:** accept
the classic-cross-reference/direct-catalog subset and reject unsupported layouts.
General PDF-format compatibility is outside JT-62 scope. This resolves the
previously recorded acceptance blocker; no parser dependency or broader format
support is required. The existing compressed-catalog test demonstrates the
required unsupported-layout rejection, not general PDF compatibility. Code
inspection confirms that a nonempty `Validate` reason, including
`UnsupportedLayout`, selects `resume.rejected` in the HTTP handler. No new
compressed-catalog Compose/k3d run is claimed by this documentation update.

### JT62-18: bounded confirmed publisher

The shared `pkg/events.Publisher` has no context-aware construction/cleanup hook.
Its API and implementation are unchanged. The function-local adapter uses
`pkg/events.MarshalEnvelope`, the existing `amqp091-go` client and JT-61's
`forwarder.Dial`; it retains `jobtrack.events`, topic exchange declaration,
persistent delivery, existing schemas and positive publisher confirmations.

A publication has a six-second total budget inside the eight-second handler
budget. Cancellation closes the AMQP connection, interrupting channel setup,
exchange/confirm RPCs, writes and confirmation waiting. Both cancellation and
normal cleanup use a connection-close deadline of 100 ms, instead of waiting
indefinitely for channel/connection close handshakes. This fits below the server's
12 s write timeout and the forwarder's 15 s HTTP timeout. No new retry loop or
queue policy was added.

The handshake test connects to a TCP peer that accepts but never completes AMQP;
a 150 ms caller deadline must return within 1 s and close the socket. Integration
tests put a TCP frame proxy in front of **real RabbitMQ** and withhold each of:
channel-open-ok, exchange-declare-ok, confirm-select-ok, publish basic.ack and
connection-close-ok. Each call must finish within 2 s with a 300 ms caller budget
and bounded cleanup. These are failure-injection tests, not substitutes for the
real deployment checks below.

Unconfirmed publications return an error and therefore HTTP 503; positive broker
confirmation permits success even if cleanup's close-ok is withheld. Lost-confirm
and retry observations carry the same result event ID. The handler's deterministic
UUIDv5 IDs and Resume's existing transactional consumer deduplication remain
unchanged, so uncertain transport success cannot apply a duplicate transition.
Exactly-once transport is not claimed. Generic publisher failures do not expose
broker URLs or credentials.

### New automated commands/results

```sh
gofmt -w functions/resume-validator
GOCACHE=/tmp/jobtrack-jt62-go-cache go test ./functions/resume-validator/internal/validator -run TestValidate -count=1
GOCACHE=/tmp/jobtrack-jt62-go-cache go test -count=1 -race -tags=integration ./functions/resume-validator/... ./tests/contract/...
GOCACHE=/tmp/jobtrack-jt62-go-cache go test ./...
GOCACHE=/tmp/jobtrack-jt62-go-cache go test -tags=integration ./...
GOCACHE=/tmp/jobtrack-jt62-go-cache go vet ./...
git diff --check
```

The corrected structural tests, targeted race/integration suite, full unit suite,
full integration suite and repository vet completed successfully. The targeted
validator package reported `ok ... 11.997s`. Logs for this follow-up are in
`/tmp/jobtrack-jt62-review-fixes-tests.log`,
`/tmp/jobtrack-jt62-review-fixes-unit.log` and
`/tmp/jobtrack-jt62-review-fixes-integration.log`. Infrastructure-dependent checks
used approved local Docker/listener access. The cross-compiled verifier also
printed a sandbox Go module stat-cache warning but returned exit 0; its built
binary was then executed successfully in both deployment environments.

### Updated real Compose verification

```sh
docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build resume-validator
GOCACHE=/tmp/jobtrack-jt62-go-cache CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/jobtrack-jt62-review-verify ./functions/resume-validator/cmd/verify
docker compose -f deploy/compose/docker-compose.yml --profile functions run --rm --no-deps -v /tmp/jobtrack-jt62-review-verify:/jt62-verify:ro --entrypoint /jt62-verify resume-validator
docker compose -f deploy/compose/docker-compose.yml --profile functions logs --tail 8 resume-validator resume-validator-forwarder
docker compose -f deploy/compose/docker-compose.yml exec -T rabbitmq rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers
```

The verifier now uploads three files through the real Resume public RPC, observes
source/result broker events and polls the real final statuses:

| Case | Source event_id | Result event_id | Resume ID | Final status |
|---|---|---|---|---|
| Valid classic PDF | `7977da18-43af-4bf9-97a2-ae885a339525` | `4de6b6cc-1386-545f-bea5-a58e2c5aef4b` | `78bb1b98-aee1-4512-a937-c6192f207b5f` | VALIDATED |
| Prefix spoof | `ce0d910f-ca5b-4c24-a037-9d9e1b6c6d2b` | `06cb7c7e-74f2-5931-911f-f63618bc15d7` | `85c81441-e825-4255-9d59-56a8eebd7ee1` | REJECTED |
| Bare xref | `52164a56-7c12-4ea5-8869-ca4f4b67fb7a` | `2d68793b-4e1b-57ac-9984-2eb53a094d23` | `2860f4ae-888e-46a4-93fc-b86939a86d0a` | REJECTED |

All three source IDs match actual validator publication and forwarder delivery
logs at 14:34:07/08/09 UTC (20:34:07/08/09 Bhutan). Plain text/plain upload again
returned INVALID_ARGUMENT. Both source/result queues had zero ready and
unacknowledged messages, one consumer each, and their dead queues were empty.

### Updated real k3d verification

The existing persistent application namespace and Secrets were reused. No broker,
queue, PVC, database record or existing Job was deleted or reset. The updated
validator was imported and patched by immutable digest:
`sha256:10c4db96010e6588c1361b4d9dff92f6f93741edcfff07ab415f8d7b9965ca26`.
Its ready Knative revision is `resume-validator-00006`.

Actual commands included:

```sh
cp /tmp/jobtrack-jt62-review-verify /tmp/jobtrack-jt62-image/verify
docker build -t jobtrack-jt62-verify:review-fixes /tmp/jobtrack-jt62-image
k3d image import jobtrack-resume-validator:local jobtrack-jt62-verify:review-fixes -c jobtrack-jt29
python3 /tmp/jobtrack-jt62-review-deploy.py
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification wait --for=condition=complete job/jt62-verify-review-fixes --timeout=45s
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs job/jt62-verify-review-fixes
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification exec deployment/rabbitmq -- rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers
```

The local deploy helper registered the imported digest in node containerd,
JSON-patched only the Knative container image, waited for the actual Service to
be Ready and created the verifier Job with the existing Secret references. The
verifier image retains the earlier Alpine/10001 layout. The existing
namespace-qualified forwarder URL and retry/queue settings were unchanged.

The first updated Job completed successfully:

| Case | Source event_id | Result event_id | Resume ID | Final status |
|---|---|---|---|---|
| Valid classic PDF | `fcbbe2e3-9c4d-476a-896a-5fde3e697085` | `057193bd-5b57-538e-a7c9-cb130fee050a` | `e7ac6bfe-3c7a-49b0-bc0c-7ccbe1267ae9` | VALIDATED |
| Prefix spoof | `bd32481f-a705-4ee8-8983-f21b64240598` | `5ab30a15-e4ab-570b-94ef-ae64078a2da8` | `9f4b9bec-6c22-489e-ab4d-248bb817c6a1` | REJECTED |
| Bare xref | `a98929a0-0419-4705-9883-1ad494bf17e8` | `a1d7bfd4-6c97-53c1-8e65-463e0041d69e` | `9c46dc56-619c-4712-9ab3-f8753da2957d` | REJECTED |

The broker-observer IDs matched forwarder delivery logs at 14:35:16/17/18 UTC.
By the time function logs were requested, Knative had scaled the pod to zero;
that log command returned `No resources found`. Consequently a second fresh Job,
`jt62-verify-review-evidence`, reused the existing verifier Pod spec and collected
function logs immediately after completion. It did not replay any queued event.

```sh
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification wait --for=condition=complete job/jt62-verify-review-evidence --timeout=45s
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs job/jt62-verify-review-evidence
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs -l serving.knative.dev/revision=resume-validator-00006 -c user-container --tail=8
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs deployment/resume-validator-forwarder --tail=6
```

| Case | Source event_id | Result event_id | Resume ID | Final status |
|---|---|---|---|---|
| Valid classic PDF | `4b44efd0-2afc-4132-94a4-d164864a02fd` | `a75125b5-7e55-5824-924b-30e4397ed675` | `a3346bd3-3ac1-4e30-81ae-0764f7695054` | VALIDATED |
| Prefix spoof | `3b9517f5-8ff7-479c-8b2f-64aba0cf7c79` | `7b266dda-26a2-5d3a-8e17-e14b06b9f7d1` | `412f6795-ba6f-43e9-b263-31779d81c65b` | REJECTED |
| Bare xref | `3667b4bf-8867-4b88-a421-cd7c9034eec4` | `a969ac0d-1bab-5b81-bc1d-d0b7b7c054e5` | `0f3c25a3-c158-4a25-b85a-802d644eb97b` | REJECTED |

These three IDs match all of the broker observer, actual validator publication
logs and forwarder `event delivered` logs at 14:37:19/19/20 UTC (20:37:19/19/20
Bhutan). Both updated Jobs again observed synchronous INVALID_ARGUMENT for plain
non-PDF upload. The real validator was activated through Knative, not replaced
by a test receiver.

Before and after the first updated Job, both source/result queues had 0 ready,
0 unacknowledged and 1 consumer. `resume.validation.dead` was empty and
`resume-validator.events.dead` contained exactly the same one diagnostic message
recorded earlier. No operation in this follow-up consumed, deleted or replayed
that message. The later fresh evidence Job also makes no DLQ calls.

Follow-up outcome: malformed-xref rejection and bounded confirmed publishing retain
the verification evidence above. The approved classic-xref/direct-catalog scope
resolves the previously pending layout decision, and unsupported-layout rejection
is required behavior. Remote CI/PR/merge/Jira workflow remains unperformed. This
scope reconciliation changed documentation only and did not rerun tests, modify
implementation, or alter the preserved diagnostic dead letter.

## Catalog lexical fix: subsequent execution on 12 November 2026

This follow-up fixes the raw-byte catalog-signature bypass within the existing
Product Owner-approved classic-xref/direct-catalog subset. It does not add a
PDF-format resolver, stream decoder, dependency, contract or shared API. All
historical test/deployment evidence, event IDs and diagnostic dead-letter details
above are preserved.

### Reproduction and change

Before the fix, this new test command failed for literal-string, comment,
unrelated-entry and nested-dictionary spoofs (`expected: false, actual: true`):

```sh
GOCACHE=/tmp/jobtrack-jt62-go-cache go test ./functions/resume-validator/internal/validator -run TestValidate_CatalogRequiresActualTypeEntry -count=1
```

The literal-string counterexample is
`<< /Bogus (/Type /Catalog) /Pages 2 0 R >>`. Tests rebuild the file's xref offsets
after substituting the root dictionary, so a lexical failure cannot be confused
with a damaged cross-reference table.

The root catalog now requires its own top-level `/Type` dictionary key and an
exact PDF name value `/Catalog`. A function-local lexical checker skips unrelated
values, respects PDF whitespace/delimiters, comments, literal strings with nested
parentheses and escapes, hex strings, arrays and nested dictionaries. PDF name
`#xx` escapes are decoded before exact comparison, including duplicate-key checks.
String contents, nested entries, and a `/Type` name used as another entry's value
cannot supply the root Type. Incorrect or indirect Type values, name prefixes,
malformed escapes, decoded NUL names, duplicate keys, unclosed values, trailing
dictionaries, unknown value keywords and nesting beyond 32 levels fail closed.

The root object is scanned through its dictionary and the real `endobj` token;
`endobj` inside a literal string or comment no longer truncates it. Non-root
object-reference checks retain their prior limited structural role. This remains
a bounded lexical/structural gate: it does not resolve object references, validate
the full page/object graph, decode streams, or support any additional PDF layout.
The approved rejection of compressed object-stream catalogs is unchanged.

Reference consulted for delimiter and PDF name-escape behavior:
[PDFium parser utilities](https://github.com/chromium/pdfium/blob/main/core/fpdfapi/parser/fpdf_parser_utility.cpp).
No PDFium code or dependency was introduced.

### Regression coverage

- Negative catalog cases: signatures in literal strings, encoded hex strings,
  comments, nested dictionaries/arrays and unrelated entries; wrong actual Type;
  Type supplied as a string, hex string, reference or dictionary; name prefixes
  and escaped suffixes; duplicate raw/decoded Type keys; malformed lexical input.
- Positive catalog cases: ordinary direct catalog; delimiter-separated names;
  escaped key/value names; comments between tokens; unrelated literal/hex values,
  arrays and nested dictionaries; literal-string escapes and line continuation;
  `endobj` text inside strings/comments. Excessive nesting is rejected.
- The HTTP handler regression asserts string-spoof input publishes
  `resume.rejected` and retains the same deterministic result ID on retries.
- The existing integration test now sends valid direct-catalog, string-spoof,
  comment-spoof and nested-dictionary-spoof events through **real disposable
  RabbitMQ -> Go forwarder -> actual validator HTTP handler -> JT-44-protected
  gRPC test server**, then observes confirmed validated/rejected output events.
  Rejected cases assert the correct resume ID and catalog rejection reason.
- Existing bare-xref, malformed/spoofed PDF, exact size-boundary, event-ID,
  internal-authentication and stalled-publisher tests remain in the executed suites.

### Actual verification

```sh
gofmt -w functions/resume-validator/internal/validator
GOCACHE=/tmp/jobtrack-jt62-go-cache go test -count=1 ./functions/resume-validator/... ./tests/contract/...
GOCACHE=/tmp/jobtrack-jt62-go-cache go test ./functions/resume-validator/internal/validator -run 'TestValidate|TestHandler' -count=1
GOCACHE=/tmp/jobtrack-jt62-go-cache go test -count=1 -tags=integration ./functions/resume-validator/... ./pkg/forwarder ./tests/contract/...
GOCACHE=/tmp/jobtrack-jt62-go-cache go test -count=1 -race -tags=integration ./functions/resume-validator/...
GOCACHE=/tmp/jobtrack-jt62-go-cache go vet ./...
git diff --check
```

The new catalog regressions changed from failing before the fix to passing after
it. Validator/contract unit tests passed; the additional final catalog/handler
run passed in 0.442 s. The integration run passed (validator 12.248 s, forwarder
57.638 s, contracts 0.440 s). The validator integration/race run passed in
12.134 s, and repository-wide vet completed with no diagnostics. Saved outputs:
`/tmp/jobtrack-jt62-catalog-unit.log`,
`/tmp/jobtrack-jt62-catalog-integration.log`,
`/tmp/jobtrack-jt62-catalog-race.log` and
`/tmp/jobtrack-jt62-catalog-vet.log`. Test-listener/Docker checks used approved
local access. These are actual new executions, not inferred results from earlier
runs; earlier full-repository unit/integration results remain historical.

Only disposable Testcontainers infrastructure was created and cleaned up by the
test suites. No existing Compose/k3d workloads, images, Jobs, Secrets, queues,
PVCs, database records or dead letters were changed or replayed. In particular,
the recorded diagnostic event `7c0be053-6f92-45d9-9754-d82312b20381` remains
untouched by this follow-up. Its current live queue state was not re-queried.

The genuine Compose/k3d runs recorded earlier predate this lexical fix. They were
not rerun or updated because this request forbids infrastructure changes; the new
isolated integration test covers the changed acceptance/rejection path. No new
real Resume status transition, deployed-image verification or remote CI result is
claimed for this follow-up.


## Final-image end-to-end verification after the catalog lexer fix

Executed 12 November 2026. This section supersedes the earlier deployed-image
verification gap recorded at the end of the lexical-fix follow-up. The repository
branch was verified as `JT-62-resume-validator`; current AGENTS.md and Jira JT-62
were read. The approved classic-xref/direct-catalog subset and required rejection
of unsupported layouts remain unchanged. The user reported that the latest full
repository unit and integration suites passed after the lexer fix; they were
not rerun in this deployment-only verification. No remote CI result is claimed.

### Safety inspection before redeployment

Inspected existing Pods, Deployments, Jobs, Knative Service, PVCs and queue
counts before making changes. Namespace `jt62-verification` already had the real
Resume, RabbitMQ, PostgreSQL and Go forwarder available; the validator was Ready
at revision 00006 and scaled to zero. Retained PVCs were Bound:

- `postgres-data`: volume `pvc-ee84c3ba-1021-4540-8eff-72ece3121c48`, 1 GiB.
- `rabbitmq-data`: volume `pvc-307d940c-fc8a-40d4-8331-bc8d40d1d690`, 1 GiB.

The namespace also had a pre-existing running legacy verifier Job and old
terminating PostgreSQL Pods. They were left untouched; they did not block the
new verification. No credential-generating setup script was run. Before/after
hash comparisons verified unchanged Secret data/UIDs, PVC identities/specs and
the `resume`, `postgres`, `rabbitmq` and `resume-validator-forwarder` Deployment
specs. Secret values were never printed. Compose redeployment checked that its
validator token and broker configuration matched the retained Resume container.

Both environments started with empty source/result queues, zero unacknowledged
messages and one consumer per source/result queue. Compose dead queues were
empty. In k3d, `resume-validator.events.dead` contained the recorded one message
and `resume.validation.dead` was empty. All four durable queues were quorum queues.
No queue was purged, read for replay, deleted or migrated.

### Versions and exact final artifacts

| Component | Actual observed identity |
|---|---|
| Docker client/server | 29.9.2 / 29.9.2 |
| Go verifier compiler | go1.27.3 darwin/arm64, cross-compiled linux/arm64 with CGO_ENABLED=0 |
| Validator builder/runtime | golang:1.27.3-alpine / alpine:3.22 |
| k3d | v5.9.0 |
| Kubernetes server/client | v1.35.5+k3s1 / v1.36.1 |
| Knative Serving | 1.24.0, verified from controller labels |
| RabbitMQ Compose / k3d | 4.4.0 / 4.1.8 |
| Final validator local tag | `jobtrack-resume-validator:jt62-final` |
| Validator OCI index / Docker-reported image ID | `sha256:666d7ff94252f24b01d8ce425e78193343f0d0a7abddc5480928b3e6a3f890e9` |
| Validator linux/arm64 manifest (build output) | `sha256:87fe214160f993ccbc88b0f8bfb0a4da753dcc81cb81985643b64b5613bb6629` |
| Validator binary SHA-256, measured in BOTH runtimes | `09b4f94a2db367ff26409fdef39e6cd28fe7e954fabe38bf21e1937cd38ccb56` |
| Verifier container index | `sha256:261aef9b2098b0ea7f2e04b5d656ff2cd603c950419958c46e7e25b301687b05` |
| Verifier executable SHA-256 | `d6c6ab530e98c81681a3101a1d1d8b14e4ff4b2ad5b1013b422e78ca8fe0be1e` |
| Temporary verifier source SHA-256 | `49aeeb8a923ed33e72312c8cbbb99bb07346f10769ae7f5e5e30d9d38c3b71c0` |

Resolved base indexes in the build output were
`golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18`
and `alpine:3.22@sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f`.
The unchanged Compose forwarder image ID was
`sha256:0353f9484955baa718bf9b63c651261eac88db8574020582f929ec442b22c3b5`;
k3d reported its forwarder container image ID as
`sha256:a0f948a3ecb706e0ae07e5c7c4d370ab08c24643d191fc9b401986fdc2fbf68c`.
Those Docker/CRI identifiers are recorded as reported, not claimed to be the
same kind of OCI digest. The final validator binary hash is identical in both
runtimes. Images remained local; no registry push was performed.

### Harness and actual commands

The existing `cmd/verify` harness was compiled from current repository source.
A **temporary fixture-only Go build overlay** added two input cases without
editing the checked-in harness or application code: the catalog-string spoof
from the regression test and `testpdf.CompressedCatalog()`. The existing valid,
prefix-spoof and bare-xref inputs, real public UploadResume/GetResume calls,
exclusive broker observation queue and plain text/plain rejection check were
retained. The overlay changed no server/library code and did not publish domain
or outcome events itself.

The catalog-spoof fixture uses
`<< /Bogus (/Type /Catalog) /Pages 2 0 R >>`; its five object offsets, xref and
trailer are rebuilt exactly as in the regression test helper. Thus rejection
exercises the root lexical check rather than intentionally damaged offsets.
The compressed-catalog fixture uses the existing approved unsupported-layout
specimen. The local source overlay and mapping are
`/tmp/jobtrack-jt62-final-verifier.go` and
`/tmp/jobtrack-jt62-final-verifier-overlay.json`. The mapping replaces only the
verifier's `cmd/verify/main.go` during compilation. These are temporary test
artifacts, not a new repository tool or a substitute HTTP receiver.

Actual image/harness build and Compose commands:

```sh
docker build -f functions/resume-validator/Dockerfile -t jobtrack-resume-validator:jt62-final .
GOCACHE=/tmp/jobtrack-jt62-go-cache CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -overlay=/tmp/jobtrack-jt62-final-verifier-overlay.json -o /tmp/jobtrack-jt62-final-verify ./functions/resume-validator/cmd/verify
python3 /tmp/jobtrack-jt62-final-safety.py before
python3 /tmp/jobtrack-jt62-final-compose.py
RESUME_VALIDATOR_IMAGE=jobtrack-resume-validator:jt62-final docker compose -f deploy/compose/docker-compose.yml --profile functions run --rm --no-deps -v /tmp/jobtrack-jt62-final-verify:/jt62-final-verify:ro --entrypoint /jt62-final-verify resume-validator
docker compose -f deploy/compose/docker-compose.yml --profile functions logs --tail 10 resume-validator resume-validator-forwarder
docker exec jobtrack-resume-validator-1 sha256sum /usr/local/bin/resume-validator
python3 /tmp/jobtrack-jt62-final-resume-evidence.py compose
```

The local Compose helper inspected rendered configuration in memory, compared
retained credentials without printing them, and ran only:
`RESUME_VALIDATOR_IMAGE=jobtrack-resume-validator:jt62-final docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --no-deps --no-build resume-validator`.
Resume, broker, database and forwarder containers were not recreated.

The verifier container was built from Alpine 3.22 with the final verifier binary
copied to `/verify`, entrypoint `/verify`, user 10001. Its build context contained
only that binary and Dockerfile. Actual k3d deployment/check commands:

```sh
docker build -t jobtrack-jt62-verify:final /tmp/jobtrack-jt62-final-verifier-image
k3d image import jobtrack-resume-validator:jt62-final jobtrack-jt62-verify:final -c jobtrack-jt29
python3 /tmp/jobtrack-jt62-final-k3d.py
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification wait --for=condition=complete job/jt62-verify-final-lexer --timeout=45s
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs job/jt62-verify-final-lexer
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs -l serving.knative.dev/service=resume-validator -c user-container --tail=12
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification logs deployment/resume-validator-forwarder --tail=8
KUBECONFIG=/tmp/jobtrack-jt62-kubeconfig kubectl -n jt62-verification exec -c user-container deployment/resume-validator-00007-deployment -- sha256sum /usr/local/bin/resume-validator
python3 /tmp/jobtrack-jt62-final-resume-evidence.py k3d
python3 /tmp/jobtrack-jt62-final-safety.py after
```

The local k3d helper registered the newly imported image's digest reference in
node containerd. It used JSON Patch with a `test` of the inspected old image
before replacing **only** `/spec/template/spec/containers/0/image`; a concurrent
change would have stopped the patch. It did not apply namespace/bootstrap
manifests, regenerate Secrets or overwrite dependency resources. The Service
became Ready at **resume-validator-00007**, with the final index digest above.
The actual Pod image ID matched that digest. The namespace-qualified forwarder
sink remained `http://resume-validator.$(POD_NAMESPACE).svc.cluster.local`.

The new Job `jt62-verify-final-lexer` was created without overwriting old Jobs,
using the prior verifier Pod spec and retained Secret references, changing only
the verifier image. It had backoffLimit 0 and restartPolicy Never. Readiness was
confirmed before uploads; the Job completed successfully. Validator logs and
binary identity were captured immediately, before scale-to-zero could remove the
Pod. The original setup scripts were never rerun.

### Actual source/outcome IDs and Resume results

Each row was observed on the actual broker through the harness, matched in
forwarder `event delivered` and validator `resume validation published` logs,
and confirmed by real Resume GetResume returning the final status. Read-only
queries against Resume's own persistence additionally confirmed source outbox
publication and durable outcome processing for every row.

#### Final Compose image

| Case | Source event_id | Result event_id | Resume ID | Final status |
|---|---|---|---|---|
| valid | `f828f32f-4d78-4e4a-873c-b49989fe3b2d` | `5f94ba57-0e25-5987-ad08-567dd21b1d30` | `2bffa749-1c94-4d53-a528-20019d8ecdc7` | VALIDATED |
| spoof | `7d63bae7-bbb0-4bf2-84dc-0d534c458ee1` | `ab8a3b44-ccd9-548b-8027-2c7d473a3fe2` | `72006168-247e-4527-b422-e48ebf40d07d` | REJECTED |
| bare-xref | `1c8bdf16-fdd2-4a45-8bae-7d2a48ee0cde` | `a0490b14-cd6f-58ce-a103-d1526c06cd98` | `b8a3fdbf-1b1f-4c9b-abce-bf20cfd7c8ae` | REJECTED |
| catalog-string-spoof | `e577fca6-ce46-480a-9bec-ce6422a69a11` | `c9aace9d-e12d-518e-973e-473ee78d20d5` | `934b6c4a-aa2f-4458-a6b7-9b73b1139652` | REJECTED |
| unsupported-compressed-catalog | `283d23e6-228b-4250-8ebd-0c7d46986b16` | `410bd662-75b2-5108-a78a-b07ae7c4c422` | `90a16f3b-2fd0-477d-8c34-24ac11f3317f` | REJECTED |

#### Final k3d Knative image

| Case | Source event_id | Result event_id | Resume ID | Final status |
|---|---|---|---|---|
| valid | `bf59065a-7186-43da-a94d-bbdce79b5be7` | `76a17744-a279-5dfa-ba48-e124471313fa` | `9c073881-1adc-4475-aa9e-784c1f0fe5db` | VALIDATED |
| spoof | `bdc1cffa-08df-4446-8685-577313401908` | `35aa022a-f1e6-507b-bf38-80618a0950b7` | `213baeff-f457-478a-bc39-a0771cab69ba` | REJECTED |
| bare-xref | `4d5508f4-eab5-4d39-ad0a-935c003f5543` | `133e5558-e20d-5ca3-b29c-baf36a941941` | `4ed1d0ce-343a-4be6-a4ac-2e4c9e0b8877` | REJECTED |
| catalog-string-spoof | `7b039c88-b832-4421-8700-c3f7679766ff` | `c3be3b4d-4014-5a2f-a05c-6aa821b15444` | `ea3d327f-7e66-4760-aab6-0927ea3aceb0` | REJECTED |
| unsupported-compressed-catalog | `1428f369-474e-4c3c-907d-969a64b921b6` | `e3d95ad7-8120-5ab0-b00d-b3f3671b6894` | `7b713de6-3f61-4fb3-abb3-5f1ae943f0b4` | REJECTED |

Compose's five validator publications and forwarder deliveries occurred at
15:20:03..07 UTC (21:20:03..07 Bhutan); k3d's occurred at 15:22:14..18 UTC
(21:22:14..18 Bhutan). All ten result IDs independently match UUIDv5 of
`resume-validator:<source event_id>` in the OID namespace.

Both harness executions also returned **INVALID_ARGUMENT** for the plain
`text/plain` upload. There were exactly five new stored resumes per environment,
with no sixth plain non-PDF record. The stored rejection reasons, in case order,
were `invalid PDF end signature`, `invalid PDF trailer signature`,
`invalid PDF catalog signature`, and the documented `unsupported PDF layout: ...`
reason. The valid case had an empty rejection reason.

For all ten stored-file cases, read-only SQL correlated the explicit
`(source_id, outcome_id, resume_id)` values above with:

```sql
SELECT c.source_id, c.outcome_id, r.resume_id, r.validation_status,
       r.rejection_reason,
       (o.published_at IS NOT NULL) AS source_published,
       (p.event_id IS NOT NULL) AS outcome_processed
FROM checks c
JOIN resumes r USING (resume_id)
LEFT JOIN outgoing_events o ON o.event_id = c.source_id
LEFT JOIN processed_events p ON p.event_id = c.outcome_id;
```

`checks` was a VALUES CTE containing only the five verified UUID triples per
environment. Both booleans were **true for every row**, and stored statuses
matched the public RPC results. No SQL update, insertion, migration or direct
validation-state change was performed; real Resume consumers applied the events.

Local evidence files:

- `/tmp/jobtrack-jt62-final-compose-results.log` and
  `/tmp/jobtrack-jt62-final-compose-delivery.log`.
- `/tmp/jobtrack-jt62-final-k3d-results.log`,
  `/tmp/jobtrack-jt62-final-k3d-validator.log` and
  `/tmp/jobtrack-jt62-final-k3d-forwarder.log`.
- `/tmp/jobtrack-jt62-final-compose-resume-evidence.log` and
  `/tmp/jobtrack-jt62-final-k3d-resume-evidence.log`.

### Retained data and final acceptance status

Before/after metadata comparison confirmed that **all 7 pre-existing Compose
resumes and all 9 pre-existing k3d resumes retained their exact IDs, statuses
and updated_at values**. Five new synthetic verification records were added in
each environment through UploadResume. Existing credentials, both PVC bindings
and all dependency Deployment specifications remained unchanged.

After the checks, source/result queues in both environments again had 0 ready,
0 unacknowledged and 1 consumer each. Compose dead queues remained empty;
k3d's result dead queue remained empty and its source dead queue still held
exactly **one** diagnostic message. No command read, purged, removed or replayed
that dead queue. The recorded diagnostic source event
`7c0be053-6f92-45d9-9754-d82312b20381` and its pre-existing PENDING Resume metadata
were not altered. The old Jobs and infrastructure records were retained.

All 996 repository file hashes were identical before documentation recording;
a final comparison allows only this test document to differ. Application source,
contracts, dependencies and branch/history were not changed by verification.
`git diff --check` passed after the evidence update.

**Implementation/end-to-end acceptance: PASS; READY TO COMMIT.** The current
lexer-containing image is proven in both required real delivery environments,
including the catalog-string regression and required unsupported-layout rejection.
No implementation or final-image verification blocker remains. The deliberately
retained diagnostic dead letter is documented test history, not a successful
fresh-delivery claim. PR creation, remote CI, merge and Jira/time-logging workflow
remain pending and were not performed; this is pre-commit readiness, not a claim
that the project's full Definition of Done or merge workflow is complete.
