package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestMetricsExposeKeySeries(t *testing.T) {
	m := NewMetrics("order")

	// Latih setiap seri agar muncul di /metrics.
	m.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))

	interceptor := m.UnaryServerInterceptor()
	_, _ = interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/saybu.order.v1.OrderService/CreateOrder"},
		func(context.Context, any) (any, error) { return nil, nil })

	m.IncOrdersCreated()
	m.ObserveOutboxLag(1500 * time.Millisecond)
	m.SetConsumerLag("order-svc", "payment.events.v1", 7)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, name := range []string{
		"saybu_order_http_requests_total",
		"saybu_order_http_request_duration_seconds",
		"saybu_order_grpc_requests_total",
		"saybu_order_grpc_server_handling_seconds",
		"saybu_order_outbox_lag_seconds",
		"saybu_order_kafka_consumer_lag",
		"saybu_order_orders_created_total",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metrik %s tidak ditemukan di /metrics", name)
		}
	}
	if !strings.Contains(body, "saybu_order_outbox_lag_seconds 1.5") {
		t.Errorf("nilai outbox lag salah:\n%s", body)
	}
	if !strings.Contains(body, `saybu_order_kafka_consumer_lag{group="order-svc",topic="payment.events.v1"} 7`) {
		t.Errorf("nilai kafka lag salah:\n%s", body)
	}
}
