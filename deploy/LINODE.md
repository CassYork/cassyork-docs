# Deploy on Linode (Docker Compose)

Target: one **Linode VM** running Postgres, Temporal, MinIO, and **admin-ui** together. Swap MinIO env for **Linode Object Storage** later by changing object-storage variables only.

## GitHub Actions — deploy on every push to `main`

Workflow: **`.github/workflows/deploy-main.yml`**

On each push to **`main`** (and manual **Actions → Deploy main → Run workflow**), GitHub:

1. **Rsyncs** the repo to the server (same exclusions as `deploy/push-to-linode.sh`; **does not** overwrite **`deploy/.env.linode`** — that file stays only on the VM).
2. Runs **`docker compose … up -d --build`** (rebuilds **admin-ui** / API).
3. Runs **`deploy/scripts/run-migrations-remote.sh`** (pending Goose migrations).

### Repository secrets (Settings → Secrets and variables → Actions)

| Secret | Required | Description |
|--------|----------|-------------|
| **`DEPLOY_HOST`** | Yes | Server public IPv4 or DNS (same host you SSH to). |
| **`DEPLOY_SSH_PRIVATE_KEY`** | Yes | Private key whose **public** half is in **`~/.ssh/authorized_keys`** on the server for the deploy user (often **`root`**). Paste the full PEM, including `BEGIN` / `END` lines. Use a **dedicated key with no passphrase** for Actions (`ssh-keygen -t ed25519 -f deploy_key -N ""`); passphrase-protected keys cannot be loaded in CI. |
| **`DEPLOY_USER`** | No | SSH user (default **`root`**). |
| **`DEPLOY_REMOTE_DIR`** | No | Deploy path on the server (default **`/opt/cassyork-docs`**). |

One-time on the VM: create **`deploy/.env.linode`** from **`deploy/env.linode.example`** with production passwords (Compose reads it on every deploy).

## 1. Create the Linode

### Option A — Linode CLI (scripted)

