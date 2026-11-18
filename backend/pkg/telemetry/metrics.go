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
	registry      *prometheus.Registry
	httpRequests  *prometheus.CounterVec
	httpDuration  *prometheus.HistogramVec
	grpcRequests  *prometheus.CounterVec
	grpcDuration  *prometheus.HistogramVec
	outboxLag     prometheus.Gauge
	kafkaLag      *prometheus.GaugeVec
	ordersCreated prometheus.Counter
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
		grpcDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "saybu", Subsystem: service, Name: "grpc_server_handling_seconds",
			Help:    "Durasi penanganan request gRPC.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "code"}),
		outboxLag: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "saybu", Subsystem: service, Name: "outbox_lag_seconds",
			Help: "Usia event outbox tertua yang belum terkirim.",
		}),
		kafkaLag: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "saybu", Subsystem: service, Name: "kafka_consumer_lag",
			Help: "Lag consumer group per topic (jumlah pesan tertinggal).",
		}, []string{"group", "topic"}),
		ordersCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "saybu", Subsystem: service, Name: "orders_created_total",
			Help: "Jumlah pesanan yang berhasil dibuat.",
		}),
	}
	reg.MustRegister(m.httpRequests, m.httpDuration, m.grpcRequests, m.grpcDuration,
		m.outboxLag, m.kafkaLag, m.ordersCreated)
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
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err).String()
		m.grpcRequests.WithLabelValues(info.FullMethod, code).Inc()
		m.grpcDuration.WithLabelValues(info.FullMethod, code).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

// ObserveOutboxLag mencatat usia event outbox tertua yang belum terkirim.
func (m *Metrics) ObserveOutboxLag(d time.Duration) {
	if d < 0 {
		d = 0
	}
	m.outboxLag.Set(d.Seconds())
}

// SetConsumerLag mencatat lag consumer group untuk satu topic.
func (m *Metrics) SetConsumerLag(group, topic string, lag int64) {
	if lag < 0 {
		lag = 0
	}
	m.kafkaLag.WithLabelValues(group, topic).Set(float64(lag))
}

// IncOrdersCreated menambah penghitung pesanan yang berhasil dibuat.
func (m *Metrics) IncOrdersCreated() { m.ordersCreated.Inc() }
