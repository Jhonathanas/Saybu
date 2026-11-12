// Command gateway adalah satu-satunya pintu masuk HTTP dari browser.
// Gateway memvalidasi JWT lalu meneruskan request ke service internal via gRPC.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"saybu/pkg/config"
	"saybu/pkg/httpx"
	logpkg "saybu/pkg/logger"
	"saybu/pkg/telemetry"
)

func main() {
	config.LoadDotEnv()
	log := logpkg.New("gateway", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metrics := telemetry.NewMetrics("gateway")

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.Recover(log))
	r.Use(httpx.Logger(log))
	r.Use(metrics.HTTPMiddleware)
	r.Use(httpx.CORS(config.List("GATEWAY_CORS_ORIGINS", []string{"http://localhost:5173"})))

	r.Get("/healthz", httpx.Live)
	r.Get("/readyz", httpx.Ready())
	r.Handle("/metrics", metrics.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/version", func(w http.ResponseWriter, req *http.Request) {
			httpx.Data(w, http.StatusOK, map[string]string{
				"service": "gateway",
				"version": "0.1.0",
				"env":     config.Get("APP_ENV", "development"),
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteError(w, req, httpx.NotFound("Endpoint tidak ditemukan."))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteError(w, req, httpx.New(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Metode tidak diizinkan."))
	})

	srv := &http.Server{
		Addr:              config.Get("GATEWAY_HTTP_ADDR", ":8081"),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("gateway started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("gateway berhenti", "error", err)
		}
	}()

	<-ctx.Done()
	log.Info("gateway shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("gateway shutdown gagal", "error", err)
	}
}
