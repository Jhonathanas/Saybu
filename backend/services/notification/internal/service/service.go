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
	"sync"

	"saybu/pkg/kafkax"
	"saybu/services/notification/internal/telegram"
)

// maxSeen membatasi jumlah event id yang diingat untuk deduplikasi.
const maxSeen = 2000

// Notifier mengubah event menjadi pesan Telegram.
//
// Tanpa database, deduplikasi dilakukan in-memory (bounded) sebagai lapis
// kedua setelah commit Kafka; duplikasi setelah crash tetap mungkin tetapi
// jarang dan tidak berbahaya.
type Notifier struct {
	client *telegram.Client
	log    *slog.Logger

	mu   sync.Mutex
	seen map[string]struct{}
	ring []string
}

// New membuat Notifier.
func New(client *telegram.Client, log *slog.Logger) *Notifier {
	if log == nil {
		log = slog.Default()
	}
	return &Notifier{
		client: client,
		log:    log,
		seen:   make(map[string]struct{}),
	}
}

// HandleEvent memproses satu event. Tipe yang tidak dikenal diabaikan.
func (n *Notifier) HandleEvent(ctx context.Context, ev kafkax.Event) error {
	if n.markSeen(ev.EventID) {
		n.log.Debug("event sudah dinotifikasi (duplikat)", "event_id", ev.EventID, "type", ev.Type)
		return nil
	}

	text, ok := n.buildMessage(ev)
	if !ok {
		return nil
	}
	if !n.client.Enabled() {
		n.log.Debug("telegram nonaktif, notifikasi dilewati", "type", ev.Type, "event_id", ev.EventID)
		return nil
	}
	if err := n.client.SendMessage(ctx, text); err != nil {
		return fmt.Errorf("service: kirim notifikasi %s: %w", ev.Type, err)
	}
	n.log.Info("notifikasi terkirim", "type", ev.Type, "event_id", ev.EventID)
	return nil
}

// markSeen mengembalikan true bila event id sudah pernah diproses.
func (n *Notifier) markSeen(id string) bool {
	if id == "" {
		return false
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, ok := n.seen[id]; ok {
		return true
	}
	n.seen[id] = struct{}{}
	n.ring = append(n.ring, id)
	if len(n.ring) > maxSeen {
		old := n.ring[0]
		n.ring = n.ring[1:]
		delete(n.seen, old)
	}
	return false
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
