package workflows

// IngestionWorkflowInput is serialized JSON-ish via Temporal — keep fields stable for Python worker.
type IngestionWorkflowInput struct {
	OrganizationID string `json:"organization_id"`
	ProjectID      string `json:"project_id"`
	DocumentID     string `json:"document_id"`
	IngestionRunID string `json:"ingestion_run_id"`
	StorageURI     string `json:"storage_uri"`
	MimeType       string `json:"mime_type"`
	PipelineID     string `json:"pipeline_id"`
	SchemaID       string `json:"schema_id"`
	ModelConfigID  string `json:"model_config_id"`
	PromptVersionID string `json:"prompt_version_id,omitempty"`
	TraceID        string `json:"trace_id"`
}
