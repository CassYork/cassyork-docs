-- +goose Up
-- Demo workspace registry + sample rows (matches internal/adminui DemoScope: org_demo / proj_demo).

CREATE TABLE organizations (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
);

CREATE TABLE projects (
  organization_id TEXT NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  project_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
  PRIMARY KEY (organization_id, project_id)
);

INSERT INTO organizations (id, display_name)
VALUES ('org_demo', 'Demo organization')
ON CONFLICT (id) DO NOTHING;

INSERT INTO projects (organization_id, project_id, display_name)
VALUES ('org_demo', 'proj_demo', 'Demo project')
ON CONFLICT (organization_id, project_id) DO NOTHING;

INSERT INTO documents (
  id, organization_id, project_id, storage_uri, mime_type,
  checksum_sha256, page_count, status, created_at
) VALUES (
  'seed-doc-welcome',
  'org_demo',
  'proj_demo',
  's3://cassyork-documents/seed/welcome.txt',
  'text/plain',
  NULL,
  0,
  'stored',
  NOW()
) ON CONFLICT (id) DO NOTHING;

INSERT INTO ingestion_runs (
  id, organization_id, project_id, document_id,
  pipeline_id, schema_id, model_config_id, prompt_version_id,
  trace_id, status,
  started_at, completed_at, failed_at, error_message, created_at
) VALUES (
  'seed-run-welcome',
  'org_demo',
  'proj_demo',
  'seed-doc-welcome',
  '', '', '', NULL,
  'seed-trace-welcome',
  'completed',
  NOW (), NOW (), NULL, NULL, NOW ()
) ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM ingestion_runs
WHERE id = 'seed-run-welcome';

DELETE FROM documents
WHERE id = 'seed-doc-welcome';

DROP TABLE IF EXISTS projects;

DROP TABLE IF EXISTS organizations;
