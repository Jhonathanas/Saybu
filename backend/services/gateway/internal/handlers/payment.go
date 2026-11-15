package handlers

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"

	paymentv1 "saybu/gen/saybu/payment/v1"
	"saybu/pkg/httpx"
	"saybu/pkg/storage"
	"saybu/services/gateway/internal/authmw"
	"saybu/services/gateway/internal/ratelimit"
)

// PaymentService adalah bagian klien gRPC payment yang dipakai gateway.
type PaymentService interface {
	GetPaymentByOrderCode(ctx context.Context, in *paymentv1.GetPaymentByOrderCodeRequest, opts ...grpc.CallOption) (*paymentv1.Payment, error)
	UploadProof(ctx context.Context, in *paymentv1.UploadProofRequest, opts ...grpc.CallOption) (*paymentv1.Payment, error)
	ListPayments(ctx context.Context, in *paymentv1.ListPaymentsRequest, opts ...grpc.CallOption) (*paymentv1.ListPaymentsResponse, error)
	VerifyPayment(ctx context.Context, in *paymentv1.VerifyPaymentRequest, opts ...grpc.CallOption) (*paymentv1.Payment, error)
}

// BankInfo adalah instruksi transfer yang ditampilkan ke pelanggan.
type BankInfo struct {
	Name          string
	AccountNumber string
	AccountHolder string
}

// Payment menangani instruksi pembayaran, unggah bukti, dan verifikasi admin.
type Payment struct {
	client     PaymentService
	log        *slog.Logger
	uploads    *storage.Disk
	maxBytes   int64
	proofLimit *ratelimit.Limiter
	bank       BankInfo
}

// NewPayment membuat handler pembayaran.
func NewPayment(client PaymentService, log *slog.Logger, uploads *storage.Disk, maxBytes int64, proofLimit *ratelimit.Limiter, bank BankInfo) *Payment {
	return &Payment{
		client:     client,
		log:        log,
		uploads:    uploads,
		maxBytes:   maxBytes,
		proofLimit: proofLimit,
		bank:       bank,
	}
}

type bankJSON struct {
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}

type paymentJSON struct {
	Method     string    `json:"method"`
	Status     string    `json:"status"`
	Amount     int64     `json:"amount"`
	Bank       *bankJSON `json:"bank"`
	ProofURL   *string   `json:"proof_url"`
	VerifiedAt *string   `json:"verified_at"`
	Note       *string   `json:"note"`
}

type adminPaymentJSON struct {
	ID            string  `json:"id"`
	OrderCode     string  `json:"order_code"`
	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone"`
	Method        string  `json:"method"`
	Status        string  `json:"status"`
	Amount        int64   `json:"amount"`
	ProofURL      *string `json:"proof_url"`
	UploadedAt    *string `json:"uploaded_at"`
	VerifiedAt    *string `json:"verified_at"`
	Note          *string `json:"note"`
	CreatedAt     string  `json:"created_at"`
}

type verifyPaymentBody struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

