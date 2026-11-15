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
	"saybu/pkg/outbox"
	"saybu/services/payment/internal/domain"
)

// TestRepoIntegration membutuhkan Postgres dengan migrasi payment sudah dijalankan.
//
//	PAYMENT_DB_DSN=... go test -tags=integration ./services/payment/internal/repo/...
func TestRepoIntegration(t *testing.T) {
	dsn := os.Getenv("PAYMENT_DB_DSN")
	if dsn == "" {
		t.Skip("PAYMENT_DB_DSN tidak diset")
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

	cleanup := func(orderID string) {
		_, _ = pool.Exec(context.Background(), `DELETE FROM payments WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox WHERE key = $1`, orderID)
	}

	events := func(p domain.Payment) ([]outbox.Record, error) {
		return []outbox.Record{{
			EventID: ids.New(),
			Topic:   "payment.events.v1",
			Key:     p.OrderID,
			Payload: []byte(`{"order_id":"` + p.OrderID + `"}`),
			Headers: map[string]string{"event-type": "payment.proof_uploaded"},
		}}, nil
	}

	orderID := ids.New()
	t.Cleanup(func() { cleanup(orderID) })

	// Pembuatan dari event order.created.
	in := CreateInput{
		EventID: ids.New(), OrderID: orderID, OrderCode: "SB-20261115-9001",
		UserID: ids.New(), Method: domain.MethodTransfer, Amount: 85000,
		CustomerName: "Siti", CustomerPhone: "081298765432",
	}
	p, created, err := r.CreateFromOrder(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if p.Status != domain.StatusUnpaid || p.Amount != 85000 || p.UserID != in.UserID {
		t.Fatalf("payment = %+v", p)
	}

	// Event yang sama -> idempotent, tidak membuat baris baru.
	dup, created, err := r.CreateFromOrder(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("event duplikat tidak boleh created")
	}
	if dup.ID != p.ID {
		t.Fatalf("id berubah: %s != %s", dup.ID, p.ID)
	}

	// Event baru untuk order yang sama -> tetap satu pembayaran.
	in2 := in
	in2.EventID = ids.New()
	again, created, err := r.CreateFromOrder(ctx, in2)
	if err != nil {
		t.Fatal(err)
	}
	if created || again.ID != p.ID {
		t.Fatalf("order ganda: created=%v id=%s", created, again.ID)
	}

	// Round-trip lookup.
	if got, err := r.GetByOrderCode(ctx, "SB-20261115-9001"); err != nil || got.ID != p.ID {
		t.Fatalf("GetByOrderCode = %+v err=%v", got, err)
	}
	if got, err := r.GetByOrderID(ctx, orderID); err != nil || got.ID != p.ID {
		t.Fatalf("GetByOrderID = %+v err=%v", got, err)
	}
	if got, err := r.GetByID(ctx, p.ID); err != nil || got.ID != p.ID {
		t.Fatalf("GetByID = %+v err=%v", got, err)
	}
	if _, err := r.GetByOrderCode(ctx, "SB-TIDAK-ADA"); !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("err = %v, want ErrPaymentNotFound", err)
	}

	// AttachProof: status -> WAITING_VERIFICATION, proof tersimpan, event outbox tertulis.
	proof := domain.Proof{
		ID: ids.New(), PaymentID: p.ID, URL: "/uploads/bukti.jpg",
		SizeBytes: 2048, ContentType: "image/jpeg", UploadedBy: in.UserID,
	}
	updated, err := r.AttachProof(ctx, p.ID, proof, events)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.StatusWaitingVerification || updated.ProofURL != proof.URL || updated.ProofUploadedAt == nil {
		t.Fatalf("updated = %+v", updated)
	}

	var proofCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payment_proofs WHERE payment_id = $1`, p.ID).Scan(&proofCount); err != nil {
		t.Fatal(err)
	}
	if proofCount != 1 {
		t.Fatalf("proof count = %d, want 1", proofCount)
	}
	var outboxCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE key = $1`, orderID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox count = %d, want 1", outboxCount)
	}

	// Upload ulang saat menunggu verifikasi tidak diizinkan.
	if _, err := r.AttachProof(ctx, p.ID, proof, events); !errors.Is(err, domain.ErrProofNotAllowed) {
		t.Fatalf("err = %v, want ErrProofNotAllowed", err)
	}

	// Verifikasi APPROVE -> PAID.
	verified, err := r.Verify(ctx, p.ID, domain.StatusPaid, "dana masuk", "admin-1", func(pay domain.Payment) ([]outbox.Record, error) {
		return []outbox.Record{{
			EventID: ids.New(), Topic: "payment.events.v1", Key: pay.OrderID,
			Payload: []byte(`{"order_id":"` + pay.OrderID + `"}`),
			Headers: map[string]string{"event-type": "payment.confirmed"},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if verified.Status != domain.StatusPaid || verified.VerifiedBy != "admin-1" || verified.VerifiedAt == nil {
		t.Fatalf("verified = %+v", verified)
	}
	// Verifikasi kedua tidak diizinkan.
	if _, err := r.Verify(ctx, p.ID, domain.StatusPaid, "", "admin-1", nil); !errors.Is(err, domain.ErrVerifyNotAllowed) {
		t.Fatalf("err = %v, want ErrVerifyNotAllowed", err)
	}

	// Alur REJECT: pembayaran kedua.
	orderID2 := ids.New()
	t.Cleanup(func() { cleanup(orderID2) })
	p2, _, err := r.CreateFromOrder(ctx, CreateInput{
		EventID: ids.New(), OrderID: orderID2, OrderCode: "SB-20261115-9002",
		Method: domain.MethodTransfer, Amount: 40000,
		CustomerName: "Budi", CustomerPhone: "081200000000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AttachProof(ctx, p2.ID, domain.Proof{ID: ids.New(), PaymentID: p2.ID, URL: "/uploads/x.png"}, events); err != nil {
		t.Fatal(err)
	}
	rejected, err := r.Verify(ctx, p2.ID, domain.StatusRejected, "bukti buram", "admin-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != domain.StatusRejected || rejected.Note != "bukti buram" {
		t.Fatalf("rejected = %+v", rejected)
	}
	// Setelah ditolak, bukti boleh diunggah ulang.
	if _, err := r.AttachProof(ctx, p2.ID, domain.Proof{ID: ids.New(), PaymentID: p2.ID, URL: "/uploads/y.png"}, events); err != nil {
		t.Fatalf("upload ulang setelah ditolak: %v", err)
	}

	// COD: status awal COD_PENDING.
	orderID3 := ids.New()
	t.Cleanup(func() { cleanup(orderID3) })
	p3, _, err := r.CreateFromOrder(ctx, CreateInput{
		EventID: ids.New(), OrderID: orderID3, OrderCode: "SB-20261115-9003",
		Method: domain.MethodCOD, Amount: 30000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p3.Status != domain.StatusCODPending {
		t.Fatalf("status COD = %s, want COD_PENDING", p3.Status)
	}

	// CancelFromOrderEvent: idempotent dan membatalkan pembayaran yang belum final.
	cancelEvent := ids.New()
	applied, err := r.CancelFromOrderEvent(ctx, cancelEvent, "order.cancelled", orderID3)
	if err != nil || !applied {
		t.Fatalf("cancel applied=%v err=%v", applied, err)
	}
	cancelled, err := r.GetByOrderID(ctx, orderID3)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != domain.StatusCancelled {
		t.Fatalf("status = %s, want CANCELLED", cancelled.Status)
	}
	// Event sama lagi -> processed=false.
	if applied, err := r.CancelFromOrderEvent(ctx, cancelEvent, "order.cancelled", orderID3); err != nil || applied {
		t.Fatalf("cancel duplikat applied=%v err=%v", applied, err)
	}

	// Pembayaran PAID tidak boleh dibatalkan oleh event.
	if applied, err := r.CancelFromOrderEvent(ctx, ids.New(), "order.expired", orderID); err != nil || !applied {
		t.Fatalf("cancel PAID applied=%v err=%v", applied, err)
	}
	stillPaid, err := r.GetByOrderID(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if stillPaid.Status != domain.StatusPaid {
		t.Fatalf("pembayaran PAID ikut dibatalkan: %s", stillPaid.Status)
	}

	// List dengan filter status.
	list, total, err := r.List(ctx, domain.StatusPaid, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total < 1 || len(list) == 0 {
		t.Fatalf("list = %d/%d", len(list), total)
	}
	for _, item := range list {
		if item.Status != domain.StatusPaid {
			t.Fatalf("filter bocor: %+v", item)
		}
	}
}
