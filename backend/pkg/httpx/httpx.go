// Package httpx berisi helper HTTP: envelope JSON, error, middleware, health.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"saybu/pkg/telemetry"
)

// Envelope adalah bentuk respons sukses standar.
type Envelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

// JSON menulis respons JSON.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// Data menulis respons sukses dengan envelope { "data": ... }.
func Data(w http.ResponseWriter, status int, data any) {
	JSON(w, status, Envelope{Data: data})
}

// DataMeta menulis respons sukses dengan data dan meta (paginasi).
func DataMeta(w http.ResponseWriter, status int, data, meta any) {
	JSON(w, status, Envelope{Data: data, Meta: meta})
}

// Detail menjelaskan satu kesalahan validasi.
type Detail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error adalah error HTTP dengan kode mesin yang stabil.
type Error struct {
	Status  int
	Code    string
	Message string
	Details []Detail
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap memungkinkan errors.Is/As ke error asli.
func (e *Error) Unwrap() error { return e.Err }

// New membuat Error HTTP.
func New(status int, code, message string, details ...Detail) *Error {
	return &Error{Status: status, Code: code, Message: message, Details: details}
}

// BadRequest (400).
func BadRequest(message string, details ...Detail) *Error {
	return New(http.StatusBadRequest, "VALIDATION_ERROR", message, details...)
}

// Unauthorized (401).
func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden (403).
func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", message)
}

// NotFound (404).
func NotFound(message string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", message)
}

// Conflict (409).
func Conflict(message string) *Error {
	return New(http.StatusConflict, "CONFLICT", message)
}

// Unprocessable (422) untuk pelanggaran aturan bisnis.
func Unprocessable(message string, details ...Detail) *Error {
	return New(http.StatusUnprocessableEntity, "BUSINESS_RULE", message, details...)
}

// Internal (500) membungkus error tak terduga.
func Internal(err error) *Error {
	return &Error{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL",
		Message: "Terjadi kesalahan internal.",
		Err:     err,
	}
}

// WriteError menulis error dalam format standar. Error tak dikenal menjadi 500.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = Internal(err)
	}

	body := map[string]any{
		"error": map[string]any{
			"code":    e.Code,
			"message": e.Message,
		},
	}
	inner := body["error"].(map[string]any)
	if len(e.Details) > 0 {
		inner["details"] = e.Details
	}
	if id := telemetry.RequestID(r.Context()); id != "" {
		inner["request_id"] = id
	}
	JSON(w, e.Status, body)
}

// RequestID menyimpan/meneruskan X-Request-Id.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(telemetry.HeaderRequestID))
		if id == "" {
			id = telemetry.NewRequestID()
		}
		w.Header().Set(telemetry.HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(telemetry.WithRequestID(r.Context(), id)))
	})
}

// Recover mengubah panic menjadi 500 tanpa mematikan server.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						"request_id", telemetry.RequestID(r.Context()),
						"method", r.Method, "path", r.URL.Path, "panic", rec)
					WriteError(w, r, Internal(nil))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Logger mencatat satu baris log per request.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("http request",
				"request_id", telemetry.RequestID(r.Context()),
				"method", r.Method, "path", r.URL.Path,
				"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
		})
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

// CORS mengizinkan origin yang terdaftar.
func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[strings.TrimSpace(o)] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-Id")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
