package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"saybu/pkg/kafkax"
	"saybu/pkg/outbox"
	"saybu/services/payment/internal/domain"
	"saybu/services/payment/internal/repo"
)

type fakeRepo struct {
	byID    map[string]domain.Payment
	byCode  map[string]string
	byOrder map[string]string

	attachEvents []outbox.Record
	verifyEvents []outbox.Record
	cancelled    []string
	created      []repo.CreateInput
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byID:    map[string]domain.Payment{},
		byCode:  map[string]string{},
		byOrder: map[string]string{},
	}
}

func (f *fakeRepo) add(p domain.Payment) {
	f.byID[p.ID] = p
	f.byCode[p.OrderCode] = p.ID
	f.byOrder[p.OrderID] = p.ID
}

func (f *fakeRepo) CreateFromOrder(_ context.Context, in repo.CreateInput) (domain.Payment, bool, error) {
	for _, p := range f.byID {
		if p.OrderID == in.OrderID {
			return p, false, nil
		}
	}
	p := domain.Payment{
		ID: in.OrderID + "-pay", OrderID: in.OrderID, OrderCode: in.OrderCode,
		UserID: in.UserID, Method: in.Method, Status: domain.InitialStatus(in.Method),
		Amount: in.Amount, CustomerName: in.CustomerName, CustomerPhone: in.CustomerPhone,
	}
	f.add(p)
	f.created = append(f.created, in)
	return p, true, nil
}

