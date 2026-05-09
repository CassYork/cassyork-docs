package adminui

// Navigation section keys (MVP sidebar).
const (
	NavDashboard   = "dashboard"
	NavDocuments   = "documents"
	NavReview      = "review"
	NavEvaluations = "evaluations"
	NavPipelines   = "pipelines"
	NavModels      = "models"
	NavSchemas     = "schemas"
	NavSettings    = "settings"
)

// OrgScope is the tenant slice for list views (query-param scoped).
type OrgScope struct {
	OrganizationID string
	ProjectID      string
}

// DashboardVM is the control-plane dashboard.
type DashboardVM struct {
	Scope          OrgScope
	KPI            DashboardKPI
	StatusCounts   StatusRollup
	RecentFailures []RunSummary
	Documents      []DocumentSummary
	Runs           []RunSummary
}

// DashboardKPI mixes aggregates available from DB today with placeholders for analytics-backed metrics.
type DashboardKPI struct {
	DocumentsProcessed      int
	StraightThroughPct      float64
	HumanReviewPct          float64
	AvgGroundTruthPct       *float64 // nil → show em dash
	SchemaFailurePct        *float64
	AvgCostPerDocUSD        *float64
	AvgProcessingSeconds    *float64
	AnalyticsMetricsPending bool
}

// StatusRollup counts ingestion runs in the loaded window by status.
type StatusRollup struct {
	Queued         int
	Running        int
	RequiresReview int
	Completed      int
	Failed         int
	Other          int
	Total          int
}

// DocumentSummary is a compact row for fragments and lists.
type DocumentSummary struct {
	ID         string
	Status     string
	MimeType   string
	CreatedAt  string
	StorageURI string
}

// RunSummary is a compact ingestion run row.
type RunSummary struct {
	ID           string
	DocumentID   string
	Status       string
	TraceID      string
	PipelineID   string
	CreatedAt    string
	ErrorPreview string
}

// OpsDocumentRow is the documents operations table row.
type OpsDocumentRow struct {
	DocumentID   string
	DisplayName  string
	Status       string
	DocumentType string
	ProjectID    string
	LatestRunID  string
	Model        string
	Confidence   string
	Validation   string
	ReviewStatus string
	CreatedAt    string
}

// DocumentDetailVM is the split-pane document workspace (viewer + extraction).
type DocumentDetailVM struct {
	Scope           OrgScope
	Document        DocumentSummary
	FullArtifactURI string
	LatestRun       *RunSummary
	RunHistory      []RunSummary
}

// RunDetailVM is technical run inspection (timeline + summary).
type RunDetailVM struct {
	Scope OrgScope
	Run   RunSummary
	Doc   DocumentSummary
}

// DocumentsPageVM is the main operations document list + ingest toolbar.
type DocumentsPageVM struct {
	Scope OrgScope
	Rows  []OpsDocumentRow
}

// PlaceholderPageVM is a routed shell page pending backend APIs (review, eval, etc.).
type PlaceholderPageVM struct {
	Scope       OrgScope
	Title       string
	Description string
	Bullets     []string
}

// SettingsVM is workspace defaults + runtime integration (read-only, secrets redacted).
type SettingsVM struct {
	Scope            OrgScope
	SavedWorkspace   bool
	ClearedWorkspace bool
	Runtime          SettingsRuntimeVM
	WebhookHint      string
}

// SettingsRuntimeVM summarizes effective environment wiring shown to operators.
type SettingsRuntimeVM struct {
	ListenAddr           string
	AdminUIListLimitDesc string
	DatabaseDisplay      string
	TemporalAddress      string
	TemporalNamespace    string
	TemporalUILink       string
	OTELExporter         string
	ObjectStorage        SettingsObjectStorageVM
}

// SettingsObjectStorageVM is redacted object-storage configuration.
type SettingsObjectStorageVM struct {
	Scheme           string
	Endpoint         string
	Region           string
	Bucket           string
	AccessKeyMasked  string
	SecretConfigured bool
	UsePathStyle     bool
}
