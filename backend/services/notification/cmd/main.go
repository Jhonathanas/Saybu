// Command notification mengonsumsi event Kafka dan mengirim notifikasi Telegram.
// Pada M0 baru menyediakan health/ready/metrics; consumer ditambahkan di M4.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"saybu/pkg/config"
	"saybu/pkg/httpx"
	logpkg "saybu/pkg/logger"
	"saybu/pkg/telemetry"
)

func main() {
	config.LoadDotEnv()
	log := logpkg.New("notification", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := config.List("KAFKA_BROKERS", []string{"localhost:29092"})
	kafka, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		log.Error("kafka client gagal", "error", err)
		os.Exit(1)
	}
	defer kafka.Close()

	metrics := telemetry.NewMetrics("notification")
	mux := httpx.NewAdminMux(metrics, httpx.NamedCheck{
		Name:  "kafka",
		Check: func(ctx context.Context) error { return kafka.Ping(ctx) },
	})
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		httpx.Data(w, http.StatusOK, map[string]string{"service": "notification", "version": "0.1.0"})
	})

	srv := &http.Server{
		Addr:              config.Get("NOTIFICATION_HTTP_ADDR", ":8086"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("notification started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("notification berhenti", "error", err)
		}
	}()

	<-ctx.Done()
	log.Info("notification shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("notification shutdown gagal", "error", err)
	}
}
