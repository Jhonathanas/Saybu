// Package domain berisi entity dan aturan bisnis pembayaran yang murni
// (tanpa dependensi ke database, gRPC, atau HTTP).
package domain

import (
	"errors"
	"strings"
	"time"
)

// Error domain pembayaran.
var (
	ErrPaymentNotFound  = errors.New("pembayaran tidak ditemukan")
	ErrProofNotAllowed  = errors.New("bukti tidak bisa diunggah pada status ini")
	ErrVerifyNotAllowed = errors.New("pembayaran tidak bisa diverifikasi pada status ini")
	ErrInvalidDecision  = errors.New("keputusan verifikasi tidak valid")
	ErrPaymentExists    = errors.New("pembayaran untuk pesanan ini sudah ada")
)

// Metode pembayaran.
const (
	MethodTransfer = "TRANSFER"
	MethodCOD      = "COD"
)

// Status pembayaran.
const (
	StatusUnpaid              = "UNPAID"
	StatusWaitingVerification = "WAITING_VERIFICATION"
	StatusPaid                = "PAID"
	StatusRejected            = "REJECTED"
	StatusCancelled           = "CANCELLED"
	StatusCODPending          = "COD_PENDING"
	StatusCODPaid             = "COD_PAID"
)

// Keputusan verifikasi admin.
const (
	DecisionApprove = "APPROVE"
	DecisionReject  = "REJECT"
)

// Payment adalah status pembayaran sebuah pesanan (snapshot saat order dibuat).
type Payment struct {
	ID              string
	OrderID         string
	OrderCode       string
	UserID          string
	Method          string
	Status          string
	Amount          int64
	CustomerName    string
	CustomerPhone   string
	ProofURL        string
	ProofUploadedAt *time.Time
	VerifiedAt      *time.Time
	VerifiedBy      string
	Note            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Proof adalah satu riwayat unggahan bukti transfer.
type Proof struct {
	ID          string
	PaymentID   string
	URL         string
	SizeBytes   int64
	ContentType string
	UploadedBy  string
	CreatedAt   time.Time
}

// UploadProofInput adalah permintaan pendaftaran bukti transfer.
type UploadProofInput struct {
	Code        string
	Phone       string
	UserID      string
	ProofURL    string
	SizeBytes   int64
	ContentType string
}

// InitialStatus menentukan status awal pembayaran dari metode pembayaran.
func InitialStatus(method string) string {
	if method == MethodCOD {
		return StatusCODPending
	}
	return StatusUnpaid
}

// IsValidMethod melaporkan apakah metode pembayaran dikenal.
func IsValidMethod(method string) bool {
	return method == MethodTransfer || method == MethodCOD
}

// IsValidStatus melaporkan apakah status pembayaran dikenal.
func IsValidStatus(status string) bool {
	switch status {
	case StatusUnpaid, StatusWaitingVerification, StatusPaid, StatusRejected,
		StatusCancelled, StatusCODPending, StatusCODPaid:
		return true
	default:
		return false
	}
}

// CanUploadProof melaporkan apakah bukti transfer boleh diunggah pada status ini.
func CanUploadProof(status string) bool {
	return status == StatusUnpaid || status == StatusRejected
}

// CanVerify melaporkan apakah pembayaran menunggu verifikasi admin.
func CanVerify(status string) bool {
	return status == StatusWaitingVerification
}

// CanMarkCODPaid melaporkan apakah pembayaran COD boleh ditandai lunas
// (saat pesanan selesai).
func CanMarkCODPaid(status string) bool {
	return status == StatusCODPending
}

// IsCancellable melaporkan apakah pembayaran boleh dibatalkan (pesanan batal/kedaluwarsa).
func IsCancellable(status string) bool {
	switch status {
	case StatusPaid, StatusCancelled, StatusCODPaid:
		return false
	default:
		return true
	}
}

// NormalizePhone menyisakan digit saja agar pencocokan nomor HP konsisten.
func NormalizePhone(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// PhoneMatches melaporkan apakah dua nomor HP sama setelah dinormalisasi.
func PhoneMatches(a, b string) bool {
	na, nb := NormalizePhone(a), NormalizePhone(b)
	return na != "" && na == nb
}

// ValidationError mengumpulkan kesalahan validasi per field.
type ValidationError struct {
	Errors []FieldError
}

// FieldError menjelaskan satu kesalahan validasi.
type FieldError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fe.Field+": "+fe.Message)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

// Add menambahkan satu kesalahan field.
func (e *ValidationError) Add(field, message string) *ValidationError {
	e.Errors = append(e.Errors, FieldError{Field: field, Message: message})
	return e
}

// OrNil mengembalikan error atau nil bila tidak ada kesalahan.
func (e *ValidationError) OrNil() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}
