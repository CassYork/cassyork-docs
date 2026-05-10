package adminui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"cassyork.dev/platform/internal/application/commands"
	"cassyork.dev/platform/internal/config"
	"cassyork.dev/platform/internal/infrastructure/blob"
	"cassyork.dev/platform/internal/infrastructure/postgres/sqlcgen"
)

const defaultListLimit = 64

// maxUploadBytes caps multipart uploads on the ingest action (single document).
const maxUploadBytes = 50 << 20 // 50 MiB

const artifactPresignTTL = 15 * time.Minute

// Server serves HTML admin routes backed by Postgres + ingestion commands.
type Server struct {
	Q       *sqlcgen.Queries
	Ingest  commands.StartDocumentIngestionHandler
	Config  config.Settings
	Blob    *blob.Client
	Logger  *slog.Logger
	ListLim int32
}

func (s *Server) listLimit() int32 {
	if s.ListLim > 0 {
		return s.ListLim
	}
	return defaultListLimit
}

// scopeFromRequest reads org/project from URL path, optional POST form fields (legacy), cookies, then DemoScope.
func (s *Server) scopeFromRequest(r *http.Request) OrgScope {
	orgID := strings.TrimSpace(r.PathValue("orgId"))
	projectID := strings.TrimSpace(r.PathValue("projectId"))
	if orgID == "" {
		orgID = strings.TrimSpace(r.FormValue("organization_id"))
	}
	if projectID == "" {
		projectID = strings.TrimSpace(r.FormValue("project_id"))
	}
	if orgID == "" {
		orgID = cookieString(r, cookieOrgIDKey)
	}
	if projectID == "" {
		projectID = cookieString(r, cookieProjIDKey)
	}
	if orgID == "" {
		orgID = DemoScope.OrganizationID
	}
	if projectID == "" {
		projectID = DemoScope.ProjectID
	}
	return OrgScope{OrganizationID: orgID, ProjectID: projectID}
}

// HomeRedirect sends users to a scoped dashboard (optional legacy ?organization_id=&project_id=).
func (s *Server) HomeRedirect(w http.ResponseWriter, r *http.Request) {
	qOrg := strings.TrimSpace(r.URL.Query().Get("organization_id"))
	qProj := strings.TrimSpace(r.URL.Query().Get("project_id"))
	var scope OrgScope
	if qOrg != "" || qProj != "" {
		scope = OrgScope{
			OrganizationID: pickOr(qOrg, DemoScope.OrganizationID),
			ProjectID:      pickOr(qProj, DemoScope.ProjectID),
		}
	} else {
		scope = OrgScope{
			OrganizationID: pickOr(cookieString(r, cookieOrgIDKey), DemoScope.OrganizationID),
			ProjectID:      pickOr(cookieString(r, cookieProjIDKey), DemoScope.ProjectID),
		}
	}
	http.Redirect(w, r, scope.Path("/dashboard"), http.StatusSeeOther)
}

func pickOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

// ScopeApply redirects GET /scope?organization_id=&project_id= to the matching scoped dashboard.
func (s *Server) ScopeApply(w http.ResponseWriter, r *http.Request) {
	org := strings.TrimSpace(r.URL.Query().Get("organization_id"))
	proj := strings.TrimSpace(r.URL.Query().Get("project_id"))
	scope := OrgScope{
		OrganizationID: pickOr(org, DemoScope.OrganizationID),
		ProjectID:      pickOr(proj, DemoScope.ProjectID),
	}
	http.Redirect(w, r, scope.Path("/dashboard"), http.StatusSeeOther)
}

func (s *Server) Dashboard(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	vm, err := s.loadDashboardVM(r.Context(), scope)
	if err != nil {
		s.Logger.Error("dashboard load", "err", err)
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}
	s.render(w, r, DashboardPage(vm))
}

func (s *Server) DocumentsList(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	docs, err := s.Q.ListDocumentsByOrgProject(r.Context(), sqlcgen.ListDocumentsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		s.Logger.Error("list documents", "err", err)
		http.Error(w, "failed to load documents", http.StatusInternalServerError)
		return
	}
	runs, err := s.Q.ListIngestionRunsByOrgProject(r.Context(), sqlcgen.ListIngestionRunsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		s.Logger.Error("list runs", "err", err)
		http.Error(w, "failed to load runs", http.StatusInternalServerError)
		return
	}
	vm := DocumentsPageVM{
		Scope: scope,
		Rows:  buildOpsRows(docs, runs, scope.ProjectID),
	}
	s.render(w, r, DocumentsPage(vm))
}

