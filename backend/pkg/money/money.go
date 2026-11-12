// Package money mengelola nominal uang dalam rupiah.
//
// Uang SELALU disimpan sebagai int64 rupiah. Tidak pernah float.
package money

import (
	"strconv"
	"strings"
)

// Rupiah adalah nominal uang dalam rupiah penuh (tanpa sen).
type Rupiah int64

// Format memformat nominal sebagai "Rp75.000".
func Format(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	s := strconv.FormatInt(amount, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := "Rp" + b.String()
	if neg {
		return "-" + out
	}
	return out
}

// Format memformat Rupiah sebagai "Rp75.000".
func (r Rupiah) Format() string { return Format(int64(r)) }

// Sum menjumlahkan nominal.
func Sum(values ...int64) int64 {
	var total int64
	for _, v := range values {
		total += v
	}
	return total
}

// Valid memastikan nominal tidak negatif.
func Valid(amount int64) bool { return amount >= 0 }
