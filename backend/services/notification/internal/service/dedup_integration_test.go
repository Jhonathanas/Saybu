//go:build integration

package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
)

// TestPostgresDedupIntegration membutuhkan saybu_notifications dengan migrasi
// notification sudah dijalankan.
//
//	NOTIFICATION_DB_DSN=... go test -tags=integration ./services/notification/internal/service/...
func TestPostgresDedupIntegration(t *testing.T) {
	dsn := os.Getenv("NOTIFICATION_DB_DSN")
	if dsn == "" {
		t.Skip("NOTIFICATION_DB_DSN tidak diset")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres tidak tersedia: %v", err)
	}

	store := NewPostgresDedup(pool)
	eventID := "integration-" + strings.ReplaceAll(ids.New(), "-", "")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM processed_events WHERE event_id = $1`, eventID)
	})

	first, err := store.Claim(ctx, eventID, "order.created")
	if err != nil || !first {
		t.Fatalf("claim pertama = %v, %v", first, err)
	}
	second, err := store.Claim(ctx, eventID, "order.created")
	if err != nil || second {
		t.Fatalf("claim kedua = %v, %v (harus duplikat)", second, err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM processed_events WHERE event_id = $1`, eventID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("status = %q, want pending", status)
	}

	if err := store.MarkSent(ctx, eventID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM processed_events WHERE event_id = $1`, eventID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "sent" {
		t.Fatalf("status = %q, want sent", status)
	}

	releaseID := fmt.Sprintf("%s-release", eventID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM processed_events WHERE event_id = $1`, releaseID)
	})
	if _, err := store.Claim(ctx, releaseID, "stock.low"); err != nil {
		t.Fatal(err)
	}
	if err := store.Release(ctx, releaseID); err != nil {
		t.Fatal(err)
	}
	retry, err := store.Claim(ctx, releaseID, "stock.low")
	if err != nil || !retry {
		t.Fatalf("claim setelah release = %v, %v", retry, err)
	}
}
