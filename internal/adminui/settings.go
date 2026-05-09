package adminui

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func (s *Server) buildSettingsVM(r *http.Request) SettingsVM {
	cfg := s.Config
	lim := s.listLimit()
	limDesc := describeListLimit(lim)

	otel := strings.TrimSpace(cfg.Telemetry.OtelExporterEndpoint)
	if otel == "" {
		otel = "Disabled — set OTEL_EXPORTER_OTLP_ENDPOINT (gRPC, e.g. :4317)."
	}

	temporalUI := strings.TrimSpace(os.Getenv("TEMPORAL_UI_URL"))
	if temporalUI == "" {
		temporalUI = "http://localhost:8088"
	}

	webhookHint := strings.TrimSpace(os.Getenv("WEBHOOK_API_PUBLIC_URL"))
	if webhookHint == "" {
		webhookHint = "Point operators at webhook-api base URL when deployed; signing secrets stay server-side only."
	}

	scheme := cfg.ObjectStorage.Scheme
	if scheme == "" {
		scheme = "s3"
	}

	return SettingsVM{
		Scope:            s.scopeFromRequest(r),
		SavedWorkspace:   r.URL.Query().Get("saved") == "1",
		ClearedWorkspace: r.URL.Query().Get("cleared") == "1",
		Runtime: SettingsRuntimeVM{
			ListenAddr:           getenvDefault("LISTEN_ADDR", ":8095"),
			AdminUIListLimitDesc: limDesc,
			DatabaseDisplay:      RedactDatabaseURL(cfg.Database.URL),
			TemporalAddress:      cfg.Temporal.Address,
			TemporalNamespace:    cfg.Temporal.Namespace,
			TemporalUILink:       temporalUI,
			OTELExporter:         otel,
			ObjectStorage: SettingsObjectStorageVM{
				Scheme:           scheme,
				Endpoint:         cfg.ObjectStorage.Endpoint,
				Region:           cfg.ObjectStorage.Region,
				Bucket:           cfg.ObjectStorage.Bucket,
				AccessKeyMasked:  MaskCredential(cfg.ObjectStorage.AccessKeyID),
				SecretConfigured: strings.TrimSpace(cfg.ObjectStorage.SecretAccessKey) != "",
				UsePathStyle:     cfg.ObjectStorage.UsePathStyle,
			},
		},
		WebhookHint: webhookHint,
	}
}

func describeListLimit(effective int32) string {
	raw := strings.TrimSpace(os.Getenv("ADMIN_UI_LIST_LIMIT"))
	if raw == "" {
		return "default (64 rows per scoped list)"
	}
	return raw + " → effective " + strconv.FormatInt(int64(effective), 10)
}

func getenvDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func setWorkspaceCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   86400 * 365,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearWorkspaceCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) SaveWorkspaceDefaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	org := strings.TrimSpace(r.FormValue("organization_id"))
	proj := strings.TrimSpace(r.FormValue("project_id"))
	if org == "" || proj == "" {
		http.Error(w, "organization_id and project_id required", http.StatusBadRequest)
		return
	}
	setWorkspaceCookie(w, cookieOrgIDKey, org)
	setWorkspaceCookie(w, cookieProjIDKey, proj)
	destScope := OrgScope{OrganizationID: org, ProjectID: proj}
	q := url.Values{}
	q.Set("saved", "1")
	http.Redirect(w, r, destScope.Path("/settings")+"?"+q.Encode(), http.StatusSeeOther)
}

func (s *Server) ClearWorkspaceDefaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	clearWorkspaceCookie(w, cookieOrgIDKey)
	clearWorkspaceCookie(w, cookieProjIDKey)
	q := url.Values{}
	q.Set("cleared", "1")
	http.Redirect(w, r, DemoScope.Path("/settings")+"?"+q.Encode(), http.StatusSeeOther)
}
