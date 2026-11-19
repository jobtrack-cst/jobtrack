# Jar-92: Per-function verification

Each function is verified once in the k3d cluster. Pattern:
1. Record the initial pod count (0 after scale-to-zero).
2. Trigger the function (event or PingSource).
3. Verify the expected result (DBar row, email, log line, downstream event).
4. Record pod count after request, then again after 60-120s to observe scale-to-zero.

## 1. email-sender

**Trigger:** Direct HTTP JSON POST to the Service URL.

```bash
SINK=$(kubectl get ksvc email-sender -o jsonpath='{.status.url}')
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$SINK/send-email" \
  -H "Content-Type: application/json" \
  -d '{"recipient":"verify@example.test","subject":"JT-92 verify","body":"Hello"}'
```

**Expected:** `204` (accepted by Mailpit SMTP).

**Evidence:** Mailpit at `http://<cluster-host>:8025` shows a new email to `verify@example.test`.

## 2. resume-validator

**Trigger:** Publish `resume.uploaded` event to RabbitMQ.

```bash
RABBIT_POD=$(kubectl get pod -l app=rabbitmq -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it "$RABBIT_POD" -- rabbitmqadmin publish exchange=jobtrack.events routing_key=resume.uploaded \
  payload='{"event_id":"11111111-1111-4111-8111-111111111111","type":"resume.uploaded","producer":"resume","occurred_at":"2026-11-19T12:00:00Z","data":{"resume_id":"22222222-2222-4222-8222-222222222222","applicant_id":"33333333-3333-4333-8333-333333333333"}}'
```

**Expected:** `resume-validator` logs `processing event event_type=resume.uploaded`. A `resume.validated` or `resume.rejected` event published.

**Evidence:** `kubectl logs deployment/resume-validator-forwarder`. Logs from `resume-validator` pod.

## 3. deadline-reminder

**Trigger:** Manual HTTP POST to the Service URL.

```bash
SINK=$(kubectl get ksvc deadline-reminder -o jsonpath='{.status.url}')
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$SINK/" -d ''
```

**Expected:** `204`. Log shows `starting deadline reminder run`. `deadline.reminder` events published for any job closing in 48h.

**Evidence:** `kubectl logs` from `deadline-reminder` pod.

## 4. weekly-summary

**Trigger:** Manual HTTP POST to the Service URL.

```bash
SINK=$(kubectl get k[vc weekly-summary -o jsonpath='{.status.url}')
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$SINK/" -d ''
```

**Expected:** `204`. Email to each admin in Mailpit. Log shows `summary email sent to N admins`.

**Evidence:** Mailpit shows weekly summary emails.

## 5. notification-dispatcher

**Trigger:** Publish `deadline.reminder` event to RabbitMQ (relayed by the forwarder).

```bash
RABBIT_POD=$(kubectl get pod -l app=rabbitmq -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it "$RABBIT_POD" -- rabbitmqadmin publish exchange=jobtrack.events routing_key=deadline.reminder \
  payload='{"event_id":"c9f7ca12-5db2-5d37-b133-1349f4c486dc","type":"deadline.reminder","producer":"deadline-reminder","occurred_at":"2026-11-19T12:00:00Z","data":{"job_id":"33333333-3333-4333-8333-333333333333","applicant_id":"22222222-2222-4222-8222-222222222222","deadline":"2026-11-21T12:00:00Z"}}'
```

**Expected:** `notification-dispatcher` logs `processing event event_type=deadline.reminder`. Notification row in `notification-inbox` DB. Email in Mailpit.

**Evidence:** `kubectl logs` from `notification-dispatcher` pod. Mailpit.

## Scale to zero evidence

After each function's verification, wait 120 seconds and rum:

```bash
kubectl get pods -n default -l serving.knative.dev/service=<function-name>
```

Expected: no pods (or only the activator). Then trigger again and observe a new pod appearing.

Screenshots to capture:
- `kubectl get k[vc` before/after
- `kubectl get pods` before trigger (0 pods), during (1 pod), after 120s (0 pods)
- Mailpit inbox for email-sender, weekly-summary, notification-dispatcher
- `kubectl logs` for each function showing `processing event`

## Limitations

- The 20 microservices stay in Docker Compose (per the 18 Nov 2026 Product Owner decision). Only the 5 functions run in k3d.
- `AUTH_ADDR` and `NOTIFICATION_INBOX_ADDR` require auth and notification-inbox Services to be reachable in the k3d namespace. If they are not deployed there, notification-dispatcher will fail with `UNRE–AILABLE`. This is a known limitation recorded in AGENTS.md Section 16.
