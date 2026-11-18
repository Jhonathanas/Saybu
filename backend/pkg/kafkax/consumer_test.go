package kafkax

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"saybu/pkg/telemetry"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestProcessSeedsRequestIDFromCorrelation(t *testing.T) {
	var got string
	c := &Consumer{
		log:         discardLogger(),
		maxAttempts: 1,
		handler: func(ctx context.Context, _ Event) error {
			got = telemetry.RequestID(ctx)
			return nil
		},
	}

	ev, err := NewEvent("order.created", "agg-1", map[string]any{"x": 1}, "order-svc")
	if err != nil {
		t.Fatal(err)
	}
	ev.CorrelationID = "corr-xyz"
	raw, err := ev.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.process(context.Background(), &kgo.Record{Topic: "order.events.v1", Value: raw}); err != nil {
		t.Fatal(err)
	}
	if got != "corr-xyz" {
		t.Fatalf("request_id di handler = %q, want corr-xyz", got)
	}
}

func TestProcessFallsBackToEventID(t *testing.T) {
	var got string
	c := &Consumer{
		log:         discardLogger(),
		maxAttempts: 1,
		handler: func(ctx context.Context, _ Event) error {
			got = telemetry.RequestID(ctx)
			return nil
		},
	}

	ev, err := NewEvent("order.created", "agg-1", nil, "order-svc")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ev.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.process(context.Background(), &kgo.Record{Topic: "order.events.v1", Value: raw}); err != nil {
		t.Fatal(err)
	}
	if got != ev.EventID {
		t.Fatalf("request_id di handler = %q, want event_id %q", got, ev.EventID)
	}
}
