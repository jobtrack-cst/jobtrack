#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
JT29_KUBECONFIG=${JT29_KUBECONFIG:-/private/tmp/jobtrack-jt29.kubeconfig}
manifest_dir=${JT29_MANIFEST_DIR:-/private/tmp/jobtrack-jt29-upstream}
mkdir -p "$manifest_dir"
k() { kubectl --kubeconfig "$JT29_KUBECONFIG" --context k3d-jobtrack-jt29 "$@"; }
k get nodes
download() {
  curl --fail --location --silent --show-error --retry 2 \
    "https://github.com/$1/releases/download/knative-v1.24.0/$2.yaml" -o "$manifest_dir/$2.yaml"
}
download knative/serving serving-crds
download knative/serving serving-core
download knative-extensions/net-kourier kourier
download knative/eventing eventing-crds
download knative/eventing eventing-core
# Retain the native attempt for reproducible diagnostics; fallback is selected below.
download knative-extensions/eventing-rabbitmq rabbitmq-source
(cd "$manifest_dir" && shasum -a 256 -c "$script_dir/upstream.sha256")
for manifest in serving-crds serving-core kourier eventing-crds eventing-core rabbitmq-source; do
  k apply -f "$manifest_dir/$manifest.yaml"
done
k -n knative-serving patch configmap config-network --type merge \
  -p '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'
k -n knative-serving patch configmap config-domain --type merge \
  -p '{"data":{"jt29.localhost":""}}'
for namespace in knative-serving knative-eventing kourier-system knative-sources; do
  k -n "$namespace" rollout status deployment --timeout=180s
done
