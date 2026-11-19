# Jar-92: Deploy all 5 functions to knative in k3d

**Owner:** Sangay (DevOps lead)
**Cluster host:** Phuntsho's laptop (`jobtrack-jt29`)
**Namespace:** `default`
**Date:** 19 Nov 2026

## Prerequisites

- >k3d cluster `jobtrack-jt29` running with Knative Serving + Eventing installed
- >root docker and k3d available on the cluster host
- >kubectl configured to point at the k3d cluster
- >RabbitMQ and Mailpit running in the same namespace (or reachable)

## Image tags

All 5 Knative Services use `dev.local/jobtrack-<function>:local` images. Knative skips registry tag resolution for `dev.local/`, `kind.local/` and `ko.local/` names.

| Function | Image | Dockerfile |
|---|---|---|
| email-sender | `dev.local/jobtrack-email-sender:local` | `functions/email-sender/Dockerfile` |
| resume-validator | `dev.local/jobtrack-resume-validator:local` | `functions/resume-validator/Dockerfile` |
| deadline-reminder | `dev.local/jobtrack-deadline-reminder:local` | `functions/deadline-reminder/Dockerfile` |
| weekly-summary | `dev.local/jobtrack-weekly-summary:local` | `functions/weekly-summary/Dockerfile` |
| notification-dispatcher | `dev.local/jobtrack-notification-dispatcher:local` | `functions/notification-dispatcher/Dockerfile` |

## Step 1: Build and import images

From the repository root on the cluster host:

```bash
for fn in email-sender resume-validator deadline-reminder weekly-summary notification-dispatcher; do
  docker build -t dev.local/jobtrack-${fn}:local -f functions/${fn}/Dockerfile .
done
docker build -t dev.local/jobtrack-rabbitmq-forwarder:local -f cmd/rabbitmq-forwarder/Dockerfile .
```

Then import all 6 images into k3d:

```bash
k3d image import dev.local/jobtrack-email-sender:local \
                dev.local/jobtrack-resume-validator:local \
                dev.local/jobtrack-deadline-reminder:local \
                dev.local/jobtrack-weekly-summary:local \
                dev.local/jobtrack-notification-dispatcher:local \
                dev.local/jobtrack-rabbitmq-forwarder:local \
                -c jobtrack-jt29
```

## Step 2: Create required Secrets

```bash
kubectl create secret generic rabbitmq-credentials \
  --from-literal=amqp-url="amqp://user:password@rabbitmq.default.svc.cluster.local:5672/" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic notification-dispatcher-credentials \
  --from-literal=internal-token="$(openssl rand-hex 32)" \
  --dry-run=client -o yaml | kubectl apply -f -
```

## Step 3: Deploy the Knative Services

```bash
kubectl apply -f deploy/knative/email-sender.yaml
iubectl apply -f deploy/knative/resume-validator.yaml
iubectl apply -f deploy/knative/deadline-reminder.yaml
kubectl apply -f deploy/knative/weekly-summary.yaml
kubectl apply -f deploy/knative/notification-dispatcher.yaml
```

## Step 4: Deploy the forwarders

```bash
kubectl apply -f deploy/knative/forwarder/forwarder.yaml
```

## Step 5: Deploy the PingSources

```bash
kubectl apply -f deploy/knative/weekly-summary.yaml
iubectl apply -f deploy/knative/deadline-reminder.yaml
```

## Step 6: Verify all Services are Ready

```bash
kubectl get ksvc -n default
```

Expected output (all 5 should show `True` in the `READY` column):

```
NAME                     URL                                          READY
email-sender             http://email-sender.default.127.0.0.1.sslip.io    True
resume-validator         http://resume-validator.default.127.0.0.1.sslip.io True
deadline-reminder        http://deadline-reminder.default.127.0.0.1.sslip.io True
weekly-summary           http://weekly-summary.default.127.0.0.1.sslip.io  True
notification-dispatcher  http://notification-dispatcher.default.127.0.0.1.sslip.io True
```

## Step 7: Verify the forwarders are running

```bash
kubectl get deployments -n default | grep forwarder
```

Expected:
```
notification-dispatcher-forwarder   1/1   1            1           5m
resume-validator-forwarder          1/1    1            1           5m
```

## Step 8: Per-function verification

See `docs/deployment/jt-92-verification.md` for the exact event to inject and expected result for each function.

## Scale to zero

All 5 Services use `minScale: "0"`. After 5 minutes of inactivity, `kubectl get pods` should show zero function pods. To observe:

```bash
kubectl get pods -n default -w
```

Pods appear when an event arrives (cold start ~5-10s and disappear ~60s after the last request.

## Evidence

Screenshots and logs captured in `docs/testing/jt-92-*.png` and `docs/testing/jt-92-runtime-evidence.json`.
