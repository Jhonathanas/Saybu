// Package service menyusun notifikasi dari event Kafka dan mengirimkannya
// ke admin lewat Telegram.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"saybu/pkg/kafkax"
	"saybu/services/notification/internal/telegram"
)

// Notifier mengubah event menjadi pesan Telegram. Dedup-nya persisten via
// DedupStore (Postgres di produksi, in-memory hanya untuk pengembangan).
type Notifier struct {
	client *telegram.Client
	log    *slog.Logger
	dedup  DedupStore
}

// New membuat Notifier. Store nil diganti dedup in-memory.
func New(client *telegram.Client, log *slog.Logger, store DedupStore) *Notifier {
	if log == nil {
		log = slog.Default()
	}
	if store == nil {
		store = NewMemoryDedup()
	}
	return &Notifier{
		client: client,
		log:    log,
		dedup:  store,
	}
}

// HandleEvent memproses satu event. Tipe yang tidak dikenal diabaikan.
func (n *Notifier) HandleEvent(ctx context.Context, ev kafkax.Event) error {
	claimed, err := n.dedup.Claim(ctx, ev.EventID, ev.Type)
	if err != nil {
		return fmt.Errorf("service: klaim event %s: %w", ev.EventID, err)
	}
	if !claimed {
		n.log.Debug("event sudah dinotifikasi (duplikat)", "event_id", ev.EventID, "type", ev.Type)
		return nil
	}

	text, ok := n.buildMessage(ev)
	if !ok {
		n.markSent(ctx, ev.EventID)
		return nil
	}
	if !n.client.Enabled() {
		n.log.Debug("telegram nonaktif, notifikasi dilewati", "type", ev.Type, "event_id", ev.EventID)
		n.markSent(ctx, ev.EventID)
		return nil
	}
	if err := n.client.SendMessage(ctx, text); err != nil {
		if releaseErr := n.dedup.Release(ctx, ev.EventID); releaseErr != nil {
			n.log.Error("lepas klaim gagal", "error", releaseErr, "event_id", ev.EventID)
		}
		return fmt.Errorf("service: kirim notifikasi %s: %w", ev.Type, err)
	}
	n.markSent(ctx, ev.EventID)
	n.log.Info("notifikasi terkirim", "type", ev.Type, "event_id", ev.EventID)
	return nil
}

// markSent tidak menggagalkan pengiriman yang sudah berhasil.
func (n *Notifier) markSent(ctx context.Context, eventID string) {
	if err := n.dedup.MarkSent(ctx, eventID); err != nil {
		n.log.Error("tandai event sent gagal", "error", err, "event_id", eventID)
	}
}

func (n *Notifier) buildMessage(ev kafkax.Event) (string, bool) {
	switch ev.Type {
	case "order.created":
		return orderCreatedMessage(ev.Payload)
	case "order.cancelled":
		return orderCancelledMessage(ev.Payload)
	case "payment.proof_uploaded":
		return paymentProofMessage(ev.Payload)
	case "payment.confirmed":
		return paymentConfirmedMessage(ev.Payload)
	case "payment.rejected":
		return paymentRejectedMessage(ev.Payload)
	case "stock.low":
		return stockLowMessage(ev.Payload)
	default:
		return "", false
	}
}

