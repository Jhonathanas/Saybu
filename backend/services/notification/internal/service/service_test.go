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
	return New(telegram.NewClient("", ""), slog.New(slog.NewTextHandler(io.Discard, nil)))
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

func TestBuildMessageBadPayload(t *testing.T) {
	n := newNotifier()
	ev := kafkax.Event{EventID: "e", Type: "order.created", Payload: json.RawMessage(`not-json`)}
	if _, ok := n.buildMessage(ev); ok {
		t.Fatal("payload rusak harus diabaikan")
	}
}

func TestMarkSeenDedup(t *testing.T) {
	n := newNotifier()
	if n.markSeen("ev-1") {
		t.Fatal("pertama kali bukan duplikat")
	}
	if !n.markSeen("ev-1") {
		t.Fatal("kedua kali harus duplikat")
	}
	if n.markSeen("") {
		t.Fatal("event id kosong tidak boleh dianggap duplikat")
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
