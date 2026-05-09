package postgres

import (
	"context"

	"cassyork.dev/platform/internal/domain/document"
	"cassyork.dev/platform/internal/infrastructure/postgres/sqlcgen"
)

type DocumentRepository struct {
	q *sqlcgen.Queries
}

func NewDocumentRepository(q *sqlcgen.Queries) *DocumentRepository {
	return &DocumentRepository{q: q}
}

func (r *DocumentRepository) Save(ctx context.Context, d *document.Document) error {
	return r.q.UpsertDocument(ctx, sqlcgen.UpsertDocumentParams{
		ID:             d.ID(),
		OrganizationID: d.OrganizationID(),
		ProjectID:      d.ProjectID(),
		StorageUri:     d.StorageURI(),
		MimeType:       d.MimeType(),
		ChecksumSha256: pgTextOptional(d.ChecksumSHA256()),
		PageCount:      int32(d.PageCount()),
		Status:         string(d.Status()),
		CreatedAt:      pgTimestamptz(d.CreatedAt()),
	})
}
