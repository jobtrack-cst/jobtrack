# JT-107: PDF cross-reference compatibility regression tests

19 November 2026, branch `JT-107-fix-pdf-xref-streams`.
Initial working tree was clean, with no conflicts. No branch changes, stash,
commit, push, PR creation, Jira updates or other-service changes were performed.

## Root cause and scope

UAT-AP-07 reported a 490,694-byte PDF 1.7 containing `/Type /XRef` and two
`%%EOF` markers rejected with `invalid PDF cross-reference count`. The checked-out
source is explicitly limited to a standalone classic xref table and a direct
catalog. It rejects xref streams and `/Prev` or `/XRefStm` layouts as unsupported.
There is no count-of-EOF check to remove in this revision. The reported deployed
reason cannot be reproduced exactly without its input and deployed image; check
that the rebuilt validator image is running when repeating UAT.

JT-107 follows the final `startxref`, parses classic sections or xref streams,
merges `/Prev` revisions newest first, and resolves `/XRefStm` hybrid entries and
compressed catalog objects. It retains the original catalog lexer and the upload
MIME/size/header/end-signature gates. Only the validator, its synthetic fixture
builder, verifier, tests and documentation change. No dependencies or contracts
change. The relevant PDF definitions are in the PDF Association's
[ISO 32000 syntax errata](https://pdf-issues.pdfa.org/32000-2-2020/clause07.html),
sections 7.5.6 and 7.5.8.

## Cases traced to JT-107

All fixtures are generated synthetic one-page documents. No personal files are
stored or read. Tests use the existing external-package, table-driven testify
conventions.

| Coverage | Test |
|---|---|
| Classic xref, unfiltered xref streams, Flate streams, PNG Up prediction, compressed catalog, hybrid, classic/stream/mixed incremental updates and three revisions | `TestValidate_CrossReferenceLayouts` |
| Exactly 490,694 bytes, PDF 1.7, `/Type /XRef`, two EOFs; representative shape of UAT-AP-07, not its actual document | `TestValidate_AcceptsUATAP07SyntheticShape` |
| Missing/truncated stream or object terminators, wrong lengths, overflow, bad `/W`, `/Index` overlap/odd count, bad `/Size`, wrong root, duplicate keys, spoofed XRef type, broken previous revision, self-loop, bad object streams, unsupported filters and external streams | `TestValidate_RejectsMalformedCrossReferences` |
| Decode bomb and excessive revision chain | `TestValidate_RejectsUnboundedMetadata` |
| Latest revision frees catalog: reject without falling back to an old valid catalog | `TestValidate_RejectsFreedCatalogWithoutFallingBack` |
| Empty, one byte, below/at/above 2,000,000 bytes, wrong MIME, invalid version/header, prefix spoof, appended executable, bad classic counts/offsets/trailer and bare xref | Existing `TestValidate_SignaturesAndBoundaries` |
| Catalog Type spoofing in strings/comments/nested values, escaped names, malformed dictionaries and depth limits | Existing `TestValidate_CatalogRequiresActualTypeEntry` |
| Compressed catalog is now accepted with structural resolution | `TestValidate_AcceptsCompressedCatalog` (replaces JT-62's unsupported-layout expectation) |
| HTTP outcome and deterministic result IDs for stream/incremental uploads; existing fetch, publish, ownership and event validation behavior | `TestHandler_ContractAndFailures`, existing malformed-event tests |
| Real RabbitMQ -> existing forwarder -> HTTP validator -> protected Resume internal RPC via bufconn -> confirmed broker result, including supported layouts and truncated rejection | `TestValidator_IntegrationBrokerForwarderInternalRPCAndOutcome` |
| Parser mutation/panic smoke check | `FuzzValidate_PDFMetadata` |

The integration test uses disposable RabbitMQ Testcontainers and the real shared
internal authentication interceptors. Resume RPC data is synthetic and served in
process; it does not substitute for a full Compose/UI UAT run.

## Execution evidence

Commands ran from the repository root with
`GOCACHE=/tmp/jobtrack-jt107-go-cache`, Go 1.27.3 on darwin/arm64.
Docker Desktop 29.9.2 was reachable. Socket-based tests and Docker integration
required execution outside the filesystem/network sandbox; no source permissions
or application security settings were changed.

| Command (after the GOCACHE prefix) | Actual result |
|---|---|
| `go test -count=1 ./functions/resume-validator/internal/validator -run 'TestValidate|TestHandler'` | PASS, 0.474 s on final source/fixtures |
| `go test ./...` | PASS across all repository packages; unchanged package results may be cached |
| `go vet ./...` | PASS, exit 0, no diagnostics |
| `go test -count=1 -tags=integration ./functions/resume-validator/... ./tests/contract/...` | PASS; validator 11.499 s, contracts 0.417 s; real disposable RabbitMQ containers |
| `go test -count=1 -race -tags=integration ./functions/resume-validator/... ./tests/contract/...` | INTERMITTENT FAILURE in the unchanged publisher cleanup timing test; one isolated rerun passed (validator 13.836 s), but the final full race run failed again |
| `go test -count=1 -race -tags=integration ./functions/resume-validator/internal/validator -run '^TestValidator_IntegrationBrokerForwarderInternalRPCAndOutcome$'` | PASS, 5.908 s |
| `go test ./functions/resume-validator/internal/validator -run '^$' -fuzz FuzzValidate_PDFMetadata -fuzztime=20s -parallel=2` | PASS, 412,604 executions on final source, no failing corpus; initial run also passed 472,584 executions |
| `git diff --check` | PASS |

The full race integration failure was
`TestBrokerPublisher_IntegrationBoundsStalledLifecycle/connection_close` at
`publisher_integration_test.go:77`: 5.006885625 s against a less-than-2-second
assertion. A prior run failed at 5.004262875 s. No data race was reported. The
publisher and this timing test were not modified; the cause of the intermittent
cleanup failure is not established here. It remains a verification concern
outside the PDF parsing change. The initial sandbox-only package run also hit
`listen tcp 127.0.0.1:0: bind: operation not permitted`; running the required tests
with socket access resolved that environmental restriction.

Execution logs are temporary local artifacts in `/tmp/jobtrack-jt107-*.log`:
`targeted`, `unit`, `vet`, `integration-standard`, `integration-retry`,
`integration-final`, `path-race`, and `fuzz-final`. Early development runs included
fixture-construction corrections; the table above distinguishes the final
checks and does not claim the entire race-enabled integration suite is green.

## Changed files

- `functions/resume-validator/internal/validator/pdf.go`: preserve upload gates and classic entry validation; return section data for revision resolution.
- `functions/resume-validator/internal/validator/pdf_xref.go`: bounded dictionary, xref-chain, stream and compressed-object resolution.
- `functions/resume-validator/internal/validator/pdf_xref_test.go`: JT-107 regressions, resource bounds and fuzz seeds.
- `functions/resume-validator/internal/validator/validator_test.go`: supported compressed catalog and HTTP outcomes.
- `functions/resume-validator/internal/validator/validator_integration_test.go`: broker/forwarder/internal-RPC outcome coverage.
- `functions/resume-validator/internal/testpdf/pdf.go`: safe generated xref-stream, hybrid and incremental fixtures.
- `functions/resume-validator/cmd/verify/main.go`: eight-case manual deployment verifier.
- `functions/resume-validator/README.md`: supported layouts and limits.
- `docs/testing/jt-107-test-cases.md`: this evidence, limitations and manual commands.

## Manual Docker Compose and UAT verification

Run from the repository root, using the existing private `deploy/compose/.env`.
Do not replace it or print interpolated Compose configuration. These commands
were prepared for manual execution; the existing Compose stack was not rebuilt
or restarted during this change.

```sh
# Rebuild validator and start its existing forwarder and required dependencies.
docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build resume-validator resume-validator-forwarder

# Build the synthetic verifier for the Docker server's architecture.
CGO_ENABLED=0 GOOS=linux GOARCH="$(docker version --format '{{.Server.Arch}}')" \
  go build -o /tmp/jobtrack-jt107-verify ./functions/resume-validator/cmd/verify

# Real Resume uploads, broker outcomes and persisted validation status.
docker compose -f deploy/compose/docker-compose.yml --profile functions run --rm --no-deps \
  -v /tmp/jobtrack-jt107-verify:/jt107-verify:ro \
  --entrypoint /jt107-verify resume-validator

docker compose -f deploy/compose/docker-compose.yml --profile functions logs --tail 30 resume-validator resume-validator-forwarder
```

The verifier creates eight synthetic resume records: classic, xref-stream,
compressed-catalog, incremental and hybrid must become VALIDATED; truncated,
prefix-spoofed and bare-xref must become REJECTED. A separate text/plain upload
must return INVALID_ARGUMENT. The observation queue is temporary; the eight
resume records remain. No queues, volumes or existing data are deleted.

For UAT-AP-07, sign in as a synthetic applicant at `http://localhost:5173`, open
`/applicant/resumes`, upload a safe Word-exported PDF 1.7 with an xref stream and
incremental history (up to 2,000,000 bytes), then refresh until VALIDATED. Repeat
with a traditional classic-xref PDF. A truncated PDF retaining `%PDF-` should
become REJECTED; oversized/wrong-MIME uploads should fail at upload. Historical
REJECTED records do not revalidate automatically: upload a new synthetic record.
Record the rebuilt image, file size, resume ID, result and UAT evidence manually.

## Remaining limitations

This is a bounded structural gate, not full ISO conformance, page rendering or a
malware scan. Metadata supports raw/FlateDecode streams, PNG predictors 10..15
with one 8-bit component and direct stream lengths. Other filters, indirect
metadata lengths and other predictor formats fail closed. Limits are 100,000
object-number slots, 128 primary revisions and 16,000,000 total decoded metadata
bytes; unusually complex valid files can exceed them. The original UAT document,
a real Word export and full Compose/UI validation remain manual verification.

After local tests and manual UAT, the member handles a JT-107 commit, push, PR
and moving the Jira card to In Review. This document does not mark JT-107 Done.
