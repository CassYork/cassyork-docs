// Package events holds domain event identifiers for cross-cutting use (logs, outbox later).
package events

const (
	DocumentRegistered        = "document.registered"
	IngestionRunCreated       = "ingestion_run.created"
	IngestionRunStarted       = "ingestion_run.started"
	IngestionRunCompleted     = "ingestion_run.completed"
	IngestionRunFailed        = "ingestion_run.failed"
	ModelExtractionCompleted  = "model.extraction.completed"
	SchemaValidationFailed    = "schema.validation.failed"
	EvaluationCompleted       = "evaluation.completed"
)
