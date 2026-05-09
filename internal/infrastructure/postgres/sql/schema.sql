-- Canonical schema for sqlc code generation (keep aligned with migrations/00001_document_and_runs.sql via Goose).

CREATE TABLE documents (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  storage_uri TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  checksum_sha256 TEXT,
  page_count INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE ingestion_runs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  document_id TEXT NOT NULL REFERENCES documents (id) ON DELETE RESTRICT,
  pipeline_id TEXT NOT NULL DEFAULT '',
  schema_id TEXT NOT NULL DEFAULT '',
  model_config_id TEXT NOT NULL DEFAULT '',
  prompt_version_id TEXT,
  trace_id TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
);

CREATE INDEX idx_ingestion_runs_document_id ON ingestion_runs (document_id);

CREATE INDEX idx_ingestion_runs_org_project ON ingestion_runs (organization_id, project_id);
