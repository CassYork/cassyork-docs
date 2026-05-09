#!/usr/bin/env bash
# Point Cassyork at Akamai Linode Object Storage using linode-cli.
#
# Prerequisites: linode-cli configure; a bucket already created in Object Storage
# (Cloud Manager or `aws s3 mb` with an endpoint override).
#
# Usage (prints template only, no key created):
#   export LINODE_OS_REGION=us-east
#   export LINODE_OS_BUCKET=my-bucket
#   ./deploy/scripts/configure-linode-object-storage.sh
#
# Write keys + env to deploy/.env.linode (creates a NEW access key — save it once):
#   ./deploy/scripts/configure-linode-object-storage.sh --apply --create-key
#
# Optional: LINODE_OS_KEY_LABEL, DEPLOY_ENV_FILE (default: deploy/.env.linode)
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="${DEPLOY_ENV_FILE:-$ROOT/deploy/.env.linode}"
EXAMPLE="$ROOT/deploy/env.linode.example"
APPLY=0
CREATE_KEY=0

for a in "$@"; do
  case "$a" in
    --apply) APPLY=1 ;;
    --create-key) CREATE_KEY=1 ;;
    -h|--help)
      grep '^#' "$0" | head -20
      exit 0
      ;;
  esac
done

if ! command -v linode-cli >/dev/null 2>&1; then
  echo "linode-cli not found. Install: brew install linode-cli && linode-cli configure" >&2
  exit 1
fi

BUCKET="${LINODE_OS_BUCKET:-}"
REGION="${LINODE_OS_REGION:-us-east}"
LABEL="${LINODE_OS_KEY_LABEL:-cassyork-object-storage}"

if [[ -z "$BUCKET" ]]; then
  echo "Set LINODE_OS_BUCKET to your Object Storage bucket name." >&2
  exit 1
fi

S3_HOST="$(
  linode-cli object-storage endpoints --json | python3 -c "
import json, sys
r = sys.argv[1]
data = json.load(sys.stdin)
for row in data:
    if row.get('region') == r and row.get('s3_endpoint'):
        print(row['s3_endpoint'])
        break
" "$REGION"
)"
if [[ -z "$S3_HOST" || "$S3_HOST" == "null" ]]; then
  echo "No s3_endpoint for region '$REGION'. Regions with an endpoint:" >&2
  linode-cli object-storage endpoints --json | python3 -c "
import json,sys
for row in json.load(sys.stdin):
    if row.get('s3_endpoint'):
        print(row['region'], '->', row['s3_endpoint'])
" >&2
  exit 1
fi

# Virtual-hosted style (subdomain bucket) — path-style is usually off for Linode.
ENDPOINT_URL="https://${S3_HOST}"
# AWS SDK / minio-go region string for this cluster (Linode uses us-east-1 for Newark E0).
STORAGE_REGION="us-east-1"
if [[ "$S3_HOST" == *"us-southeast"* ]]; then
  STORAGE_REGION="us-southeast-1"
fi

echo "Linode Object Storage (from linode-cli):"
echo "  region (API):     $REGION"
echo "  S3 endpoint host: $S3_HOST"
echo "  bucket:           $BUCKET"
echo "  OBJECT_STORAGE_ENDPOINT (HTTPS): $ENDPOINT_URL"
echo "  OBJECT_STORAGE_REGION (for SDK): $STORAGE_REGION"
echo ""

if [[ "$APPLY" -ne 1 ]]; then
  echo "Dry run. To merge into $ENV_FILE and create a key, run:"
  echo "  $0 --apply --create-key"
  echo ""
  echo "Afterwards start the stack without bundled MinIO (default in docker-compose.linode.yml):"
  echo "  docker compose --env-file $ENV_FILE -f deploy/docker-compose.linode.yml up -d --build"
  exit 0
fi

if [[ ! -f "$ENV_FILE" ]]; then
  if [[ -f "$EXAMPLE" ]]; then
    cp "$EXAMPLE" "$ENV_FILE"
    echo "Created $ENV_FILE from env.linode.example — review POSTGRES_PASSWORD before production."
  else
    echo "Missing $ENV_FILE and $EXAMPLE" >&2
    exit 1
  fi
fi

ACCESS=""
SECRET=""
if [[ "$CREATE_KEY" -eq 1 ]]; then
  echo "Creating Object Storage access key (label: ${LABEL})..."
  JSON="$(linode-cli object-storage keys-create --label "${LABEL}-$(date +%s)" --regions "$REGION" --json)"
  ACCESS="$(echo "$JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); d=d[0] if isinstance(d,list) else d; print(d.get('access_key',''))")"
  SECRET="$(echo "$JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); d=d[0] if isinstance(d,list) else d; print(d.get('secret_key',''))")"
  if [[ -z "$ACCESS" || -z "$SECRET" || "$ACCESS" == "null" ]]; then
    echo "Failed to parse new key from linode-cli output." >&2
    echo "$JSON" >&2
    exit 1
  fi
  echo "Key created. The secret is written only to $ENV_FILE — back it up; you cannot read it again from the API."
elif [[ -n "${LINODE_OS_ACCESS_KEY_ID:-}" && -n "${LINODE_OS_SECRET_ACCESS_KEY:-}" ]]; then
  ACCESS="$LINODE_OS_ACCESS_KEY_ID"
  SECRET="$LINODE_OS_SECRET_ACCESS_KEY"
  echo "Using existing keys from LINODE_OS_ACCESS_KEY_ID / LINODE_OS_SECRET_ACCESS_KEY."
else
  echo "For --apply, pass --create-key, or set LINODE_OS_ACCESS_KEY_ID and LINODE_OS_SECRET_ACCESS_KEY in the environment." >&2
  exit 1
fi

export MERGE_ENV_FILE="$ENV_FILE"
export MS_ACCESS="$ACCESS"
export MS_SECRET="$SECRET"
export MS_ENDPOINT="$ENDPOINT_URL"
export MS_REGION="${STORAGE_REGION}"
export MS_BUCKET="$BUCKET"

python3 << 'PY'
import os
from pathlib import Path

path = Path(os.environ["MERGE_ENV_FILE"])
updates = {
    "OBJECT_STORAGE_SCHEME": "s3",
    "OBJECT_STORAGE_ENDPOINT": os.environ["MS_ENDPOINT"],
    "OBJECT_STORAGE_REGION": os.environ["MS_REGION"],
    "OBJECT_STORAGE_BUCKET": os.environ["MS_BUCKET"],
    "OBJECT_STORAGE_ACCESS_KEY_ID": os.environ["MS_ACCESS"],
    "OBJECT_STORAGE_SECRET_ACCESS_KEY": os.environ["MS_SECRET"],
    "OBJECT_STORAGE_USE_PATH_STYLE": "false",
}

lines = path.read_text().splitlines()
out = []
seen = set(updates)
for line in lines:
    stripped = line.strip()
    if stripped.startswith("#") or "=" not in line:
        out.append(line)
        continue
    key = line.split("=", 1)[0]
    if key in updates:
        out.append(f"{key}={updates[key]}")
        seen.discard(key)
    else:
        out.append(line)
for key in sorted(seen):
    out.append(f"{key}={updates[key]}")

path.write_text("\n".join(out) + "\n")
PY

echo ""
echo "Updated $ENV_FILE."
echo "Redeploy without MinIO (external Object Storage is default):"
echo "  docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml up -d --build"
