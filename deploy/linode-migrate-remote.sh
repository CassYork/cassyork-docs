#!/usr/bin/env bash
# Run Goose on the Linode over SSH (same connection env as linode-ssh.sh).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_CONN="$ROOT/deploy/linode-connection.env"

if [[ -z "${LINODE_HOST:-}" ]] && [[ -f "$ENV_CONN" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_CONN"
  set +a
fi

: "${LINODE_HOST:?Set LINODE_HOST or deploy/linode-connection.env}"
USER="${LINODE_SSH_USER:-root}"
DIR="${LINODE_REMOTE_DIR:-/opt/cassyork-docs}"

SSH=(ssh -i "$HOME/.ssh/id_ed25519" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)

exec "${SSH[@]}" "${USER}@${LINODE_HOST}" "bash ${DIR}/deploy/scripts/run-migrations-remote.sh"
