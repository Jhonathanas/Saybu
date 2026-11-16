// Package service berisi usecase pembayaran: status, unggah bukti, verifikasi
// admin, dan konsumsi event pesanan.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"saybu/pkg/ids"
	"saybu/pkg/kafkax"
	"saybu/pkg/outbox"
	"saybu/services/payment/internal/domain"
	"saybu/services/payment/internal/repo"
)

// Nama topik dan producer untuk event pembayaran.
const (
	TopicPaymentEvents = "payment.events.v1"
	producerName       = "payment-svc"
)

// orderStatusCompleted adalah status pesanan yang memicu pelunasan COD.
const orderStatusCompleted = "COMPLETED"

// Repo adalah kebutuhan service terhadap database.
type Repo interface {
	CreateFromOrder(ctx context.Context, in repo.CreateInput) (domain.Payment, bool, error)
	GetByOrderCode(ctx context.Context, code string) (domain.Payment, error)
	GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error)
	GetByID(ctx context.Context, id string) (domain.Payment, error)
	List(ctx context.Context, status string, page, limit int) ([]domain.Payment, int64, error)
	AttachProof(ctx context.Context, paymentID string, proof domain.Proof, events func(domain.Payment) ([]outbox.Record, error)) (domain.Payment, error)
	Verify(ctx context.Context, paymentID, toStatus, note, verifiedBy string, events func(domain.Payment) ([]outbox.Record, error)) (domain.Payment, error)
	CancelFromOrderEvent(ctx context.Context, eventID, eventType, orderID string) (bool, error)
	MarkCODPaidFromOrderEvent(ctx context.Context, eventID, eventType, orderID string) (bool, error)
}

// Service adalah usecase pembayaran.
type Service struct {
	repo Repo
	log  *slog.Logger
	now  func() time.Time
}

// New membuat Service.
func New(r Repo, log *slog.Logger) (*Service, error) {
	if r == nil {
		return nil, errors.New("service: repo wajib")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: r, log: log, now: time.Now}, nil
}

// GetByOrderCode mengambil status pembayaran untuk pelacakan.
func (s *Service) GetByOrderCode(ctx context.Context, code, phone, userID string) (domain.Payment, error) {
	p, err := s.repo.GetByOrderCode(ctx, code)
	if err != nil {
		return domain.Payment{}, err
	}
	if err := authorize(p, phone, userID); err != nil {
		return domain.Payment{}, err
	}
	return p, nil
}

// UploadProof mendaftarkan bukti transfer yang sudah diunggah gateway.
func (s *Service) UploadProof(ctx context.Context, in domain.UploadProofInput) (domain.Payment, error) {
	if strings.TrimSpace(in.Code) == "" {
		return domain.Payment{}, (&domain.ValidationError{}).Add("code", "Kode pesanan wajib diisi.")
	}
	if strings.TrimSpace(in.ProofURL) == "" {
		return domain.Payment{}, (&domain.ValidationError{}).Add("proof_url", "Bukti transfer wajib diunggah.")
	}

	p, err := s.repo.GetByOrderCode(ctx, in.Code)
	if err != nil {
		return domain.Payment{}, err
	}
	if err := authorize(p, in.Phone, in.UserID); err != nil {
		return domain.Payment{}, err
	}
	if !domain.CanUploadProof(p.Status) {
		return domain.Payment{}, domain.ErrProofNotAllowed
	}

	proof := domain.Proof{
		ID:          ids.New(),
		PaymentID:   p.ID,
		URL:         in.ProofURL,
		SizeBytes:   in.SizeBytes,
		ContentType: in.ContentType,
		UploadedBy:  uploadedBy(in),
	}
	updated, err := s.repo.AttachProof(ctx, p.ID, proof,
		func(pay domain.Payment) ([]outbox.Record, error) { return s.proofUploadedEvent(pay) })
	if err != nil {
		return domain.Payment{}, err
	}
	return updated, nil
}

// List mengambil daftar pembayaran untuk panel admin.
func (s *Service) List(ctx context.Context, status string, page, limit int) ([]domain.Payment, int64, int, int, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "" && !domain.IsValidStatus(status) {
		return nil, 0, 0, 0, (&domain.ValidationError{}).Add("status", "Status pembayaran tidak dikenal.")
	}
	page, limit = normalizePage(page, limit)
	payments, total, err := s.repo.List(ctx, status, page, limit)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return payments, total, page, limit, nil
}

// Verify menyetujui atau menolak bukti transfer (admin).
func (s *Service) Verify(ctx context.Context, id, decision, note, adminID string) (domain.Payment, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Payment{}, (&domain.ValidationError{}).Add("id", "ID pembayaran wajib diisi.")
	}
	decision = strings.ToUpper(strings.TrimSpace(decision))
	note = strings.TrimSpace(note)

	var toStatus string
	switch decision {
	case domain.DecisionApprove:
		toStatus = domain.StatusPaid
	case domain.DecisionReject:
		toStatus = domain.StatusRejected
		if note == "" {
			return domain.Payment{}, (&domain.ValidationError{}).Add("note", "Alasan penolakan wajib diisi.")
		}
	default:
		return domain.Payment{}, domain.ErrInvalidDecision
	}

	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Payment{}, err
	}
	if !domain.CanVerify(p.Status) {
		return domain.Payment{}, domain.ErrVerifyNotAllowed
	}

	updated, err := s.repo.Verify(ctx, p.ID, toStatus, note, adminID,
		func(pay domain.Payment) ([]outbox.Record, error) {
			if toStatus == domain.StatusPaid {
				return s.confirmedEvent(pay)
			}
			return s.rejectedEvent(pay, note)
		})
	if err != nil {
		return domain.Payment{}, err
	}
	return updated, nil
}

