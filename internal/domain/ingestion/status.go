package ingestion

type Status string

const (
	StatusQueued              Status = "queued"
	StatusRunning             Status = "running"
	StatusWaitingOnProvider   Status = "waiting_on_provider"
	StatusValidating          Status = "validating"
	StatusRequiresReview      Status = "requires_review"
	StatusCompleted           Status = "completed"
	StatusFailed              Status = "failed"
	StatusCancelled           Status = "cancelled"
)
