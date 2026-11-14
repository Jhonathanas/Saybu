// Package domain berisi entity dan aturan bisnis pesanan yang murni
// (tanpa dependensi ke database, gRPC, atau HTTP).
package domain

import (
	"errors"
	"strings"
	"time"
)

// Error domain pesanan.
var (
	ErrOrderNotFound         = errors.New("pesanan tidak ditemukan")
	ErrInvalidTransition     = errors.New("transisi status tidak sah")
	ErrCancelNotAllowed      = errors.New("pesanan tidak bisa dibatalkan pada status ini")
	ErrIdempotencyMismatch   = errors.New("idempotency key dipakai dengan isi berbeda")
	ErrIdempotencyExists     = errors.New("idempotency key sudah ada")
	ErrInsufficientStock     = errors.New("stok tidak cukup")
	ErrProductUnavailable    = errors.New("produk tidak tersedia")
	ErrZoneRequired          = errors.New("zona ongkir wajib untuk pengiriman")
	ErrAddressRequired       = errors.New("alamat wajib untuk pengiriman")
	ErrPhoneMismatch         = errors.New("nomor HP tidak cocok dengan pesanan")
	ErrIdempotencyKeyMissing = errors.New("idempotency key wajib")
	ErrZoneNotFound          = errors.New("zona ongkir tidak ditemukan")
)

// Status pesanan.
const (
	StatusPendingPayment      = "PENDING_PAYMENT"
	StatusWaitingVerification = "WAITING_VERIFICATION"
	StatusPaid                = "PAID"
	StatusPreparing           = "PREPARING"
	StatusShipped             = "SHIPPED"
	StatusReadyPickup         = "READY_PICKUP"
	StatusCompleted           = "COMPLETED"
	StatusCancelled           = "CANCELLED"
	StatusExpired             = "EXPIRED"
)

// Fulfillment: cara pesanan dipenuhi.
const (
	FulfillmentDelivery = "DELIVERY"
	FulfillmentPickup   = "PICKUP"
)

// PaymentMethod: cara pembayaran.
const (
	PaymentTransfer = "TRANSFER"
	PaymentCOD      = "COD"
)

// Customer adalah data pembeli (snapshot).
type Customer struct {
	Name  string
	Phone string
	Email string
	Note  string
}

// Address adalah alamat pengiriman (snapshot).
type Address struct {
	Line       string
	RTRW       string
	Kelurahan  string
	Kecamatan  string
	City       string
	PostalCode string
	Landmark   string
}

// Item adalah satu baris pesanan (snapshot produk saat checkout).
type Item struct {
	ProductID string
	Name      string
	Unit      string
	Qty       int64 // satuan terkecil (gram untuk kg)
	UnitPrice int64
	LineTotal int64
}

// StatusEntry mencatat satu perubahan status.
type StatusEntry struct {
	Status string
	Note   string
	Actor  string
	At     time.Time
}

// Order adalah pesanan.
type Order struct {
	ID            string
	Code          string
	UserID        string
	Status        string
	Fulfillment   string
	PaymentMethod string
	Customer      Customer
	Address       Address
	ZoneID        string
	ZoneName      string
	ShippingFee   int64
	Subtotal      int64
	Total         int64
	Items         []Item
	History       []StatusEntry
	CancelReason  string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ItemInput adalah permintaan item dari client (harga dihitung server).
type ItemInput struct {
	ProductID string
	Qty       int64
}

// CreateInput adalah permintaan checkout yang sudah dipetakan dari transport.
type CreateInput struct {
	Customer      Customer
	Fulfillment   string
	ZoneID        string
	Address       Address
	PaymentMethod string
	Items         []ItemInput
	UserID        string
}

// FieldError menjelaskan satu kesalahan validasi.
type FieldError struct {
	Field   string
	Message string
}

// StockShort melaporkan kekurangan stok satu produk.
type StockShort struct {
	ProductID string
	Name      string
	Requested int64
	Available int64
}

// InsufficientStockError membawa detail produk yang stoknya kurang.
type InsufficientStockError struct {
	Items []StockShort
}

func (e *InsufficientStockError) Error() string {
	return "stok tidak cukup"
}

// Is membuat errors.Is(err, ErrInsufficientStock) bernilai true.
func (e *InsufficientStockError) Is(target error) bool { return target == ErrInsufficientStock }

// ZoneNotFoundError menandai zona ongkir yang tidak dikenal.
//
// Ini pelanggaran aturan bisnis (bukan kesalahan bentuk input), sehingga
// dipetakan ke 422 agar konsisten dengan endpoint kuotasi ongkir.
type ZoneNotFoundError struct {
	ZoneID string
}

func (e *ZoneNotFoundError) Error() string { return "zona ongkir tidak ditemukan" }

// Is membuat errors.Is(err, ErrZoneNotFound) bernilai true.
func (e *ZoneNotFoundError) Is(target error) bool { return target == ErrZoneNotFound }

// ValidationError mengumpulkan kesalahan validasi per field.
type ValidationError struct {
	Errors []FieldError
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

// IsWeightUnit melaporkan apakah satuan memakai gram sebagai satuan terkecil.
func IsWeightUnit(unit string) bool { return unit == "kg" }

// LineTotal menghitung total satu baris dalam rupiah penuh.
//
// Untuk satuan berat, harga adalah per kg sedangkan qty dalam gram, sehingga
// total = harga * qty / 1000 (dibulatkan ke rupiah terdekat). Untuk satuan
// lain, total = harga * qty.
func LineTotal(unit string, price, qty int64) int64 {
	if IsWeightUnit(unit) {
		return (price*qty + 500) / 1000
	}
	return price * qty
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

// DisplayQty memformat qty (satuan terkecil) menjadi teks yang enak dibaca.
func DisplayQty(unit string, qty int64) string {
	if IsWeightUnit(unit) {
		return formatKg(qty) + " kg"
	}
	return formatInt(qty) + " " + unit
}

func formatKg(grams int64) string {
	whole := grams / 1000
	rem := grams % 1000
	if rem == 0 {
		return formatInt(whole)
	}
	frac := formatInt(rem)
	for len(frac) < 3 {
		frac = "0" + frac
	}
	frac = strings.TrimRight(frac, "0")
	return formatInt(whole) + "," + frac
}

func formatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
