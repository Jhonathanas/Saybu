package ids

import (
	"strings"
	"testing"
	"time"
)

func TestNewUnique(t *testing.T) {
	a, b := New(), New()
	if a == "" || b == "" {
		t.Fatal("id kosong")
	}
	if a == b {
		t.Fatal("id duplikat")
	}
	if len(a) != 36 {
		t.Fatalf("panjang id = %d, want 36", len(a))
	}
}

func TestOrderCode(t *testing.T) {
	// 18:00 UTC = 01:00 WIB hari berikutnya.
	utc := time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
	got := OrderCode(utc, 7)
	if got != "SB-20261111-0007" {
		t.Fatalf("OrderCode = %q", got)
	}
	if !strings.HasPrefix(got, "SB-") {
		t.Fatalf("prefix salah: %q", got)
	}
}
