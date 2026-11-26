package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"saybu/pkg/kafkax"
	"saybu/services/notification/internal/telegram"
)

func newNotifier() *Notifier {
	return New(telegram.NewClient("", ""), slog.New(slog.NewTextHandler(io.Discard, nil)), NewMemoryDedup())
}

func event(eventType string, payload map[string]any) kafkax.Event {
	raw, _ := json.Marshal(payload)
	return kafkax.Event{EventID: "ev-1", Type: eventType, Payload: raw}
}

func TestOrderCreatedMessage(t *testing.T) {
	n := newNotifier()
	text, ok := n.buildMessage(event("order.created", map[string]any{
		"code": "SB-20261115-0001", "fulfillment": "DELIVERY", "payment_method": "TRANSFER",
		"zone_name": "Kec. Sukamaju", "address_line": "Jl. Melati 12", "total": 85000,
		"customer": map[string]any{"name": "Siti", "phone": "081298765432"},
		"items":    []map[string]any{{"name": "Bayam", "unit": "ikat", "qty": 2}},
	}))
	if !ok {
		t.Fatal("order.created harus menghasilkan pesan")
	}
	for _, want := range []string{"SB-20261115-0001", "Siti", "Rp85.000", "Kec. Sukamaju", "1 item"} {
		if !strings.Contains(text, want) {
			t.Errorf("pesan tidak memuat %q:\n%s", want, text)
		}
	}
}

func TestPaymentMessages(t *testing.T) {
	n := newNotifier()

	proof, _ := n.buildMessage(event("payment.proof_uploaded", map[string]any{"code": "SB-1", "amount": 85000}))
	if !strings.Contains(proof, "Bukti transfer") || !strings.Contains(proof, "Rp85.000") {
		t.Errorf("proof = %q", proof)
	}

	confirmed, _ := n.buildMessage(event("payment.confirmed", map[string]any{"code": "SB-1", "amount": 85000}))
	if !strings.Contains(confirmed, "terverifikasi") {
		t.Errorf("confirmed = %q", confirmed)
	}

	rejected, _ := n.buildMessage(event("payment.rejected", map[string]any{"code": "SB-1", "reason": "bukti buram"}))
	if !strings.Contains(rejected, "ditolak") || !strings.Contains(rejected, "bukti buram") {
		t.Errorf("rejected = %q", rejected)
	}
}

func TestBuildMessageIgnoresUnknown(t *testing.T) {
	n := newNotifier()
	if _, ok := n.buildMessage(event("product.updated", map[string]any{})); ok {
		t.Fatal("tipe tak dikenal harus diabaikan")
	}
}

func TestStockLowMessage(t *testing.T) {
	n := newNotifier()

	kg, ok := n.buildMessage(event("stock.low", map[string]any{
		"product_id": "p1", "name": "Pisang Cavendish", "remaining": 1500, "unit": "kg", "threshold": 5000,
	}))
	if !ok {
		t.Fatal("stock.low harus menghasilkan pesan")
	}
	for _, want := range []string{"Stok menipis", "Pisang Cavendish", "1,5 kg", "5 kg"} {
		if !strings.Contains(kg, want) {
			t.Errorf("pesan tidak memuat %q:\n%s", want, kg)
		}
	}

	ikat, _ := n.buildMessage(event("stock.low", map[string]any{
		"name": "Bayam", "remaining": 3, "unit": "ikat", "threshold": 5,
	}))
	if !strings.Contains(ikat, "3 ikat") || !strings.Contains(ikat, "5 ikat") {
		t.Errorf("ikat = %q", ikat)
	}

	// Nama kosong: tidak ada pesan.
	if _, ok := n.buildMessage(event("stock.low", map[string]any{"remaining": 1, "unit": "pcs"})); ok {
		t.Fatal("stock.low tanpa nama harus diabaikan")
	}
}

func TestBuildMessageBadPayload(t *testing.T) {
	n := newNotifier()
	ev := kafkax.Event{EventID: "e", Type: "order.created", Payload: json.RawMessage(`not-json`)}
	if _, ok := n.buildMessage(ev); ok {
		t.Fatal("payload rusak harus diabaikan")
	}
}

func TestMemoryDedupClaim(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryDedup()

	first, err := store.Claim(ctx, "ev-1", "order.created")
	if err != nil || !first {
		t.Fatalf("claim pertama = %v, %v", first, err)
	}
	second, err := store.Claim(ctx, "ev-1", "order.created")
	if err != nil || second {
		t.Fatalf("claim kedua = %v, %v (harus duplikat)", second, err)
	}
	if err := store.Release(ctx, "ev-1"); err != nil {
		t.Fatal(err)
	}
	retry, err := store.Claim(ctx, "ev-1", "order.created")
	if err != nil || !retry {
		t.Fatalf("claim setelah release = %v, %v", retry, err)
	}
	if err := store.MarkSent(ctx, "ev-1"); err != nil {
		t.Fatal(err)
	}
}

type countingDedup struct {
	claims   int
	marks    int
	releases int
	claim    bool
}

func (c *countingDedup) Claim(context.Context, string, string) (bool, error) {
	c.claims++
	return c.claim, nil
}
func (c *countingDedup) MarkSent(context.Context, string) error {
	c.marks++
	return nil
}
func (c *countingDedup) Release(context.Context, string) error {
	c.releases++
	return nil
}

func TestHandleEventUsesDedupStore(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ev := event("payment.confirmed", map[string]any{"code": "SB-1", "amount": 1000})

	store := &countingDedup{claim: true}
	n := New(telegram.NewClient("", ""), log, store)
	if err := n.HandleEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if store.claims != 1 || store.marks != 1 || store.releases != 0 {
		t.Fatalf("store = %+v", store)
	}

	duplicate := &countingDedup{claim: false}
	n = New(telegram.NewClient("", ""), log, duplicate)
	if err := n.HandleEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if duplicate.claims != 1 || duplicate.marks != 0 {
		t.Fatalf("duplikat tidak boleh diproses ulang: %+v", duplicate)
	}
}

func TestHandleEventDisabledClientSkips(t *testing.T) {
	n := newNotifier()
	ev := event("payment.confirmed", map[string]any{"code": "SB-1", "amount": 1000})
	if err := n.HandleEvent(context.Background(), ev); err != nil {
		t.Fatalf("telegram nonaktif harus dilewati tanpa error: %v", err)
	}
	if err := n.HandleEvent(context.Background(), ev); err != nil {
		t.Fatalf("duplikat harus dilewati: %v", err)
	}
}

func TestRupiah(t *testing.T) {
	cases := map[int64]string{
		0:       "Rp0",
		1000:    "Rp1.000",
		85000:   "Rp85.000",
		1234567: "Rp1.234.567",
		-5000:   "-Rp5.000",
	}
	for in, want := range cases {
		if got := rupiah(in); got != want {
			t.Errorf("rupiah(%d) = %q, want %q", in, got, want)
		}
	}
}
