#!/usr/bin/env python3
"""JT-93: fail unless the full Compose application target set is UP."""
import json
from pathlib import Path
import subprocess
import sys
from urllib.request import urlopen

compose = Path(__file__).resolve().parents[1] / "docker-compose.yml"
# Parse in memory: never print interpolated configuration containing secrets.
config = json.loads(subprocess.check_output(
    ["docker", "compose", "-f", str(compose), "config", "--format", "json"],
    text=True,
))
expected = {
    name for name, svc in config["services"].items()
    if name == "gateway" or svc.get("build", {}).get("dockerfile", "").startswith("services/")
}
if len(expected) != 21:
    sys.exit("FAIL: Compose must define exactly 21 application targets")
try:
    with urlopen("http://127.0.0.1:9090/api/v1/targets?state=active", timeout=10) as response:
        result = json.load(response)
    if result["status"] != "success":
        sys.exit("FAIL: Prometheus API did not succeed")
    targets = result["data"]["activeTargets"]
    actual = [t["labels"].get("service") for t in targets]
    if len(actual) != 21 or set(actual) != expected:
        sys.exit("FAIL: expected exactly the 21 Compose application services")
    for target in targets:
        name = target["labels"]["service"]
        if (target["labels"].get("job") != "jobtrack"
                or target["scrapeUrl"] != f"http://{name}:8080/metrics"
                or target["health"] != "up"):
            sys.exit(f"FAIL: {name} has a wrong scrape configuration or is not UP")
except (OSError, ValueError, KeyError) as exc:
    sys.exit(f"FAIL: cannot validate Prometheus targets ({type(exc).__name__})")
print("PASS: all 21 Compose application targets scrape /metrics on port 8080 and are UP")
