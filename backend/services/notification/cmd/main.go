// Command notification mengonsumsi event Kafka dan mengirim notifikasi Telegram.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"saybu/pkg/config"
	"saybu/pkg/httpx"
	"saybu/pkg/kafkax"
	logpkg "saybu/pkg/logger"
	"saybu/pkg/telemetry"
	"saybu/services/notification/internal/service"
	"saybu/services/notification/internal/telegram"
)

func main() {
	config.LoadDotEnv()
	log := logpkg.New("notification", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	telegramClient := telegram.NewClient(
		config.Get("TELEGRAM_BOT_TOKEN", ""),
		config.Get("TELEGRAM_ADMIN_CHAT_ID", ""),
	)
	if !telegramClient.Enabled() {
		log.Warn("telegram nonaktif: TELEGRAM_BOT_TOKEN/TELEGRAM_ADMIN_CHAT_ID kosong; notifikasi dilewati")
	}

	notifier := service.New(telegramClient, log)

	metrics := telemetry.NewMetrics("notification")
	checks := []httpx.NamedCheck{}

	// Konsumer event pesanan + pembayaran + katalog -> notifikasi Telegram.
	if brokers := config.List("KAFKA_BROKERS", nil); len(brokers) > 0 {
		consumer, err := kafkax.NewConsumer(brokers, "notification-svc",
			[]string{"order.events.v1", "payment.events.v1", "catalog.events.v1"}, notifier.HandleEvent, log)
		if err != nil {
			log.Error("konsumer event gagal disiapkan", "error", err)
			os.Exit(1)
		}
		defer consumer.Close()
		go func() {
			if err := consumer.Run(ctx); err != nil {
				log.Error("konsumer event berhenti", "error", err)
			}
		}()
		go consumer.ReportLagLoop(ctx, metrics, 15*time.Second)

		checks = append(checks, httpx.NamedCheck{Name: "kafka", Check: consumer.Ping})
	} else {
		log.Warn("KAFKA_BROKERS kosong; konsumer notifikasi tidak dijalankan")
	}

	mux := httpx.NewAdminMux(metrics, checks...)
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
