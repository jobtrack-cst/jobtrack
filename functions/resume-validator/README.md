# Resume validator (JT-62)

Stateless HTTP function consuming **binary CloudEvents 1.0**, as emitted by the
JT-61 Go forwarder: POST `/`, `Content-Type: application/json`, unchanged
`EventEnvelope` body, matching `Ce-Id`/`Ce-Type`, and a nonempty `Ce-Source`.
The producer must be `resume` and the typed payload `ResumeUploaded`.
Malformed events return 400; identity mismatches with the fetched file return 422.
The forwarder owns the unchanged JT-61 retry/dead-letter policy for non-2xx replies.

Configuration: `SERVICE_NAME=resume-validator`, `RESUME_ADDR`, `RABBITMQ_URL`,
`INTERNAL_TOKEN`, optional `HTTP_PORT` (8080). The token is validated at startup;
only the JT-44 internal client interceptor attaches it to GetResumeFile.
No database or local files are used. `/healthz` and `/metrics` report process health.

The validator checks exact `application/pdf`, 1..2,000,000 decoded upload bytes,
a PDF 1.0..1.7 or 2.0 header at byte zero, and terminal `startxref`/`%%EOF`.
JT-107 extends the original JT-62 classic-only scope to classic cross-reference
tables, cross-reference streams, hybrid files and incremental updates. The final
`startxref` selects the current revision; `/Prev` links supply earlier entries,
with newer entries (including deletions) taking precedence. Multiple `%%EOF`
markers are valid. A bare `xref` or `/Type /XRef` token is insufficient.

Classic tables retain subsection, entry, object offset/generation and trailer
checks. Xref streams require a real `/Type /XRef` dictionary, valid `/Size`, `/W`,
optional `/Index`, exact decoded entry length and valid object references.
Metadata streams support unfiltered or FlateDecode data, including PNG predictors
10..15 with one 8-bit component. Compressed catalog objects are resolved through
validated `/ObjStm` headers and type-2 xref entries. Stream lengths and terminators
are checked; unsupported encodings or external streams fail closed.

Catalog identification still requires the root dictionary's actual top-level
`/Type /Catalog` name. The existing lexer handles escaped names, strings,
comments, nested values and duplicate keys without interpreting embedded text
as dictionary entries. Malformed tokens and nesting beyond 32 levels fail closed.
The parser bounds object-number space to 100,000 entries, revision traversal to
128 sections and total decoded metadata to 16,000,000 bytes to prevent unbounded
work. These bounds can reject unusually complex but otherwise valid PDFs.

This remains a structural/signature gate, not a full PDF parser, page renderer
or malware scanner. It does not validate the full page/object graph. Less common
stream filters, multi-component/non-8-bit predictors and indirect metadata-stream
lengths remain unsupported. No parser dependency, contract, transport or service
security changes are introduced. Historical JT-62 unsupported-layout evidence is
retained in its test document; [JT-107 test cases](../../docs/testing/jt-107-test-cases.md)
record the expanded scope and current verification.

Successful validation publishes `resume.validated`; failed format checks publish
`resume.rejected` with a fixed reason. Fetch failures and unconfirmed publishes
return 503. The function-local publisher uses `pkg/events.MarshalEnvelope` and
the existing approved AMQP client, preserving the shared publisher's exchange,
persistent delivery and confirmation conventions. The shared `pkg/events`
Publisher exposes no cancellable setup/close API, so JT-62 does not alter it.

Each publication gets a fresh connection and a six-second total budget, bounded
by the handler's eight-second deadline. `forwarder.Dial` bounds TCP/handshake;
a cancellation watcher closes the connection to interrupt channel creation,
exchange declaration, confirm setup, writes and confirmation waiting. Cleanup
uses `CloseDeadline` with a 100 ms bound, including when the broker withholds
close-ok. Thus the handler remains within its 12 s write/15 s forwarder budgets.
HTTP success requires a positive broker confirmation. Cleanup failure after a
positive confirmation does not undo that fact; uncertain publications return
503 and retry through the unchanged JT-61 policy. There is no local retry loop.

The result ID is UUIDv5 of `resume-validator:<input event_id>` in the OID namespace.
The input occurrence timestamp is retained as the logical event timestamp. Repeated
input therefore produces the same result envelope. Resume's existing transactional
`processed_events` consumer deduplicates its effects, including after a function
restart or an uncertain publisher confirmation/HTTP acknowledgment. The function
can repeat computation/publication; it never claims exactly-once transport.

Build and start from the repository root:

```sh
docker compose -f deploy/compose/docker-compose.yml --profile functions up -d --build resume-validator resume-validator-forwarder
go test ./functions/resume-validator/... ./tests/contract/...
go test -tags=integration ./functions/resume-validator/...
go vet ./functions/resume-validator/...
```

For Knative, create the namespace's `rabbitmq-credentials` Secret (`amqp-url`) and
`resume-validator-credentials` Secret (`internal-token`, matching Resume), and
provide the Resume service at `resume:9090`. Use a persistent application broker,
not the JT-29 disposable spike broker. Set the manifest's image to the deployed
registry digest, then apply `deploy/knative/resume-validator.yaml` and
`deploy/knative/forwarder`. For imported k3d images use the matching local digest;
Knative attempts registry resolution for mutable tags even with IfNotPresent.

`cmd/verify` is a deployment test utility, not an HTTP sink. Run it on the
application network with `RESUME_ADDR` and `RABBITMQ_URL`. It creates a synthetic
applicant identity, uploads classic, xref-stream, compressed-catalog, hybrid and incremental PDFs,
a truncated PDF, a prefix-spoofed PDF and the bare-xref regression through the real public gRPC service, observes source/result events on an exclusive broker queue,
and polls GetResume for VALIDATED/REJECTED. It does not publish synthetic source
or result events and never edits the database. The temporary observation queue is
deleted when its connection closes. The eight synthetic resume records remain.
A text/plain upload must return INVALID_ARGUMENT: the existing upload contract
prevents a plain non-PDF from being stored and later transitioning to REJECTED.

Actual evidence and acceptance status: [JT-62 test cases](../../docs/testing/jt-62-test-cases.md).
