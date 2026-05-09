package adminui

import (
	"net/http"
	"net/url"
	"strings"
)

// DemoScope is the default workspace when no cookies or path segments apply.
var DemoScope = OrgScope{OrganizationID: "org_demo", ProjectID: "proj_demo"}

func escSeg(s string) string {
	return url.PathEscape(strings.TrimSpace(s))
}

// Root returns /orgs/{org}/projects/{proj} with path-safe escaping per segment.
func (s OrgScope) Root() string {
	return "/orgs/" + escSeg(s.OrganizationID) + "/projects/" + escSeg(s.ProjectID)
}

// Path joins suffix segments under the scoped root (suffix must not start with /).
func (s OrgScope) Path(suffix string) string {
	suffix = strings.Trim(suffix, "/")
	if suffix == "" {
		return s.Root()
	}
	return s.Root() + "/" + suffix
}

// DocumentLink is the detail URL for a document within the scope.
func DocumentLink(scope OrgScope, documentID string) string {
	return scope.Path("documents/" + escSeg(documentID))
}

// RunLink is the detail URL for an ingestion run within the scope.
func RunLink(scope OrgScope, runID string) string {
	return scope.Path("runs/" + escSeg(runID))
}

func DocumentsFragmentURL(s OrgScope) string {
	return s.Path("ui/fragments/documents")
}

func RunsFragmentURL(s OrgScope) string {
	return s.Path("ui/fragments/runs")
}

func OpsDocumentsFragmentURL(s OrgScope) string {
	return s.Path("ui/fragments/ops-documents")
}

// ScopeFromQuery replaces org/project on base when the query string carries organization_id / project_id (legacy bookmark support).
func ScopeFromQuery(r *http.Request, base OrgScope) OrgScope {
	org := strings.TrimSpace(r.URL.Query().Get("organization_id"))
	proj := strings.TrimSpace(r.URL.Query().Get("project_id"))
	sc := base
	if org != "" {
		sc.OrganizationID = org
	}
	if proj != "" {
		sc.ProjectID = proj
	}
	return sc
}

// LegacyRedirect maps old unscoped GET routes into scoped URLs (honours query org/project when present).
func LegacyRedirect(page string) http.HandlerFunc {
	page = strings.Trim(page, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		sc := ScopeFromQuery(r, DemoScope)
		http.Redirect(w, r, sc.Path(page), http.StatusPermanentRedirect)
	}
}

// LegacyDocumentRedirect maps GET /documents/{docId} to a scoped document URL.
func LegacyDocumentRedirect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		docID := strings.TrimSpace(r.PathValue("docId"))
		if docID == "" {
			http.NotFound(w, r)
			return
		}
		sc := ScopeFromQuery(r, DemoScope)
		http.Redirect(w, r, DocumentLink(sc, docID), http.StatusPermanentRedirect)
	}
}

// LegacyRunRedirect maps GET /runs/{runId} to a scoped run URL.
func LegacyRunRedirect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := strings.TrimSpace(r.PathValue("runId"))
		if runID == "" {
			http.NotFound(w, r)
			return
		}
		sc := ScopeFromQuery(r, DemoScope)
		http.Redirect(w, r, RunLink(sc, runID), http.StatusPermanentRedirect)
	}
}
