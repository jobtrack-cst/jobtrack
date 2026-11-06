#!/usr/bin/env bash
# Deploy only the selected JT-29 fallback; existing data and clusters are retained.
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
export JT29_KUBECONFIG=${JT29_KUBECONFIG:-/private/tmp/jobtrack-jt29.kubeconfig}
k() { kubectl --kubeconfig "$JT29_KUBECONFIG" --context k3d-jobtrack-jt29 "$@"; }
k apply -f "$script_dir/namespace.yaml"
python3 "$script_dir/credentials.py"
k apply -k "$script_dir"
k -n jt29-spike rollout status deployment/rabbitmq --timeout=180s
k -n jt29-spike wait --for=condition=Ready ksvc/hello-world --timeout=180s
# Native attempt needs RabbitMQ topology CRDs not selected for this spike.
if k -n knative-sources get deployment rabbitmq-controller-manager >/dev/null 2>&1; then
  k -n knative-sources scale deployment/rabbitmq-controller-manager --replicas=0
fi
docker build -f "$script_dir/forwarder/Dockerfile" -t jobtrack-jt29-forwarder:spike "$repo_root"
k3d image import jobtrack-jt29-forwarder:spike -c jobtrack-jt29
k apply -f "$script_dir/forwarder.yaml"
# The local-only spike tag is mutable; ensure a rebuilt image is actually used.
k -n jt29-spike rollout restart deployment/rabbitmq-forwarder
k -n jt29-spike rollout status deployment/rabbitmq-forwarder --timeout=180s
k -n jt29-spike wait --for=condition=Ready pingsource/hello-schedule --timeout=180s
python3 "$script_dir/verify.py"
