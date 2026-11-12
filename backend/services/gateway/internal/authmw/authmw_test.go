package authmw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"saybu/pkg/authn"
)

func newTokens(t *testing.T) *authn.Manager {
	t.Helper()
	m, err := authn.NewManager(strings.Repeat("s", 32), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAuthenticateAcceptsValidToken(t *testing.T) {
	tokens := newTokens(t)
	token, _, err := tokens.NewAccessToken("user-1", "admin")
	if err != nil {
		t.Fatal(err)
	}

	var got Identity
	h := Authenticate(tokens)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := IdentityFrom(r.Context())
		if !ok {
			t.Fatal("identity tidak ada di context")
		}
		got = id
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got.UserID != "user-1" || got.Role != "admin" {
		t.Fatalf("identity = %+v", got)
	}
}

func TestAuthenticateRejects(t *testing.T) {
	tokens := newTokens(t)

	tests := []struct {
		name   string
		header string
	}{
		{name: "tanpa header", header: ""},
		{name: "skema salah", header: "Basic abc"},
		{name: "token kosong", header: "Bearer "},
		{name: "token rusak", header: "Bearer bukan-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Authenticate(tokens)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("handler tidak boleh dipanggil")
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name     string
		identity *Identity
		want     int
	}{
		{name: "admin diizinkan", identity: &Identity{UserID: "u", Role: "admin"}, want: http.StatusNoContent},
		{name: "customer ditolak", identity: &Identity{UserID: "u", Role: "customer"}, want: http.StatusForbidden},
		{name: "tanpa identity", identity: nil, want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			if tt.identity != nil {
				req = req.WithContext(WithIdentity(req.Context(), *tt.identity))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
