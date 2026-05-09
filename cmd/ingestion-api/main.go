package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"cassyork.dev/platform/internal/application/commands"
	"cassyork.dev/platform/internal/config"
	"cassyork.dev/platform/internal/httphelp"
	"cassyork.dev/platform/internal/infrastructure/postgres"
	"cassyork.dev/platform/internal/infrastructure/temporaladapter"
	"cassyork.dev/platform/internal/observability"
	temporalcli "cassyork.dev/platform/internal/temporalio"
)

// Ingestion API — uploads, persisted metadata, starts Temporal workflows, writes artifacts to object storage.
func main() {
	cfg := config.Load()
	serviceName := getenv("OTEL_SERVICE_NAME", "cassyork-ingestion-api")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := observability.InitTracer(ctx, serviceName, cfg.OtelExporterEndpoint)
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

	tc, err := temporalcli.NewClient(cfg.Address, cfg.Namespace)
	if err != nil {
		log.Printf("temporal: %v", err)
		os.Exit(1)
	}
	defer tc.Close()

	q := postgres.NewQueries(pool)
	handler := commands.StartDocumentIngestionHandler{
		Documents: postgres.NewDocumentRepository(q),
		Runs:      postgres.NewRunRepository(q),
		Workflow:  temporaladapter.ClientStarter{Client: tc},
	}

	logger := httphelp.Logger("ingestion-api")
	mux := httphelp.NewMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httphelp.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/projects/{projectId}/documents/ingest", func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("projectId")
		orgID := r.Header.Get("X-Org-Id")
		if orgID == "" {
			orgID = "org_demo"
		}
		traceID := uuid.NewString()
		docID := "doc_" + uuid.NewString()
		runID := "run_" + uuid.NewString()

		res, err := handler.Handle(r.Context(), commands.StartDocumentIngestionCommand{
			OrganizationID:  orgID,
			ProjectID:       projectID,
			DocumentID:      docID,
			IngestionRunID:  runID,
			StorageURI:      cfg.ObjectStorage.ArtifactURI("pending", docID),
			MimeType:        "application/octet-stream",
			ChecksumSHA256:  "",
			PipelineID:      r.FormValue("pipeline_id"),
			SchemaID:        r.FormValue("schema_id"),
			ModelConfigID:   r.FormValue("model_config_id"),
			PromptVersionID: r.FormValue("prompt_version_id"),
			TraceID:         traceID,
			Now:             time.Now().UTC(),
		})
		if err != nil {
			logger.Error("start ingestion", "err", err)
			httphelp.JSON(w, http.StatusInternalServerError, map[string]string{"error": "ingestion_start_failed"})
			return
		}
		httphelp.JSON(w, http.StatusAccepted, map[string]any{
			"document_id":       res.DocumentID,
			"ingestion_run_id":  res.IngestionRunID,
			"status":            res.Status,
			"trace_id":          res.TraceID,
			"temporal_workflow": res.TemporalWorkflowID,
		})
	})

	addr := getenv("LISTEN_ADDR", ":8081")
	logger.Info("starting ingestion-api", "addr", addr)
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
