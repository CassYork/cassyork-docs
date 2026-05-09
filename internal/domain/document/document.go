package document

import (
	"errors"
	"time"
)

var ErrInvalidDocument = errors.New("invalid document")

// AggregateRoot for the uploaded artifact. Knows nothing about models, prompts, or runs.
type Document struct {
	id             string
	organizationID string
	projectID      string
	storageURI     string
	mimeType       string
	checksumSHA256 string
	pageCount      int
	status         Status
	createdAt      time.Time
}

func NewDocument(
	id, organizationID, projectID, storageURI, mimeType string,
	checksumSHA256 string,
	pageCount int,
	createdAt time.Time,
) (*Document, error) {
	if id == "" || organizationID == "" || projectID == "" || storageURI == "" {
		return nil, ErrInvalidDocument
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	st := StatusStored
	if checksumSHA256 == "" {
		st = StatusUploaded
	}
	return &Document{
		id:             id,
		organizationID: organizationID,
		projectID:      projectID,
		storageURI:     storageURI,
		mimeType:       mimeType,
		checksumSHA256: checksumSHA256,
		pageCount:      pageCount,
		status:         st,
		createdAt:      createdAt.UTC(),
	}, nil
}

func (d *Document) ID() string             { return d.id }
func (d *Document) OrganizationID() string { return d.organizationID }
func (d *Document) ProjectID() string      { return d.projectID }
func (d *Document) StorageURI() string     { return d.storageURI }
func (d *Document) MimeType() string       { return d.mimeType }
func (d *Document) ChecksumSHA256() string { return d.checksumSHA256 }
func (d *Document) PageCount() int         { return d.pageCount }
func (d *Document) Status() Status         { return d.status }
func (d *Document) CreatedAt() time.Time   { return d.createdAt }

func (d *Document) RecordPagesStored(count int) {
	if count > 0 {
		d.pageCount = count
	}
	d.status = StatusStored
}
