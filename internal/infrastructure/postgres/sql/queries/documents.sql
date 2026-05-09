-- name: UpsertDocument :exec
INSERT INTO documents (
  id, organization_id, project_id, storage_uri, mime_type,
  checksum_sha256, page_count, status, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
  storage_uri = EXCLUDED.storage_uri,
  mime_type = EXCLUDED.mime_type,
  checksum_sha256 = EXCLUDED.checksum_sha256,
  page_count = EXCLUDED.page_count,
  status = EXCLUDED.status;

-- name: ListDocumentsByOrgProject :many
SELECT id, organization_id, project_id, storage_uri, mime_type,
  checksum_sha256, page_count, status, created_at
FROM documents
WHERE organization_id = $1 AND project_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: GetDocumentByID :one
SELECT id, organization_id, project_id, storage_uri, mime_type,
  checksum_sha256, page_count, status, created_at
FROM documents
WHERE id = $1;
