#!/bin/sh
# Validate without printing credentials; the base profile needs no Grafana settings.
set -eu
case "${GF_SECURITY_ADMIN_USER:-}" in
  ""|change-me) echo "Set GRAFANA_ADMIN_USER in deploy/compose/.env" >&2; exit 1 ;;
esac
case "${GF_SECURITY_ADMIN_PASSWORD:-}" in
  ""|change-me) echo "Set GRAFANA_ADMIN_PASSWORD in deploy/compose/.env" >&2; exit 1 ;;
esac
if [ "${#GF_SECURITY_ADMIN_PASSWORD}" -lt 12 ]; then
  echo "GRAFANA_ADMIN_PASSWORD must contain at least 12 characters" >&2
  exit 1
fi
exec /run.sh
