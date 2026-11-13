// Command catalog menangani produk, kategori, harga, dan stok.
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
	"saybu/services/catalog/internal/repo"
	"saybu/services/catalog/internal/service"
	transportgrpc "saybu/services/catalog/internal/transport/grpc"
)

func main() {
	config.LoadDotEnv()
	log := logpkg.New("catalog", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, config.MustGet("CATALOG_DB_DSN"))
	if err != nil {
		log.Error("koneksi postgres gagal", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc, err := service.New(repo.New(pool))
	if err != nil {
		log.Error("inisialisasi catalog service gagal", "error", err)
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

		checks = append(checks, httpx.NamedCheck{Name: "kafka", Check: producer.Ping})
	}

	err = grpcapp.Run(ctx, grpcapp.Options{
		Name:      "catalog",
		GRPCAddr:  config.Get("CATALOG_GRPC_ADDR", ":50052"),
		AdminAddr: config.Get("CATALOG_ADMIN_ADDR", ":9102"),
		Logger:    log,
		Checks:    checks,
		Register: func(srv *grpc.Server) {
			transportgrpc.Register(srv, svc, log)
		},
	})
	if err != nil {
		log.Error("catalog service berhenti", "error", err)
		os.Exit(1)
	}
}
