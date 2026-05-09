# Platform split (Go APIs + Python workers)

This repository implements a **document AI ingestion control plane** with strict separation between edge/control-plane APIs (Go) and ML/document intelligence (Python), orchestrated by **Temporal**, with **Postgres** for authoritative state, **S3-compatible object storage** for blobs, **OpenTelemetry** for traces, and **Grafana** (with Jaeger) for visualization.

## Go (customer-facing and internal HTTP)

| Binary | Role |
|--------|------|
| `customer-api` | Public REST contract (`/v1/...`): documents, runs, ground truth, comparisons. |
| `ingestion-api` | Uploads / register-from-URI, persists metadata, writes to S3, **starts Temporal workflows**. |
| `status-api` | Read-heavy status polling; optional caching layer later. |
| `webhook-api` | Outbound webhook scheduling, retries, signing. |
| `internal-api` | Service-to-service and admin operations (catalog, flags, maintenance). |

Shared Go packages: `internal/config`, `internal/httphelp`, `internal/observability` (OTLP gRPC), `internal/temporalio`, `internal/workflows` (payload structs + workflow/task-queue names aligned with Python).

## Python (Temporal workers)

Package: `python/src/cassyork_workers`.

Owns **model adapters**, prompt experiments, **document parsing**, **JSON Schema validation**, **evaluation / ground-truth comparison**, confidence scoring, and future **human-review** steps — implemented as **Temporal activities** and composed in **`DocumentIngestionWorkflow`**.

Workflow and activity names must stay aligned with Go constants in `internal/workflows/names.go`.

## Cross-language contracts

- **Task queue**: `document-processing`
- **Workflow type**: `DocumentIngestionWorkflow`
- **Payload**: `IngestionWorkflowInput` / `IngestionWorkflowInput` (Pydantic) — mirror of `internal/workflows/payloads.go`

## Configuration (`internal/config`)

- **`DATABASE_URL`** — primary relational DB connection string (opaque to app code). **`POSTGRES_URL`** is still read if `DATABASE_URL` is unset.
- **`OBJECT_STORAGE_*`** — blob endpoints and credentials (`OBJECT_STORAGE_BUCKET`, `OBJECT_STORAGE_ENDPOINT`, …). Legacy **`S3_*`** keys still map to the same settings.
- **`ArtifactURI`** builds document storage references from `OBJECT_STORAGE_SCHEME` (defaults to `s3` for common S3-compatible APIs); domain **`storage_uri`** stays an opaque string.

## Data stores

- **Relational DB**: organizations, projects, documents, runs, schemas, prompts, webhooks, audit (tables evolve via Goose).
- **Object storage**: raw uploads, raw outputs, artifacts (implementation-specific; URIs are logical references).

## Observability

- Set `OTEL_EXPORTER_OTLP_ENDPOINT` (gRPC, typically `:4317`) on Go binaries and the Python worker (`OTEL_SERVICE_NAME` per process).
- `docker-compose.yml` runs **OTel Collector → Jaeger** and **Grafana** with a Jaeger datasource.

## sqlc

Persistence queries are generated from SQL ([sqlc](https://sqlc.dev/)):

- Config: `sqlc.yaml` at repo root.
- Schema (for compile-time checking): `internal/infrastructure/postgres/sql/schema.sql` — **keep in sync** with Goose migrations in `migrations/*.sql`.
- Queries: `internal/infrastructure/postgres/sql/queries/*.sql`.
- Generated Go: `internal/infrastructure/postgres/sqlcgen/` (checked in; do not hand-edit).

Regenerate after schema/query changes:

```bash
make sqlc
# or: go generate ./internal/infrastructure/postgres/...
```

## Goose (migrations)

Schema changes ship as [Goose](https://github.com/pressly/goose) SQL migrations in `migrations/`.

After Postgres is up and the `cassyork` database exists (`deploy/postgres-init/01-create-cassyork-db.sql` via Docker, or create DB manually):

```bash
make migrate-up      # apply pending migrations
make migrate-status  # show applied / pending
make migrate-down    # roll back last migration (dev only)
```

Override connection: `DATABASE_URL=postgres://... make migrate-up` (legacy: `POSTGRES_URL`).

## DDD (light)

- **Go**: `internal/domain` (Document, Ingestion `Run` aggregates + event name constants), `internal/application/commands` + `internal/application/ports`, `internal/infrastructure` (Temporal adapter, Postgres via sqlc). HTTP handlers stay thin and call command handlers.
- **Python**: `domain/` (aggregates, validation rules, provider **ports**), `application/services` (pipeline orchestration), `infrastructure/` (stub provider, Postgres load/save for runs). **Domain never imports infrastructure**; activities only adapt Temporal ↔ application services.
- **Non‑negotiable**: `Document` ≠ `IngestionRun`; completed runs are immutable; reruns are new runs.

## Local development

1. `docker compose up -d` — Postgres, Temporal (+ UI on `:8088`), MinIO (`:9000`), collector, Jaeger UI (`:16686`), Grafana (`:3000`). Postgres init creates only the **`cassyork` database** (`deploy/postgres-init/01-create-cassyork-db.sql`). Then apply schema: **`make migrate-up`** (Goose).
2. Export env: `TEMPORAL_ADDRESS=localhost:7233`, **`DATABASE_URL`** (primary; **`POSTGRES_URL`** still accepted as fallback) for the same DB as **`make migrate-up`**, and **`OBJECT_STORAGE_*`** for blob storage (legacy **`S3_*`** keys still work). **Ingestion API** and **Python worker** must share the same database URL.
3. Run Go binaries from repo root with distinct `LISTEN_ADDR` values (defaults in each `main.go`).
4. Run Python worker: `PYTHONPATH=python/src python -m cassyork_workers.worker_main`
