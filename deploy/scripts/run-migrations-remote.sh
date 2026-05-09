#!/usr/bin/env bash
# Run Goose migrations against the cassyork DB inside Docker Compose on this host.
# Expects deploy/.env.linode with POSTGRES_PASSWORD; Compose project cassyork-linode (network cassyork-linode_default).
#
# Usage (from repo root on the server):
#   bash deploy/scripts/run-migrations-remote.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

if [[ ! -f deploy/.env.linode ]]; then
  echo "deploy/.env.linode missing on server — create it once from deploy/env.linode.example" >&2
  exit 1
fi

GOOSE_PW="$(grep '^POSTGRES_PASSWORD=' deploy/.env.linode | cut -d= -f2-)"

export GOOSE_PW
docker run --rm --network cassyork-linode_default \
  -v "$ROOT/migrations:/migrations:ro" \
  -e "GOOSE_PW" \
  golang:1.23-alpine \
  sh -c 'go install github.com/pressly/goose/v3/cmd/goose@v3.24.1 && exec /go/bin/goose -dir /migrations postgres "postgres://temporal:${GOOSE_PW}@postgresql:5432/cassyork?sslmode=disable" up'
