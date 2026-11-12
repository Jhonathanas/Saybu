package httpx

import (
	"context"
	"net/http"
	"time"

	"saybu/pkg/telemetry"
)

// Check memeriksa kesiapan satu dependensi.
type Check func(ctx context.Context) error

// NamedCheck adalah Check dengan nama untuk laporan readiness.
type NamedCheck struct {
	Name  string
	Check Check
}

// Live adalah liveness probe: proses hidup.
func Live(w http.ResponseWriter, _ *http.Request) {
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready menjalankan semua check. 503 bila ada yang gagal.
func Ready(checks ...NamedCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		results := make(map[string]string, len(checks))
		ok := true
		for _, c := range checks {
			if err := c.Check(ctx); err != nil {
				results[c.Name] = "error: " + err.Error()
				ok = false
				continue
			}
			results[c.Name] = "ok"
		}

		status := http.StatusOK
		state := "ready"
		if !ok {
			status = http.StatusServiceUnavailable
			state = "not_ready"
		}
		JSON(w, status, map[string]any{"status": state, "checks": results})
	}
}

// NewAdminMux membuat mux admin: /healthz, /readyz, /metrics.
func NewAdminMux(m *telemetry.Metrics, checks ...NamedCheck) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Live)
	mux.Handle("GET /readyz", Ready(checks...))
	if m != nil {
		mux.Handle("GET /metrics", m.Handler())
	}
	return mux
}
