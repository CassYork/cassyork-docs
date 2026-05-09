package document

import (
	"errors"
	"time"
)

var ErrUnknownDocumentStatus = errors.New("unknown document status")

// RehydrateDocument restores aggregate from persistence.
func RehydrateDocument(
	id, organizationID, projectID, storageURI, mimeType, checksumSHA256 string,
	pageCount int,
	status Status,
	createdAt time.Time,
) (*Document, error) {
	if id == "" || organizationID == "" || projectID == "" || storageURI == "" {
		return nil, ErrInvalidDocument
	}
	return &Document{
		id:             id,
		organizationID: organizationID,
		projectID:      projectID,
		storageURI:     storageURI,
		mimeType:       mimeType,
		checksumSHA256: checksumSHA256,
		pageCount:      pageCount,
		status:         status,
		createdAt:      createdAt.UTC(),
	}, nil
}

// ParseDocumentStatus maps persistence text to Status.
func ParseDocumentStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusUploaded, StatusStored:
		return Status(s), nil
	default:
		return "", ErrUnknownDocumentStatus
	}
}
