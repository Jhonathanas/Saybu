package telemetry

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Metrics mengumpulkan metrik HTTP dan gRPC untuk satu service.
type Metrics struct {
	registry     *prometheus.Registry
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	grpcRequests *prometheus.CounterVec
}

// NewMetrics membuat registry metrik dengan namespace "saybu" dan subsystem nama service.
func NewMetrics(service string) *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		registry: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "saybu", Subsystem: service, Name: "http_requests_total",
			Help: "Jumlah request HTTP.",
		}, []string{"method", "path", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "saybu", Subsystem: service, Name: "http_request_duration_seconds",
			Help:    "Durasi request HTTP.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		grpcRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "saybu", Subsystem: service, Name: "grpc_requests_total",
			Help: "Jumlah request gRPC.",
		}, []string{"method", "code"}),
	}
	reg.MustRegister(m.httpRequests, m.httpDuration, m.grpcRequests)
	return m
}

// Handler mengembalikan handler /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware mencatat jumlah dan durasi request.
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		m.httpRequests.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rec.status)).Inc()
		m.httpDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
	})
}

// UnaryServerInterceptor mencatat metrik request gRPC.
func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		m.grpcRequests.WithLabelValues(info.FullMethod, status.Code(err).String()).Inc()
		return resp, err
	}
}
