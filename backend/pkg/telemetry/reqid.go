// Package telemetry menyediakan request-id dan metrik Prometheus.
package telemetry

import (
	"context"

	"saybu/pkg/ids"
)

// HeaderRequestID adalah header yang dipakai lintas HTTP dan gRPC metadata.
const HeaderRequestID = "X-Request-Id"

type ctxKey struct{}

// NewRequestID membuat request id baru (UUIDv7).
func NewRequestID() string { return ids.New() }

// WithRequestID menyimpan request id di context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID mengambil request id dari context (kosong bila tidak ada).
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}
