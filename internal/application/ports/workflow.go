package ports

import (
	"context"

	"cassyork.dev/platform/internal/workflows"
)

// WorkflowStarter starts asynchronous ingestion orchestration (Temporal).
type WorkflowStarter interface {
	StartDocumentIngestion(ctx context.Context, in workflows.IngestionWorkflowInput) error
}
