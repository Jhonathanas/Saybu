package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMethodPredicates(t *testing.T) {
	read := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, m := range read {
		if !ReadMethods(&http.Request{Method: m}) {
			t.Errorf("ReadMethods(%s) = false", m)
		}
		if WriteMethods(&http.Request{Method: m}) {
			t.Errorf("WriteMethods(%s) = true", m)
		}
	}
	write := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, m := range write {
		if !WriteMethods(&http.Request{Method: m}) {
			t.Errorf("WriteMethods(%s) = false", m)
		}
	}
}

func TestMiddlewareLimitsAndSetsRetryAfter(t *testing.T) {
	limiter := New(1, time.Minute)
	h := Middleware(limiter, ReadMethods, ByIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("request pertama = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("request kedua = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("header Retry-After wajib ada")
	}
}

func TestMiddlewareSkipsNonMatchingMethods(t *testing.T) {
	limiter := New(1, time.Minute)
	h := Middleware(limiter, WriteMethods, ByIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET tidak dihitung oleh pembatas tulis, jadi tidak pernah 429.
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET ke-%d = %d, want 200", i, rec.Code)
		}
	}
}

func TestMiddlewareNilLimiterAllows(t *testing.T) {
	h := Middleware(nil, ReadMethods, ByIP)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("limiter nil = %d, want 200", rec.Code)
	}
}
