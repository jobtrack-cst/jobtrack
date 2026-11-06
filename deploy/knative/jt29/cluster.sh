#!/usr/bin/env bash
# Run from any directory. Never changes the user's default Kubernetes context.
set -euo pipefail
if ! k3d cluster list -o json | python3 -c 'import json,sys; sys.exit(not any(c["name"] == "jobtrack-jt29" for c in json.load(sys.stdin)))'; then
  k3d cluster create jobtrack-jt29 --image rancher/k3s:v1.35.5-k3s1 \
    --servers 1 --agents 0 --servers-memory 5g --api-port 127.0.0.1:6550 \
    --port '127.0.0.1:18080:80@loadbalancer' \
    --k3s-arg '--disable=traefik@server:0' --k3s-arg '--disable=metrics-server@server:0' \
    --kubeconfig-update-default=false --kubeconfig-switch-context=false \
    --no-rollback --timeout 180s
fi
JT29_KUBECONFIG=${JT29_KUBECONFIG:-/private/tmp/jobtrack-jt29.kubeconfig}
umask 077
k3d kubeconfig get jobtrack-jt29 > "$JT29_KUBECONFIG"
chmod 600 "$JT29_KUBECONFIG"
kubectl --kubeconfig "$JT29_KUBECONFIG" --context k3d-jobtrack-jt29 get nodes
