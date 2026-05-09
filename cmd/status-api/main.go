package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cassyork.dev/platform/internal/config"
	"cassyork.dev/platform/internal/httphelp"
	"cassyork.dev/platform/internal/observability"
)

// Status API — read-optimized run/document status (cache-friendly, polling clients).
func main() {
	cfg := config.Load()
	serviceName := getenv("OTEL_SERVICE_NAME", "cassyork-status-api")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := observability.InitTracer(ctx, serviceName, cfg.OtelExporterEndpoint)
	if err != nil {
		log.Printf("otel init: %v", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	logger := httphelp.Logger("status-api")
	mux := httphelp.NewMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httphelp.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/runs/{runId}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("runId")
		httphelp.JSON(w, http.StatusOK, map[string]any{
			"id":     id,
			"status": "queued",
		})
	})

	addr := getenv("LISTEN_ADDR", ":8082")
	logger.Info("starting status-api", "addr", addr)
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
