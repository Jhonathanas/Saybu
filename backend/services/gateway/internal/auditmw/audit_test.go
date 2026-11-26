package auditmw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "saybu/gen/saybu/auth/v1"
	"saybu/pkg/telemetry"
	"saybu/services/gateway/internal/authmw"
)

type fakeRecorder struct {
	calls []*authv1.RecordAuditRequest
	err   error
}

func (f *fakeRecorder) RecordAudit(_ context.Context, req *authv1.RecordAuditRequest, _ ...googlegrpc.CallOption) (*emptypb.Empty, error) {
	f.calls = append(f.calls, req)
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

// capture menjalankan handler melalui middleware dan mengembalikan log JSON
// terakhir yang tercatat (atau nil bila tidak ada) beserta recorder-nya.
func capture(t *testing.T, method string, status int) (map[string]any, *fakeRecorder) {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	recorder := &fakeRecorder{}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
	h := Middleware(log, recorder)(next)

	req := httptest.NewRequest(method, "/api/v1/admin/orders/o-1/status", nil)
	ctx := telemetry.WithRequestID(req.Context(), "req-123")
	ctx = authmw.WithIdentity(ctx, authmw.Identity{UserID: "u-admin", Role: "admin"})
	h.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	line := bytes.TrimSpace(buf.Bytes())
	if len(line) == 0 {
		return nil, recorder
	}
	// Ambil baris terakhir (satu log per request).
	if i := bytes.LastIndexByte(line, '\n'); i >= 0 {
		line = line[i+1:]
	}
	var out map[string]any
	if err := json.Unmarshal(line, &out); err != nil {
		t.Fatalf("log bukan JSON valid: %s", line)
	}
	return out, recorder
}

func TestMiddlewareLogsSuccessfulMutation(t *testing.T) {
	rec, _ := capture(t, http.MethodPatch, http.StatusOK)
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
	if rec, _ := capture(t, http.MethodGet, http.StatusOK); rec != nil {
		t.Fatalf("GET tidak boleh dicatat: %+v", rec)
	}
}

func TestMiddlewareSkipsFailedMutations(t *testing.T) {
	if rec, _ := capture(t, http.MethodPatch, http.StatusConflict); rec != nil {
		t.Fatalf("mutasi gagal tidak boleh dicatat: %+v", rec)
	}
}

func TestMiddlewarePersistsSuccessfulMutation(t *testing.T) {
	_, recorder := capture(t, http.MethodDelete, http.StatusNoContent)
	if len(recorder.calls) != 1 {
		t.Fatalf("RecordAudit calls = %d, want 1", len(recorder.calls))
	}
	got := recorder.calls[0]
	if got.GetActorId() != "u-admin" || got.GetActorRole() != "admin" || got.GetAction() != http.MethodDelete {
		t.Fatalf("req = %+v", got)
	}
	if got.GetPath() != "/api/v1/admin/orders/o-1/status" || got.GetStatus() != 204 || got.GetRequestId() != "req-123" {
		t.Fatalf("req = %+v", got)
	}
}

func TestMiddlewareSkipsPersistenceForFailedMutation(t *testing.T) {
	_, recorder := capture(t, http.MethodPost, http.StatusBadRequest)
	if len(recorder.calls) != 0 {
		t.Fatalf("mutasi gagal tidak boleh dipersisten: %+v", recorder.calls)
	}
}

func TestMiddlewareKeepsResponseWhenPersistenceFails(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	recorder := &fakeRecorder{err: errors.New("auth down")}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", nil)
	ctx := authmw.WithIdentity(req.Context(), authmw.Identity{UserID: "u-admin", Role: "admin"})
	rr := httptest.NewRecorder()
	Middleware(log, recorder)(next).ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !bytes.Contains(buf.Bytes(), []byte("audit persist gagal")) {
		t.Fatalf("kegagalan audit harus dicatat: %s", buf.String())
	}
}
