# JT-29: k3d, Knative and RabbitMQ cluster spike

Owner: Phuntsho Namgyel. Verified on **7 November 2026, Asia/Thimphu**.
Jira: https://jobtrack-cst.atlassian.net/browse/JT-29
Base commit: `26d443b`; branch: `JT-29-cluster-spike`.

## Decision and scope

Use k3d with Knative Serving and Eventing, Kourier ingress (explicitly approved by
Phuntsho), and the **Go RabbitMQ-to-HTTP forwarder fallback** permitted by JT-29.
The native RabbitMQSource was attempted first. Its controller could not synchronize
RabbitMQ topology informers because the `Exchange`, `Queue` and `Binding` CRDs were
absent. The source never acquired reconciled status. Do not confuse a Running
controller Pod with a working source.

The fallback uses the already approved `github.com/rabbitmq/amqp091-go` dependency.
No new Go dependencies, protobuf contracts, business services or production
functions were added. The hello-world sink is Knative's official `event-display`
sample, pinned by digest, deployed as Knative Service `hello-world`.

```text
synthetic publisher -> jobtrack.events (durable topic exchange)
                    -> jt29.hello (durable queue, binding #, prefetch 1)
                    -> Go forwarder -> HTTP CloudEvent -> Knative hello-world
PingSource (every minute, Asia/Thimphu) ----------------> Knative hello-world
```

The forwarder preserves the complete validated protojson EventEnvelope as HTTP
data. `ce-id` is `event_id`, `ce-type` is the existing domain routing type,
`ce-source` is `/jobtrack/jt29/rabbitmq`, and `ce-specversion` is `1.0`.
It acknowledges only HTTP 2xx, requeues failed deliveries with a one-second delay,
uses a 15-second HTTP timeout, and does not follow redirects. Malformed envelopes
are rejected without requeue. The proof publisher uses RabbitMQ's management API;
the message then traverses RabbitMQ and the forwarder's real AMQP consumer.

## Host and versions

Read-only inspection found no existing Kubernetes context or k3d cluster. Existing
JobTrack PostgreSQL, RabbitMQ and Mailpit containers, and an unrelated RabbitMQ
container, were preserved throughout.

| Component | Observed configuration |
| --- | --- |
| Mac | darwin/arm64, `hw.memsize=17179869184` (16 GiB) |
| Docker Desktop engine | 29.9.2; 8 CPUs; 8,319,705,088 bytes (~7.75 GiB) allocated |
| kubectl | v1.36.1, Kustomize v5.8.1 |
| k3d | v5.9.0, installed using Homebrew |
| k3s | `rancher/k3s:v1.35.5-k3s1`; one server, no agents; 5 GiB memory limit |
| Serving / Eventing / Kourier / source attempt | `knative-v1.24.0` |
| Hello sink | `gcr.io/knative-releases/knative.dev/eventing/cmd/event_display@sha256:e991bf5c8c0d37ec1660eb8b340a893c2525ef5b35203ffc236ee39fb7edae6c` |
| RabbitMQ | `rabbitmq:4-management-alpine@sha256:4841f6384ef8313c3594cd3f2e9f40c10805f8a61c45f08a9a5d9ddea8199267` |

Traefik is disabled because Kourier owns ingress. Metrics-server is disabled in
this small spike; Knative uses its own autoscaling metrics. Kubernetes API and
HTTP ingress bind only `127.0.0.1:6550` and `127.0.0.1:18080`. RabbitMQ is a
ClusterIP service; its management port is exposed temporarily on loopback by the
verification script. No gRPC port is published.

Knative's route domain is `jt29.localhost`; the hello route is
`http://hello-world.jt29-spike.jt29.localhost`. Use the explicit Host header below
instead of modifying system DNS. Internal sources still target the Kubernetes
service name `hello-world.jt29-spike.svc.cluster.local`.

The node used approximately **1.285 GiB / 5 GiB** in an observed Docker stats
snapshot. This is a point-in-time spike measurement, not capacity validation for
20 services, PostgreSQL, monitoring and all functions.

## Exact setup commands

Run from the repository root, with Docker Desktop running. Review these scripts
before use: they install cluster-level Knative resources into **only**
`k3d-jobtrack-jt29`. They never select or modify the user's default context.
No script deletes a cluster, container, volume or namespace.

```bash
brew install k3d
bash deploy/knative/jt29/cluster.sh
bash deploy/knative/jt29/install.sh
bash deploy/knative/jt29/deploy.sh
```

`cluster.sh` creates the isolated cluster if absent and exports its credentials to
`/private/tmp/jobtrack-jt29.kubeconfig` with mode 600. Re-running it reuses the
cluster. This file contains credentials: never print or commit it. Override its
path using `JT29_KUBECONFIG` consistently across all scripts if necessary.

The exact cluster creation command is:

