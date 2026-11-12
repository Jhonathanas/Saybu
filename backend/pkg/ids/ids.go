// Package ids membuat identifier yang dipakai lintas service.
package ids

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// New mengembalikan UUIDv7 (time-ordered) sebagai string.
// UUIDv7 ramah index karena urut berdasarkan waktu pembuatan.
func New() string {
	u, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return u.String()
}

// NewUUID mengembalikan UUIDv7.
func NewUUID() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return u
}

// OrderCode membuat kode pesanan publik, mis. SB-20261111-0007.
func OrderCode(t time.Time, seq int64) string {
	return fmt.Sprintf("SB-%s-%04d", t.In(time.FixedZone("WIB", 7*60*60)).Format("20060102"), seq)
}
