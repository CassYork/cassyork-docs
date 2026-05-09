package ingestion

import (
	"fmt"
	"time"
)

// RehydrateRun rebuilds aggregate state from persistence (postgres).
func RehydrateRun(
	id, organizationID, projectID, documentID string,
	pipelineID, schemaID, modelConfigID, promptVersionID, traceID string,
	status Status,
	startedAt, completedAt, failedAt *time.Time,
	errorMessage string,
) (*Run, error) {
	if id == "" || organizationID == "" || projectID == "" || documentID == "" || traceID == "" {
		return nil, ErrInvalidRun
	}
	return &Run{
		id:               id,
		organizationID: organizationID,
		projectID:      projectID,
		documentID:     documentID,
		pipelineID:     pipelineID,
		schemaID:       schemaID,
		modelConfigID:  modelConfigID,
		promptVersionID: promptVersionID,
		status:         status,
		traceID:        traceID,
		startedAt:      cloneTime(startedAt),
		completedAt:    cloneTime(completedAt),
		failedAt:       cloneTime(failedAt),
		errorMessage:   errorMessage,
	}, nil
}

// ParseStatus maps stored enum text to Status.
func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusQueued, StatusRunning, StatusWaitingOnProvider, StatusValidating,
		StatusRequiresReview, StatusCompleted, StatusFailed, StatusCancelled:
		return Status(s), nil
	default:
		return "", fmt.Errorf("unknown run status: %q", s)
	}
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := t.UTC()
	return &c
}
