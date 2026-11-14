//go:build integration

package repo

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
	"saybu/services/shipping/internal/domain"
)

// TestRepoIntegration membutuhkan Postgres dengan migrasi shipping sudah dijalankan.
//
//	SHIPPING_DB_DSN=... go test -tags=integration ./services/shipping/internal/repo/...
func TestRepoIntegration(t *testing.T) {
	dsn := os.Getenv("SHIPPING_DB_DSN")
	if dsn == "" {
		t.Skip("SHIPPING_DB_DSN tidak diset")
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

	raw := ids.New()
	suffix := raw[len(raw)-8:]

	r := New(pool)

	created, err := r.CreateZone(ctx, domain.Zone{
		ID: ids.New(), Name: "Zona " + suffix,
		Areas: []string{"A", "B"}, Fee: 11000, FreeMinSubtotal: 150000, IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM zones WHERE id = $1`, created.ID)
	})
	if created.Name != "Zona "+suffix || len(created.Areas) != 2 || created.Fee != 11000 {
		t.Fatalf("created = %+v", created)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("created_at kosong")
	}

	// Nama duplikat ditolak.
	if _, err := r.CreateZone(ctx, domain.Zone{
		ID: ids.New(), Name: "Zona " + suffix, Fee: 1, IsActive: true,
	}); !errors.Is(err, domain.ErrZoneExists) {
		t.Fatalf("duplikat nama: err = %v", err)
	}

	// Update.
	created.Fee = 13000
	created.Areas = []string{"A"}
	updated, err := r.UpdateZone(ctx, created)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Fee != 13000 || len(updated.Areas) != 1 {
		t.Fatalf("updated = %+v", updated)
	}

	// List hanya aktif.
	list, err := r.ListZones(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, z := range list {
		if z.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("zona aktif tidak muncul di ListZones")
	}

	// Nonaktifkan lalu pastikan tidak muncul di list publik.
	created.IsActive = false
	if _, err := r.UpdateZone(ctx, created); err != nil {
		t.Fatal(err)
	}
	list, _ = r.ListZones(ctx, false)
	for _, z := range list {
		if z.ID == created.ID {
			t.Fatal("zona nonaktif muncul di ListZones publik")
		}
	}

	// Update id tak dikenal.
	if _, err := r.UpdateZone(ctx, domain.Zone{ID: ids.New(), Name: "X", Fee: 1}); !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("update tak dikenal: err = %v", err)
	}

	// Get tak dikenal.
	if _, err := r.GetZone(ctx, ids.New()); !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("get tak dikenal: err = %v", err)
	}

	// UUID tidak valid diperlakukan sebagai tidak ditemukan (bukan 500).
	if _, err := r.GetZone(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("get uuid invalid: err = %v", err)
	}

	// Delete.
	if err := r.DeleteZone(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteZone(ctx, created.ID); !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("delete ulang: err = %v", err)
	}
}