func orderCreatedMessage(raw json.RawMessage) (string, bool) {
	var p struct {
		Code          string `json:"code"`
		Fulfillment   string `json:"fulfillment"`
		PaymentMethod string `json:"payment_method"`
		ZoneName      string `json:"zone_name"`
		AddressLine   string `json:"address_line"`
		Total         int64  `json:"total"`
		Customer      struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
		} `json:"customer"`
		Items []struct {
			Name string `json:"name"`
			Unit string `json:"unit"`
			Qty  int64  `json:"qty"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", false
	}

	var b strings.Builder
	fmt.Fprintf(&b, "🛒 Pesanan baru %s\n", p.Code)
	fmt.Fprintf(&b, "Nama: %s\n", p.Customer.Name)
	fmt.Fprintf(&b, "HP: %s\n", p.Customer.Phone)
	fmt.Fprintf(&b, "Total: %s\n", rupiah(p.Total))
	if p.PaymentMethod == "COD" {
		b.WriteString("Pembayaran: COD (bayar di tempat)\n")
	} else {
		b.WriteString("Pembayaran: Transfer bank\n")
	}
	if p.Fulfillment == "PICKUP" {
		b.WriteString("Ambil di tempat\n")
	} else {
		fmt.Fprintf(&b, "Kirim ke: %s\n", p.ZoneName)
		if p.AddressLine != "" {
			fmt.Fprintf(&b, "Alamat: %s\n", p.AddressLine)
		}
	}
	fmt.Fprintf(&b, "%d item", len(p.Items))
	return b.String(), true
}

func orderCancelledMessage(raw json.RawMessage) (string, bool) {
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
		By     string `json:"by"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "❌ Pesanan dibatalkan %s\n", p.Code)
	if p.By != "" {
		fmt.Fprintf(&b, "Oleh: %s\n", p.By)
	}
	if p.Reason != "" {
		fmt.Fprintf(&b, "Alasan: %s", p.Reason)
	}
	return strings.TrimRight(b.String(), "\n"), true
}

func paymentProofMessage(raw json.RawMessage) (string, bool) {
	var p struct {
		Code   string `json:"code"`
		Amount int64  `json:"amount"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", false
	}
	return fmt.Sprintf("📎 Bukti transfer masuk %s\nNominal: %s\nMenunggu verifikasi admin.", p.Code, rupiah(p.Amount)), true
}

func paymentConfirmedMessage(raw json.RawMessage) (string, bool) {
	var p struct {
		Code   string `json:"code"`
		Amount int64  `json:"amount"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", false
	}
	return fmt.Sprintf("✅ Pembayaran terverifikasi %s\nNominal: %s", p.Code, rupiah(p.Amount)), true
}

func paymentRejectedMessage(raw json.RawMessage) (string, bool) {
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", false
	}
	msg := fmt.Sprintf("⚠️ Bukti transfer ditolak %s", p.Code)
	if p.Reason != "" {
		msg += "\nAlasan: " + p.Reason
	}
	return msg, true
}

func stockLowMessage(raw json.RawMessage) (string, bool) {
	var p struct {
		Name      string `json:"name"`
		Remaining int64  `json:"remaining"`
		Unit      string `json:"unit"`
		Threshold int64  `json:"threshold"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", false
	}
	if strings.TrimSpace(p.Name) == "" {
		return "", false
	}
	return fmt.Sprintf("📉 Stok menipis: %s\nTersisa %s (ambang %s)",
		p.Name, qtyDisplay(p.Unit, p.Remaining), qtyDisplay(p.Unit, p.Threshold)), true
}

// qtyDisplay memformat qty (satuan terkecil) sesuai satuannya.
func qtyDisplay(unit string, qty int64) string {
	if unit == "kg" {
		return formatKg(qty) + " kg"
	}
	if unit == "" {
		return strconv.FormatInt(qty, 10)
	}
	return strconv.FormatInt(qty, 10) + " " + unit
}

// formatKg memformat gram menjadi "1,5" (tanpa satuan).
func formatKg(grams int64) string {
	whole := grams / 1000
	rem := grams % 1000
	if rem == 0 {
		return strconv.FormatInt(whole, 10)
	}
	frac := strconv.FormatInt(rem, 10)
	for len(frac) < 3 {
		frac = "0" + frac
	}
	frac = strings.TrimRight(frac, "0")
	return strconv.FormatInt(whole, 10) + "," + frac
}

// rupiah memformat nominal sebagai "Rp1.234.567".
func rupiah(v int64) string {
	s := strconv.FormatInt(v, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := "Rp" + strings.Join(parts, ".")
	if neg {
		out = "-" + out
	}
	return out
}
