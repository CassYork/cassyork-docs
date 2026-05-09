#!/usr/bin/env bash
# postgres official image runs *.sql in a way that wraps CREATE DATABASE in a transaction;
# CREATE DATABASE must run outside a transaction — use separate psql -c invocations.
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -c "CREATE DATABASE cassyork;"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -c "GRANT ALL PRIVILEGES ON DATABASE cassyork TO \"$POSTGRES_USER\";"
