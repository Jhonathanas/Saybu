// Package handlers berisi handler REST gateway.
package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "saybu/gen/saybu/auth/v1"
	"saybu/pkg/httpx"
	"saybu/services/gateway/internal/authmw"
	"saybu/services/gateway/internal/ratelimit"
)

// AuthService adalah bagian klien gRPC auth yang dipakai gateway.
type AuthService interface {
	Register(ctx context.Context, in *authv1.RegisterRequest, opts ...grpc.CallOption) (*authv1.AuthResponse, error)
	Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.AuthResponse, error)
	Refresh(ctx context.Context, in *authv1.RefreshRequest, opts ...grpc.CallOption) (*authv1.AuthResponse, error)
	Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.User, error)
	ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error)
}

// CookieConfig konfigurasi cookie refresh token.
type CookieConfig struct {
	Name   string
	Path   string
	Secure bool
}

// Auth menangani endpoint /auth/* dan /me.
type Auth struct {
	client        AuthService
	log           *slog.Logger
	cookie        CookieConfig
	registerLimit *ratelimit.Limiter
	loginLimit    *ratelimit.Limiter
}

// NewAuth membuat handler auth.
func NewAuth(client AuthService, log *slog.Logger, cookie CookieConfig, registerLimit, loginLimit *ratelimit.Limiter) *Auth {
	if cookie.Name == "" {
		cookie.Name = "saybu_rt"
	}
	if cookie.Path == "" {
		cookie.Path = "/api/v1/auth"
	}
	return &Auth{
		client:        client,
		log:           log,
		cookie:        cookie,
		registerLimit: registerLimit,
		loginLimit:    loginLimit,
	}
}

type registerBody struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginBody struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type userJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at,omitempty"`
}

type authData struct {
	User        userJSON `json:"user"`
	AccessToken string   `json:"access_token"`
	ExpiresIn   int64    `json:"expires_in"`
}

// Register menangani POST /api/v1/auth/register.
func (h *Auth) Register(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, r, h.registerLimit, clientIP(r)) {
		return
	}

	var body registerBody
	if !decodeJSON(w, r, &body) {
		return
	}

	res, err := h.client.Register(outgoingContext(r), &authv1.RegisterRequest{
		Name:     body.Name,
		Phone:    body.Phone,
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}

	h.setRefreshCookie(w, res.GetRefreshToken(), res.GetRefreshExpiresIn())
	httpx.Data(w, http.StatusCreated, toAuthData(res))
}

// Login menangani POST /api/v1/auth/login.
func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if !decodeJSON(w, r, &body) {
		return
	}

	key := clientIP(r) + "|" + strings.ToLower(strings.TrimSpace(body.Identifier))
	if !h.allow(w, r, h.loginLimit, key) {
		return
	}

	res, err := h.client.Login(outgoingContext(r), &authv1.LoginRequest{
		Identifier: body.Identifier,
		Password:   body.Password,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}

	h.setRefreshCookie(w, res.GetRefreshToken(), res.GetRefreshExpiresIn())
	httpx.Data(w, http.StatusOK, toAuthData(res))
}

// Refresh menangani POST /api/v1/auth/refresh (memakai cookie).
func (h *Auth) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cookie.Name)
	if err != nil || cookie.Value == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("Sesi tidak ditemukan. Silakan masuk lagi."))
		return
	}

	res, err := h.client.Refresh(outgoingContext(r), &authv1.RefreshRequest{RefreshToken: cookie.Value})
	if err != nil {
		h.clearRefreshCookie(w)
		WriteGRPCError(w, r, h.log, err)
		return
	}

	h.setRefreshCookie(w, res.GetRefreshToken(), res.GetRefreshExpiresIn())
	httpx.Data(w, http.StatusOK, toAuthData(res))
}

// Logout menangani POST /api/v1/auth/logout.
func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(h.cookie.Name); err == nil && cookie.Value != "" {
		if _, err := h.client.Logout(outgoingContext(r), &authv1.LogoutRequest{RefreshToken: cookie.Value}); err != nil {
			h.log.Warn("logout gRPC gagal", "error", err)
		}
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me menangani GET /api/v1/me.
func (h *Auth) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := authmw.IdentityFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.Unauthorized("Token akses diperlukan."))
		return
	}

	user, err := h.client.GetUser(r.Context(), &authv1.GetUserRequest{Id: id.UserID})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toUserJSON(user))
}

// ListUsers menangani GET /api/v1/admin/users.
func (h *Auth) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)

	res, err := h.client.ListUsers(r.Context(), &authv1.ListUsersRequest{
		Page:  int32(page),
		Limit: int32(limit),
		Query: r.URL.Query().Get("q"),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}

	users := make([]userJSON, 0, len(res.GetUsers()))
	for _, u := range res.GetUsers() {
		users = append(users, toUserJSON(u))
	}

	totalPages := int32(0)
	if res.GetLimit() > 0 {
		totalPages = int32(math.Ceil(float64(res.GetTotal()) / float64(res.GetLimit())))
	}

	httpx.DataMeta(w, http.StatusOK, users, map[string]any{
		"page":        res.GetPage(),
		"limit":       res.GetLimit(),
		"total":       res.GetTotal(),
		"total_pages": totalPages,
	})
}

func (h *Auth) setRefreshCookie(w http.ResponseWriter, value string, maxAge int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    value,
		Path:     h.cookie.Path,
		MaxAge:   int(maxAge),
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Auth) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     h.cookie.Path,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Auth) allow(w http.ResponseWriter, r *http.Request, limiter *ratelimit.Limiter, key string) bool {
	return allowRate(w, r, limiter, key)
}

// allowRate membatasi laju per key; menulis 429 dan mengembalikan false bila ditolak.
func allowRate(w http.ResponseWriter, r *http.Request, limiter *ratelimit.Limiter, key string) bool {
	if limiter == nil {
		return true
	}
	ok, retryAfter := limiter.Allow(key)
	if ok {
		return true
	}
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	httpx.WriteError(w, r, httpx.New(http.StatusTooManyRequests, "RATE_LIMITED",
		"Terlalu banyak percobaan. Coba lagi nanti."))
	return false
}

func toAuthData(res *authv1.AuthResponse) authData {
	return authData{
		User:        toUserJSON(res.GetUser()),
		AccessToken: res.GetAccessToken(),
		ExpiresIn:   res.GetExpiresIn(),
	}
}

func toUserJSON(u *authv1.User) userJSON {
	return userJSON{
		ID:        u.GetId(),
		Name:      u.GetName(),
		Phone:     u.GetPhone(),
		Email:     u.GetEmail(),
		Role:      u.GetRole(),
		CreatedAt: u.GetCreatedAt(),
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		httpx.WriteError(w, r, httpx.BadRequest("Body JSON tidak valid."))
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		httpx.WriteError(w, r, httpx.BadRequest("Body JSON hanya boleh berisi satu objek."))
		return false
	}
	return true
}

func queryInt(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func clientIP(r *http.Request) string {
	return httpx.ClientIP(r)
}

func outgoingContext(r *http.Request) context.Context {
	ip := clientIP(r)
	return metadata.AppendToOutgoingContext(r.Context(), "x-forwarded-for", ip, "x-real-ip", ip)
}