// GetPayment menangani GET /api/v1/orders/{code}/payment?phone=...
func (h *Payment) GetPayment(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetPaymentByOrderCode(outgoingContext(r), &paymentv1.GetPaymentByOrderCodeRequest{
		Code:   chiURLParam(r, "code"),
		Phone:  r.URL.Query().Get("phone"),
		UserId: identityUserID(r),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, h.toPaymentJSON(res))
}

// UploadProof menangani POST /api/v1/orders/{code}/payment-proof (multipart).
func (h *Payment) UploadProof(w http.ResponseWriter, r *http.Request) {
	if !allowRate(w, r, h.proofLimit, "proof:"+clientIP(r)) {
		return
	}
	if h.uploads == nil {
		httpx.WriteError(w, r, httpx.New(http.StatusServiceUnavailable, "UNAVAILABLE", "Unggah berkas tidak dikonfigurasi."))
		return
	}

	// Batasi ukuran body; beri ruang untuk overhead multipart.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+(1<<20))
	if err := r.ParseMultipartForm(h.maxBytes); err != nil {
		httpx.WriteError(w, r, httpx.BadRequest("Berkas terlalu besar atau form tidak valid."))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, httpx.BadRequest("Field 'file' wajib diisi."))
		return
	}
	defer file.Close()

	if header.Size > h.maxBytes {
		httpx.WriteError(w, r, httpx.BadRequest("Ukuran berkas melebihi batas 5 MB."))
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !storage.AllowedProofExt[ext] {
		httpx.WriteError(w, r, httpx.BadRequest("Format berkas harus JPG, PNG, atau PDF."))
		return
	}

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	contentType := http.DetectContentType(head[:n])
	if !strings.HasPrefix(contentType, "image/") && contentType != "application/pdf" {
		httpx.WriteError(w, r, httpx.BadRequest("Berkas bukan gambar atau PDF yang valid."))
		return
	}
	reader := io.MultiReader(bytes.NewReader(head[:n]), file)

	url, err := h.uploads.SaveWith(r.Context(), ext, reader, storage.AllowedProofExt)
	if err != nil {
		h.log.Error("simpan bukti transfer gagal", "error", err)
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	res, err := h.client.UploadProof(outgoingContext(r), &paymentv1.UploadProofRequest{
		Code:        chiURLParam(r, "code"),
		Phone:       r.FormValue("phone"),
		UserId:      identityUserID(r),
		ProofUrl:    url,
		SizeBytes:   header.Size,
		ContentType: contentType,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, h.toPaymentJSON(res))
}

// AdminListPayments menangani GET /api/v1/admin/payments.
func (h *Payment) AdminListPayments(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	res, err := h.client.ListPayments(outgoingContext(r), &paymentv1.ListPaymentsRequest{
		Status: r.URL.Query().Get("status"),
		Page:   int32(page),
		Limit:  int32(limit),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	out := make([]adminPaymentJSON, 0, len(res.GetPayments()))
	for _, p := range res.GetPayments() {
		out = append(out, toAdminPaymentJSON(p))
	}
	httpx.DataMeta(w, http.StatusOK, out, map[string]any{
		"page":  res.GetPage(),
		"limit": res.GetLimit(),
		"total": res.GetTotal(),
	})
}

// AdminVerifyPayment menangani POST /api/v1/admin/payments/{id}/verify.
func (h *Payment) AdminVerifyPayment(w http.ResponseWriter, r *http.Request) {
	var body verifyPaymentBody
	if !decodeJSON(w, r, &body) {
		return
	}
	id, ok := authmw.IdentityFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.Unauthorized("Token akses diperlukan."))
		return
	}
	res, err := h.client.VerifyPayment(outgoingContext(r), &paymentv1.VerifyPaymentRequest{
		Id:       chi.URLParam(r, "id"),
		Decision: body.Decision,
		Note:     body.Note,
		AdminId:  id.UserID,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toAdminPaymentJSON(res))
}

func (h *Payment) toPaymentJSON(p *paymentv1.Payment) paymentJSON {
	out := paymentJSON{
		Method: p.GetMethod(),
		Status: p.GetStatus(),
		Amount: p.GetAmount(),
	}
	if p.GetMethod() == "TRANSFER" {
		out.Bank = &bankJSON{
			Name:          h.bank.Name,
			AccountNumber: h.bank.AccountNumber,
			AccountHolder: h.bank.AccountHolder,
		}
	}
	if v := p.GetProofUrl(); v != "" {
		out.ProofURL = &v
	}
	if v := p.GetVerifiedAt(); v != "" {
		out.VerifiedAt = &v
	}
	if v := p.GetNote(); v != "" {
		out.Note = &v
	}
	return out
}

func toAdminPaymentJSON(p *paymentv1.Payment) adminPaymentJSON {
	out := adminPaymentJSON{
		ID:            p.GetId(),
		OrderCode:     p.GetOrderCode(),
		CustomerName:  p.GetCustomerName(),
		CustomerPhone: p.GetCustomerPhone(),
		Method:        p.GetMethod(),
		Status:        p.GetStatus(),
		Amount:        p.GetAmount(),
		CreatedAt:     p.GetCreatedAt(),
	}
	if v := p.GetProofUrl(); v != "" {
		out.ProofURL = &v
	}
	if v := p.GetProofUploadedAt(); v != "" {
		out.UploadedAt = &v
	}
	if v := p.GetVerifiedAt(); v != "" {
		out.VerifiedAt = &v
	}
	if v := p.GetNote(); v != "" {
		out.Note = &v
	}
	return out
}
