// Package ratelimit menyediakan rate limiter fixed-window in-memory.
// Cukup untuk satu instance gateway; bila gateway di-scale, ganti dengan Redis.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	count int
	reset time.Time
}

// Limiter membatasi jumlah percobaan per key dalam satu window.
type Limiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	limit     int
	window    time.Duration
	now       func() time.Time
	nextSweep time.Time
}

// New membuat Limiter dengan jam sistem.
func New(limit int, window time.Duration) *Limiter {
	return NewWithClock(limit, window, time.Now)
}

// NewWithClock membuat Limiter dengan jam khusus (untuk test).
func NewWithClock(limit int, window time.Duration, now func() time.Time) *Limiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
		now:     now,
	}
}

// Allow mencatat satu percobaan. Bila ditolak, retryAfter menunjukkan sisa waktu
// sampai window berikutnya.
func (l *Limiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	// Bersihkan bucket kedaluwarsa secara berkala agar memori tidak tumbuh
	// tanpa batas saat banyak key (mis. banyak IP) muncul.
	if !now.Before(l.nextSweep) {
		for k, b := range l.buckets {
			if !now.Before(b.reset) {
				delete(l.buckets, k)
			}
		}
		l.nextSweep = now.Add(l.window)
	}

	b, ok := l.buckets[key]
	if !ok || !now.Before(b.reset) {
		l.buckets[key] = &bucket{count: 1, reset: now.Add(l.window)}
		return true, 0
	}
	if b.count >= l.limit {
		return false, b.reset.Sub(now)
	}
	b.count++
	return true, 0
}

// Size mengembalikan jumlah bucket aktif (untuk test/pemantauan).
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
