// Command auth menangani registrasi, login, refresh token, dan profil user.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"saybu/pkg/authn"
	"saybu/pkg/config"
	"saybu/pkg/grpcapp"
	"saybu/pkg/httpx"
	logpkg "saybu/pkg/logger"
	"saybu/services/auth/internal/domain"
	"saybu/services/auth/internal/repo"
	"saybu/services/auth/internal/service"
	transportgrpc "saybu/services/auth/internal/transport/grpc"
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

	tokens, err := authn.NewManager(
		config.MustGet("JWT_SECRET"),
		config.Duration("JWT_ACCESS_TTL", 15*time.Minute),
	)
	if err != nil {
		log.Error("konfigurasi token gagal", "error", err)
		os.Exit(1)
	}

	svc, err := service.New(service.Options{
		Repo:       repo.New(pool),
		Hasher:     domain.NewHasher(),
		Tokens:     tokens,
		RefreshTTL: config.Duration("JWT_REFRESH_TTL", 30*24*time.Hour),
	})
	if err != nil {
		log.Error("inisialisasi auth service gagal", "error", err)
		os.Exit(1)
	}

	err = grpcapp.Run(ctx, grpcapp.Options{
		Name:      "auth",
		GRPCAddr:  config.Get("AUTH_GRPC_ADDR", ":50051"),
		AdminAddr: config.Get("AUTH_ADMIN_ADDR", ":9101"),
		Logger:    log,
		Checks: []httpx.NamedCheck{
			{Name: "postgres", Check: func(ctx context.Context) error { return pool.Ping(ctx) }},
		},
		Register: func(srv *grpc.Server) {
			transportgrpc.Register(srv, svc, log)
		},
	})
	if err != nil {
		log.Error("auth service berhenti", "error", err)
		os.Exit(1)
	}
}
