package ports

import (
	"context"

	"cassyork.dev/platform/internal/domain/document"
	"cassyork.dev/platform/internal/domain/ingestion"
)

// DocumentRepository persists the Document aggregate (implementation is infrastructure-specific).
type DocumentRepository interface {
	Save(ctx context.Context, d *document.Document) error
}

// RunRepository persists the IngestionRun aggregate.
type RunRepository interface {
	Save(ctx context.Context, r *ingestion.Run) error
}
