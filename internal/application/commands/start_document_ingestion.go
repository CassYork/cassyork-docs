package commands

import (
	"context"
	"fmt"
	"time"

	"cassyork.dev/platform/internal/application/ports"
	"cassyork.dev/platform/internal/domain/document"
	"cassyork.dev/platform/internal/domain/ingestion"
	"cassyork.dev/platform/internal/workflows"
)

// StartDocumentIngestionCommand is the use case input — thin HTTP maps here.
type StartDocumentIngestionCommand struct {
	OrganizationID  string
	ProjectID       string
	DocumentID      string
	IngestionRunID  string
	StorageURI      string
	MimeType        string
	ChecksumSHA256  string
	PipelineID      string
	SchemaID        string
	ModelConfigID   string
	PromptVersionID string
	TraceID         string
	Now             time.Time
}

type StartDocumentIngestionResult struct {
	DocumentID      string
	IngestionRunID  string
	Status          string
	TraceID         string
	TemporalWorkflowID string
}

type StartDocumentIngestionHandler struct {
	Documents ports.DocumentRepository
	Runs      ports.RunRepository
	Workflow  ports.WorkflowStarter
}

func (h StartDocumentIngestionHandler) Handle(ctx context.Context, cmd StartDocumentIngestionCommand) (StartDocumentIngestionResult, error) {
	if cmd.Now.IsZero() {
		cmd.Now = time.Now().UTC()
	}
	doc, err := document.NewDocument(
		cmd.DocumentID,
		cmd.OrganizationID,
		cmd.ProjectID,
		cmd.StorageURI,
		cmd.MimeType,
		cmd.ChecksumSHA256,
		0,
		cmd.Now,
	)
	if err != nil {
		return StartDocumentIngestionResult{}, err
	}

	run, err := ingestion.NewQueuedRun(
		cmd.IngestionRunID,
		cmd.OrganizationID,
		cmd.ProjectID,
		doc.ID(),
		cmd.PipelineID,
		cmd.SchemaID,
		cmd.ModelConfigID,
		cmd.PromptVersionID,
		cmd.TraceID,
		cmd.Now,
	)
	if err != nil {
		return StartDocumentIngestionResult{}, err
	}

	if err := h.Documents.Save(ctx, doc); err != nil {
		return StartDocumentIngestionResult{}, fmt.Errorf("save document: %w", err)
	}
	if err := h.Runs.Save(ctx, run); err != nil {
		return StartDocumentIngestionResult{}, fmt.Errorf("save run: %w", err)
	}

	in := workflows.IngestionWorkflowInput{
		OrganizationID:  cmd.OrganizationID,
		ProjectID:       cmd.ProjectID,
		DocumentID:      doc.ID(),
		IngestionRunID:  run.ID(),
		StorageURI:      doc.StorageURI(),
		MimeType:        doc.MimeType(),
		PipelineID:      cmd.PipelineID,
		SchemaID:        cmd.SchemaID,
		ModelConfigID:   cmd.ModelConfigID,
		PromptVersionID: cmd.PromptVersionID,
		TraceID:         cmd.TraceID,
	}
	if err := h.Workflow.StartDocumentIngestion(ctx, in); err != nil {
		return StartDocumentIngestionResult{}, fmt.Errorf("start workflow: %w", err)
	}

	return StartDocumentIngestionResult{
		DocumentID:           doc.ID(),
		IngestionRunID:       run.ID(),
		Status:               string(run.Status()),
		TraceID:              cmd.TraceID,
		TemporalWorkflowID:   run.ID(),
	}, nil
}