// HandleOrderEvent memproses event dari order.events.v1.
//
// order.created membuat pembayaran baru; order.cancelled/order.expired
// menandai pembayaran batal. Handler idempotent lewat processed_events.
func (s *Service) HandleOrderEvent(ctx context.Context, ev kafkax.Event) error {
	switch ev.Type {
	case "order.created":
		var p struct {
			OrderID       string `json:"order_id"`
			Code          string `json:"code"`
			UserID        string `json:"user_id"`
			PaymentMethod string `json:"payment_method"`
			Total         int64  `json:"total"`
			Customer      struct {
				Name  string `json:"name"`
				Phone string `json:"phone"`
			} `json:"customer"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("service: payload order.created tidak valid: %w", err)
		}
		if strings.TrimSpace(p.OrderID) == "" || strings.TrimSpace(p.Code) == "" {
			return fmt.Errorf("service: event order.created tanpa order_id/code")
		}
		if !domain.IsValidMethod(p.PaymentMethod) {
			return fmt.Errorf("service: metode pembayaran tidak dikenal: %q", p.PaymentMethod)
		}
		_, created, err := s.repo.CreateFromOrder(ctx, repo.CreateInput{
			EventID:       ev.EventID,
			OrderID:       p.OrderID,
			OrderCode:     p.Code,
			UserID:        p.UserID,
			Method:        p.PaymentMethod,
			Amount:        p.Total,
			CustomerName:  p.Customer.Name,
			CustomerPhone: p.Customer.Phone,
		})
		if err != nil {
			return err
		}
		if !created {
			s.log.Debug("pembayaran sudah ada (event duplikat)", "event_id", ev.EventID, "code", p.Code)
		}
		return nil

	case "order.cancelled", "order.expired":
		var p struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("service: payload %s tidak valid: %w", ev.Type, err)
		}
		if strings.TrimSpace(p.OrderID) == "" {
			return fmt.Errorf("service: event %s tanpa order_id", ev.Type)
		}
		if _, err := s.repo.CancelFromOrderEvent(ctx, ev.EventID, ev.Type, p.OrderID); err != nil {
			return err
		}
		return nil

	case "order.status_changed":
		var p struct {
			OrderID string `json:"order_id"`
			To      string `json:"to"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("service: payload order.status_changed tidak valid: %w", err)
		}
		if strings.TrimSpace(p.OrderID) == "" {
			return fmt.Errorf("service: event order.status_changed tanpa order_id")
		}
		// Hanya pesanan yang selesai yang melunasi COD; transisi lain diabaikan.
		if strings.TrimSpace(p.To) != orderStatusCompleted {
			return nil
		}
		if _, err := s.repo.MarkCODPaidFromOrderEvent(ctx, ev.EventID, ev.Type, p.OrderID); err != nil {
			return err
		}
		return nil

	default:
		return nil
	}
}

func authorize(p domain.Payment, phone, userID string) error {
	if userID != "" && p.UserID != "" && p.UserID == userID {
		return nil
	}
	if phone != "" && domain.PhoneMatches(p.CustomerPhone, phone) {
		return nil
	}
	// Jangan membocorkan keberadaan pembayaran milik orang lain.
	return domain.ErrPaymentNotFound
}

func uploadedBy(in domain.UploadProofInput) string {
	if strings.TrimSpace(in.UserID) != "" {
		return in.UserID
	}
	return "guest"
}

func (s *Service) proofUploadedEvent(p domain.Payment) ([]outbox.Record, error) {
	payload := map[string]any{
		"payment_id": p.ID,
		"order_id":   p.OrderID,
		"code":       p.OrderCode,
		"amount":     p.Amount,
		"proof_url":  p.ProofURL,
	}
	rec, err := newRecord("payment.proof_uploaded", p.OrderID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func (s *Service) confirmedEvent(p domain.Payment) ([]outbox.Record, error) {
	payload := map[string]any{
		"payment_id":  p.ID,
		"order_id":    p.OrderID,
		"code":        p.OrderCode,
		"amount":      p.Amount,
		"verified_by": p.VerifiedBy,
	}
	rec, err := newRecord("payment.confirmed", p.OrderID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func (s *Service) rejectedEvent(p domain.Payment, reason string) ([]outbox.Record, error) {
	payload := map[string]any{
		"payment_id": p.ID,
		"order_id":   p.OrderID,
		"code":       p.OrderCode,
		"reason":     reason,
	}
	rec, err := newRecord("payment.rejected", p.OrderID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func newRecord(eventType, key string, payload any) (outbox.Record, error) {
	ev, err := kafkax.NewEvent(eventType, key, payload, producerName)
	if err != nil {
		return outbox.Record{}, fmt.Errorf("service: bangun event %s: %w", eventType, err)
	}
	raw, err := ev.Marshal()
	if err != nil {
		return outbox.Record{}, fmt.Errorf("service: marshal event %s: %w", eventType, err)
	}
	return outbox.Record{
		EventID: ev.EventID,
		Topic:   TopicPaymentEvents,
		Key:     key,
		Payload: raw,
		Headers: map[string]string{"event-type": eventType},
	}, nil
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
