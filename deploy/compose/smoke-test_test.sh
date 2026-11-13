#!/usr/bin/env bash
# JT-55: run without Docker, network access, or the repository's .env.
set -euo pipefail
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/jobtrack-smoke-test.XXXXXX")
scratch=$(cd "$scratch" && pwd -P)
trap 'rm -rf "$scratch"' EXIT
trap 'exit 1' HUP INT TERM
fixture="$scratch/compose with spaces"
mkdir -p "$fixture" "$scratch/bin"
cp "$script_dir/smoke-test.sh" "$fixture/"
touch "$fixture/docker-compose.yml"
cat > "$fixture/.env" <<'ENV'
SERVICE_DB_PASSWORD=synthetic_db_password
RABBITMQ_USER=synthetic_user
RABBITMQ_PASSWORD=synthetic_password
ENV
export FIXTURE="$fixture" CALLS="$scratch/calls"
export PATH="$scratch/bin:$PATH"
cat > "$scratch/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ $PWD == "$FIXTURE" ]] || exit 90
printf '%s\n' "$*" >> "$CALLS"
if [[ $1 == compose ]]; then
  [[ $2 == -f && $3 == docker-compose.yml ]] || exit 91
  shift 3
  case $1 in
    ps) echo mock-container ;;
    exec) echo "${DB_COUNT:-20}" ;;
    port)
      case "$2:$3" in
        rabbitmq:15672) printf '%s\n' "$RABBIT_BINDING"; exit "${RABBIT_STATUS:-0}" ;;
        mailpit:8025) printf '%s\n' "$MAIL_BINDING"; exit "${MAIL_STATUS:-0}" ;;
        *) exit 92 ;;
      esac ;;
    *) exit 93 ;;
  esac
elif [[ $1 == inspect ]]; then
  echo healthy
elif [[ $1 == run ]]; then
  # Record the original authentication and isolation command, never execute it.
  exit 0
else
  exit 94
fi
MOCK
cat > "$scratch/bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf 'curl %s\n' "$*" >> "$CALLS"
[[ $* != *"${CURL_FAIL:-never-match}"* ]] || exit 22
MOCK
chmod +x "$scratch/bin/docker" "$scratch/bin/curl"
fail() { echo "FAIL: $1" >&2; exit 1; }
contains() { grep -Fq -- "$1" "$2" || fail "missing assertion: $1"; }
run_case() {
  : > "$CALLS"
  if (cd "$scratch" && bash "$fixture/smoke-test.sh") > "$scratch/output" 2>&1; then
    [[ $1 == success ]] || fail "expected failure: $2"
  else
    [[ $1 == failure ]] || { cat "$scratch/output"; fail "expected success: $2"; }
    contains "$2" "$scratch/output"
  fi
}
export RABBIT_BINDING MAIL_BINDING
for bindings in '0.0.0.0:15672 0.0.0.0:8025' '127.0.0.1:15673 127.0.0.1:18025' '[::]:15673 [::1]:18025' ':::15673 ::1:18025'; do
  read -r RABBIT_BINDING MAIL_BINDING <<< "$bindings"
  run_case success ports
  contains "curl -fsS -u synthetic_user:synthetic_password http://localhost:${RABBIT_BINDING##*:}/api/overview" "$CALLS"
  contains "curl -fsS http://localhost:${MAIL_BINDING##*:}/api/v1/info" "$CALLS"
  contains 'compose -f docker-compose.yml port rabbitmq 15672' "$CALLS"
  contains 'compose -f docker-compose.yml port mailpit 8025' "$CALLS"
  contains 'SELECT count(*) FROM pg_database' "$CALLS"
  contains 'PGPASSWORD=synthetic_db_password' "$CALLS"
  contains 'psql -h postgres -U "$db" -d "$db"' "$CALLS"
  contains 'psql -h postgres -U auth -d employer' "$CALLS"
  contains 'All checks passed.' "$scratch/output"
done
echo 'PASS: default/remapped IPv4 and IPv6 ports, authenticated requests and database checks'
for service in rabbitmq mailpit; do
  for invalid in '' garbage 15673 '0.0.0.0:nope' '0.0.0.0:0' '0.0.0.0:65536' '0.0.0.0:99999999999999999' '0.0.0.0:-1' '0.0.0.0:8025/udp' $'0.0.0.0:8025\n[::]:8025'; do
    RABBIT_BINDING=0.0.0.0:15673 MAIL_BINDING=0.0.0.0:18025
    if [[ $service == rabbitmq ]]; then RABBIT_BINDING=$invalid; else MAIL_BINDING=$invalid; fi
    run_case failure "$service: invalid published port"
  done
  RABBIT_BINDING=0.0.0.0:15673 MAIL_BINDING=0.0.0.0:18025
  if [[ $service == rabbitmq ]]; then export RABBIT_STATUS=1; else export MAIL_STATUS=1; fi
  run_case failure "$service: could not resolve published port"
  unset RABBIT_STATUS MAIL_STATUS
done
echo 'PASS: unresolved, empty, malformed and out-of-range bindings fail per service'
RABBIT_BINDING=0.0.0.0:15673 MAIL_BINDING=0.0.0.0:18025
export CURL_FAIL=api/overview
run_case failure 'RabbitMQ management API not reachable on port 15673'
export CURL_FAIL=api/v1/info
run_case failure 'Mailpit not reachable on port 18025'
unset CURL_FAIL
export DB_COUNT=19
run_case failure 'expected 20 service databases, found 19'
echo 'PASS: curl failures and exactly-20-databases failure preserved'
