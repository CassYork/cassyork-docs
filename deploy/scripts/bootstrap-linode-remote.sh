#!/usr/bin/env bash
# One-shot: power-cycle Linode if needed to reset root password, install pubkey + Docker,
# rsync repo, compose up, goose migrations. Requires linode-cli configured locally.
#
# Usage (from repo root):
#   LINODE_ID=97368225 LINODE_IP=x.x.x.x bash deploy/scripts/bootstrap-linode-remote.sh
#
set -euo pipefail

LINODE_ID="${LINODE_ID:?Set LINODE_ID}"
LINODE_IP="${LINODE_IP:?Set LINODE_IP}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

ROOTPASS="$(openssl rand -base64 22 | tr -d '/\n+= ')Ab9!z"
while [[ ${#ROOTPASS} -lt 18 ]]; do ROOTPASS="${ROOTPASS}kL9!"; done

ssh_pw() {
  SSHPASS="$ROOTPASS" sshpass -e ssh \
    -o StrictHostKeyChecking=accept-new \
    -o PreferredAuthentications=password \
    -o PubkeyAuthentication=no "$@"
}

echo "=== Shut down Linode ${LINODE_ID} ==="
linode-cli linodes shutdown "$LINODE_ID" --json >/dev/null
for _ in $(seq 1 80); do
  st=$(linode-cli linodes view "$LINODE_ID" --json | jq -r '(if type=="array" then .[0] else . end).status')
  [[ "$st" == "offline" ]] && break
  sleep 2
done

echo "=== Reset root password (requires powered off) ==="
linode-cli linodes linode-reset-password "$LINODE_ID" --root_pass "$ROOTPASS" --json >/dev/null

echo "=== Boot ==="
linode-cli linodes boot "$LINODE_ID" --json >/dev/null
for _ in $(seq 1 90); do
  st=$(linode-cli linodes view "$LINODE_ID" --json | jq -r '(if type=="array" then .[0] else . end).status')
  [[ "$st" == "running" ]] && break
  sleep 2
done

echo "=== Wait for SSH (password) ==="
for _ in $(seq 1 60); do
  if ssh_pw root@"$LINODE_IP" 'echo ssh_ready' 2>/dev/null; then break; fi
  sleep 3
done

echo "=== Install SSH pubkey ==="
ssh_pw root@"$LINODE_IP" "mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
ssh_pw root@"$LINODE_IP" "grep -qxF '$(cat ~/.ssh/id_ed25519.pub)' ~/.ssh/authorized_keys" 2>/dev/null || \
  cat ~/.ssh/id_ed25519.pub | ssh_pw root@"$LINODE_IP" "cat >> ~/.ssh/authorized_keys"

SSH_KEY=(ssh -i "$HOME/.ssh/id_ed25519" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)

echo "=== Verify key auth ==="
"${SSH_KEY[@]}" root@"$LINODE_IP" 'echo key_ok'

echo "=== Docker ==="
"${SSH_KEY[@]}" root@"$LINODE_IP" 'command -v docker >/dev/null && docker --version || (export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq ca-certificates curl git && curl -fsSL https://get.docker.com | sh)'

POSTGRES_PW="$(openssl rand -base64 24 | tr -d '/+=\n')"
MINIO_PW="$(openssl rand -base64 24 | tr -d '/+=\n')"
export ROOT POSTGRES_PW MINIO_PW LINODE_IP

echo "=== Write ${ROOT}/deploy/.env.linode ==="
python3 << 'PY'
from pathlib import Path
import os
root = Path(os.environ["ROOT"])
linode_ip = os.environ["LINODE_IP"]
pg = os.environ["POSTGRES_PW"]
minio = os.environ["MINIO_PW"]
src = root / "deploy" / "env.linode.example"
dst = root / "deploy" / ".env.linode"
lines = []
for line in src.read_text().splitlines():
    if line.startswith("POSTGRES_PASSWORD="):
        line = "POSTGRES_PASSWORD=" + pg
    elif line.startswith("OBJECT_STORAGE_SECRET_ACCESS_KEY="):
        line = "OBJECT_STORAGE_SECRET_ACCESS_KEY=" + minio
    lines.append(line)
text = "\n".join(lines) + "\n"
text += f"WEBHOOK_API_PUBLIC_URL=http://{linode_ip}:8095\n"
dst.write_text(text)
PY

echo "=== Rsync to /opt/cassyork-docs ==="
rsync -az --delete \
  -e "ssh -i $HOME/.ssh/id_ed25519 -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new" \
  --exclude '.git' \
  --exclude '.venv' \
  --exclude 'venv' \
  --exclude '__pycache__' \
  --exclude 'bin' \
  --exclude 'dist' \
  --exclude '*.sqlite3' \
  --exclude '.env' \
  "$ROOT/" "root@${LINODE_IP}:/opt/cassyork-docs/"

scp -i "$HOME/.ssh/id_ed25519" -o IdentitiesOnly=yes \
  "$ROOT/deploy/.env.linode" "root@${LINODE_IP}:/opt/cassyork-docs/deploy/.env.linode"

echo "=== Compose up ==="
"${SSH_KEY[@]}" root@"$LINODE_IP" 'cd /opt/cassyork-docs && docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml up -d --build'

"${SSH_KEY[@]}" root@"$LINODE_IP" 'cd /opt/cassyork-docs && docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml ps'

echo "=== Goose migrations ==="
"${SSH_KEY[@]}" root@"$LINODE_IP" "docker run --rm --network cassyork-linode_default \
  -v /opt/cassyork-docs/migrations:/migrations:ro \
  golang:1.23-alpine \
  sh -ceu 'go install github.com/pressly/goose/v3/cmd/goose@v3.24.1 && \
    /go/bin/goose -dir /migrations postgres \"postgres://temporal:${POSTGRES_PW}@postgresql:5432/cassyork?sslmode=disable\" up'"

echo "=== Health ==="
for _ in $(seq 1 30); do
  if curl -sf --max-time 5 "http://${LINODE_IP}:8095/healthz" >/dev/null; then
    curl -sS "http://${LINODE_IP}:8095/healthz"
    echo ""
    echo "OK http://${LINODE_IP}:8095/"
    exit 0
  fi
  sleep 2
done
echo "Health check timed out (services may still be starting)." >&2
exit 1
