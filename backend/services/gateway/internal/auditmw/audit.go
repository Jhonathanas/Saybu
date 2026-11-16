// Package auditmw mencatat aksi admin yang berhasil sebagai audit log
// terstruktur. Ini melengkapi (bukan menggantikan) riwayat perubahan status
// pesanan di order_status_history.
package auditmw

import (
	"log/slog"
	"net/http"

	"saybu/pkg/telemetry"
	"saybu/services/gateway/internal/authmw"
)

// Middleware mencatat satu log audit untuk setiap mutasi admin yang berhasil
// (POST/PATCH/DELETE) dengan actor, aksi, path, status, dan request_id.
func Middleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			if !isMutation(r.Method) || rec.status < 200 || rec.status >= 300 {
				return
			}
			actor, _ := authmw.IdentityFrom(r.Context())
			log.Info("admin action",
				"audit", true,
				"actor_id", actor.UserID,
				"actor_role", actor.Role,
				"action", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"request_id", telemetry.RequestID(r.Context()),
			)
		})
	}
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
