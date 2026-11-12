// Command auth menangani registrasi, login, refresh token, dan profil user.
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
	log := logpkg.New("auth", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, config.MustGet("AUTH_DB_DSN"))
	if err != nil {
		log.Error("koneksi postgres gagal", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	err = grpcapp.Run(ctx, grpcapp.Options{
		Name:      "auth",
		GRPCAddr:  config.Get("AUTH_GRPC_ADDR", ":50051"),
		AdminAddr: config.Get("AUTH_ADMIN_ADDR", ":9101"),
		Logger:    log,
		Checks: []httpx.NamedCheck{
			{Name: "postgres", Check: func(ctx context.Context) error { return pool.Ping(ctx) }},
		},
	})
	if err != nil {
		log.Error("auth service berhenti", "error", err)
		os.Exit(1)
	}
}
