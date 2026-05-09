#!/usr/bin/env bash
# Open SSH to the Linode (same connection env as push-to-linode.sh).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_CONN="$ROOT/deploy/linode-connection.env"

if [[ -z "${LINODE_HOST:-}" ]] && [[ -f "$ENV_CONN" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_CONN"
  set +a
fi

: "${LINODE_HOST:?Set LINODE_HOST or add it to deploy/linode-connection.env (see deploy/linode-connection.env.example)}"
HOST="$LINODE_HOST"
USER="${LINODE_SSH_USER:-root}"

exec ssh -o StrictHostKeyChecking=accept-new "${USER}@${HOST}"
