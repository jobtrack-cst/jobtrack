#!/bin/bash
# Checks that the local Compose environment works (JT-27).
# Run from the repo root after "up -d": bash deploy/compose/smoke-test.sh
set -euo pipefail

cd "$(dirname "$0")"
COMPOSE="docker compose -f docker-compose.yml"

set -a
# shellcheck disable=SC1091
source .env
set +a

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; exit 1; }

published_port() {
  local service=$1 container_port=$2 binding port
  binding=$($COMPOSE port "$service" "$container_port") \
    || fail "$service: could not resolve published port for $container_port" >&2
  # Compose prints IPv4, bracketed IPv6, or unbracketed IPv6 host:port.
  local binding_pattern='^([0-9.]+|\[[0-9a-fA-F:.]+\]|[0-9a-fA-F:]*:[0-9a-fA-F:.]*):([0-9]{1,5})$'
  [[ $binding =~ $binding_pattern ]] \
    || fail "$service: invalid published port for $container_port" >&2
  port=${BASH_REMATCH[2]}
  (( 10#$port >= 1 && 10#$port <= 65535 )) \
    || fail "$service: invalid published port for $container_port" >&2
  printf '%s\n' "$((10#$port))"
}

# Must match postgres-init/01-create-service-databases.sh.
SERVICES="auth employer company_verification moderation audit_log analytics
job_posting category_tag job_search saved_jobs notification_inbox
applicant_profile resume application_status review interview
application offer messaging job_alert"

# 1. Containers are up and healthy.
for svc in postgres rabbitmq; do
  id=$($COMPOSE ps -q "$svc")
  [ -n "$id" ] || fail "$svc is not running"
  status=$(docker inspect -f '{{.State.Health.Status}}' "$id")
  [ "$status" = "healthy" ] || fail "$svc health is $status"
done
[ -n "$($COMPOSE ps -q --status running mailpit)" ] || fail "mailpit is not running"
pass "postgres and rabbitmq healthy, mailpit running"

# 2. Exactly 20 service databases exist.
count=$($COMPOSE exec -T postgres psql -U postgres -tAc \
  "SELECT count(*) FROM pg_database WHERE datname NOT IN ('postgres','template0','template1')")
[ "$count" = "20" ] || fail "expected 20 service databases, found $count"
pass "20 service databases exist"

# 3 and 4. Log in over the Docker network with a password, like a real service.
docker run --rm --network jobtrack_default \
  -e PGPASSWORD="$SERVICE_DB_PASSWORD" -e SERVICES="$SERVICES" \
  postgres:18-alpine sh -c '
    for db in $SERVICES; do
      psql -h postgres -U "$db" -d "$db" -tAc "SELECT 1" >/dev/null \
        || { echo "FAIL: $db cannot connect to its own database"; exit 1; }
    done
    echo "PASS: every service user connects to its own database"
    if psql -h postgres -U auth -d employer -tAc "SELECT 1" >/dev/null 2>&1; then
      echo "FAIL: auth user can connect to the employer database"; exit 1
    fi
    echo "PASS: a service user cannot connect to another service database"
  '

# 5. RabbitMQ management API and Mailpit respond.
rabbitmq_port=$(published_port rabbitmq 15672) || exit 1
curl -fsS -u "$RABBITMQ_USER:$RABBITMQ_PASSWORD" "http://localhost:$rabbitmq_port/api/overview" >/dev/null \
  || fail "RabbitMQ management API not reachable on port $rabbitmq_port"
pass "RabbitMQ management API responds"
mailpit_port=$(published_port mailpit 8025) || exit 1
curl -fsS "http://localhost:$mailpit_port/api/v1/info" >/dev/null \
  || fail "Mailpit not reachable on port $mailpit_port"
pass "Mailpit responds"

echo "All checks passed."
