//go:build integration

package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
	"saybu/pkg/kafkax"
	"saybu/services/catalog/internal/domain"
	"saybu/services/catalog/internal/repo"
)

// TestOrderEventConsumerIntegration menguji jalur Kafka -> konsumer catalog:
// order.cancelled / order.expired melepas stok, dan pengiriman ulang event yang
// sama tidak menggandakan pelepasan (idempoten lewat processed_events).
//
//	KAFKA_BROKERS=... CATALOG_DB_DSN=... go test -tags=integration ./services/catalog/internal/service/...
func TestOrderEventConsumerIntegration(t *testing.T) {
	dsn := os.Getenv("CATALOG_DB_DSN")
	rawBrokers := os.Getenv("KAFKA_BROKERS")
	if dsn == "" || rawBrokers == "" {
		t.Skip("CATALOG_DB_DSN / KAFKA_BROKERS tidak diset")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres tidak tersedia: %v", err)
	}

	r := repo.New(pool)
	svc, err := New(r)
	if err != nil {
		t.Fatal(err)
	}

	product, err := r.CreateProduct(ctx, domain.Product{
		ID: ids.New(), Name: "Kafka Test", Slug: "kafka-test-" + strings.ReplaceAll(ids.New(), "-", ""),
		Unit: domain.UnitPcs, Price: 1000, Stock: 1000, MinQty: 1, StepQty: 1, IsActive: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM stock_reservations WHERE product_id = $1`, product.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM products WHERE id = $1`, product.ID)
	})

	orderID := ids.New()
	if _, err := r.ReserveStock(ctx, orderID, []domain.StockItem{{ProductID: product.ID, Qty: 200}}); err != nil {
		t.Fatal(err)
	}
	if stock(t, ctx, pool, product.ID) != 800 {
		t.Fatalf("stok setelah reserve = %d, want 800", stock(t, ctx, pool, product.ID))
	}

	topic := fmt.Sprintf("saybu.test.orderevents.%d", time.Now().UnixNano())
	group := topic + ".group"
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	consumer, err := kafkax.NewConsumer(strings.Split(rawBrokers, ","), group, []string{topic}, svc.HandleOrderEvent, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(consumer.Close)
	go func() { _ = consumer.Run(ctx) }()

	producer, err := kafkax.NewProducer(strings.Split(rawBrokers, ","))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(producer.Close)

	// Beri waktu konsumer join group sebelum publish.
	time.Sleep(2 * time.Second)

	ev, err := kafkax.NewEvent("order.cancelled", orderID, map[string]string{"order_id": orderID}, "order-svc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM processed_events WHERE event_id = $1`, ev.EventID)
	})

	if err := producer.Publish(ctx, topic, orderID, ev); err != nil {
		t.Fatal(err)
	}

	// Stok harus kembali ke 1000.
	if !waitStock(t, ctx, pool, product.ID, 1000, 20*time.Second) {
		t.Fatalf("stok tidak kembali ke 1000 (kini %d)", stock(t, ctx, pool, product.ID))
	}

	// Kirim ulang event yang sama: harus diabaikan (stok tetap 1000).
	if err := producer.Publish(ctx, topic, orderID, ev); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Second)
	if got := stock(t, ctx, pool, product.ID); got != 1000 {
		t.Fatalf("stok setelah event duplikat = %d, want 1000 (tidak boleh dobel)", got)
	}
}

func stock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, productID string) int64 {
	t.Helper()
	var v int64
	if err := pool.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1`, productID).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func waitStock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, productID string, want int64, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if stock(t, ctx, pool, productID) == want {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
