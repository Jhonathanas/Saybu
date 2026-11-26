// Package auditmw mencatat aksi admin yang berhasil sebagai audit log
// terstruktur dan menyimpannya persisten lewat auth service. Ini melengkapi
// (bukan menggantikan) riwayat perubahan status pesanan di order_status_history.
package auditmw

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "saybu/gen/saybu/auth/v1"
	"saybu/pkg/telemetry"
	"saybu/services/gateway/internal/authmw"
)

// auditTimeout membatasi dampak audit terhadap latency response. Audit adalah
// best-effort: kegagalan penyimpanan tidak boleh menggagalkan mutasi yang sudah
// berhasil, tetapi tetap dicatat di log untuk diagnosis.
const auditTimeout = 3 * time.Second

// Recorder adalah klien gRPC RecordAudit (diimplementasikan auth service client).
type Recorder interface {
	RecordAudit(context.Context, *authv1.RecordAuditRequest, ...googlegrpc.CallOption) (*emptypb.Empty, error)
}

// Middleware mencatat satu log audit untuk setiap mutasi admin yang berhasil
// (POST/PATCH/DELETE) dengan actor, aksi, path, status, dan request_id.
func Middleware(log *slog.Logger, recorder Recorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			if !isMutation(r.Method) || rec.status < 200 || rec.status >= 300 {
				return
			}
			actor, _ := authmw.IdentityFrom(r.Context())
			requestID := telemetry.RequestID(r.Context())
			log.Info("admin action",
				"audit", true,
				"actor_id", actor.UserID,
				"actor_role", actor.Role,
				"action", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"request_id", requestID,
			)
			if recorder == nil {
				return
			}

			// Request context sudah selesai saat audit ditulis; pakai context baru
			// dengan batas waktu agar RPC tetap punya celah saat client putus.
			ctx, cancel := context.WithTimeout(context.Background(), auditTimeout)
			defer cancel()
			_, err := recorder.RecordAudit(ctx, &authv1.RecordAuditRequest{
				ActorId:   actor.UserID,
				ActorRole: actor.Role,
				Action:    r.Method,
				Path:      r.URL.Path,
				Status:    int32(rec.status),
				RequestId: requestID,
			})
			if err != nil {
				log.Error("audit persist gagal", "error", err, "request_id", requestID)
			}
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
