//go:build integration

package repo

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
	"saybu/pkg/outbox"
	"saybu/services/order/internal/domain"
)

// TestRepoIntegration membutuhkan Postgres dengan migrasi order sudah dijalankan.
//
//	ORDER_DB_DSN=... go test -tags=integration ./services/order/internal/repo/...
func TestRepoIntegration(t *testing.T) {
	dsn := os.Getenv("ORDER_DB_DSN")
	if dsn == "" {
		t.Skip("ORDER_DB_DSN tidak diset")
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

	r := New(pool)

	cleanup := func(id string) {
		_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, id)
	}

	newOrder := func(userID string, expires time.Time) domain.Order {
		return domain.Order{
			ID: ids.New(), UserID: userID,
			Fulfillment: domain.FulfillmentDelivery, PaymentMethod: domain.PaymentTransfer,
			Customer: domain.Customer{Name: "Siti", Phone: "081234567890", Email: "siti@example.com"},
			Address:  domain.Address{Line: "Jl. Melati 12", City: "Bandung"},
			ZoneID:   ids.New(), ZoneName: "Kec. Sukamaju", ShippingFee: 10000,
			Subtotal: 25000, Total: 35000,
			Items: []domain.Item{
				{ProductID: ids.New(), Name: "Bayam", Unit: "ikat", Qty: 2, UnitPrice: 5000, LineTotal: 10000},
				{ProductID: ids.New(), Name: "Tomat", Unit: "kg", Qty: 1000, UnitPrice: 15000, LineTotal: 15000},
			},
			ExpiresAt: expires,
		}
	}

	events := func(o domain.Order) ([]outbox.Record, error) {
		return []outbox.Record{{
			EventID: ids.New(),
			Topic:   "order.events.v1",
			Key:     o.ID,
			Payload: []byte(`{"order_id":"` + o.ID + `"}`),
			Headers: map[string]string{"event-type": "order.created"},
		}}, nil
	}

	// Dua pesanan berurutan: kode harian naik satu.
	o1, err := r.CreateOrder(ctx, newOrder("", time.Now().UTC().Add(time.Hour)), Idempotency{}, events)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanup(o1.ID) })

	o2, err := r.CreateOrder(ctx, newOrder("", time.Now().UTC().Add(time.Hour)), Idempotency{}, events)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanup(o2.ID) })

	if !strings.HasPrefix(o1.Code, "SB-") {
		t.Fatalf("kode = %q", o1.Code)
	}
	if o1.Code == o2.Code {
		t.Fatalf("kode harus unik: %s == %s", o1.Code, o2.Code)
	}
	if o1.Status != domain.StatusPendingPayment || len(o1.Items) != 2 {
		t.Fatalf("order = %+v", o1)
	}
	if len(o1.History) != 1 || o1.History[0].Status != domain.StatusPendingPayment {
		t.Fatalf("riwayat awal = %+v", o1.History)
	}

	// Kode harian: nomor urut o2 = o1 + 1.
	if seq1, seq2 := codeSeq(o1.Code), codeSeq(o2.Code); seq2 != seq1+1 {
		t.Fatalf("nomor urut tidak berurutan: %d lalu %d", seq1, seq2)
	}

	// Event outbox tertulis di transaksi yang sama.
	var outboxCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE key = $1`, o1.ID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox count = %d, want 1", outboxCount)
	}

	// GetOrderByCode round-trip.
	got, err := r.GetOrderByCode(ctx, o1.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != o1.ID || got.Customer.Name != "Siti" || len(got.Items) != 2 {
		t.Fatalf("got = %+v", got)
	}

	// Tidak ditemukan.
	if _, err := r.GetOrderByCode(ctx, "SB-TIDAK-ADA"); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("err = %v", err)
	}

	// Idempotensi: kunci yang sama menolak dengan ErrIdempotencyExists.
	key := "idem-" + ids.New()
	o3 := newOrder("", time.Now().UTC().Add(time.Hour))
	created, err := r.CreateOrder(ctx, o3, Idempotency{Key: key, RequestHash: "hash-1"}, events)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanup(created.ID) })

	it, found, err := r.GetIdempotency(ctx, key)
	if err != nil || !found {
		t.Fatalf("idempotency = %+v found=%v err=%v", it, found, err)
	}
	if it.OrderID != created.ID || it.RequestHash != "hash-1" {
		t.Fatalf("idempotency = %+v", it)
	}

	o4 := newOrder("", time.Now().UTC().Add(time.Hour))
	if _, err := r.CreateOrder(ctx, o4, Idempotency{Key: key, RequestHash: "hash-2"}, events); !errors.Is(err, domain.ErrIdempotencyExists) {
		t.Fatalf("err = %v, want ErrIdempotencyExists", err)
	}

	// ListOrdersByUser.
	userID := ids.New()
	ou, err := r.CreateOrder(ctx, newOrder(userID, time.Now().UTC().Add(time.Hour)), Idempotency{}, events)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanup(ou.ID) })

	list, total, err := r.ListOrdersByUser(ctx, userID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != ou.ID {
		t.Fatalf("list = %d/%d %+v", len(list), total, list)
	}

	// Transisi legal.
	cancelled, err := r.Transition(ctx, ou.ID, domain.StatusPendingPayment, domain.StatusCancelled, "berubah pikiran", "customer", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != domain.StatusCancelled || cancelled.CancelReason != "berubah pikiran" {
		t.Fatalf("cancelled = %+v", cancelled)
	}

	// Transisi ilegal (status sudah CANCELLED).
	if _, err := r.Transition(ctx, ou.ID, domain.StatusPendingPayment, domain.StatusCancelled, "", "customer", nil); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}

	// ExpireDue: pesanan kedaluwarsa ditandai EXPIRED + event.
	oe, err := r.CreateOrder(ctx, newOrder("", time.Now().UTC().Add(-time.Hour)), Idempotency{}, events)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanup(oe.ID) })

	expired, err := r.ExpireDue(ctx, 100, func(o domain.Order) ([]outbox.Record, error) {
		return []outbox.Record{{
			EventID: ids.New(), Topic: "order.events.v1", Key: o.ID,
			Payload: []byte(`{"order_id":"` + o.ID + `"}`),
			Headers: map[string]string{"event-type": "order.expired"},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	foundExpired := false
	for _, e := range expired {
		if e.ID == oe.ID {
			foundExpired = true
		}
	}
	if !foundExpired {
		t.Fatalf("pesanan kedaluwarsa tidak dikembalikan: %+v", expired)
	}
	reloaded, err := r.GetOrderByID(ctx, oe.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != domain.StatusExpired {
		t.Fatalf("status = %s, want EXPIRED", reloaded.Status)
	}

	// Pesanan yang belum kedaluwarsa tidak boleh ikut.
	reloadedO1, _ := r.GetOrderByID(ctx, o1.ID)
	if reloadedO1.Status != domain.StatusPendingPayment {
		t.Fatalf("pesanan belum kedaluwarsa ikut berubah: %s", reloadedO1.Status)
	}
}

func codeSeq(code string) int64 {
	parts := strings.Split(code, "-")
	if len(parts) != 3 {
		return -1
	}
	var n int64
	for _, c := range parts[2] {
		n = n*10 + int64(c-'0')
	}
	return n
}
