package ratelimit

import (
	"math"
	"net/http"
	"strconv"

	"saybu/pkg/httpx"
)

// KeyFunc menghasilkan kunci pembatas dari request (mis. IP atau id pengguna).
type KeyFunc func(r *http.Request) string

// Middleware membatasi laju request per kunci. only menentukan apakah request
// ikut dihitung; keyOf menghasilkan kunci. Limiter nil berarti tanpa batas.
func Middleware(l *Limiter, only func(*http.Request) bool, keyOf KeyFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if l != nil && only(r) {
				ok, retryAfter := l.Allow(keyOf(r))
				if !ok {
					seconds := int(math.Ceil(retryAfter.Seconds()))
					if seconds < 1 {
						seconds = 1
					}
					w.Header().Set("Retry-After", strconv.Itoa(seconds))
					httpx.WriteError(w, r, httpx.New(http.StatusTooManyRequests, "RATE_LIMITED",
						"Terlalu banyak permintaan. Coba lagi nanti."))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ReadMethods benar untuk metode baca (GET/HEAD/OPTIONS).
func ReadMethods(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// WriteMethods benar untuk metode tulis (POST/PUT/PATCH/DELETE).
func WriteMethods(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// ByIP membuat kunci dari IP klien.
func ByIP(r *http.Request) string { return "ip:" + httpx.ClientIP(r) }
