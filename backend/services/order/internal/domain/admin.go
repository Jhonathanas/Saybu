package domain

import (
	"strings"
	"time"
)

// ListOrdersFilter adalah filter daftar pesanan untuk panel admin.
//
// Tanggal memakai zona Asia/Jakarta (UI) dan inklusif pada kedua ujung.
type ListOrdersFilter struct {
	Status   string // status persis (opsional)
	DateFrom string // YYYY-MM-DD (opsional)
	DateTo   string // YYYY-MM-DD (opsional)
	Query    string // cari kode / nama / nomor HP pelanggan (opsional)
	Page     int
	Limit    int
}

// Normalize merapikan filter: trim spasi dan batasi halaman.
func (f *ListOrdersFilter) Normalize() {
	f.Status = strings.TrimSpace(f.Status)
	f.DateFrom = strings.TrimSpace(f.DateFrom)
	f.DateTo = strings.TrimSpace(f.DateTo)
	f.Query = strings.TrimSpace(f.Query)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
}

// Validate memastikan filter sah sebelum dikirim ke database.
func (f ListOrdersFilter) Validate() error {
	if f.Status != "" && !IsValidStatus(f.Status) {
		return (&ValidationError{}).Add("status", "Status tidak dikenal.")
	}
	for field, val := range map[string]string{"date_from": f.DateFrom, "date_to": f.DateTo} {
		if val == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", val); err != nil {
			return (&ValidationError{}).Add(field, "Format tanggal harus YYYY-MM-DD.")
		}
	}
	return nil
}

// TodayStats adalah ringkasan operasional hari ini untuk dashboard admin.
//
// OrdersCount menghitung pesanan yang dibuat hari ini; RevenuePaid menjumlahkan
// total pesanan hari ini yang sudah lunas; AwaitingVerification menghitung
// seluruh pesanan yang menunggu verifikasi (backlog, bukan hanya hari ini).
type TodayStats struct {
	OrdersCount          int64
	RevenuePaid          int64
	AwaitingVerification int64
}

// RevenueStatuses adalah status pesanan yang dianggap sudah lunas untuk
// perhitungan pendapatan.
var RevenueStatuses = []string{
	StatusPaid, StatusPreparing, StatusShipped, StatusReadyPickup, StatusCompleted,
}
