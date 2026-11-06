"""Publish a synthetic envelope and correlate actual sink logs and a fresh PingSource.

Uses Python's standard library and kubectl only. Never prints Secret values.
"""
import base64
import datetime
import json
import os
import socket
import subprocess
import time
import urllib.request
import uuid

k = ["kubectl", "--kubeconfig", os.environ.get("JT29_KUBECONFIG", "/private/tmp/jobtrack-jt29.kubeconfig"),
     "--context", "k3d-jobtrack-jt29", "-n", "jt29-spike"]


def kubectl(*args):
    result = subprocess.run(k + list(args), capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError("kubectl check failed: " + args[0])
    return result.stdout


def wait_for(check, seconds=100):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            if check():
                return
        except (OSError, ValueError):
            pass
        time.sleep(2)
    raise RuntimeError("Verification timed out")


def sink_logs():
    return kubectl("logs", "-l", "serving.knative.dev/service=hello-world", "-c", "user-container",
                   "--tail=300", "--max-log-requests=5")


def main():
    for kind, name in [("ksvc", "hello-world"), ("pingsource", "hello-schedule")]:
        obj = json.loads(kubectl("get", kind, name, "-o", "json"))
        if not any(c["type"] == "Ready" and c["status"] == "True" for c in obj["status"].get("conditions", [])):
            raise RuntimeError(kind + " is not Ready")
    secret = json.loads(kubectl("get", "secret", "rabbitmq-credentials", "-o", "json"))["data"]
    username, password = [base64.b64decode(secret[x]).decode() for x in ("username", "password")]
    authorization = "Basic " + base64.b64encode(f"{username}:{password}".encode()).decode()
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        local_port = sock.getsockname()[1]
    process = subprocess.Popen(k + ["port-forward", "--address", "127.0.0.1", "service/rabbitmq",
                                    f"{local_port}:15672"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        def api(path, payload=None):
            req = urllib.request.Request(f"http://127.0.0.1:{local_port}/api/{path}",
                                         data=None if payload is None else json.dumps(payload).encode(),
                                         headers={"Authorization": authorization, "Content-Type": "application/json"})
            with urllib.request.urlopen(req, timeout=5) as response:
                return json.load(response)

        wait_for(lambda: api("overview")["rabbitmq_version"], seconds=30)
        wait_for(lambda: api("queues/%2F/jt29.hello").get("consumers", 0) == 1)
        event_id = str(uuid.uuid4())
        envelope = {"event_id": event_id, "type": "user.registered", "producer": "jt29-spike",
                    "occurred_at": datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z"),
                    "data": {"@type": "type.googleapis.com/jobtrack.events.v1.UserRegistered",
                             "user_id": str(uuid.uuid4())}}
        published = api("exchanges/%2F/jobtrack.events/publish", {
            "properties": {"content_type": "application/json", "delivery_mode": 2, "message_id": event_id},
            "routing_key": "user.registered", "payload": json.dumps(envelope), "payload_encoding": "string"})
        if not published.get("routed"):
            raise RuntimeError("RabbitMQ did not route the envelope")
        wait_for(lambda: event_id in sink_logs())
        print("PASS RabbitMQ -> Go forwarder -> Knative hello-world; event_id=" + event_id, flush=True)
        wait_for(lambda: api("queues/%2F/jt29.hello").get("messages", -1) == 0)
        print("PASS queue drained after HTTP delivery (ready + unacknowledged = 0)", flush=True)
        marker = "JT-29 scheduled hello-world " + str(uuid.uuid4())
        kubectl("patch", "pingsource", "hello-schedule", "--type=merge", "-p",
                json.dumps({"spec": {"data": json.dumps({"message": marker})}}))
        wait_for(lambda: marker in sink_logs())
        print("PASS PingSource -> Knative hello-world; marker=" + marker, flush=True)
        logs = sink_logs()
        if "dev.knative.sources.ping" not in logs or "user.registered" not in logs:
            raise RuntimeError("Expected CloudEvent types absent")
        print("PASS CloudEvent types user.registered and dev.knative.sources.ping observed", flush=True)
    finally:
        process.terminate()
        process.wait(timeout=10)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # HTTP exception strings and Kubernetes diagnostics may contain private values.
        raise SystemExit("FAIL JT-29 verification: " + type(error).__name__)