1. Install [Linode CLI](https://www.linode.com/docs/products/tools/cli/get-started/) (for example `brew install linode-cli` or `pipx install linode-cli`).
2. Authenticate once: `linode-cli configure` (paste a Personal Access Token from Cloud Manager).
3. From the repo root, create an instance and print JSON (with optional `jq` summary):

```bash
make linode-create
# or
bash deploy/linode-create.sh
```

Override defaults with environment variables (see comments at the top of `deploy/linode-create.sh`), for example:

```bash
LINODE_REGION=us-ord LINODE_LABEL=cassyork-prod bash deploy/linode-create.sh
```

For **CI or headless environments**, export **`LINODE_CLI_TOKEN`** (same token as in Cloud Manager) and **`LINODE_ROOT_PASSWORD`** before running the script. With a normal terminal, run **`linode-cli configure`** once and omit `LINODE_CLI_TOKEN`; you can omit `LINODE_ROOT_PASSWORD` to be prompted for the VM root password (avoid putting secrets in shell history when you pass them via env).

### Option B — Cloud Manager

- **Distribution:** Ubuntu 22.04 LTS (or newer).
- **Plan:** At least **2 GB RAM / 1 CPU** for Postgres + Temporal + MinIO + admin-ui (more if you add workers and traffic).
- Note the **public IPv4** address.

## 2. Initial server setup

SSH in as root or a sudo user:

```bash
sudo apt update && sudo apt install -y git ca-certificates curl
# Docker (official convenience script — review https://docs.docker.com/engine/install/ubuntu/ for production hardening)
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
```

Log out and back in so `docker` works without `sudo`.

## 3. Firewall (recommended)

```bash
sudo ufw allow OpenSSH
sudo ufw allow 8095/tcp    # admin-ui (or 80/443 if you put Caddy/NGINX in front)
sudo ufw enable
```

Use **8095** only on the LAN or behind Cloudflare / VPN unless you add TLS termination.

## 4. Clone and configure

### On the server (git clone)

```bash
git clone <your-repo-url> cassyork-docs
cd cassyork-docs
cp deploy/env.linode.example deploy/.env.linode
nano deploy/.env.linode   # set POSTGRES_PASSWORD, OBJECT_STORAGE_SECRET_ACCESS_KEY, etc.
```

### From your laptop (rsync + compose)

Use this when you develop locally and want the same tree on the VM without cloning there:

1. Copy `deploy/linode-connection.env.example` → **`deploy/linode-connection.env`** (gitignored) and set **`LINODE_HOST`** to the instance public IPv4 (`make linode-status` lists Cassyork-labeled instances).
2. Copy **`deploy/env.linode.example`** → **`deploy/.env.linode`** and fill secrets on your laptop — **`make linode-push`** rsyncs the repo and copies **`deploy/.env.linode`** to the server, then runs Compose.

```bash
make linode-push
```

Requires **SSH** to `root@LINODE_HOST` (same key Linode CLI stored under `authorized_users` when you configured defaults). Install Docker on the VM first (section 2) if needed.

## 5. Start the stack

From the **repository root**:

```bash
docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml up -d --build
docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml ps
```

The Compose file starts **Postgres, Temporal, admin-ui** by default. **MinIO is optional** (`profiles: minio`) — use Object Storage (section 10) in production, or add **`--profile minio`** and point **`OBJECT_STORAGE_*`** at **`http://minio:9000`** for a self-contained VM.

## 6. Run database migrations (once per fresh volume)

Use Goose **v3** against the `cassyork` database (the Hub image `pressly/goose` is unreliable; use a one-shot Go toolchain container):

```bash
PW="$(grep '^POSTGRES_PASSWORD=' deploy/.env.linode | cut -d= -f2-)"
docker run --rm --network cassyork-linode_default \
  -v "$(pwd)/migrations:/migrations:ro" \
  golang:1.23-alpine \
  sh -ceu "go install github.com/pressly/goose/v3/cmd/goose@v3.24.1 && \
    /go/bin/goose -dir /migrations postgres \"postgres://temporal:${PW}@postgresql:5432/cassyork?sslmode=disable\" up"
```

Ensure the Compose network name matches (`docker network ls`; default project name **`cassyork-linode`**).

Alternatively, from your laptop with Postgres forwarded:

```bash
POSTGRES_URL=postgres://temporal:...@LINODE_IP:5432/cassyork?sslmode=disable make migrate-up
```

(open port **5432** only temporarily or use SSH tunnel).

## 7. Smoke test

```bash
curl -sS http://LINODE_IP:8095/healthz
```

Open in a browser:

`http://LINODE_IP:8095/orgs/org_demo/projects/proj_demo/dashboard`

## 8. Python Temporal worker (required for real ingestion)

The Go stack starts workflows; **workers must run** to execute activities. On the same VM or another host:

```bash
cd cassyork-docs/python
# configure env to match POSTGRES_URL, TEMPORAL_ADDRESS, MinIO/S3
pip install -e .
PYTHONPATH=src python -m cassyork_workers.worker_main
```

Prefer **systemd** or a second Compose service for production.

## 9. HTTPS and hostname (optional)

Put **Caddy** or **NGINX** on ports 80/443 and reverse-proxy to `admin-ui:8095`, or use Linode **NodeBalancer** + TLS termination.

## 10. Linode Object Storage (recommended)

Create a **bucket** in Cloud Manager (or with `aws s3 mb` against your cluster endpoint). Note the **region** your bucket uses (for Newark-style clusters this is often API region **`us-east`**, endpoint host **`us-east-1.linodeobjects.com`**).

### Configure with Linode CLI

From the repo root (requires **`linode-cli configure`**):

```bash
# Inspect clusters / endpoints (optional)
linode-cli object-storage endpoints --json | jq '.[] | select(.s3_endpoint!=null)'

# Merge HTTPS endpoint + credentials into deploy/.env.linode (creates a new access key):
export LINODE_OS_BUCKET="your-bucket-name"
export LINODE_OS_REGION="us-east"   # API region from endpoints listing
./deploy/scripts/configure-linode-object-storage.sh --apply --create-key
```

Or supply existing keys instead of `--create-key`:

```bash
export LINODE_OS_ACCESS_KEY_ID="..."
export LINODE_OS_SECRET_ACCESS_KEY="..."
./deploy/scripts/configure-linode-object-storage.sh --apply
```

The script sets **`OBJECT_STORAGE_USE_PATH_STYLE=false`** and **`OBJECT_STORAGE_ENDPOINT=https://…`** for virtual-hosted style.

Redeploy **without** the MinIO container (default):

```bash
docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml up -d --build
```

Optional local MinIO only for debugging:

```bash
# In deploy/.env.linode use OBJECT_STORAGE_ENDPOINT=http://minio:9000 and matching keys.
docker compose --profile minio --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml up -d --build
```

Align bucket **CORS** with your browser origin if the UI loads artifacts directly from Object Storage.
