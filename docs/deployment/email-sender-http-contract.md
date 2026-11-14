# Email sender HTTP contract (JT-77)

Common reference for Notification Dispatcher and Weekly Summary. The request
format has been agreed with Sangay and Jigden, as confirmed by Phuntsho.
Implementation and deployment instructions: [function README](../../functions/email-sender/README.md).

## Endpoint and request

On the Compose network, call **POST http://email-sender:8080/send-email** with
`Content-Type: application/json`. The function has no host-published HTTP port.
In Knative, use the deployed email-sender Service URL with the same `/send-email`
path; the Compose hostname is not the Knative URL.

```http
POST /send-email HTTP/1.1
Host: email-sender:8080
Content-Type: application/json

{"recipient":"student@example.test","subject":"Weekly summary","body":"Synthetic weekly summary.\nSecond line."}
```

All three fields are required strings. Email content is plain text, UTF-8;
HTML is not rendered. SMTP output uses `text/plain; charset=UTF-8` with
quoted-printable body encoding, normalized CRLF line endings and dot escaping.

| Field or payload | Maximum size | Validation |
| --- | --- | --- |
| `recipient` | 254 bytes | One bare ASCII mailbox parsed by Go `net/mail`, containing `@`; no display name, recipient list, surrounding whitespace or control characters, including CR/LF |
| `subject` | 200 UTF-8 bytes | Nonblank; no control characters, including CR/LF |
| `body` | 65536 UTF-8 bytes | Nonblank; CR, LF and tab allowed, other control characters rejected |
| Entire JSON request body | 131072 bytes | Includes JSON syntax, escaping and whitespace; invalid UTF-8, unknown fields and trailing JSON rejected |

Field limits apply to decoded strings, without trimming. Multibyte UTF-8
characters consume more than one byte. Subjects are MIME encoded for SMTP.

## Responses

These are the responses for `/send-email`; error bodies contain fixed safe text,
not SMTP diagnostics. No response JSON schema is required by the callers.

| HTTP status | Meaning |
| --- | --- |
| 204 No Content | SMTP returned final 250 after DATA. Email accepted for delivery; empty HTTP body. This alone does not prove inbox arrival. |
| 400 Bad Request | Invalid JSON, invalid/missing field, invalid UTF-8 or request-body read failure other than size overflow |
| 405 Method Not Allowed | Method is not POST; response includes `Allow: POST` |
| 413 Request Entity Too Large | JSON request body exceeds 131072 bytes |
| 415 Unsupported Media Type | Missing, malformed or non-`application/json` Content-Type; valid media-type parameters are accepted |
| 503 Service Unavailable | SMTP connection, protocol, write or acceptance-reply failure, including timeout; response text is `email delivery unavailable` |

The handler checks method, then media type, then request size/JSON and fields,
before attempting SMTP. Requests rejected during validation do not send email.
Transport disconnects or expired connection deadlines can prevent receipt of
an HTTP response.

## Timeouts and shutdown

| Budget | Value | Starts/applies |
| --- | --- | --- |
| HTTP header read | 5 s | Reading request headers |
| Entire HTTP request read | 10 s | Reading headers and body together; not an extra 10 s after headers |
| SMTP operation | 8 s | After request validation; includes DNS/TCP, greeting, commands, DATA and final reply |
| HTTP write | 22 s | From completion of request headers; allows remaining read time, SMTP and response margin |
| Graceful shutdown | 22 s | On SIGTERM/SIGINT, stop accepting and drain active HTTP requests |
| Knative request timeout | 25 s | Configured in the Serving manifest |
| Compose stop grace | 25 s | Before container forced termination |

The read and SMTP work occur sequentially: **8 s SMTP is not the full HTTP
request duration**. Earlier request cancellation/deadlines can interrupt SMTP;
a socket deadline and cancellation watcher bound its I/O. Callers should allow
the full HTTP budget plus network margin. A 30 s caller timeout is a suggested
integration choice, not an existing caller configuration or a guarantee about
Knative cold-start time. Work still active at the shutdown grace limit may be
interrupted by process exit.

## Retries and delivery uncertainty

Do not retry an unchanged 4xx request; correct the request first. A 503 may be
transient or uncertain, so retry only with awareness of possible duplication.
The function makes one SMTP attempt and leaves retry policy to its callers.

**Retrying 503 is not guaranteed duplicate-free.** SMTP might have accepted
DATA but its final reply was lost or timed out, causing the function to return
503. The HTTP success response can also be lost after acceptance, leaving the
caller uncertain. Retrying either outcome can send the email again. This
stateless API has no idempotency key, persistent deduplication or exactly-once
delivery guarantee. An SMTP failure response does not prove non-delivery.

## Configuration boundary

Email-sender itself requires `SMTP_ADDR` and `SMTP_FROM`. Compose uses
`SMTP_ADDR=mailpit:1025` and defaults `SMTP_FROM=noreply@jobtrack.test`.
`HTTP_PORT` defaults to 8080. `SERVICE_NAME=email-sender` is a deployment
convention; the logger uses this fixed name. Compose also supports
`EMAIL_SENDER_FROM` and `EMAIL_SENDER_IMAGE` overrides. SMTP is unauthenticated
plaintext on the project network; no external email provider or credentials
are configured. No `INTERNAL_TOKEN` is used by this HTTP function.

`EMAIL_SENDER_URL` is only a possible caller-side convention. It is not read
by email-sender and is not an established required setting in this change.
Each caller may choose its own configuration name; if it introduces this
variable, it must document whether the value is a base URL or full endpoint.
The complete Compose endpoint remains `http://email-sender:8080/send-email`
regardless of that choice.
