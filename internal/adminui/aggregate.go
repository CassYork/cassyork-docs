package adminui

import (
	"path"
	"strings"

	"cassyork.dev/platform/internal/domain/ingestion"
	"cassyork.dev/platform/internal/infrastructure/postgres/sqlcgen"
)

func rollupRuns(runs []sqlcgen.IngestionRun) StatusRollup {
	var r StatusRollup
	for _, run := range runs {
		r.Total++
		switch ingestion.Status(run.Status) {
		case ingestion.StatusQueued:
			r.Queued++
		case ingestion.StatusRunning, ingestion.StatusWaitingOnProvider, ingestion.StatusValidating:
			r.Running++
		case ingestion.StatusRequiresReview:
			r.RequiresReview++
		case ingestion.StatusCompleted:
			r.Completed++
		case ingestion.StatusFailed, ingestion.StatusCancelled:
			r.Failed++
		default:
			r.Other++
		}
	}
	return r
}

func dashboardKPI(docs []sqlcgen.Document, runs []sqlcgen.IngestionRun) DashboardKPI {
	rollup := rollupRuns(runs)
	k := DashboardKPI{
		DocumentsProcessed:      len(docs),
		AnalyticsMetricsPending: true,
	}
	if rollup.Total > 0 {
		k.StraightThroughPct = float64(rollup.Completed) / float64(rollup.Total) * 100
		k.HumanReviewPct = float64(rollup.RequiresReview) / float64(rollup.Total) * 100
	}
	return k
}

func recentFailures(runs []sqlcgen.IngestionRun, max int) []RunSummary {
	out := make([]RunSummary, 0, max)
	for _, run := range runs {
		if ingestion.Status(run.Status) != ingestion.StatusFailed {
			continue
		}
		errPreview := ""
		if run.ErrorMessage.Valid {
			errPreview = shorten(run.ErrorMessage.String, 120)
		}
		out = append(out, RunSummary{
			ID:           run.ID,
			DocumentID:   run.DocumentID,
			Status:       run.Status,
			TraceID:      shorten(run.TraceID, 12),
			PipelineID:   run.PipelineID,
			CreatedAt:    formatTime(run.CreatedAt),
			ErrorPreview: errPreview,
		})
		if len(out) >= max {
			break
		}
	}
	return out
}

func buildOpsRows(docs []sqlcgen.Document, runs []sqlcgen.IngestionRun, projectID string) []OpsDocumentRow {
	out := make([]OpsDocumentRow, 0, len(docs))
	for _, d := range docs {
		lr := latestRunForDoc(d.ID, runs)
		row := OpsDocumentRow{
			DocumentID:   d.ID,
			DisplayName:  displayNameFromStorage(d.StorageUri, d.ID),
			Status:       d.Status,
			DocumentType: inferDocType(d.MimeType, d.StorageUri),
			ProjectID:    projectID,
			LatestRunID:  "",
			Model:        "—",
			Confidence:   "—",
			Validation:   "—",
			ReviewStatus: "—",
			CreatedAt:    formatTime(d.CreatedAt),
		}
		if lr != nil {
			row.LatestRunID = lr.ID
			if lr.ModelConfigID != "" {
				row.Model = shorten(lr.ModelConfigID, 24)
			}
			row.ReviewStatus = reviewHint(lr.Status)
		}
		out = append(out, row)
	}
	return out
}

func latestRunForDoc(docID string, runs []sqlcgen.IngestionRun) *sqlcgen.IngestionRun {
	var best *sqlcgen.IngestionRun
	for i := range runs {
		r := &runs[i]
		if r.DocumentID != docID {
			continue
		}
		if best == nil || runIsNewerThan(r, best) {
			best = r
		}
	}
	return best
}

func runIsNewerThan(a, b *sqlcgen.IngestionRun) bool {
	if !a.CreatedAt.Valid {
		return false
	}
	if !b.CreatedAt.Valid {
		return true
	}
	return a.CreatedAt.Time.After(b.CreatedAt.Time)
}

func reviewHint(status string) string {
	switch ingestion.Status(status) {
	case ingestion.StatusRequiresReview:
		return "Open"
	case ingestion.StatusCompleted:
		return "None"
	case ingestion.StatusFailed:
		return "—"
	default:
		return "—"
	}
}

func inferDocType(mime, storageURI string) string {
	lower := strings.ToLower(storageURI + mime)
	switch {
	case strings.Contains(lower, "invoice"):
		return "Invoice"
	case strings.Contains(lower, "manifest"):
		return "Manifest"
	case strings.HasSuffix(lower, ".pdf") || strings.Contains(mime, "pdf"):
		return "PDF"
	case strings.HasPrefix(mime, "image/"):
		return "Image"
	default:
		return "Document"
	}
}

func displayNameFromStorage(uri, id string) string {
	if uri != "" {
		u := strings.TrimSpace(uri)
		if i := strings.LastIndex(u, "/"); i >= 0 && i < len(u)-1 {
			base := u[i+1:]
			if base != "" {
				return shorten(base, 48)
			}
		}
	}
	return shorten(id, 40)
}

func documentSummaryFromRow(d sqlcgen.Document) DocumentSummary {
	return DocumentSummary{
		ID:         d.ID,
		Status:     d.Status,
		MimeType:   d.MimeType,
		CreatedAt:  formatTime(d.CreatedAt),
		StorageURI: shorten(d.StorageUri, 80),
	}
}

func documentSummaryFull(d sqlcgen.Document) DocumentSummary {
	return DocumentSummary{
		ID:         d.ID,
		Status:     d.Status,
		MimeType:   d.MimeType,
		CreatedAt:  formatTime(d.CreatedAt),
		StorageURI: d.StorageUri,
	}
}

// DocumentDisplayTitle picks a human-readable title from storage URI or ID.
func DocumentDisplayTitle(d DocumentSummary) string {
	return displayNameFromStorage(d.StorageURI, d.ID)
}

func runSummaryFromRow(r sqlcgen.IngestionRun) RunSummary {
	errPreview := ""
	if r.ErrorMessage.Valid {
		errPreview = shorten(r.ErrorMessage.String, 120)
	}
	return RunSummary{
		ID:           r.ID,
		DocumentID:   r.DocumentID,
		Status:       r.Status,
		TraceID:      shorten(r.TraceID, 24),
		PipelineID:   r.PipelineID,
		CreatedAt:    formatTime(r.CreatedAt),
		ErrorPreview: errPreview,
	}
}

// ArtifactViewerKind selects how to embed the artifact in the document detail page.
func ArtifactViewerKind(mime string) string {
	m := strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.Contains(m, "pdf"):
		return "pdf"
	case strings.HasPrefix(m, "image/"):
		return "image"
	default:
		return "download"
	}
}

func pathLastSegment(uri string) string {
	if uri == "" {
		return ""
	}
	// handle s3://bucket/key/path
	s := strings.TrimSpace(uri)
	s = strings.TrimPrefix(s, "s3://")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	return path.Base(s)
}
