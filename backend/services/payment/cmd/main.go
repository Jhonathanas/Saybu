// Command payment menangani pembayaran manual, bukti transfer, dan verifikasi.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/config"
	"saybu/pkg/grpcapp"
	"saybu/pkg/httpx"
	logpkg "saybu/pkg/logger"
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

	err = grpcapp.Run(ctx, grpcapp.Options{
		Name:      "payment",
		GRPCAddr:  config.Get("PAYMENT_GRPC_ADDR", ":50055"),
		AdminAddr: config.Get("PAYMENT_ADMIN_ADDR", ":9105"),
		Logger:    log,
		Checks: []httpx.NamedCheck{
			{Name: "postgres", Check: func(ctx context.Context) error { return pool.Ping(ctx) }},
		},
	})
	if err != nil {
		log.Error("payment service berhenti", "error", err)
		os.Exit(1)
	}
}
