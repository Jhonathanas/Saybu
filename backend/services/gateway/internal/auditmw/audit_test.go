package auditmw

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"saybu/pkg/telemetry"
	"saybu/services/gateway/internal/authmw"
)

// capture menjalankan handler melalui middleware dan mengembalikan log JSON
// terakhir yang tercatat (atau nil bila tidak ada).
func capture(t *testing.T, method string, status int) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
	h := Middleware(log)(next)

	req := httptest.NewRequest(method, "/api/v1/admin/orders/o-1/status", nil)
	ctx := telemetry.WithRequestID(req.Context(), "req-123")
	ctx = authmw.WithIdentity(ctx, authmw.Identity{UserID: "u-admin", Role: "admin"})
	h.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	line := bytes.TrimSpace(buf.Bytes())
	if len(line) == 0 {
		return nil
	}
	// Ambil baris terakhir (satu log per request).
	if i := bytes.LastIndexByte(line, '\n'); i >= 0 {
		line = line[i+1:]
	}
	var out map[string]any
	if err := json.Unmarshal(line, &out); err != nil {
		t.Fatalf("log bukan JSON valid: %s", line)
	}
	return out
}

func TestMiddlewareLogsSuccessfulMutation(t *testing.T) {
	rec := capture(t, http.MethodPatch, http.StatusOK)
	if rec == nil {
		t.Fatal("mutasi sukses harus tercatat")
	}
	if rec["audit"] != true {
		t.Fatalf("audit = %v", rec["audit"])
	}
	if rec["actor_id"] != "u-admin" || rec["actor_role"] != "admin" {
		t.Fatalf("actor = %+v", rec)
	}
	if rec["action"] != http.MethodPatch || rec["status"] != float64(200) {
		t.Fatalf("aksi = %+v", rec)
	}
	if rec["path"] != "/api/v1/admin/orders/o-1/status" {
		t.Fatalf("path = %v", rec["path"])
	}
	if rec["request_id"] != "req-123" {
		t.Fatalf("request_id = %v", rec["request_id"])
	}
}

func TestMiddlewareSkipsReads(t *testing.T) {
	if rec := capture(t, http.MethodGet, http.StatusOK); rec != nil {
		t.Fatalf("GET tidak boleh dicatat: %+v", rec)
	}
}

func TestMiddlewareSkipsFailedMutations(t *testing.T) {
	if rec := capture(t, http.MethodPatch, http.StatusConflict); rec != nil {
		t.Fatalf("mutasi gagal tidak boleh dicatat: %+v", rec)
	}
}
