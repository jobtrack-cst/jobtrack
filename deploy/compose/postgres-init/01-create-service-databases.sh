#!/bin/bash
# Creates one database and one user per service (AGENTS.md Section 3).
# Runs only on the first start of an empty Postgres volume.
set -euo pipefail

: "${SERVICE_DB_PASSWORD:?SERVICE_DB_PASSWORD must be set}"

# Service names in snake_case, matching AGENTS.md Section 4.
SERVICES=(
  auth employer company_verification moderation audit_log analytics
  job_posting category_tag job_search saved_jobs notification_inbox
  applicant_profile resume application_status review interview
  application offer messaging job_alert
)

for db in "${SERVICES[@]}"; do
  echo "Creating database and user: ${db}"
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
    -v db="$db" -v pw="$SERVICE_DB_PASSWORD" <<'SQL'
CREATE USER :"db" WITH PASSWORD :'pw';
CREATE DATABASE :"db" OWNER :"db";
-- Only the owning service user may connect to its database.
REVOKE CONNECT ON DATABASE :"db" FROM PUBLIC;
SQL
done

echo "Created ${#SERVICES[@]} service databases."
