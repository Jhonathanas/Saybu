package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"saybu/pkg/telemetry"
)

func TestData(t *testing.T) {
	w := httptest.NewRecorder()
	Data(w, http.StatusCreated, map[string]string{"id": "1"})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}
	var body Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data == nil {
		t.Fatal("data nil")
	}
}

func TestWriteErrorKnown(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(telemetry.WithRequestID(r.Context(), "req-123"))

	WriteError(w, r, Conflict("stok habis"))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "CONFLICT" || body.Error.RequestID != "req-123" {
		t.Fatalf("body = %+v", body)
	}
}

func TestWriteErrorUnknownBecomesInternal(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	WriteError(w, r, errors.New("boom"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	var got string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = telemetry.RequestID(r.Context())
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got == "" {
		t.Fatal("request id kosong")
	}
	if w.Header().Get(telemetry.HeaderRequestID) != got {
		t.Fatal("header request id tidak cocok")
	}
}

func TestReady(t *testing.T) {
	ok := Ready(NamedCheck{Name: "db", Check: func(ctx context.Context) error { return nil }})
	w := httptest.NewRecorder()
	ok(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	bad := Ready(NamedCheck{Name: "db", Check: func(ctx context.Context) error { return errors.New("down") }})
	w = httptest.NewRecorder()
	bad(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
