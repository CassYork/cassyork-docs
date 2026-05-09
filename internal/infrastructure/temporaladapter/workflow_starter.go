package temporaladapter

import (
	"context"

	"go.temporal.io/sdk/client"

	"cassyork.dev/platform/internal/workflows"
)

// ClientStarter implements application/ports.WorkflowStarter using Temporal.
type ClientStarter struct {
	Client client.Client
}

func (c ClientStarter) StartDocumentIngestion(ctx context.Context, in workflows.IngestionWorkflowInput) error {
	_, err := c.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        in.IngestionRunID,
		TaskQueue: workflows.DocumentProcessingTaskQueue,
	}, workflows.DocumentIngestionWorkflowName, in)
	return err
}
