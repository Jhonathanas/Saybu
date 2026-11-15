// Command payment menangani pembayaran manual, bukti transfer, dan verifikasi.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"saybu/pkg/config"
	"saybu/pkg/grpcapp"
	"saybu/pkg/httpx"
	"saybu/pkg/kafkax"
	logpkg "saybu/pkg/logger"
	"saybu/pkg/outbox"
	"saybu/services/payment/internal/repo"
	"saybu/services/payment/internal/service"
	transportgrpc "saybu/services/payment/internal/transport/grpc"
)

func main() {
	config.LoadDotEnv()
	log := logpkg.New("payment", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, config.MustGet("PAYMENT_DB_DSN"))
	if err != nil {
		log.Error("koneksi postgres gagal", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc, err := service.New(repo.New(pool), log)
	if err != nil {
		log.Error("inisialisasi payment service gagal", "error", err)
		os.Exit(1)
	}

	checks := []httpx.NamedCheck{
		{Name: "postgres", Check: func(ctx context.Context) error { return pool.Ping(ctx) }},
	}

	// Producer Kafka + relay outbox (opsional bila broker tidak dikonfigurasi).
	if brokers := config.List("KAFKA_BROKERS", nil); len(brokers) > 0 {
		producer, err := kafkax.NewProducer(brokers)
		if err != nil {
			log.Error("koneksi kafka gagal", "error", err)
			os.Exit(1)
		}
		defer producer.Close()

		relay := outbox.NewRelay(outbox.NewStore(pool), producer, log)
		go relay.Run(ctx)

		// Konsumer event pesanan: order.created membuat pembayaran,
		// batal/kedaluwarsa menandai pembayaran batal.
		consumer, err := kafkax.NewConsumer(brokers, "payment-svc",
			[]string{"order.events.v1"}, svc.HandleOrderEvent, log)
		if err != nil {
			log.Error("konsumer order.events gagal disiapkan", "error", err)
			os.Exit(1)
		}
		defer consumer.Close()
		go func() {
			if err := consumer.Run(ctx); err != nil {
				log.Error("konsumer order.events berhenti", "error", err)
			}
		}()

		checks = append(checks, httpx.NamedCheck{Name: "kafka", Check: producer.Ping})
	}

	err = grpcapp.Run(ctx, grpcapp.Options{
		Name:      "payment",
		GRPCAddr:  config.Get("PAYMENT_GRPC_ADDR", ":50055"),
		AdminAddr: config.Get("PAYMENT_ADMIN_ADDR", ":9105"),
		Logger:    log,
		Checks:    checks,
		Register: func(srv *grpc.Server) {
			transportgrpc.Register(srv, svc, log)
		},
	})
	if err != nil {
		log.Error("payment service berhenti", "error", err)
		os.Exit(1)
	}
}
