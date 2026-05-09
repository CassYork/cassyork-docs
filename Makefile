.PHONY: sqlc templ migrate-up migrate-down migrate-status compose-db dev-admin-ui install-air

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
