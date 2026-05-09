package document

// Status is lifecycle for the stored artifact only — not extraction status.
type Status string

const (
	StatusUploaded Status = "uploaded"
	StatusStored   Status = "stored"
)
