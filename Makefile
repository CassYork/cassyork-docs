.PHONY: sqlc templ migrate-up migrate-down migrate-status compose-db dev-admin-ui install-air linode-create linode-bootstrap linode-status linode-ssh linode-push linode-migrate-remote linode-os-config linode-up linode-down linode-logs

GOOSE ?= go run github.com/pressly/goose/v3/cmd/goose@v3.24.1

# Postgres + Temporal for local admin-ui / ingestion (requires Docker: OrbStack or Docker Desktop).
compose-db:
	docker compose up -d postgresql temporal

AIR ?= go run github.com/air-verse/air@v1.61.1

# Live reload admin-ui: templ generate + rebuild + restart on .go / .templ changes.
dev-admin-ui:
	$(AIR) -c .air.toml

# Install air globally (optional); preferred path is `make dev-admin-ui` using go run.
install-air:
	go install github.com/air-verse/air@v1.61.1

# Create a Linode VM with linode-cli (see deploy/LINODE.md). Requires: linode-cli + linode-cli configure.
linode-create:
	bash deploy/linode-create.sh

# Full remote bootstrap (shutdown → reset root password → Docker → rsync → compose → migrations).
# Requires: linode-cli + LINODE_ID / LINODE_IP (see deploy/scripts/bootstrap-linode-remote.sh).
linode-bootstrap:
	bash deploy/scripts/bootstrap-linode-remote.sh

# Show Cassyork Linode(s) from the API (requires linode-cli configure).
linode-status:
	@linode-cli linodes list --json | jq -r '.[] | select(.label|test("cassyork")) | "\(.label)\t\(.ipv4[0])\t\(.status)\tid=\(.id)"' \
		|| linode-cli linodes list

# SSH to VM — set LINODE_HOST or create deploy/linode-connection.env from deploy/linode-connection.env.example.
linode-ssh:
	bash deploy/linode-ssh.sh

# Rsync repo + deploy/.env.linode + compose up + migrations on the VM.
linode-push:
	bash deploy/push-to-linode.sh

# Run Goose migrations only on the VM (uses ~/.ssh/id_ed25519 + deploy/linode-connection.env).
linode-migrate-remote:
	bash deploy/linode-migrate-remote.sh

# Merge Linode Object Storage settings into deploy/.env.linode via linode-cli (needs LINODE_OS_BUCKET, etc.).
linode-os-config:
	bash deploy/scripts/configure-linode-object-storage.sh

# Linode / production Compose (requires deploy/.env.linode — copy deploy/env.linode.example).
linode-up:
	docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml up -d --build

linode-down:
	docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml down

linode-logs:
	docker compose --env-file deploy/.env.linode -f deploy/docker-compose.linode.yml logs -f admin-ui

templ:
	go run github.com/a-h/templ/cmd/templ@latest generate ./internal/adminui

# Prefer DATABASE_URL; fall back to POSTGRES_URL for older env files.
DB_URL = $(or $(DATABASE_URL),$(POSTGRES_URL),postgres://temporal:temporal@127.0.0.1:5432/cassyork?sslmode=disable)

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate

migrate-up:
	$(GOOSE) -dir migrations postgres "$(DB_URL)" up

migrate-down:
	$(GOOSE) -dir migrations postgres "$(DB_URL)" down

migrate-status:
	$(GOOSE) -dir migrations postgres "$(DB_URL)" status
