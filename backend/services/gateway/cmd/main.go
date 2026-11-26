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
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/health/grpc_health_v1"

	authv1 "saybu/gen/saybu/auth/v1"
	catalogv1 "saybu/gen/saybu/catalog/v1"
	orderv1 "saybu/gen/saybu/order/v1"
	paymentv1 "saybu/gen/saybu/payment/v1"
	shippingv1 "saybu/gen/saybu/shipping/v1"
	"saybu/pkg/authn"
	"saybu/pkg/config"
	"saybu/pkg/grpcx"
	"saybu/pkg/httpx"
	logpkg "saybu/pkg/logger"
	"saybu/pkg/storage"
	"saybu/pkg/telemetry"
	"saybu/services/gateway/internal/auditmw"
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

	catalogConn, err := grpcx.Dial(config.Get("CATALOG_GRPC_ADDR", "localhost:50052"))
	if err != nil {
		log.Error("koneksi catalog gagal", "error", err)
		os.Exit(1)
	}
	defer catalogConn.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
	catalogHealth := grpc_health_v1.NewHealthClient(catalogConn)

	shippingConn, err := grpcx.Dial(config.Get("SHIPPING_GRPC_ADDR", "localhost:50053"))
	if err != nil {
		log.Error("koneksi shipping gagal", "error", err)
		os.Exit(1)
	}
	defer shippingConn.Close()

	shippingClient := shippingv1.NewShippingServiceClient(shippingConn)
	shippingHealth := grpc_health_v1.NewHealthClient(shippingConn)

	orderConn, err := grpcx.Dial(config.Get("ORDER_GRPC_ADDR", "localhost:50054"))
	if err != nil {
		log.Error("koneksi order gagal", "error", err)
		os.Exit(1)
	}
	defer orderConn.Close()

	orderClient := orderv1.NewOrderServiceClient(orderConn)
	orderHealth := grpc_health_v1.NewHealthClient(orderConn)

	paymentConn, err := grpcx.Dial(config.Get("PAYMENT_GRPC_ADDR", "localhost:50055"))
	if err != nil {
		log.Error("koneksi payment gagal", "error", err)
		os.Exit(1)
	}
	defer paymentConn.Close()

	paymentClient := paymentv1.NewPaymentServiceClient(paymentConn)
	paymentHealth := grpc_health_v1.NewHealthClient(paymentConn)

	uploads, err := storage.NewDisk(
		config.Get("UPLOAD_DIR", "./data/uploads"),
		config.Get("UPLOAD_URL_PREFIX", "/uploads"),
	)
	if err != nil {
		log.Error("inisialisasi penyimpanan unggahan gagal", "error", err)
		os.Exit(1)
	}
	catalogHandler := handlers.NewCatalog(
		catalogClient,
		log,
		uploads,
		config.Int64("UPLOAD_MAX_BYTES", 5<<20),
	)

	shippingHandler := handlers.NewShipping(shippingClient, log)

	orderHandler := handlers.NewOrder(orderClient, log, ratelimit.New(10, time.Minute))

	adminHandler := handlers.NewAdmin(orderClient, catalogClient, log)

	paymentHandler := handlers.NewPayment(
		paymentClient,
		log,
		uploads,
		config.Int64("UPLOAD_MAX_BYTES", 5<<20),
		ratelimit.New(5, time.Minute),
		handlers.BankInfo{
			Name:          config.Get("BANK_NAME", ""),
			AccountNumber: config.Get("BANK_ACCOUNT", ""),
			AccountHolder: config.Get("BANK_HOLDER", ""),
		},
	)

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
	r.Use(httpx.SecurityHeaders(config.Get("APP_ENV", "development") == "production"))
	r.Use(metrics.HTTPMiddleware)
	r.Use(httpx.CORS(config.List("GATEWAY_CORS_ORIGINS", []string{"http://localhost:5173"})))

	r.Get("/healthz", httpx.Live)
	r.Get("/readyz", httpx.Ready(
		httpx.NamedCheck{
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
		},
		httpx.NamedCheck{
			Name: "catalog",
			Check: func(ctx context.Context) error {
				resp, err := catalogHealth.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "catalog"})
				if err != nil {
					return err
				}
				if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
					return fmt.Errorf("status catalog: %s", resp.GetStatus())
				}
				return nil
			},
		},
		httpx.NamedCheck{
			Name: "shipping",
			Check: func(ctx context.Context) error {
				resp, err := shippingHealth.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "shipping"})
				if err != nil {
					return err
				}
				if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
					return fmt.Errorf("status shipping: %s", resp.GetStatus())
				}
				return nil
			},
		},
		httpx.NamedCheck{
			Name: "order",
			Check: func(ctx context.Context) error {
				resp, err := orderHealth.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "order"})
				if err != nil {
					return err
				}
				if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
					return fmt.Errorf("status order: %s", resp.GetStatus())
				}
				return nil
			},
		},
		httpx.NamedCheck{
			Name: "payment",
			Check: func(ctx context.Context) error {
				resp, err := paymentHealth.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "payment"})
				if err != nil {
					return err
				}
				if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
					return fmt.Errorf("status payment: %s", resp.GetStatus())
				}
				return nil
			},
		},
	))
	r.Handle("/metrics", metrics.Handler())

	// Gambar produk dilayani langsung dari disk (M7 bisa pindah ke object storage/CDN).
	r.Handle(config.Get("UPLOAD_URL_PREFIX", "/uploads")+"/*", http.StripPrefix(
		config.Get("UPLOAD_URL_PREFIX", "/uploads")+"/",
		noDirListing(http.FileServer(http.Dir(uploads.Dir()))),
	))

	r.Route("/api/v1", func(r chi.Router) {
		// Batas baca umum: 120/menit per IP (API_SPEC §10).
		r.Use(ratelimit.Middleware(ratelimit.New(120, time.Minute), ratelimit.ReadMethods, ratelimit.ByIP))

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

		r.Get("/categories", catalogHandler.ListCategories)
		r.Get("/products", catalogHandler.ListProducts)
		r.Get("/products/{slug}", catalogHandler.GetProduct)

		r.Get("/shipping/zones", shippingHandler.ListZones)
		r.Get("/shipping/quote", shippingHandler.Quote)

		// Checkout & pelacakan: guest boleh, akun dikenali bila login.
		r.Group(func(r chi.Router) {
			r.Use(authmw.Optional(tokens))
			r.Post("/orders", orderHandler.CreateOrder)
			r.Get("/orders/{code}", orderHandler.GetOrder)
			r.Post("/orders/{code}/cancel", orderHandler.CancelOrder)
			r.Get("/orders/{code}/payment", paymentHandler.GetPayment)
			r.Post("/orders/{code}/payment-proof", paymentHandler.UploadProof)
		})

		r.Group(func(r chi.Router) {
			r.Use(authmw.Authenticate(tokens))
			r.Get("/me", authHandler.Me)
			r.Get("/me/orders", orderHandler.ListMyOrders)

			r.Route("/admin", func(r chi.Router) {
				r.Use(authmw.RequireRole("admin"))
				// Batas tulis admin: 60/menit per user (API_SPEC §10).
				r.Use(ratelimit.Middleware(ratelimit.New(60, time.Minute), ratelimit.WriteMethods,
					func(req *http.Request) string {
						if id, ok := authmw.IdentityFrom(req.Context()); ok {
							return "user:" + id.UserID
						}
						return ratelimit.ByIP(req)
					}))
				r.Use(auditmw.Middleware(log, authClient))
				r.Get("/users", authHandler.ListUsers)

				r.Get("/orders", adminHandler.ListOrders)
				r.Get("/orders/{id}", adminHandler.GetOrder)
				r.Patch("/orders/{id}/status", adminHandler.UpdateStatus)
				r.Get("/stats/today", adminHandler.TodayStats)

				r.Get("/products", catalogHandler.AdminListProducts)
				r.Post("/products", catalogHandler.CreateProduct)
				r.Patch("/products/{id}", catalogHandler.UpdateProduct)
				r.Patch("/products/{id}/stock", catalogHandler.SetStock)
				r.Delete("/products/{id}", catalogHandler.DeleteProduct)
				r.Post("/uploads", catalogHandler.Upload)

				r.Get("/shipping/zones", shippingHandler.AdminListZones)
				r.Post("/shipping/zones", shippingHandler.CreateZone)
				r.Patch("/shipping/zones/{id}", shippingHandler.UpdateZone)
				r.Delete("/shipping/zones/{id}", shippingHandler.DeleteZone)

				r.Get("/payments", paymentHandler.AdminListPayments)
				r.Post("/payments/{id}/verify", paymentHandler.AdminVerifyPayment)
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

// noDirListing menolak permintaan ke direktori (mencegah daftar isi folder).
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
