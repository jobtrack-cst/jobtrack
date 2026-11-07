# JT-29 verification cases

Story: [JT-29](https://jobtrack-cst.atlassian.net/browse/JT-29).
Owner: Phuntsho Namgyel. Execution date: 7 November 2026, Asia/Thimphu.
Environment and actual event evidence: [deployment guide](../deployment/jt-29-cluster-spike.md).

| ID | Case / partition | Expected result | Result |
| --- | --- | --- | --- |
| JT29-01 | Host and isolated cluster | 16 GiB host; node Ready; default context retained | PASS |
| JT29-02 | Serving/Eventing/Kourier | Core Pods Ready; hello Knative Service Ready | PASS |
| JT29-03 | Native source without topology CRDs | Record dependency failure, select authorized fallback | PASS: source unreconciled, missing Exchange/Queue/Binding CRDs observed |
| JT29-04 | Real AMQP-to-HTTP transport | Unique published event appears in hello sink logs; queue drains | PASS |
| JT29-05 | Fresh PingSource schedule data | Unique marker received with ping CloudEvent type | PASS |
| JT29-06 | HTTP 200 and 204 success partition | Ack only after sink response; preserve envelope and identity | PASS, table-driven Go test |
| JT29-07 | HTTP 400, 500 and 302 failure partition | No Ack; Nack with requeue; redirect not followed | PASS, table-driven Go test |
| JT29-08 | Invalid envelope | No HTTP call; reject without requeue | PASS, table-driven Go test |
| JT29-09 | Canceled HTTP request | No Ack; Nack with requeue | PASS, table-driven Go test |
| JT29-10 | Repeat setup/deployment | Reuse cluster and credentials; rebuild/import forwarder; both paths pass again | PASS |
| JT29-11 | Host-header ingress through Kourier | HTTP 200 on localhost port 18080 with configured route domain | PASS after correcting default domain |

The focused tests exercise delivery outcomes, unchanged envelope bytes and
CloudEvent headers. They mock acknowledgement operations, not the AMQP library.
JT29-04 is the actual broker/cluster integration proof, not a mocked delivery.
No production function or business service is exercised.

Executed results:

| Check | Observed result |
| --- | --- |
| `go vet ./...` | PASS; no diagnostics |
| `go test -count=1 ./...` | PASS; all test packages `ok`, generated packages have no tests |
| `go test -race -cover -count=1 ./deploy/knative/jt29/forwarder` | PASS; no race reports; 31.2% package statement coverage |
| Function coverage | `forward` 100%; `run`/`main` 0% in unit profile; startup/AMQP loop exercised by live cluster proof |
| Forwarder Docker build and k3d image import | PASS |
| Shell/Python syntax, Kustomize rendering and `git diff --check` | PASS |
| Kubernetes server dry-run, base and forwarder manifests | PASS |
| Upstream SHA-256 checks | All six pinned manifests OK |
| Fresh actual RabbitMQ and PingSource delivery | PASS on initial run, reusable deployment run and post-domain-change verification |

These results were recorded before committing or pushing the spike. GitHub CI
must be checked separately on the PR; local test results do not establish CI status.

Repeat local checks from the repository root:

```bash
go vet ./...
go test -count=1 ./...
go test -race -cover -count=1 ./deploy/knative/jt29/forwarder
bash -n deploy/knative/jt29/cluster.sh deploy/knative/jt29/install.sh deploy/knative/jt29/deploy.sh
kubectl kustomize deploy/knative/jt29
git diff --check
```

Repeat real delivery with the isolated cluster running:

```bash
python3 deploy/knative/jt29/verify.py
```

The verification uses generated synthetic credentials without printing them and
never connects to the existing Compose broker. Preserve the cluster for review.
