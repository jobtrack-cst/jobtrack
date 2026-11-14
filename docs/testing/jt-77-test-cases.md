# JT-77 email-sender test cases

Automated tests: `go test ./functions/email-sender/...`.
Tests use an SMTP port fake and an in-process TCP SMTP server.

| Case | Partition/boundary | Expected |
| --- | --- | --- |
| Valid request | Plain-text multiline body | Delivery called once, 204 |
| SMTP protocol | MAIL/RCPT/DATA and dot-prefixed body | Intended envelope and MIME message, final 250 required |
| SMTP rejection | RCPT rejected | Error from SMTP adapter, handler maps delivery errors to 503 |
| Malformed JSON | Broken/null/empty/unknown/trailing JSON | 400, no delivery |
| Missing/blank values | Empty recipient, subject or body | 400, no delivery |
| Invalid address | Invalid mailbox or recipient list | 400, no delivery |
| Header injection | Recipient CR/LF; subject newline | 400, no delivery |
| Body control | NUL | 400, no delivery |
| Subject bytes | 200 / 201 | 204 / 400 |
| Body bytes | 65,536 / 65,537 | 204 / 400 |
| Request bytes | 131,073 | 413, no delivery |
| Wrong method | GET/PUT | 405 and Allow: POST |
| Wrong media type | text/plain | 415 |
| Sensitive failure | Fake error contains secrets/address | No error details in response or logs |
| Configuration | Missing address, bad ports/sender/injection | Startup configuration rejected |
| Delayed body + delayed SMTP | Scaled read 1 s, body delay 700 ms, DATA reply delay 700 ms | Old 1.2 s write deadline loses response despite acceptance; aligned 2.2 s returns 204 |
| Stalled SMTP after DATA | Earlier caller deadline of 500 ms, no final reply | SMTP error, never successful delivery result; connection closed |
| Rejected DATA | Final SMTP 550 | SMTP error, never successful delivery result |
| Graceful shutdown | Begin shutdown while SMTP acceptance is gated | Shutdown waits; release acceptance yields 204 and clean shutdown |
| Production budgets | Read 10 s + SMTP 8 s + response margin | Write and shutdown budgets at least 22 s |

Timeout regression tests are in `internal/sender/timeouts_test.go`. They run
against localhost listeners, with short controlled delays and channel gates;
all waits and socket operations are bounded. The stalled SMTP test exercises
an earlier context deadline through the same connection deadline/cancellation
path used by the eight-second handler budget. Knative uses 25 s and Compose
stop grace is 25 s; manifest configuration is inspected/validated separately.

Manual review: start the Compose functions profile email-sender service, send
POST from the Compose network and observe 204; inspect the actual message in
Mailpit at localhost:8025. Confirm recipient, subject, body and safe logs. This
check has not been performed by the automated tests. Knative rollout and SMTP
reachability must be checked in the existing cluster.

Prior manual verification on 14 Nov 2026 returned 204 and independently found
exactly one matching synthetic email in Mailpit (message ID
`73vJdIYgMLucmRBskaEsSv`), with matching recipient, subject and plain-text body.
This timeout correction does not repeat live delivery or claim a Knative rollout.
