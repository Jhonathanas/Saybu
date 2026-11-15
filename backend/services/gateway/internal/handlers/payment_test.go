package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"

	paymentv1 "saybu/gen/saybu/payment/v1"
	"saybu/pkg/storage"
	"saybu/services/gateway/internal/authmw"
)

type fakePayment struct {
	get    func(context.Context, *paymentv1.GetPaymentByOrderCodeRequest) (*paymentv1.Payment, error)
	upload func(context.Context, *paymentv1.UploadProofRequest) (*paymentv1.Payment, error)
	list   func(context.Context, *paymentv1.ListPaymentsRequest) (*paymentv1.ListPaymentsResponse, error)
	verify func(context.Context, *paymentv1.VerifyPaymentRequest) (*paymentv1.Payment, error)
}

func (f *fakePayment) GetPaymentByOrderCode(ctx context.Context, in *paymentv1.GetPaymentByOrderCodeRequest, _ ...grpc.CallOption) (*paymentv1.Payment, error) {
	return f.get(ctx, in)
}
func (f *fakePayment) UploadProof(ctx context.Context, in *paymentv1.UploadProofRequest, _ ...grpc.CallOption) (*paymentv1.Payment, error) {
	return f.upload(ctx, in)
}
func (f *fakePayment) ListPayments(ctx context.Context, in *paymentv1.ListPaymentsRequest, _ ...grpc.CallOption) (*paymentv1.ListPaymentsResponse, error) {
	return f.list(ctx, in)
}
func (f *fakePayment) VerifyPayment(ctx context.Context, in *paymentv1.VerifyPaymentRequest, _ ...grpc.CallOption) (*paymentv1.Payment, error) {
	return f.verify(ctx, in)
}

func newPaymentHandler(t *testing.T, client PaymentService) *Payment {
	t.Helper()
	disk, err := storage.NewDisk(t.TempDir(), "/uploads")
	if err != nil {
		t.Fatal(err)
	}
	return NewPayment(client, slog.New(slog.NewTextHandler(io.Discard, nil)), disk, 5<<20, nil,
		BankInfo{Name: "BCA", AccountNumber: "1234567890", AccountHolder: "Saybu"})
}

func decodePaymentBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body bukan json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestGetPaymentTransferIncludesBank(t *testing.T) {
	h := newPaymentHandler(t, &fakePayment{
		get: func(_ context.Context, in *paymentv1.GetPaymentByOrderCodeRequest) (*paymentv1.Payment, error) {
			if in.GetPhone() != "081298765432" {
				t.Errorf("phone = %q", in.GetPhone())
			}
			return &paymentv1.Payment{Method: "TRANSFER", Status: "UNPAID", Amount: 85000}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.GetPayment(rec, httptest.NewRequest(http.MethodGet, "/api/v1/orders/SB-1/payment?phone=081298765432", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	data := decodePaymentBody(t, rec)["data"].(map[string]any)
	bank, ok := data["bank"].(map[string]any)
	if !ok {
		t.Fatalf("bank hilang: %+v", data)
	}
	if bank["account_number"] != "1234567890" {
		t.Fatalf("bank = %+v", bank)
	}
	if data["proof_url"] != nil {
		t.Fatalf("proof_url = %v, want null", data["proof_url"])
	}
}

func TestGetPaymentCODHasNoBank(t *testing.T) {
	h := newPaymentHandler(t, &fakePayment{
		get: func(context.Context, *paymentv1.GetPaymentByOrderCodeRequest) (*paymentv1.Payment, error) {
			return &paymentv1.Payment{Method: "COD", Status: "COD_PENDING", Amount: 50000}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.GetPayment(rec, httptest.NewRequest(http.MethodGet, "/api/v1/orders/SB-1/payment", nil))
	data := decodePaymentBody(t, rec)["data"].(map[string]any)
	if data["bank"] != nil {
		t.Fatalf("bank harus null untuk COD: %+v", data)
	}
}

func multipartProof(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = w.WriteField("phone", "081298765432")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return body, w.FormDataContentType()
}

func TestUploadProofMissingFile(t *testing.T) {
	h := newPaymentHandler(t, &fakePayment{})

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("phone", "081298765432")
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/SB-1/payment-proof", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadProof(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUploadProofInvalidExtension(t *testing.T) {
	h := newPaymentHandler(t, &fakePayment{})

	body, ct := multipartProof(t, "bukti.txt", []byte("hello"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/SB-1/payment-proof", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.UploadProof(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUploadProofSuccess(t *testing.T) {
	var gotURL string
	h := newPaymentHandler(t, &fakePayment{
		upload: func(_ context.Context, in *paymentv1.UploadProofRequest) (*paymentv1.Payment, error) {
			gotURL = in.GetProofUrl()
			return &paymentv1.Payment{Method: "TRANSFER", Status: "WAITING_VERIFICATION", ProofUrl: in.GetProofUrl()}, nil
		},
	})

	// Header PNG minimal agar terdeteksi sebagai image/png.
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	body, ct := multipartProof(t, "bukti.png", png)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/SB-1/payment-proof", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.UploadProof(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotURL == "" {
		t.Fatal("proof_url tidak diteruskan ke service")
	}
}

func TestAdminListPayments(t *testing.T) {
	h := newPaymentHandler(t, &fakePayment{
		list: func(_ context.Context, in *paymentv1.ListPaymentsRequest) (*paymentv1.ListPaymentsResponse, error) {
			if in.GetStatus() != "WAITING_VERIFICATION" {
				t.Errorf("status filter = %q", in.GetStatus())
			}
			return &paymentv1.ListPaymentsResponse{
				Payments: []*paymentv1.Payment{{Id: "p-1", OrderCode: "SB-1", Status: "WAITING_VERIFICATION", Amount: 85000}},
				Page:     1, Limit: 20, Total: 1,
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.AdminListPayments(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/payments?status=WAITING_VERIFICATION", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	out := decodePaymentBody(t, rec)
	list, ok := out["data"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("data = %+v", out["data"])
	}
}

func TestAdminVerifyPaymentNoIdentity(t *testing.T) {
	h := newPaymentHandler(t, &fakePayment{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/payments/p-1/verify",
		bytes.NewReader([]byte(`{"decision":"APPROVE"}`)))
	rec := httptest.NewRecorder()
	h.AdminVerifyPayment(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdminVerifyPaymentSuccess(t *testing.T) {
	var gotAdmin string
	h := newPaymentHandler(t, &fakePayment{
		verify: func(_ context.Context, in *paymentv1.VerifyPaymentRequest) (*paymentv1.Payment, error) {
			gotAdmin = in.GetAdminId()
			if in.GetDecision() != "APPROVE" {
				t.Errorf("decision = %q", in.GetDecision())
			}
			return &paymentv1.Payment{Id: "p-1", OrderCode: "SB-1", Status: "PAID", Amount: 85000}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/payments/p-1/verify",
		bytes.NewReader([]byte(`{"decision":"APPROVE","note":"dana masuk"}`)))
	req = req.WithContext(authmw.WithIdentity(req.Context(), authmw.Identity{UserID: "admin-1", Role: "admin"}))
	rec := httptest.NewRecorder()
	h.AdminVerifyPayment(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotAdmin != "admin-1" {
		t.Fatalf("admin_id = %q", gotAdmin)
	}
}
