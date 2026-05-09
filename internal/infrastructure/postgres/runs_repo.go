package postgres

import (
	"context"

	"cassyork.dev/platform/internal/domain/ingestion"
	"cassyork.dev/platform/internal/infrastructure/postgres/sqlcgen"
)

type RunRepository struct {
	q *sqlcgen.Queries
}

func NewRunRepository(q *sqlcgen.Queries) *RunRepository {
	return &RunRepository{q: q}
}

func (r *RunRepository) Save(ctx context.Context, run *ingestion.Run) error {
	return r.q.UpsertIngestionRun(ctx, sqlcgen.UpsertIngestionRunParams{
		ID:              run.ID(),
		OrganizationID:  run.OrganizationID(),
		ProjectID:       run.ProjectID(),
		DocumentID:      run.DocumentID(),
		PipelineID:      run.PipelineID(),
		SchemaID:        run.SchemaID(),
		ModelConfigID:   run.ModelConfigID(),
		PromptVersionID: pgTextOptional(run.PromptVersionID()),
		TraceID:         run.TraceID(),
		Status:          string(run.Status()),
		StartedAt:       pgTimestamptzPtr(run.StartedAt()),
		CompletedAt:     pgTimestamptzPtr(run.CompletedAt()),
		FailedAt:        pgTimestamptzPtr(run.FailedAt()),
		ErrorMessage:    pgTextOptional(run.ErrorMessage()),
	})
}