```bash
k3d cluster create jobtrack-jt29 --image rancher/k3s:v1.35.5-k3s1 \
  --servers 1 --agents 0 --servers-memory 5g --api-port 127.0.0.1:6550 \
  --port '127.0.0.1:18080:80@loadbalancer' \
  --k3s-arg '--disable=traefik@server:0' \
  --k3s-arg '--disable=metrics-server@server:0' \
  --kubeconfig-update-default=false --kubeconfig-switch-context=false \
  --no-rollback --timeout 180s
```

`install.sh` downloads pinned release manifests to
`/private/tmp/jobtrack-jt29-upstream`, checks the committed SHA-256 values, applies
Serving CRDs/core, Kourier, Eventing CRDs/core and the native source attempt,
then waits for Deployments. It selects Kourier using:

```bash
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig \
  --context k3d-jobtrack-jt29 -n knative-serving patch configmap config-network \
  --type merge -p '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig \
  --context k3d-jobtrack-jt29 -n knative-serving patch configmap config-domain \
  --type merge -p '{"data":{"jt29.localhost":""}}'
```

`deploy.sh` creates namespace `jt29-spike`, generates a new isolated RabbitMQ
credential Secret only if absent, applies Kustomize workloads, builds/imports the
local forwarder image, and runs verification. Credentials are passed to kubectl
through stdin, never command arguments or files. Existing Secrets are retained.
The image build copies only the listed source directories, not `.env` files.
Re-running deployment restarts only the spike forwarder and reconciles spike
resources; the broker's `emptyDir` is intentionally disposable on Pod replacement.
Never place real data in this broker.

### Reproduce the native-source attempt

After installation, credentials and the base workloads exist:

```bash
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig \
  --context k3d-jobtrack-jt29 -n knative-sources \
  scale deployment/rabbitmq-controller-manager --replicas=1
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig \
  --context k3d-jobtrack-jt29 apply -f deploy/knative/jt29/native-source.yaml
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig \
  --context k3d-jobtrack-jt29 -n jt29-spike \
  wait --for=condition=Ready rabbitmqsource/hello-native --timeout=60s
```

The observed attempt remained unreconciled for several minutes; logs reported:

```text
failed to list *v1beta1.Exchange: the server could not find the requested resource (get exchanges.rabbitmq.com)
failed to list *v1beta1.Queue: the server could not find the requested resource (get queues.rabbitmq.com)
failed to list *v1beta1.Binding: the server could not find the requested resource (get bindings.rabbitmq.com)
```

`predeclared: true` avoids topology creation during source reconciliation but does
not remove the controller's informer dependencies. No extra topology operator or
cert-manager was introduced. This proves failure of the attempted minimal
installation, not that RabbitMQSource cannot work with its full prerequisites.
The diagnostic Source/CRDs remain for inspection. `deploy.sh` scales only the
failed source controller to zero, preventing repeated error logs or competing
consumers; its webhook remains installed. The selected fallback does not require
native-source readiness.

## Verification and observed evidence

```bash
python3 deploy/knative/jt29/verify.py
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig \
  --context k3d-jobtrack-jt29 get nodes
kubectl --kubeconfig /private/tmp/jobtrack-jt29.kubeconfig \
  --context k3d-jobtrack-jt29 -n jt29-spike get ksvc,pingsource,deployments
```

`verify.py` requires Ready hello/PingSource resources, checks for one AMQP
consumer, publishes a fresh UUID-bearing synthetic `user.registered` envelope to
`jobtrack.events`, asserts `routed=true`, finds that exact UUID in the actual
function logs, and checks that the queue is drained. It then patches PingSource
data with a fresh marker and waits for that marker in the function logs. It stops
only its own port-forward process. The PingSource retains the latest synthetic
marker; applying Kustomize restores its checked-in sample data.

Observed successful output on 7 November, approximately 00:43–00:44 BTT:

```text
PASS RabbitMQ -> Go forwarder -> Knative hello-world; event_id=99280fb5-07fd-446a-8c3f-e15be9ea696b
PASS queue drained after HTTP delivery (ready + unacknowledged = 0)
PASS PingSource -> Knative hello-world; marker=JT-29 scheduled hello-world 26b17177-02f7-4c31-8452-c88d759933a9
PASS CloudEvent types user.registered and dev.knative.sources.ping observed
```

The sink logged matching CloudEvent attributes:

```text
specversion: 1.0
type: user.registered
source: /jobtrack/jt29/rabbitmq
id: 99280fb5-07fd-446a-8c3f-e15be9ea696b
```

The fresh scheduled event had type `dev.knative.sources.ping`, source
`/apis/v1/namespaces/jt29-spike/pingsources/hello-schedule`, and timestamp
`2026-11-06T18:44:00.167488922Z` (**7 November 00:44 BTT**).
Its data contained the exact fresh marker above. The forwarder's JSON log also
recorded `event delivered` with the same RabbitMQ event UUID.

