package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	l := New(2, time.Minute)

	for i := 0; i < 2; i++ {
		ok, _ := l.Allow("ip")
		if !ok {
			t.Fatalf("percobaan ke-%d harus diizinkan", i+1)
		}
	}

	ok, retry := l.Allow("ip")
	if ok {
		t.Fatal("percobaan melebihi limit harus ditolak")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("retryAfter = %v", retry)
	}
}

func TestAllowSeparateKeys(t *testing.T) {
	l := New(1, time.Minute)

	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("key a harus diizinkan")
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("key b tidak boleh terpengaruh key a")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("key a harus ditolak pada percobaan kedua")
	}
}

func TestAllowResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewWithClock(1, time.Minute, func() time.Time { return now })

	if ok, _ := l.Allow("ip"); !ok {
		t.Fatal("percobaan pertama harus diizinkan")
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Fatal("percobaan kedua harus ditolak")
	}

	now = now.Add(time.Minute)
	if ok, _ := l.Allow("ip"); !ok {
		t.Fatal("setelah window habis harus diizinkan lagi")
	}
}

func TestNewNormalizesInvalidConfig(t *testing.T) {
	l := New(0, 0)
	if l.limit != 1 {
		t.Fatalf("limit = %d, want 1", l.limit)
	}
	if l.window != time.Minute {
		t.Fatalf("window = %v, want 1m", l.window)
	}
}

func TestAllowEvictsExpiredBuckets(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewWithClock(5, time.Minute, func() time.Time { return now })

	// Banyak key unik pada satu window; memori tumbuh.
	for i := 0; i < 100; i++ {
		l.Allow("ip-" + string(rune('a'+i%26)) + string(rune('0'+i/26)))
	}
	if got := l.Size(); got == 0 {
		t.Fatal("bucket seharusnya terisi")
	}

	// Lewati satu window: sweep berikutnya harus membersihkan bucket lama.
	now = now.Add(2 * time.Minute)
	l.Allow("ip-baru")
	if got := l.Size(); got != 1 {
		t.Fatalf("Size = %d, want 1 setelah eviction", got)
	}
}
