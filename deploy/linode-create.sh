#!/usr/bin/env bash
# Create a Linode VM for Cassyork docs via Akamai Linode CLI (linode-cli).
#
# Prerequisites:
#   - Install: https://www.linode.com/docs/products/tools/cli/get-started/
#     e.g. brew install linode-cli   OR   pipx install linode-cli
#   - Authenticate once: linode-cli configure   (paste Personal Access Token)
#
# Environment (optional):
#   LINODE_LABEL              Instance label (default: cassyork-docs-<unix_ts>)
#   LINODE_REGION             Region slug (default: us-east)
#   LINODE_TYPE               Plan slug (default: g6-standard-2 — ~2 GB RAM)
#   LINODE_IMAGE              Image slug (default: linode/ubuntu22.04)
#   LINODE_ROOT_PASSWORD      Root password (omit to be prompted). API requires length 11–128 and Linode strength rules.
#   LINODE_CLI_TOKEN          Personal Access Token (skips config file; required in CI/headless)
#
# Examples:
#   ./deploy/linode-create.sh
#   LINODE_REGION=us-ord LINODE_LABEL=my-vm ./deploy/linode-create.sh
#   LINODE_ROOT_PASSWORD='...' LINODE_TYPE=g6-nanode-1 ./deploy/linode-create.sh

set -euo pipefail

if ! command -v linode-cli >/dev/null 2>&1; then
  echo "linode-cli not found. Install it (e.g. brew install linode-cli or pipx install linode-cli), then run: linode-cli configure" >&2
  exit 1
fi

linode_cli_configured() {
  [[ -n "${LINODE_CLI_TOKEN:-}" ]] && return 0
  local f
  for f in "${HOME}/.config/linode-cli" "${HOME}/.linode-cli"; do
    if [[ -f "$f" ]] && grep -qE '^[[:space:]]*default-user[[:space:]]*=' "$f" 2>/dev/null; then
      return 0
    fi
  done
  return 1
}

# linode-cli opens a browser / reads stdin when unconfigured; agents and pipes have no TTY.
if [[ ! -t 0 ]]; then
  if ! linode_cli_configured; then
    echo "No TTY: export LINODE_CLI_TOKEN=<Personal Access Token> from Cloud Manager, or run linode-cli configure in an interactive terminal." >&2
    exit 1
  fi
  if [[ -z "${LINODE_ROOT_PASSWORD:-}" ]]; then
    echo "No TTY: set LINODE_ROOT_PASSWORD for the new VM root account (or run this script in a terminal to be prompted)." >&2
    exit 1
  fi
fi

LABEL="${LINODE_LABEL:-cassyork-docs-$(date +%s)}"
REGION="${LINODE_REGION:-us-east}"
TYPE="${LINODE_TYPE:-g6-standard-2}"
IMAGE="${LINODE_IMAGE:-linode/ubuntu22.04}"

echo "Creating Linode:"
echo "  label:  $LABEL"
echo "  region: $REGION"
echo "  type:   $TYPE"
echo "  image:  $IMAGE"
echo ""

cmd=(linode-cli linodes create
  --label "$LABEL"
  --region "$REGION"
  --type "$TYPE"
  --image "$IMAGE"
)

if [[ -n "${LINODE_ROOT_PASSWORD:-}" ]]; then
  cmd+=(--root_pass "$LINODE_ROOT_PASSWORD")
else
  cmd+=(--root_pass)
fi

cmd+=(--json)

out="$("${cmd[@]}")" || exit $?

echo "$out"
echo ""

if command -v jq >/dev/null 2>&1; then
  # linode-cli --json may return one object or a single-element array.
  id="$(echo "$out" | jq -r '(if type == "array" then .[0] else . end) | .id // empty')"
  ipv4="$(echo "$out" | jq -r '(if type == "array" then .[0] else . end) | .ipv4[0] // empty')"
  if [[ -n "$id" && "$id" != null ]]; then
    echo "Summary:"
    echo "  id:   $id"
    [[ -n "$ipv4" && "$ipv4" != null ]] && echo "  ipv4: $ipv4"
    echo ""
    echo "Next: SSH as root when provisioning finishes, then follow deploy/LINODE.md (Docker, clone repo, deploy/.env.linode, make linode-up)."
  fi
else
  echo "Tip: install jq to print id/ipv4 from JSON output."
fi
