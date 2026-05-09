-- name: UpsertIngestionRun :exec
INSERT INTO ingestion_runs (
  id, organization_id, project_id, document_id,
  pipeline_id, schema_id, model_config_id, prompt_version_id,
  trace_id, status, started_at, completed_at, failed_at, error_message
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (id) DO UPDATE SET
  pipeline_id = EXCLUDED.pipeline_id,
  schema_id = EXCLUDED.schema_id,
  model_config_id = EXCLUDED.model_config_id,
  prompt_version_id = EXCLUDED.prompt_version_id,
  trace_id = EXCLUDED.trace_id,
  status = EXCLUDED.status,
  started_at = EXCLUDED.started_at,
  completed_at = EXCLUDED.completed_at,
  failed_at = EXCLUDED.failed_at,
  error_message = EXCLUDED.error_message;

-- name: ListIngestionRunsByOrgProject :many
SELECT id, organization_id, project_id, document_id,
  pipeline_id, schema_id, model_config_id, prompt_version_id,
  trace_id, status, started_at, completed_at, failed_at, error_message, created_at
FROM ingestion_runs
WHERE organization_id = $1 AND project_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: GetIngestionRunByID :one
SELECT id, organization_id, project_id, document_id,
  pipeline_id, schema_id, model_config_id, prompt_version_id,
  trace_id, status, started_at, completed_at, failed_at, error_message, created_at
FROM ingestion_runs
WHERE id = $1;

-- name: ListIngestionRunsForDocument :many
SELECT id, organization_id, project_id, document_id,
  pipeline_id, schema_id, model_config_id, prompt_version_id,
  trace_id, status, started_at, completed_at, failed_at, error_message, created_at
FROM ingestion_runs
WHERE document_id = $1
ORDER BY created_at DESC
LIMIT $2;
