package ingestion

import (
	"errors"
	"time"
)

var (
	ErrInvalidRun          = errors.New("invalid ingestion run")
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrImmutableCompletion = errors.New("completed run cannot change")
)

// Run is one processing attempt for a document — separate aggregate from Document.
type Run struct {
	id              string
	organizationID  string
	projectID       string
	documentID      string
	pipelineID      string
	schemaID        string
	modelConfigID   string
	promptVersionID string
	status          Status
	traceID         string
	startedAt       *time.Time
	completedAt     *time.Time
	failedAt        *time.Time
	errorMessage    string
}

func NewQueuedRun(
	id, organizationID, projectID, documentID, pipelineID, schemaID, modelConfigID, promptVersionID, traceID string,
	now time.Time,
) (*Run, error) {
	if id == "" || organizationID == "" || projectID == "" || documentID == "" || traceID == "" {
		return nil, ErrInvalidRun
	}
	_ = now
	return &Run{
		id:              id,
		organizationID: organizationID,
		projectID:       projectID,
		documentID:      documentID,
		pipelineID:      pipelineID,
		schemaID:        schemaID,
		modelConfigID:   modelConfigID,
		promptVersionID: promptVersionID,
		status:          StatusQueued,
		traceID:         traceID,
	}, nil
}

func (r *Run) ID() string                  { return r.id }
func (r *Run) OrganizationID() string      { return r.organizationID }
func (r *Run) ProjectID() string           { return r.projectID }
func (r *Run) DocumentID() string          { return r.documentID }
func (r *Run) PipelineID() string          { return r.pipelineID }
func (r *Run) SchemaID() string            { return r.schemaID }
func (r *Run) ModelConfigID() string       { return r.modelConfigID }
func (r *Run) PromptVersionID() string    { return r.promptVersionID }
func (r *Run) Status() Status              { return r.status }
func (r *Run) TraceID() string             { return r.traceID }
func (r *Run) StartedAt() *time.Time       { return r.startedAt }
func (r *Run) CompletedAt() *time.Time     { return r.completedAt }
func (r *Run) FailedAt() *time.Time        { return r.failedAt }
func (r *Run) ErrorMessage() string        { return r.errorMessage }

func (r *Run) MarkStarted(now time.Time) error {
	if r.status == StatusCompleted {
		return ErrImmutableCompletion
	}
	switch r.status {
	case StatusQueued:
		r.status = StatusRunning
		t := now.UTC()
		r.startedAt = &t
		return nil
	default:
		return ErrInvalidTransition
	}
}

func (r *Run) MarkWaitingOnProvider(now time.Time) error {
	if r.status == StatusCompleted {
		return ErrImmutableCompletion
	}
	if r.status != StatusRunning {
		return ErrInvalidTransition
	}
	r.status = StatusWaitingOnProvider
	_ = now
	return nil
}

func (r *Run) MarkValidating(now time.Time) error {
	if r.status == StatusCompleted {
		return ErrImmutableCompletion
	}
	r.status = StatusValidating
	_ = now
	return nil
}

func (r *Run) MarkRequiresReview(now time.Time) error {
	if r.status == StatusCompleted {
		return ErrImmutableCompletion
	}
	r.status = StatusRequiresReview
	_ = now
	return nil
}

func (r *Run) MarkCompleted(now time.Time) error {
	if r.status == StatusCompleted {
		return ErrImmutableCompletion
	}
	switch r.status {
	case StatusRunning, StatusWaitingOnProvider, StatusValidating, StatusRequiresReview:
		r.status = StatusCompleted
		t := now.UTC()
		r.completedAt = &t
		return nil
	default:
		return ErrInvalidTransition
	}
}

func (r *Run) MarkFailed(now time.Time, message string) error {
	if r.status == StatusCompleted {
		return ErrImmutableCompletion
	}
	switch r.status {
	case StatusQueued, StatusRunning, StatusWaitingOnProvider, StatusValidating:
		r.status = StatusFailed
		t := now.UTC()
		r.failedAt = &t
		r.errorMessage = message
		return nil
	default:
		return ErrInvalidTransition
	}
}
