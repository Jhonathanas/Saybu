// Package domain berisi entity dan aturan bisnis ongkir yang murni
// (tanpa dependensi ke database, gRPC, atau HTTP).
package domain

import (
	"errors"
	"strings"
	"time"
)

// Error domain ongkir.
var (
	ErrZoneNotFound = errors.New("zona tidak ditemukan")
	ErrZoneExists   = errors.New("nama zona sudah dipakai")
)

// Zone adalah satu wilayah pengiriman dengan tarif tetap.
type Zone struct {
	ID              string
	Name            string
	Areas           []string
	Fee             int64
	FreeMinSubtotal int64
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Quote adalah hasil perhitungan ongkir untuk sebuah subtotal.
type Quote struct {
	ZoneID       string
	ZoneName     string
	Fee          int64
	FreeShipping bool
	Subtotal     int64
	Total        int64
}

// ComputeQuote menghitung ongkir zona untuk subtotal tertentu.
// Gratis ongkir bila FreeMinSubtotal > 0 dan subtotal memenuhinya.
func ComputeQuote(z Zone, subtotal int64) Quote {
	fee := z.Fee
	free := false
	if z.FreeMinSubtotal > 0 && subtotal >= z.FreeMinSubtotal {
		fee = 0
		free = true
	}
	return Quote{
		ZoneID:       z.ID,
		ZoneName:     z.Name,
		Fee:          fee,
		FreeShipping: free,
		Subtotal:     subtotal,
		Total:        subtotal + fee,
	}
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
