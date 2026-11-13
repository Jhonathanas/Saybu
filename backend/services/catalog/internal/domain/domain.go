// Package domain berisi entity dan aturan bisnis katalog yang murni
// (tanpa dependensi ke database, gRPC, atau HTTP).
package domain

import (
	"errors"
	"strings"
	"time"
)

// Error domain katalog.
var (
	ErrProductNotFound   = errors.New("produk tidak ditemukan")
	ErrCategoryNotFound  = errors.New("kategori tidak ditemukan")
	ErrSlugExists        = errors.New("slug sudah dipakai")
	ErrInsufficientStock = errors.New("stok tidak cukup")
	ErrStockNegative     = errors.New("stok tidak boleh negatif")
	ErrReservationEmpty  = errors.New("tidak ada reservasi untuk order ini")
)

// Satuan produk yang didukung.
const (
	UnitKg    = "kg"
	UnitIkat  = "ikat"
	UnitPcs   = "pcs"
	UnitBuah  = "buah"
	UnitPaket = "paket"
)

// AllowedUnits adalah daftar satuan yang sah.
var AllowedUnits = []string{UnitKg, UnitIkat, UnitPcs, UnitBuah, UnitPaket}

// Category adalah kategori produk.
type Category struct {
	ID       string
	Name     string
	Slug     string
	Sort     int32
	IsActive bool
}

// Product adalah produk yang dijual.
//
// Price adalah harga per `Unit` dalam rupiah penuh. Stock disimpan dalam
// satuan terkecil: gram bila Unit == "kg", satuan lain bila bukan.
type Product struct {
	ID                string
	Name              string
	Slug              string
	Description       string
	Category          *Category
	Unit              string
	Price             int64
	Stock             int64
	MinQty            int64
	StepQty           int64
	LowStockThreshold int64
	IsActive          bool
	Images            []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// StockItem adalah permintaan sejumlah produk.
type StockItem struct {
	ProductID string
	Qty       int64
}

// StockStatus melaporkan ketersediaan satu produk.
type StockStatus struct {
	ProductID string
	Name      string
	Requested int64
	Available int64
	OK        bool
}

// ProductFilter adalah parameter pencarian produk.
type ProductFilter struct {
	CategorySlug    string
	Query           string
	Sort            string // newest | price_asc | price_desc
	Page            int
	Limit           int
	IncludeInactive bool
}

// FieldError menjelaskan satu kesalahan validasi.
type FieldError struct {
	Field   string
	Message string
}

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

// IsKg melaporkan apakah stok unit ini memakai gram sebagai satuan terkecil.
func IsKg(unit string) bool { return unit == UnitKg }

// StepDefault mengembalikan langkah qty bawaan untuk sebuah satuan.
func StepDefault(unit string) int64 {
	if IsKg(unit) {
		return 100 // 100 gram
	}
	return 1
}

// MinDefault mengembalikan qty minimum bawaan untuk sebuah satuan.
func MinDefault(unit string) int64 {
	if IsKg(unit) {
		return 100
	}
	return 1
}

// DisplayQty memformat qty (satuan terkecil) menjadi teks yang enak dibaca.
func DisplayQty(unit string, qty int64) string {
	if IsKg(unit) {
		return formatKg(qty) + " kg"
	}
	return formatInt(qty) + " " + unit
}

// formatKg mengubah gram menjadi teks kilogram tanpa nol yang tak perlu.
func formatKg(grams int64) string {
	whole := grams / 1000
	rem := grams % 1000
	if rem == 0 {
		return formatInt(whole)
	}
	frac := formatInt(rem)
	// 250 -> 0,25 ; 1500 -> 1,5
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
