package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cassyork.dev/platform/internal/adminui"
	"cassyork.dev/platform/internal/application/commands"
	"cassyork.dev/platform/internal/config"
	"cassyork.dev/platform/internal/httphelp"
	"cassyork.dev/platform/internal/infrastructure/postgres"
	"cassyork.dev/platform/internal/infrastructure/temporaladapter"
	"cassyork.dev/platform/internal/observability"
	temporalcli "cassyork.dev/platform/internal/temporalio"
)

// Admin UI — operations control plane + AI reliability surfaces (HTMX + templ).
func main() {
	cfg := config.Load()
	serviceName := getenv("OTEL_SERVICE_NAME", "cassyork-admin-ui")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := observability.InitTracer(ctx, serviceName, cfg.Telemetry.OtelExporterEndpoint)
	if err != nil {
		log.Printf("otel init: %v", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	pool, err := postgres.NewPool(ctx, cfg.Database.URL)
	if err != nil {
		log.Printf("postgres: %v", err)
		os.Exit(1)
	}
	defer pool.Close()

	tc, err := temporalcli.NewClient(cfg.Temporal.Address, cfg.Temporal.Namespace)
	if err != nil {
		log.Printf("temporal: %v", err)
		os.Exit(1)
	}
	defer tc.Close()

	q := postgres.NewQueries(pool)
	ingest := commands.StartDocumentIngestionHandler{
		Documents: postgres.NewDocumentRepository(q),
		Runs:      postgres.NewRunRepository(q),
		Workflow:  temporaladapter.ClientStarter{Client: tc},
	}

	logger := httphelp.Logger("admin-ui")
	srv := &adminui.Server{
		Q:       q,
		Ingest:  ingest,
		Config:  cfg,
		Logger:  logger,
		ListLim: adminui.ParseListLimit(os.Getenv("ADMIN_UI_LIST_LIMIT")),
	}

	mux := httphelp.NewMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httphelp.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	scoped := "/orgs/{orgId}/projects/{projectId}"
	mux.HandleFunc("GET "+scoped+"/dashboard", srv.Dashboard)
	mux.HandleFunc("GET "+scoped+"/documents/{docId}", srv.DocumentDetail)
	mux.HandleFunc("GET "+scoped+"/documents", srv.DocumentsList)
	mux.HandleFunc("GET "+scoped+"/runs/{runId}", srv.RunDetail)
	mux.HandleFunc("GET "+scoped+"/review-queue", srv.ReviewQueue)
	mux.HandleFunc("GET "+scoped+"/evaluations", srv.Evaluations)
	mux.HandleFunc("GET "+scoped+"/pipelines", srv.Pipelines)
	mux.HandleFunc("GET "+scoped+"/models", srv.Models)
	mux.HandleFunc("GET "+scoped+"/schemas", srv.Schemas)
	mux.HandleFunc("GET "+scoped+"/settings", srv.Settings)
	mux.HandleFunc("GET "+scoped+"/ui/fragments/documents", srv.FragmentDocuments)
	mux.HandleFunc("GET "+scoped+"/ui/fragments/ops-documents", srv.FragmentOpsDocuments)
	mux.HandleFunc("GET "+scoped+"/ui/fragments/runs", srv.FragmentRuns)
	mux.HandleFunc("POST "+scoped+"/ui/actions/ingest", srv.ActionIngest)
	mux.HandleFunc("POST "+scoped+"/ui/actions/workspace-defaults", srv.SaveWorkspaceDefaults)
	mux.HandleFunc("POST "+scoped+"/ui/actions/clear-workspace-defaults", srv.ClearWorkspaceDefaults)

	mux.HandleFunc("GET /scope", srv.ScopeApply)
	mux.HandleFunc("GET /", srv.HomeRedirect)

	mux.HandleFunc("GET /dashboard", adminui.LegacyRedirect("dashboard"))
	mux.HandleFunc("GET /documents/{docId}", adminui.LegacyDocumentRedirect())
	mux.HandleFunc("GET /documents", adminui.LegacyRedirect("documents"))
	mux.HandleFunc("GET /runs/{runId}", adminui.LegacyRunRedirect())
	mux.HandleFunc("GET /review-queue", adminui.LegacyRedirect("review-queue"))
	mux.HandleFunc("GET /evaluations", adminui.LegacyRedirect("evaluations"))
	mux.HandleFunc("GET /pipelines", adminui.LegacyRedirect("pipelines"))
	mux.HandleFunc("GET /models", adminui.LegacyRedirect("models"))
	mux.HandleFunc("GET /schemas", adminui.LegacyRedirect("schemas"))
	mux.HandleFunc("GET /settings", adminui.LegacyRedirect("settings"))
	mux.HandleFunc("GET /ui/fragments/documents", adminui.LegacyRedirect("ui/fragments/documents"))
	mux.HandleFunc("GET /ui/fragments/ops-documents", adminui.LegacyRedirect("ui/fragments/ops-documents"))
	mux.HandleFunc("GET /ui/fragments/runs", adminui.LegacyRedirect("ui/fragments/runs"))
	mux.HandleFunc("POST /ui/actions/ingest", srv.ActionIngest)

	addr := getenv("LISTEN_ADDR", ":8095")
	logger.Info("starting admin-ui", "addr", addr)
	if err := httphelp.Listen(ctx, addr, mux); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
