// Command order menangani checkout, state machine pesanan, dan pelacakan.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	shippingv1 "saybu/gen/saybu/shipping/v1"
	"saybu/pkg/config"
	"saybu/pkg/grpcapp"
	"saybu/pkg/grpcx"
	"saybu/pkg/httpx"
	"saybu/pkg/kafkax"
	logpkg "saybu/pkg/logger"
	"saybu/pkg/outbox"
	"saybu/services/order/internal/repo"
	"saybu/services/order/internal/service"
	transportgrpc "saybu/services/order/internal/transport/grpc"
)

func main() {
	config.LoadDotEnv()
	log := logpkg.New("order", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, config.MustGet("ORDER_DB_DSN"))
	if err != nil {
		log.Error("koneksi postgres gagal", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	catalogConn, err := grpcx.Dial(config.Get("CATALOG_GRPC_ADDR", "localhost:50052"))
	if err != nil {
		log.Error("koneksi catalog gagal", "error", err)
		os.Exit(1)
	}
	defer catalogConn.Close()

	shippingConn, err := grpcx.Dial(config.Get("SHIPPING_GRPC_ADDR", "localhost:50053"))
	if err != nil {
		log.Error("koneksi shipping gagal", "error", err)
		os.Exit(1)
	}
	defer shippingConn.Close()

	svc, err := service.New(
		repo.New(pool),
		catalogv1.NewCatalogServiceClient(catalogConn),
		shippingv1.NewShippingServiceClient(shippingConn),
		log,
		config.Duration("ORDER_EXPIRY", 24*time.Hour),
	)
	if err != nil {
		log.Error("inisialisasi order service gagal", "error", err)
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

	// Job kedaluwarsa: PENDING_PAYMENT lewat expires_at -> EXPIRED + event.
	go svc.RunExpiry(ctx, config.Duration("ORDER_EXPIRY_SWEEP", time.Minute))

	err = grpcapp.Run(ctx, grpcapp.Options{
		Name:      "order",
		GRPCAddr:  config.Get("ORDER_GRPC_ADDR", ":50054"),
		AdminAddr: config.Get("ORDER_ADMIN_ADDR", ":9104"),
		Logger:    log,
		Checks:    checks,
		Register: func(srv *grpc.Server) {
			transportgrpc.Register(srv, svc, log)
		},
	})
	if err != nil {
		log.Error("order service berhenti", "error", err)
		os.Exit(1)
	}
}
