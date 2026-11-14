// Package authmw menyediakan middleware autentikasi JWT untuk gateway.
package authmw

import (
	"context"
	"net/http"
	"strings"

	"saybu/pkg/authn"
	"saybu/pkg/httpx"
)

type ctxKey struct{}

// Identity adalah user yang sudah terautentikasi.
type Identity struct {
	UserID string
	Role   string
}

// WithIdentity menyimpan identity di context.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// IdentityFrom mengambil identity dari context.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Authenticate memvalidasi header Authorization: Bearer <token>.
func Authenticate(tokens *authn.Manager) func(http.Handler) http.Handler {
	const prefix = "Bearer "

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(header, prefix) {
				httpx.WriteError(w, r, httpx.Unauthorized("Token akses diperlukan."))
				return
			}
			raw := strings.TrimSpace(strings.TrimPrefix(header, prefix))
			if raw == "" {
				httpx.WriteError(w, r, httpx.Unauthorized("Token akses diperlukan."))
				return
			}

			claims, err := tokens.Parse(raw)
			if err != nil {
				httpx.WriteError(w, r, httpx.Unauthorized("Sesi tidak valid. Silakan masuk lagi."))
				return
			}

			ctx := WithIdentity(r.Context(), Identity{UserID: claims.Subject, Role: claims.Role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Optional mencoba memvalidasi token bila ada, tetapi tidak menolak request
// tanpa token. Berguna untuk endpoint guest (checkout, pelacakan) yang tetap
// mengenali pengguna yang login.
func Optional(tokens *authn.Manager) func(http.Handler) http.Handler {
	const prefix = "Bearer "

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(header, prefix) {
				raw := strings.TrimSpace(strings.TrimPrefix(header, prefix))
				if raw != "" {
					if claims, err := tokens.Parse(raw); err == nil {
						r = r.WithContext(WithIdentity(r.Context(),
							Identity{UserID: claims.Subject, Role: claims.Role}))
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole memastikan user punya salah satu role yang diizinkan.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.Unauthorized("Token akses diperlukan."))
				return
			}
			if !allowed[id.Role] {
				httpx.WriteError(w, r, httpx.Forbidden("Anda tidak punya akses ke sumber daya ini."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
