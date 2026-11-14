//go:build integration

package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
	"saybu/pkg/outbox"
	"saybu/services/catalog/internal/domain"
)

// TestRepoIntegration membutuhkan Postgres dengan migrasi catalog sudah dijalankan.
//
//	CATALOG_DB_DSN=... go test -tags=integration ./services/catalog/internal/repo/...
func TestRepoIntegration(t *testing.T) {
	dsn := os.Getenv("CATALOG_DB_DSN")
	if dsn == "" {
		t.Skip("CATALOG_DB_DSN tidak diset")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	// Daftarkan lewat t.Cleanup agar pool baru ditutup setelah cleanup data
	// (defer berjalan lebih dulu daripada t.Cleanup).
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres tidak tersedia: %v", err)
	}

	raw := strings.ReplaceAll(ids.New(), "-", "")
	n, err := strconv.ParseUint(raw[len(raw)-8:], 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%08d", n%100000000)

	catID := ids.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug) VALUES ($1, $2, $3)
	`, catID, "Kategori "+suffix, "kategori-"+suffix); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM categories WHERE id = $1`, catID)
	})

	r := New(pool)
	slug := "tomat-" + suffix

	created, err := r.CreateProduct(ctx, domain.Product{
		ID: ids.New(), Name: "Tomat", Slug: slug, Unit: domain.UnitKg,
		Price: 15000, Stock: 10000, MinQty: 100, StepQty: 100, IsActive: true,
		Category: &domain.Category{ID: catID},
		Images:   []string{"/uploads/a.jpg", "/uploads/b.jpg"},
	}, []outbox.Record{newTestEvent("product.updated", slug)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM products WHERE id = $1`, created.ID)
	})

	if len(created.Images) != 2 {
		t.Fatalf("images = %+v", created.Images)
	}
	if created.Category == nil || created.Category.ID != catID {
		t.Fatalf("category = %+v", created.Category)
	}

	// Duplikat slug.
	if _, err := r.CreateProduct(ctx, domain.Product{
		ID: ids.New(), Name: "Tomat Lain", Slug: slug, Unit: domain.UnitKg,
		Price: 1, MinQty: 1, StepQty: 1, IsActive: true,
	}, nil); !errors.Is(err, domain.ErrSlugExists) {
		t.Fatalf("duplikat slug: err = %v", err)
	}

	// Ambil berdasarkan slug.
	got, err := r.GetProductBySlug(ctx, slug, false)
	if err != nil || got.ID != created.ID {
		t.Fatalf("GetProductBySlug = %+v, %v", got, err)
	}

	// List dengan filter kategori.
	list, total, err := r.ListProducts(ctx, domain.ProductFilter{
		CategorySlug: "kategori-" + suffix, Page: 1, Limit: 20,
	})
	if err != nil || total < 1 || len(list) < 1 {
		t.Fatalf("ListProducts = %d, %d, %v", len(list), total, err)
	}

	// Set stok.
	updated, err := r.SetStock(ctx, created.ID, 2000, nil)
	if err != nil || updated.Stock != 2000 {
		t.Fatalf("SetStock = %+v, %v", updated, err)
	}

	// Reservasi stok.
	orderID := ids.New()
	insufficient, err := r.ReserveStock(ctx, orderID, []domain.StockItem{{ProductID: created.ID, Qty: 1000}})
	if err != nil || len(insufficient) != 0 {
		t.Fatalf("ReserveStock = %+v, %v", insufficient, err)
	}
	after, _ := r.GetProductByID(ctx, created.ID, true)
	if after.Stock != 1000 {
		t.Fatalf("stok setelah reserve = %d, want 1000", after.Stock)
	}

	// Reservasi melebihi stok harus gagal & tidak mengubah stok.
	insufficient, err = r.ReserveStock(ctx, ids.New(), []domain.StockItem{{ProductID: created.ID, Qty: 999999}})
	if err != nil || len(insufficient) != 1 {
		t.Fatalf("reserve berlebih = %+v, %v", insufficient, err)
	}

	// Release mengembalikan stok.
	if err := r.ReleaseStock(ctx, orderID); err != nil {
		t.Fatal(err)
	}
	released, _ := r.GetProductByID(ctx, created.ID, true)
	if released.Stock != 2000 {
		t.Fatalf("stok setelah release = %d, want 2000", released.Stock)
	}

	// ReleaseStockOnce: idempotent terhadap event_id (dipakai konsumer Kafka).
	orderID2 := ids.New()
	if _, err := r.ReserveStock(ctx, orderID2, []domain.StockItem{{ProductID: created.ID, Qty: 500}}); err != nil {
		t.Fatal(err)
	}
	eventID := ids.New()
	processed, err := r.ReleaseStockOnce(ctx, eventID, orderID2)
	if err != nil || !processed {
		t.Fatalf("ReleaseStockOnce = %v, %v", processed, err)
	}
	afterOnce, _ := r.GetProductByID(ctx, created.ID, true)
	if afterOnce.Stock != 2000 {
		t.Fatalf("stok setelah release once = %d, want 2000", afterOnce.Stock)
	}
	// Event yang sama diulang tidak berefek lagi (stok tidak bertambah dua kali).
	processed, err = r.ReleaseStockOnce(ctx, eventID, orderID2)
	if err != nil || processed {
		t.Fatalf("ReleaseStockOnce ulang = %v, %v", processed, err)
	}
	afterRepeat, _ := r.GetProductByID(ctx, created.ID, true)
	if afterRepeat.Stock != 2000 {
		t.Fatalf("stok setelah release once ulang = %d, want 2000", afterRepeat.Stock)
	}

	// Soft delete.
	if _, err := r.SetActive(ctx, created.ID, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetProductBySlug(ctx, slug, false); !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("produk nonaktif seharusnya 404, err = %v", err)
	}
}

func newTestEvent(eventType, key string) outbox.Record {
	return outbox.Record{
		EventID: ids.New(),
		Topic:   "catalog.events.v1",
		Key:     key,
		Payload: []byte(`{"test":true}`),
		Headers: map[string]string{"event-type": eventType},
	}
}