func (s *Server) DocumentDetail(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	docID := r.PathValue("docId")
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	row, err := s.Q.GetDocumentByID(r.Context(), docID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.Logger.Error("get document", "err", err)
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if row.OrganizationID != scope.OrganizationID || row.ProjectID != scope.ProjectID {
		http.NotFound(w, r)
		return
	}
	runs, err := s.Q.ListIngestionRunsForDocument(r.Context(), sqlcgen.ListIngestionRunsForDocumentParams{
		DocumentID: docID,
		Limit:      s.listLimit(),
	})
	if err != nil {
		s.Logger.Error("list runs for doc", "err", err)
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	history := make([]RunSummary, 0, len(runs))
	var latest *RunSummary
	for _, run := range runs {
		rs := runSummaryFromRow(run)
		history = append(history, rs)
	}
	if len(history) > 0 {
		cp := history[0]
		latest = &cp
	}
	docSummary := documentSummaryFull(row)
	vm := DocumentDetailVM{
		Scope:           scope,
		Document:        docSummary,
		FullArtifactURI: row.StorageUri,
		LatestRun:       latest,
		RunHistory:      history,
	}
	if s.Blob != nil {
		key, err := blob.ValidateArtifactURI(s.Config.ObjectStorage, row.StorageUri)
		if err == nil && !blob.IsPlaceholderPendingKey(key) {
			vm.ArtifactURL = ArtifactLink(scope, docID)
			vm.ArtifactViewer = ArtifactViewerKind(row.MimeType)
		}
	}
	s.render(w, r, DocumentDetailPage(vm))
}

func (s *Server) DocumentArtifact(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	docID := r.PathValue("docId")
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	row, err := s.Q.GetDocumentByID(r.Context(), docID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.Logger.Error("artifact document", "err", err)
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if row.OrganizationID != scope.OrganizationID || row.ProjectID != scope.ProjectID {
		http.NotFound(w, r)
		return
	}
	if s.Blob == nil {
		http.Error(w, "object storage not configured", http.StatusServiceUnavailable)
		return
	}
	key, err := blob.ValidateArtifactURI(s.Config.ObjectStorage, row.StorageUri)
	if err != nil {
		s.Logger.Warn("artifact uri", "err", err)
		http.Error(w, "invalid artifact reference", http.StatusBadRequest)
		return
	}
	if blob.IsPlaceholderPendingKey(key) {
		http.Error(w, "no file uploaded for this document", http.StatusNotFound)
		return
	}
	url, err := s.Blob.PresignGetURL(r.Context(), key, artifactPresignTTL)
	if err != nil {
		s.Logger.Error("artifact presign", "err", err)
		http.Error(w, "could not generate download URL", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) RunDetail(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	runID := r.PathValue("runId")
	if runID == "" {
		http.NotFound(w, r)
		return
	}
	runRow, err := s.Q.GetIngestionRunByID(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.Logger.Error("get run", "err", err)
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if runRow.OrganizationID != scope.OrganizationID || runRow.ProjectID != scope.ProjectID {
		http.NotFound(w, r)
		return
	}
	docRow, err := s.Q.GetDocumentByID(r.Context(), runRow.DocumentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.Logger.Error("get document for run", "err", err)
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	vm := RunDetailVM{
		Scope: scope,
		Run:   runSummaryFromRow(runRow),
		Doc:   documentSummaryFull(docRow),
	}
	s.render(w, r, RunDetailPage(vm))
}

func (s *Server) ReviewQueue(w http.ResponseWriter, r *http.Request) {
	vm := PlaceholderPageVM{
		Scope:       s.scopeFromRequest(r),
		Title:       "Review queue",
		Description: "Operational control when AI is uncertain — priority, SLA, correction loop, and keyboard-first review.",
		Bullets: []string{
			"Reason taxonomy: missing fields, low confidence, schema failure, model disagreement, drift.",
			"Split viewer + correction panel; optional multi-model comparison strip.",
			"Approve / save correction / mark unreadable / escalate; optional save-as-ground-truth.",
		},
	}
	s.render(w, r, ReviewQueuePage(vm))
}

func (s *Server) Evaluations(w http.ResponseWriter, r *http.Request) {
	vm := PlaceholderPageVM{
		Scope:       s.scopeFromRequest(r),
		Title:       "Evaluations",
		Description: "AI reliability lab — batch runs against datasets, regression gates, and field-level impact before promoting prompts or models.",
		Bullets: []string{
			"Batch evaluations · datasets · regression suite · field performance · prompt experiments.",
			"Matrix view surfaces faster vs more accurate vs cheaper; regression warnings when averages hide field regressions.",
		},
	}
	s.render(w, r, EvaluationsPage(vm))
}

func (s *Server) Pipelines(w http.ResponseWriter, r *http.Request) {
	vm := PlaceholderPageVM{
		Scope:       s.scopeFromRequest(r),
		Title:       "Pipelines",
		Description: "Typed step list with routing, retries, and versioning — production behavior as code, not drag-and-drop theater.",
		Bullets: []string{
			"Steps: detect type → preprocess → extract → normalize → validate → score → evaluate → route → deliver.",
			"Duplicate / draft / promote / rollback / run test on pipeline versions.",
		},
	}
	s.render(w, r, PipelinesPage(vm))
}

func (s *Server) Models(w http.ResponseWriter, r *http.Request) {
	vm := PlaceholderPageVM{
		Scope:       s.scopeFromRequest(r),
		Title:       "Models",
		Description: "Provider + model configs, health, spend, and routing policies that decide who processes each document under what guardrails.",
		Bullets: []string{
			"Per-model latency / failure / cost telemetry; routing policies with confidence and cost ceilings.",
		},
	}
	s.render(w, r, ModelsPage(vm))
}

func (s *Server) Schemas(w http.ResponseWriter, r *http.Request) {
	vm := PlaceholderPageVM{
		Scope:       s.scopeFromRequest(r),
		Title:       "Schemas",
		Description: "Output contracts — required fields, validation, accuracy by field, immutable production versions.",
		Bullets: []string{
			"Draft → test against dataset → promote; prevents silent downstream breakage.",
		},
	}
	s.render(w, r, SchemasPage(vm))
}

func (s *Server) Settings(w http.ResponseWriter, r *http.Request) {
	vm := s.buildSettingsVM(r)
	s.render(w, r, SettingsFullPage(vm))
}

func (s *Server) FragmentDocuments(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	docs, err := s.Q.ListDocumentsByOrgProject(r.Context(), sqlcgen.ListDocumentsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		s.Logger.Error("list documents", "err", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	vm := mapDocuments(docs)
	s.render(w, r, DocumentsTable(vm))
}

func (s *Server) FragmentOpsDocuments(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	docs, err := s.Q.ListDocumentsByOrgProject(r.Context(), sqlcgen.ListDocumentsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		s.Logger.Error("list documents", "err", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	runs, err := s.Q.ListIngestionRunsByOrgProject(r.Context(), sqlcgen.ListIngestionRunsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		s.Logger.Error("list runs", "err", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	rows := buildOpsRows(docs, runs, scope.ProjectID)
	s.render(w, r, OpsDocumentsTable(scope, rows))
}

func (s *Server) FragmentRuns(w http.ResponseWriter, r *http.Request) {
	scope := s.scopeFromRequest(r)
	runs, err := s.Q.ListIngestionRunsByOrgProject(r.Context(), sqlcgen.ListIngestionRunsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		s.Logger.Error("list runs", "err", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	vm := mapRuns(runs)
	s.render(w, r, RunsTable(vm))
}

func (s *Server) ActionIngest(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			http.Error(w, "bad multipart form", http.StatusBadRequest)
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
	}
	scope := s.scopeFromRequest(r)

	traceID := uuid.NewString()
	docID := "doc_" + uuid.NewString()
	runID := "run_" + uuid.NewString()

	file, fh, errFile := r.FormFile("file")
	if errFile != nil && !errors.Is(errFile, http.ErrMissingFile) {
		s.Logger.Error("ingest form file", "err", errFile)
		s.render(w, r, IngestResultFragment(false, "Could not read file field.", "", "", false))
		return
	}
	hasFile := fh != nil && fh.Filename != ""
	if hasFile {
		defer file.Close()
	}

	fileStored := false
	storageURI := s.Config.ObjectStorage.ArtifactURI("pending", docID)
	mimeType := "application/octet-stream"
	checksum := ""

	if hasFile {
		if s.Blob == nil {
			s.render(w, r, IngestResultFragment(false, "File upload requires a working object storage client (check OBJECT_STORAGE_*).", "", "", false))
			return
		}
		if fh.Size > maxUploadBytes {
			s.render(w, r, IngestResultFragment(false, "File exceeds maximum upload size (50 MiB).", "", "", false))
			return
		}

		safeName := blob.SanitizeUploadFilename(fh.Filename)
		objectKey := s.Config.ObjectStorage.ObjectKey("pending", docID, safeName)
		storageURI = s.Config.ObjectStorage.ArtifactURI("pending", docID, safeName)

		peek := make([]byte, 512)
		n, err := io.ReadFull(file, peek)
		switch err {
		case nil, io.EOF, io.ErrUnexpectedEOF:
		default:
			s.Logger.Error("ingest read upload", "err", err)
			s.render(w, r, IngestResultFragment(false, "Could not read uploaded file.", "", "", false))
			return
		}
		peek = peek[:n]

		body := io.MultiReader(bytes.NewReader(peek), file)
		detected := http.DetectContentType(peek)
		mimeType = detected
		if t := fh.Header.Get("Content-Type"); t != "" && !strings.EqualFold(t, "application/octet-stream") {
			mimeType = t
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		h := sha256.New()
		tee := io.TeeReader(body, h)
		putSize := fh.Size
		if putSize < 0 {
			putSize = -1
		}
		if err := s.Blob.Put(r.Context(), objectKey, tee, putSize, mimeType); err != nil {
			s.Logger.Error("ingest blob put", "err", err)
			s.render(w, r, IngestResultFragment(false, "Could not store file in object storage.", "", "", false))
			return
		}
		checksum = hex.EncodeToString(h.Sum(nil))
		fileStored = true
	}

	res, err := s.Ingest.Handle(r.Context(), commands.StartDocumentIngestionCommand{
		OrganizationID:  scope.OrganizationID,
		ProjectID:       scope.ProjectID,
		DocumentID:      docID,
		IngestionRunID:  runID,
		StorageURI:      storageURI,
		MimeType:        mimeType,
		ChecksumSHA256:  checksum,
		PipelineID:      r.FormValue("pipeline_id"),
		SchemaID:        r.FormValue("schema_id"),
		ModelConfigID:   r.FormValue("model_config_id"),
		PromptVersionID: r.FormValue("prompt_version_id"),
		TraceID:         traceID,
		Now:             time.Now().UTC(),
	})
	if err != nil {
		s.Logger.Error("ingest", "err", err)
		s.render(w, r, IngestResultFragment(false, err.Error(), "", "", false))
		return
	}
	s.render(w, r, IngestResultFragment(true, "", res.DocumentID, res.IngestionRunID, fileStored))
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		s.Logger.Error("render", "err", err)
	}
}

func (s *Server) loadDashboardVM(ctx context.Context, scope OrgScope) (DashboardVM, error) {
	docs, err := s.Q.ListDocumentsByOrgProject(ctx, sqlcgen.ListDocumentsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		return DashboardVM{}, err
	}
	runs, err := s.Q.ListIngestionRunsByOrgProject(ctx, sqlcgen.ListIngestionRunsByOrgProjectParams{
		OrganizationID: scope.OrganizationID,
		ProjectID:      scope.ProjectID,
		Limit:          s.listLimit(),
	})
	if err != nil {
		return DashboardVM{}, err
	}
	kpi := dashboardKPI(docs, runs)
	return DashboardVM{
		Scope:          scope,
		KPI:            kpi,
		StatusCounts:   rollupRuns(runs),
		RecentFailures: recentFailures(runs, 8),
		Documents:      mapDocuments(docs),
		Runs:           mapRuns(runs),
	}, nil
}

func mapDocuments(rows []sqlcgen.Document) []DocumentSummary {
	out := make([]DocumentSummary, 0, len(rows))
	for _, d := range rows {
		out = append(out, DocumentSummary{
			ID:         d.ID,
			Status:     d.Status,
			MimeType:   d.MimeType,
			CreatedAt:  formatTime(d.CreatedAt),
			StorageURI: shorten(d.StorageUri, 72),
		})
	}
	return out
}

func mapRuns(rows []sqlcgen.IngestionRun) []RunSummary {
	out := make([]RunSummary, 0, len(rows))
	for _, run := range rows {
		out = append(out, runSummaryFromRow(run))
	}
	return out
}

// ParseListLimit returns a positive limit from env ADMIN_UI_LIST_LIMIT, or 0 for default.
func ParseListLimit(env string) int32 {
	if env == "" {
		return 0
	}
	n, err := strconv.Atoi(env)
	if err != nil || n <= 0 || n > 500 {
		return 0
	}
	return int32(n)
}