func (f *fakeRepo) GetByOrderCode(_ context.Context, code string) (domain.Payment, error) {
	id, ok := f.byCode[code]
	if !ok {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	return f.byID[id], nil
}

func (f *fakeRepo) GetByOrderID(_ context.Context, orderID string) (domain.Payment, error) {
	id, ok := f.byOrder[orderID]
	if !ok {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	return f.byID[id], nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (domain.Payment, error) {
	p, ok := f.byID[id]
	if !ok {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	return p, nil
}

func (f *fakeRepo) List(_ context.Context, status string, _, _ int) ([]domain.Payment, int64, error) {
	var out []domain.Payment
	for _, p := range f.byID {
		if status == "" || p.Status == status {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) AttachProof(_ context.Context, paymentID string, proof domain.Proof, events func(domain.Payment) ([]outbox.Record, error)) (domain.Payment, error) {
	p, ok := f.byID[paymentID]
	if !ok {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if !domain.CanUploadProof(p.Status) {
		return domain.Payment{}, domain.ErrProofNotAllowed
	}
	p.Status = domain.StatusWaitingVerification
	p.ProofURL = proof.URL
	if events != nil {
		recs, err := events(p)
		if err != nil {
			return domain.Payment{}, err
		}
		f.attachEvents = append(f.attachEvents, recs...)
	}
	f.byID[paymentID] = p
	return p, nil
}

func (f *fakeRepo) Verify(_ context.Context, paymentID, toStatus, note, verifiedBy string, events func(domain.Payment) ([]outbox.Record, error)) (domain.Payment, error) {
	p, ok := f.byID[paymentID]
	if !ok {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if !domain.CanVerify(p.Status) {
		return domain.Payment{}, domain.ErrVerifyNotAllowed
	}
	p.Status = toStatus
	p.Note = note
	p.VerifiedBy = verifiedBy
	if events != nil {
		recs, err := events(p)
		if err != nil {
			return domain.Payment{}, err
		}
		f.verifyEvents = append(f.verifyEvents, recs...)
	}
	f.byID[paymentID] = p
	return p, nil
}

func (f *fakeRepo) CancelFromOrderEvent(_ context.Context, _, _, orderID string) (bool, error) {
	f.cancelled = append(f.cancelled, orderID)
	return true, nil
}

func newService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	r := newFakeRepo()
	s, err := New(r, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return s, r
}

func paidPayment() domain.Payment {
	return domain.Payment{
		ID: "pay-1", OrderID: "o-1", OrderCode: "SB-20261115-0001",
		UserID: "u-1", Method: domain.MethodTransfer, Status: domain.StatusUnpaid,
		Amount: 85000, CustomerName: "Siti", CustomerPhone: "081298765432",
	}
}

func TestGetByOrderCodeAuthorizesByPhone(t *testing.T) {
	s, r := newService(t)
	r.add(paidPayment())

	if _, err := s.GetByOrderCode(context.Background(), "SB-20261115-0001", "081298765432", ""); err != nil {
		t.Fatalf("phone cocok: %v", err)
	}
	if _, err := s.GetByOrderCode(context.Background(), "SB-20261115-0001", "081200000000", ""); !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("phone beda: err = %v, want ErrPaymentNotFound", err)
	}
	if _, err := s.GetByOrderCode(context.Background(), "SB-20261115-0001", "", "u-1"); err != nil {
		t.Fatalf("pemilik akun: %v", err)
	}
}

func TestUploadProofValidatesAndEmitsEvent(t *testing.T) {
	s, r := newService(t)
	r.add(paidPayment())

	if _, err := s.UploadProof(context.Background(), domain.UploadProofInput{Code: "SB-20261115-0001", Phone: "081298765432"}); err == nil {
		t.Fatal("proof_url kosong harus error")
	}

	p, err := s.UploadProof(context.Background(), domain.UploadProofInput{
		Code: "SB-20261115-0001", Phone: "081298765432", ProofURL: "/uploads/x.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.StatusWaitingVerification {
		t.Fatalf("status = %q", p.Status)
	}
	if len(r.attachEvents) != 1 {
		t.Fatalf("event = %+v", r.attachEvents)
	}
	ev, err := kafkax.Unmarshal(r.attachEvents[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Type != "payment.proof_uploaded" {
		t.Fatalf("type = %q", ev.Type)
	}
}

func TestUploadProofRejectsWrongStatus(t *testing.T) {
	s, r := newService(t)
	p := paidPayment()
	p.Status = domain.StatusPaid
	r.add(p)

	_, err := s.UploadProof(context.Background(), domain.UploadProofInput{
		Code: "SB-20261115-0001", Phone: "081298765432", ProofURL: "/uploads/x.jpg",
	})
	if !errors.Is(err, domain.ErrProofNotAllowed) {
		t.Fatalf("err = %v, want ErrProofNotAllowed", err)
	}
}

func TestVerifyApproveAndReject(t *testing.T) {
	s, r := newService(t)
	p := paidPayment()
	p.Status = domain.StatusWaitingVerification
	r.add(p)

	if _, err := s.Verify(context.Background(), "pay-1", "APPROVE", "", "admin-1"); err != nil {
		t.Fatal(err)
	}
	if len(r.verifyEvents) != 1 {
		t.Fatalf("event = %+v", r.verifyEvents)
	}
	ev, _ := kafkax.Unmarshal(r.verifyEvents[0].Payload)
	if ev.Type != "payment.confirmed" {
		t.Fatalf("type = %q", ev.Type)
	}

	// Reject butuh alasan.
	r.add(func() domain.Payment {
		q := paidPayment()
		q.ID = "pay-2"
		q.OrderCode = "SB-2"
		q.Status = domain.StatusWaitingVerification
		return q
	}())
	if _, err := s.Verify(context.Background(), "pay-2", "REJECT", "", "admin-1"); err == nil {
		t.Fatal("reject tanpa alasan harus error")
	}
	if _, err := s.Verify(context.Background(), "pay-2", "REJECT", "bukti buram", "admin-1"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyInvalidDecision(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.Verify(context.Background(), "pay-1", "MAYBE", "", "admin-1"); !errors.Is(err, domain.ErrInvalidDecision) {
		t.Fatalf("err = %v, want ErrInvalidDecision", err)
	}
}

func TestListRejectsUnknownStatus(t *testing.T) {
	s, _ := newService(t)
	var ve *domain.ValidationError
	if _, _, _, _, err := s.List(context.Background(), "NOPE", 1, 20); !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if _, _, _, _, err := s.List(context.Background(), "", 1, 20); err != nil {
		t.Fatal(err)
	}
}

func TestHandleOrderCreatedCreatesPayment(t *testing.T) {
	s, r := newService(t)
	payload, _ := json.Marshal(map[string]any{
		"order_id": "o-1", "code": "SB-1", "user_id": "u-1",
		"payment_method": "TRANSFER", "total": 85000,
		"customer": map[string]any{"name": "Siti", "phone": "081298765432"},
	})
	ev := kafkax.Event{EventID: "ev-1", Type: "order.created", Payload: payload}

	if err := s.HandleOrderEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(r.created) != 1 {
		t.Fatalf("created = %+v", r.created)
	}
	if r.created[0].Method != "TRANSFER" || r.created[0].Amount != 85000 {
		t.Fatalf("created = %+v", r.created[0])
	}
}

func TestHandleOrderCancelledCancelsPayment(t *testing.T) {
	s, r := newService(t)
	payload, _ := json.Marshal(map[string]any{"order_id": "o-1"})
	ev := kafkax.Event{EventID: "ev-1", Type: "order.cancelled", Payload: payload}

	if err := s.HandleOrderEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(r.cancelled) != 1 || r.cancelled[0] != "o-1" {
		t.Fatalf("cancelled = %+v", r.cancelled)
	}
}

func TestHandleOrderEventErrors(t *testing.T) {
	s, _ := newService(t)

	bad := kafkax.Event{EventID: "e", Type: "order.created", Payload: json.RawMessage(`not-json`)}
	if err := s.HandleOrderEvent(context.Background(), bad); err == nil {
		t.Fatal("payload rusak harus error")
	}

	missing := kafkax.Event{EventID: "e", Type: "order.created", Payload: json.RawMessage(`{}`)}
	if err := s.HandleOrderEvent(context.Background(), missing); err == nil {
		t.Fatal("order_id kosong harus error")
	}

	badMethod, _ := json.Marshal(map[string]any{"order_id": "o-1", "code": "SB-1", "payment_method": "CASH"})
	if err := s.HandleOrderEvent(context.Background(), kafkax.Event{EventID: "e", Type: "order.created", Payload: badMethod}); err == nil {
		t.Fatal("metode tak dikenal harus error")
	}

	unknown := kafkax.Event{EventID: "e", Type: "order.status_changed", Payload: json.RawMessage(`{}`)}
	if err := s.HandleOrderEvent(context.Background(), unknown); err != nil {
		t.Fatalf("tipe lain harus diabaikan: %v", err)
	}
}
