package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "saybu/gen/saybu/auth/v1"
	"saybu/services/gateway/internal/authmw"
	"saybu/services/gateway/internal/ratelimit"
)

type fakeAuth struct {
	register func(context.Context, *authv1.RegisterRequest) (*authv1.AuthResponse, error)
	login    func(context.Context, *authv1.LoginRequest) (*authv1.AuthResponse, error)
	refresh  func(context.Context, *authv1.RefreshRequest) (*authv1.AuthResponse, error)
	logout   func(context.Context, *authv1.LogoutRequest) (*emptypb.Empty, error)
	getUser  func(context.Context, *authv1.GetUserRequest) (*authv1.User, error)
	list     func(context.Context, *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error)
}

func (f *fakeAuth) Register(ctx context.Context, in *authv1.RegisterRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return f.register(ctx, in)
}

func (f *fakeAuth) Login(ctx context.Context, in *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return f.login(ctx, in)
}

func (f *fakeAuth) Refresh(ctx context.Context, in *authv1.RefreshRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return f.refresh(ctx, in)
}

func (f *fakeAuth) Logout(ctx context.Context, in *authv1.LogoutRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.logout(ctx, in)
}

func (f *fakeAuth) GetUser(ctx context.Context, in *authv1.GetUserRequest, _ ...grpc.CallOption) (*authv1.User, error) {
	return f.getUser(ctx, in)
}

func (f *fakeAuth) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, _ ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return f.list(ctx, in)
}

func newHandler(client AuthService) *Auth {
	return NewAuth(client, slog.New(slog.NewTextHandler(io.Discard, nil)), CookieConfig{Name: "saybu_rt"}, nil, nil)
}

