#!/usr/bin/env bash
# Sync this repository to a Linode VM and run docker compose (see deploy/LINODE.md).
#
# Prerequisites:
#   - SSH as root (or set LINODE_SSH_USER): Linode CLI often adds your SSH key via authorized_users.
#   - Docker on the VM (install per deploy/LINODE.md if needed).
#   - Secrets in deploy/.env.linode locally — they are copied to the server.
#
# Usage:
#   export LINODE_HOST=203.0.113.10
#   bash deploy/push-to-linode.sh
#
# Or create deploy/linode-connection.env (see linode-connection.env.example).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY_DIR="$ROOT/deploy"
ENV_CONN="$DEPLOY_DIR/linode-connection.env"

if [[ -z "${LINODE_HOST:-}" ]] && [[ -f "$ENV_CONN" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_CONN"
  set +a
fi

HOST="${LINODE_HOST:?Set LINODE_HOST (public IPv4) or create deploy/linode-connection.env}"
REMOTE_USER="${LINODE_SSH_USER:-root}"
REMOTE_DIR="${LINODE_REMOTE_DIR:-/opt/cassyork-docs}"

RSYNC=(rsync -az --delete
  -e "ssh -o StrictHostKeyChecking=accept-new"
  --exclude '.git'
  --exclude '.venv'
  --exclude 'venv'
  --exclude '__pycache__'
  --exclude 'bin'
  --exclude 'dist'
  --exclude '*.sqlite3'
  --exclude '.env'
  --exclude 'deploy/.env.linode'
)

echo "Syncing repo to ${REMOTE_USER}@${HOST}:${REMOTE_DIR}/"
"${RSYNC[@]}" "$ROOT/" "${REMOTE_USER}@${HOST}:${REMOTE_DIR}/"

if [[ -f "$DEPLOY_DIR/.env.linode" ]]; then
  echo "Copying deploy/.env.linode (secrets) to server..."
  scp -o StrictHostKeyChecking=accept-new "$DEPLOY_DIR/.env.linode" "${REMOTE_USER}@${HOST}:${REMOTE_DIR}/deploy/.env.linode"
else
  echo "Warning: no deploy/.env.linode found locally — ensure it exists on the server before compose will succeed." >&2
fi

echo "Starting stack on server..."
ssh -o StrictHostKeyChecking=accept-new "${REMOTE_USER}@${HOST}" bash -s <<REMOTE
set -euo pipefail
cd "${REMOTE_DIR}"
docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml up -d --build
docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml ps
bash deploy/scripts/run-migrations-remote.sh
REMOTE

echo ""
echo "Smoke test (from any machine that can reach the VM):"
echo "  curl -sS http://${HOST}:8095/healthz"
