// Command gateway adalah satu-satunya pintu masuk HTTP dari browser.
// Gateway memvalidasi JWT lalu meneruskan request ke service internal via gRPC.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/health/grpc_health_v1"

	authv1 "saybu/gen/saybu/auth/v1"
	"saybu/pkg/authn"
	"saybu/pkg/config"
	"saybu/pkg/grpcx"
	"saybu/pkg/httpx"
	logpkg "saybu/pkg/logger"
	"saybu/pkg/telemetry"
	"saybu/services/gateway/internal/authmw"
	"saybu/services/gateway/internal/handlers"
	"saybu/services/gateway/internal/ratelimit"
)

func main() {
	config.LoadDotEnv()
	log := logpkg.New("gateway", config.Get("LOG_LEVEL", "info"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tokens, err := authn.NewManager(
		config.MustGet("JWT_SECRET"),
		config.Duration("JWT_ACCESS_TTL", 15*time.Minute),
	)
	if err != nil {
		log.Error("konfigurasi token gagal", "error", err)
		os.Exit(1)
	}

	authConn, err := grpcx.Dial(config.Get("AUTH_GRPC_ADDR", "localhost:50051"))
	if err != nil {
		log.Error("koneksi auth gagal", "error", err)
		os.Exit(1)
	}
	defer authConn.Close()

	authClient := authv1.NewAuthServiceClient(authConn)
	authHealth := grpc_health_v1.NewHealthClient(authConn)

	cookieSecure := config.Bool("COOKIE_SECURE", config.Get("APP_ENV", "development") == "production")
	authHandler := handlers.NewAuth(authClient, log, handlers.CookieConfig{
		Name:   "saybu_rt",
		Path:   "/api/v1/auth",
		Secure: cookieSecure,
	}, ratelimit.New(3, time.Minute), ratelimit.New(5, time.Minute))

	metrics := telemetry.NewMetrics("gateway")

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.Recover(log))
	r.Use(httpx.Logger(log))
	r.Use(metrics.HTTPMiddleware)
	r.Use(httpx.CORS(config.List("GATEWAY_CORS_ORIGINS", []string{"http://localhost:5173"})))

	r.Get("/healthz", httpx.Live)
	r.Get("/readyz", httpx.Ready(httpx.NamedCheck{
		Name: "auth",
		Check: func(ctx context.Context) error {
			resp, err := authHealth.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "auth"})
			if err != nil {
				return err
			}
			if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
				return fmt.Errorf("status auth: %s", resp.GetStatus())
			}
			return nil
		},
	}))
	r.Handle("/metrics", metrics.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/version", func(w http.ResponseWriter, req *http.Request) {
			httpx.Data(w, http.StatusOK, map[string]string{
				"service": "gateway",
				"version": "0.1.0",
				"env":     config.Get("APP_ENV", "development"),
			})
		})

		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/refresh", authHandler.Refresh)
		r.Post("/auth/logout", authHandler.Logout)

		r.Group(func(r chi.Router) {
			r.Use(authmw.Authenticate(tokens))
			r.Get("/me", authHandler.Me)

			r.Route("/admin", func(r chi.Router) {
				r.Use(authmw.RequireRole("admin"))
				r.Get("/users", authHandler.ListUsers)
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