The reusable `cluster.sh` and `install.sh` were rerun successfully, including all
six checksum checks. `deploy.sh` then completed successfully with retained
credentials and the digest-pinned broker. That run delivered event
`b19715cd-0d89-4311-9713-c8f4733cffc4` and a fresh PingSource marker.
After the local-domain correction, independent verification again passed with
event `60affe7a-7b09-45cf-87b9-e26293b61892` and marker
`JT-29 scheduled hello-world 153b06af-cc68-46f5-9bcb-c0aa6c385066`.

Local Kourier ingress was also verified, returning **HTTP 200**:

```bash
curl --fail --silent --show-error --max-time 30 -o /dev/null \
  -w 'Kourier HTTP %{http_code}\n' http://127.0.0.1:18080 -X POST \
  -H 'Host: hello-world.jt29-spike.jt29.localhost' \
  -H 'Ce-Specversion: 1.0' -H 'Ce-Id: jt29-ingress-check' \
  -H 'Ce-Type: jt29.ingress' -H 'Ce-Source: /jobtrack/jt29/http' \
  -H 'Content-Type: application/json' --data '{"message":"JT-29 ingress hello-world"}'
```

| JT-29 acceptance criterion | Result | Evidence |
| --- | --- | --- |
| k3d cluster running on chosen host | PASS | `k3d-jobtrack-jt29-server-0 Ready`, `v1.35.5+k3s1`; host RAM verified |
| Knative Serving and Eventing installed | PASS | Core Deployments Running/Ready; hello and PingSource Ready |
| Hello function receives RabbitMQ message through source or fallback | PASS | Fresh event UUID correlated at broker, AMQP forwarder and sink; queue drained |
| PingSource triggers scheduled hello function | PASS | Fresh schedule marker received at the next minute; CloudEvent type/source verified |
| Findings, approach and setup commands documented | PASS | This guide, task manifests/scripts and linked test cases |

See [JT-29 test cases](../testing/jt-29-test-cases.md) for local checks and repeatability
results. A checked box in Jira or a Ready source object alone is not delivery proof.

## Troubleshooting and limits

- Docker socket, Go-cache and host RAM inspection were sandbox-blocked initially;
  elevated execution was explicitly approved. Do not change daemon settings to
  bypass permissions. The existing Docker Desktop allocation was sufficient for
  this spike and was not changed.
- First image pulls delayed Pods in `ContainerCreating`. Wait for actual readiness;
  inspect `kubectl get pods -A` using the explicit spike context before retrying.
- The first external ingress check returned HTTP 404 with the default cluster-local
  domain. Setting `config-domain` to `jt29.localhost` and using the matching Host
  header corrected it; the repeated request returned HTTP 200. This is included in
  `install.sh`; no DNS or Docker Desktop settings were changed.
- Native source failures are dependency failures above. Do not install additional
  operators without a team decision; the forwarder fallback is already authorized.
- Run verification after the broker and PingSource are Ready. It waits up to
  100 seconds for each event and reports failure with a nonzero exit status.
- This broker uses `emptyDir`. Durable queues do not survive Pod/data loss here.
  Production broker storage, credentials, reconnect/backoff, dead-letter policy,
  duplicate-safe consumers and operational health belong to implementation work.
- Forwarding is at least once: a process crash after HTTP success but before AMQP
  acknowledgement can duplicate delivery. The hello sink has no business writes.
  No production idempotency or resilience guarantee is established by this spike.
- HTTP is confined to local cluster networking. Production transport security and
  NetworkPolicies, including traffic from Knative adapters, remain unverified and
  must be coordinated with JT-44. No internal gRPC security claim is made here.
- The hello Service permits scale to zero (`min-scale=0`, `max-scale=1`), but this
  spike does not claim a measured cold-start SLA or full-system capacity.
- Upstream Knative manifests are checksum-pinned, but their Kourier Envoy tag is
  upstream-managed. Revalidate architecture support and image resolution on other
  machines. The runtime proof here is arm64.

The cluster is intentionally left running for review. Do not run prune/delete or
volume-removal commands. Stopping or deleting it requires Phuntsho's permission.
The initial live-spike execution did not commit, push, merge or modify Jira.
The subsequent PR workflow commits and pushes only the reviewed JT-29 files;
merging and Jira updates remain separate actions.

## Primary references

- [Knative Serving YAML installation](https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/)
- [Knative Eventing YAML installation](https://knative.dev/docs/install/yaml-install/eventing/install-eventing-with-yaml/)
- [RabbitMQSource configuration](https://knative.dev/docs/eventing/sources/rabbitmq-source/)
- [Pinned source reconciler](https://github.com/knative-extensions/eventing-rabbitmq/blob/knative-v1.24.0/pkg/reconciler/source/rabbitmqsource.go)
- [Pinned event-display manifest](https://github.com/knative/eventing/releases/download/knative-v1.24.0/event-display.yaml)
