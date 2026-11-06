"""Create spike-only credentials without printing or writing their values to disk."""
import json
import os
import secrets
import subprocess

k = ["kubectl", "--kubeconfig", os.environ.get("JT29_KUBECONFIG", "/private/tmp/jobtrack-jt29.kubeconfig"),
     "--context", "k3d-jobtrack-jt29", "-n", "jt29-spike"]
existing = subprocess.run(k + ["get", "secret", "rabbitmq-credentials", "--ignore-not-found", "-o", "name"],
                          capture_output=True, text=True, check=True)
if existing.stdout.strip():
    print("Existing spike credentials retained")
else:
    password = secrets.token_hex(24)
    obj = {"apiVersion": "v1", "kind": "Secret", "type": "Opaque",
           "metadata": {"name": "rabbitmq-credentials", "namespace": "jt29-spike"},
           "stringData": {"username": "jt29", "password": password,
                          "uri": "rabbitmq.jt29-spike.svc.cluster.local",
                          "amqp-url": f"amqp://jt29:{password}@rabbitmq.jt29-spike.svc.cluster.local:5672/"}}
    result = subprocess.run(k + ["create", "-f", "-"], input=json.dumps(obj), text=True, capture_output=True)
    if result.returncode:
        raise SystemExit("Secret creation failed; values withheld")
    print("Created spike credentials; values withheld")
