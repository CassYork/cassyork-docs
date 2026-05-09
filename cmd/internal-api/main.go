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

// Internal service APIs — auth between Go services / admin operations (not customer-facing).
func main() {
	cfg := config.Load()
	serviceName := getenv("OTEL_SERVICE_NAME", "cassyork-internal-api")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := observability.InitTracer(ctx, serviceName, cfg.OtelExporterEndpoint)
	if err != nil {
		log.Printf("otel init: %v", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	logger := httphelp.Logger("internal-api")
	mux := httphelp.NewMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httphelp.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /internal/v1/catalog/model-configs", func(w http.ResponseWriter, _ *http.Request) {
		httphelp.JSON(w, http.StatusCreated, map[string]string{"id": "model_cfg_stub"})
	})

	addr := getenv("LISTEN_ADDR", ":8090")
	logger.Info("starting internal-api", "addr", addr)
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
