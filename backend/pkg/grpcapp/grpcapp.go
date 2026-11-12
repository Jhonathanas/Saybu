// Package grpcapp menjalankan service gRPC beserta server admin HTTP
// (health, readiness, metrics) dan graceful shutdown.
package grpcapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"saybu/pkg/grpcx"
	"saybu/pkg/httpx"
	"saybu/pkg/telemetry"
)

// Options konfigurasi service.
type Options struct {
	Name      string
	GRPCAddr  string
	AdminAddr string
	Logger    *slog.Logger
	Metrics   *telemetry.Metrics
	Checks    []httpx.NamedCheck
	Register  func(*grpc.Server)
}

// Run menjalankan service sampai ctx dibatalkan.
func Run(ctx context.Context, opts Options) error {
	if opts.Logger == nil {
		return errors.New("grpcapp: logger wajib")
	}
	if opts.GRPCAddr == "" || opts.AdminAddr == "" {
		return errors.New("grpcapp: alamat gRPC dan admin wajib")
	}
	if opts.Metrics == nil {
		opts.Metrics = telemetry.NewMetrics(opts.Name)
	}

	srv := grpcx.NewServer(opts.Logger, opts.Metrics)

	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus(opts.Name, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	if opts.Register != nil {
		opts.Register(srv)
	}
	reflection.Register(srv)

	lis, err := net.Listen("tcp", opts.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC %s: %w", opts.GRPCAddr, err)
	}

	adminSrv := &http.Server{
		Addr:              opts.AdminAddr,
		Handler:           httpx.NewAdminMux(opts.Metrics, opts.Checks...),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			opts.Logger.Error("admin server berhenti", "error", err)
		}
	}()
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			opts.Logger.Error("grpc server berhenti", "error", err)
		}
	}()

	opts.Logger.Info("service started", "grpc_addr", opts.GRPCAddr, "admin_addr", opts.AdminAddr)
	<-ctx.Done()
	opts.Logger.Info("service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hs.Shutdown()

	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		srv.Stop()
	}

	_ = adminSrv.Shutdown(shutdownCtx)
	return nil
}
