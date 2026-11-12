package kafkax

import (
	"testing"
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
