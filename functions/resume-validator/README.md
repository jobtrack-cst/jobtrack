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

The validator checks exact `application/pdf`, 1..2,000,000 decoded bytes, a PDF
1.0..1.7 or 2.0 header at byte zero, and terminal `startxref`/`%%EOF`. For a
standalone classic xref table it checks subsection counts, entry syntax, in-use
object offsets and object/generation numbers, trailer `/Size` and `/Root`, and
the referenced direct catalog dictionary. A bare `xref` token is insufficient.
Catalog identification requires the root dictionary's actual top-level `/Type`
entry to have the name value `/Catalog`; names are decoded with PDF `#xx`
escapes and compared exactly. Literal/hex strings, comments, nested dictionaries,
arrays and unrelated entries cannot supply that entry. Unrelated values are
skipped lexically; malformed tokens, duplicate decoded keys, missing `endobj`
and nesting beyond 32 levels fail closed. Literal-string escapes and nested
parentheses are handled without interpreting their contents as PDF entries.
This is a bounded structural/signature gate, not a complete PDF parser, rendering
check or malware scanner; it does not validate the full page/object graph.

**Product Owner-approved scope (12 Nov 2026, Jira JT-62):** supported PDFs are
limited to classic cross-reference tables with directly accessible catalog
objects. Cross-reference streams, compressed object-stream catalogs, and hybrid
or incremental layouts requiring `/XRefStm` or `/Prev` resolution are unsupported
and must be rejected. General PDF-format compatibility is outside JT-62 scope.
The implementation fails closed for these layouts and publishes `resume.rejected`;
valid PDFs within the supported subset can become VALIDATED after the checks above.

The old optimistic xref-stream signature branch was removed rather than claiming
structural support without a resolver. No PDF parser dependency was introduced.
The compressed-catalog specimen tests required unsupported-layout rejection; it
is not a positive compatibility test. The investigation and approved scope are
recorded in the test document.

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
applicant identity, uploads a minimal PDF, a prefix-spoofed PDF and the bare-xref
regression through the real public gRPC service, observes source/result events on an exclusive broker queue,
and polls GetResume for VALIDATED/REJECTED. It does not publish synthetic source
or result events and never edits the database. The temporary observation queue is
deleted when its connection closes. The three synthetic resume records remain.
A text/plain upload must return INVALID_ARGUMENT: the existing upload contract
prevents a plain non-PDF from being stored and later transitioning to REJECTED.

Actual evidence and acceptance status: [JT-62 test cases](../../docs/testing/jt-62-test-cases.md).