func sampleAuthResponse() *authv1.AuthResponse {
	return &authv1.AuthResponse{
		User: &authv1.User{
			Id: "u-1", Name: "Sari", Phone: "0812", Email: "sari@example.com", Role: "customer",
		},
		AccessToken:      "access-token",
		ExpiresIn:        900,
		RefreshToken:     "refresh-token",
		RefreshExpiresIn: 2592000,
	}
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body bukan JSON: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestRegisterSuccess(t *testing.T) {
	var gotReq *authv1.RegisterRequest
	h := newHandler(&fakeAuth{
		register: func(_ context.Context, in *authv1.RegisterRequest) (*authv1.AuthResponse, error) {
			gotReq = in
			return sampleAuthResponse(), nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"name":"Sari","phone":"0812","email":"sari@example.com","password":"rahasia123"}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if gotReq.GetName() != "Sari" || gotReq.GetEmail() != "sari@example.com" {
		t.Fatalf("request = %+v", gotReq)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "saybu_rt" || cookies[0].Value != "refresh-token" {
		t.Fatalf("cookies = %+v", cookies)
	}
	if !cookies[0].HttpOnly {
		t.Fatal("cookie refresh harus HttpOnly")
	}

	body := decodeBody(t, rec)
	data, _ := body["data"].(map[string]any)
	if data["access_token"] != "access-token" {
		t.Fatalf("data = %+v", data)
	}
	if _, ok := data["refresh_token"]; ok {
		t.Fatal("refresh token tidak boleh ada di body")
	}
}

func TestRegisterRejectsInvalidJSON(t *testing.T) {
	h := newHandler(&fakeAuth{})

	tests := []string{
		`{`,
		`{"name":"Sari","tidak_dikenal":true}`,
		`{"name":"Sari"}{"name":"Lain"}`,
	}

	for _, payload := range tests {
		rec := httptest.NewRecorder()
		h.Register(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(payload)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("payload %q: status = %d, want 400", payload, rec.Code)
		}
	}
}

func TestRegisterMapsGRPCError(t *testing.T) {
	st := status.New(codes.InvalidArgument, "Data tidak valid.")
	st, err := st.WithDetails(&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{
		{Field: "email", Description: "format email salah"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	h := newHandler(&fakeAuth{
		register: func(context.Context, *authv1.RegisterRequest) (*authv1.AuthResponse, error) {
			return nil, st.Err()
		},
	})

	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"x"}`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := decodeBody(t, rec)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Fatalf("error = %+v", errObj)
	}
	details, _ := errObj["details"].([]any)
	if len(details) != 1 {
		t.Fatalf("details = %+v", details)
	}
}

func TestLoginSuccess(t *testing.T) {
	var got *authv1.LoginRequest
	h := newHandler(&fakeAuth{
		login: func(_ context.Context, in *authv1.LoginRequest) (*authv1.AuthResponse, error) {
			got = in
			return sampleAuthResponse(), nil
		},
	})

	rec := httptest.NewRecorder()
	h.Login(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"identifier":"0812","password":"rahasia123"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got.GetIdentifier() != "0812" {
		t.Fatalf("request = %+v", got)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Fatal("cookie refresh harus diset")
	}
}

func TestLoginRateLimited(t *testing.T) {
	body := `{"identifier":"0812","password":"salah"}`

	h := NewAuth(&fakeAuth{
		login: func(context.Context, *authv1.LoginRequest) (*authv1.AuthResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "Kredensial salah.")
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), CookieConfig{}, nil, ratelimit.New(1, time.Minute))

	first := httptest.NewRecorder()
	h.Login(first, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body)))
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("percobaan pertama: status = %d, want 401", first.Code)
	}

	second := httptest.NewRecorder()
	h.Login(second, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body)))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("percobaan kedua: status = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("header Retry-After wajib ada")
	}
}

func TestRefresh(t *testing.T) {
	t.Run("tanpa cookie", func(t *testing.T) {
		h := newHandler(&fakeAuth{})
		rec := httptest.NewRecorder()
		h.Refresh(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("sukses merotasi cookie", func(t *testing.T) {
		var got string
		h := newHandler(&fakeAuth{
			refresh: func(_ context.Context, in *authv1.RefreshRequest) (*authv1.AuthResponse, error) {
				got = in.GetRefreshToken()
				return sampleAuthResponse(), nil
			},
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "saybu_rt", Value: "lama"})
		rec := httptest.NewRecorder()
		h.Refresh(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if got != "lama" {
			t.Fatalf("refresh token dikirim = %q", got)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Value != "refresh-token" {
			t.Fatalf("cookies = %+v", cookies)
		}
	})

	t.Run("token tidak valid menghapus cookie", func(t *testing.T) {
		h := newHandler(&fakeAuth{
			refresh: func(context.Context, *authv1.RefreshRequest) (*authv1.AuthResponse, error) {
				return nil, status.Error(codes.Unauthenticated, "Sesi tidak valid.")
			},
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "saybu_rt", Value: "lama"})
		rec := httptest.NewRecorder()
		h.Refresh(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
			t.Fatalf("cookie harus dihapus: %+v", cookies)
		}
	})
}

func TestLogout(t *testing.T) {
	var got string
	h := newHandler(&fakeAuth{
		logout: func(_ context.Context, in *authv1.LogoutRequest) (*emptypb.Empty, error) {
			got = in.GetRefreshToken()
			return &emptypb.Empty{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "saybu_rt", Value: "lama"})
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got != "lama" {
		t.Fatalf("refresh token = %q", got)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("cookie harus dihapus: %+v", cookies)
	}
}

func TestMe(t *testing.T) {
	t.Run("tanpa identity", func(t *testing.T) {
		h := newHandler(&fakeAuth{})
		rec := httptest.NewRecorder()
		h.Me(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("sukses", func(t *testing.T) {
		h := newHandler(&fakeAuth{
			getUser: func(_ context.Context, in *authv1.GetUserRequest) (*authv1.User, error) {
				if in.GetId() != "u-1" {
					t.Fatalf("id = %q", in.GetId())
				}
				return &authv1.User{Id: "u-1", Name: "Sari", Role: "customer"}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req = req.WithContext(authmw.WithIdentity(req.Context(), authmw.Identity{UserID: "u-1", Role: "customer"}))
		rec := httptest.NewRecorder()
		h.Me(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		data, _ := decodeBody(t, rec)["data"].(map[string]any)
		if data["name"] != "Sari" {
			t.Fatalf("data = %+v", data)
		}
	})
}

func TestListUsers(t *testing.T) {
	h := newHandler(&fakeAuth{
		list: func(_ context.Context, in *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
			if in.GetPage() != 2 || in.GetLimit() != 5 || in.GetQuery() != "sari" {
				t.Fatalf("request = %+v", in)
			}
			return &authv1.ListUsersResponse{
				Users: []*authv1.User{{Id: "u-1", Name: "Sari"}, {Id: "u-2", Name: "Budi"}},
				Page:  2, Limit: 5, Total: 12,
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.ListUsers(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=2&limit=5&q=sari", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	meta, _ := body["meta"].(map[string]any)
	if meta["total_pages"] != float64(3) {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestWriteGRPCErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid argument", err: status.Error(codes.InvalidArgument, "salah"), wantStatus: 400, wantCode: "VALIDATION_ERROR"},
		{name: "unauthenticated", err: status.Error(codes.Unauthenticated, "masuk dulu"), wantStatus: 401, wantCode: "UNAUTHORIZED"},
		{name: "permission denied", err: status.Error(codes.PermissionDenied, "tidak boleh"), wantStatus: 403, wantCode: "FORBIDDEN"},
		{name: "not found", err: status.Error(codes.NotFound, "tidak ada"), wantStatus: 404, wantCode: "NOT_FOUND"},
		{name: "already exists", err: status.Error(codes.AlreadyExists, "sudah ada"), wantStatus: 409, wantCode: "CONFLICT"},
		{name: "failed precondition", err: status.Error(codes.FailedPrecondition, "stok habis"), wantStatus: 422, wantCode: "BUSINESS_RULE"},
		{name: "resource exhausted", err: status.Error(codes.ResourceExhausted, "pelan"), wantStatus: 429, wantCode: "RATE_LIMITED"},
		{name: "unavailable", err: status.Error(codes.Unavailable, "down"), wantStatus: 503, wantCode: "UNAVAILABLE"},
		{name: "internal", err: status.Error(codes.Internal, "boom"), wantStatus: 500, wantCode: "INTERNAL"},
		{name: "bukan grpc", err: errors.New("boom"), wantStatus: 500, wantCode: "INTERNAL"},
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteGRPCError(rec, httptest.NewRequest(http.MethodGet, "/", nil), log, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			errObj, _ := decodeBody(t, rec)["error"].(map[string]any)
			if errObj["code"] != tt.wantCode {
				t.Fatalf("code = %v, want %s", errObj["code"], tt.wantCode)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name   string
		xff    string
		remote string
		want   string
	}{
		{name: "xff tunggal", xff: "203.0.113.5", remote: "10.0.0.1:1234", want: "203.0.113.5"},
		{name: "xff rantai", xff: "203.0.113.5, 10.0.0.9", remote: "10.0.0.1:1234", want: "203.0.113.5"},
		{name: "tanpa xff", remote: "10.0.0.1:1234", want: "10.0.0.1"},
		{name: "remote tanpa port", remote: "10.0.0.1", want: "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(req); got != tt.want {
				t.Fatalf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQueryInt(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?page=3&limit=0&x=abc", nil)
	if got := queryInt(req, "page", 1); got != 3 {
		t.Fatalf("page = %d", got)
	}
	if got := queryInt(req, "limit", 20); got != 20 {
		t.Fatalf("limit = %d", got)
	}
	if got := queryInt(req, "x", 7); got != 7 {
		t.Fatalf("x = %d", got)
	}
	if got := queryInt(req, "missing", 9); got != 9 {
		t.Fatalf("missing = %d", got)
	}
}
