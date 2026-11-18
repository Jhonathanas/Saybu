package kafkax

import (
	"context"
	"testing"

	"saybu/pkg/telemetry"
)

func TestEventRoundTrip(t *testing.T) {
	ev, err := NewEvent("order.created", "agg-1", map[string]any{"total": 85000}, "order-svc")
	if err != nil {
		t.Fatal(err)
	}
	if ev.EventID == "" || ev.Version != 1 || ev.OccurredAt.IsZero() {
		t.Fatalf("event tidak lengkap: %+v", ev)
	}

	raw, err := ev.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.EventID != ev.EventID || got.Type != "order.created" || got.AggregateID != "agg-1" {
		t.Fatalf("roundtrip = %+v", got)
	}
	if string(got.Payload) != `{"total":85000}` {
		t.Fatalf("payload = %s", got.Payload)
	}
}

func TestUnmarshalRejectsMissingID(t *testing.T) {
	if _, err := Unmarshal([]byte(`{"type":"x"}`)); err == nil {
		t.Fatal("harus error tanpa event_id")
	}
}

func TestNewEventCtxCarriesCorrelationID(t *testing.T) {
	ctx := telemetry.WithRequestID(context.Background(), "req-123")
	ev, err := NewEventCtx(ctx, "order.created", "agg-1", map[string]any{"total": 1}, "order-svc")
	if err != nil {
		t.Fatal(err)
	}
	if ev.CorrelationID != "req-123" {
		t.Fatalf("correlation_id = %q, want req-123", ev.CorrelationID)
	}

	raw, err := ev.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.CorrelationID != "req-123" {
		t.Fatalf("correlation_id setelah roundtrip = %q", got.CorrelationID)
	}
}

func TestNewEventCtxWithoutRequestID(t *testing.T) {
	ev, err := NewEventCtx(context.Background(), "order.created", "agg-1", nil, "order-svc")
	if err != nil {
		t.Fatal(err)
	}
	if ev.CorrelationID != "" {
		t.Fatalf("correlation_id = %q, want kosong", ev.CorrelationID)
	}
}
