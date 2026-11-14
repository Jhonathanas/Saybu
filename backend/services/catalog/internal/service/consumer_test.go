package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"saybu/pkg/kafkax"
	"saybu/services/catalog/internal/domain"
)

func orderEvent(eventType, eventID, orderID string) kafkax.Event {
	payload, _ := json.Marshal(map[string]any{"order_id": orderID, "code": "SB-1"})
	return kafkax.Event{EventID: eventID, Type: eventType, Payload: payload}
}

func TestHandleOrderEventReleasesStock(t *testing.T) {
	s, r := newService(t)

	for _, eventType := range []string{"order.cancelled", "order.expired"} {
		if err := s.HandleOrderEvent(context.Background(), orderEvent(eventType, "ev-"+eventType, "o-1")); err != nil {
			t.Fatalf("%s: %v", eventType, err)
		}
	}
	if len(r.releaseOnce) != 2 {
		t.Fatalf("release tidak dipanggil: %+v", r.releaseOnce)
	}
	if r.releaseOnce[0] != "ev-order.cancelled:o-1" {
		t.Fatalf("argumen salah: %+v", r.releaseOnce)
	}
}

func TestHandleOrderEventIgnoresOtherTypes(t *testing.T) {
	s, r := newService(t)

	if err := s.HandleOrderEvent(context.Background(), orderEvent("order.created", "ev-1", "o-1")); err != nil {
		t.Fatal(err)
	}
	if len(r.releaseOnce) != 0 {
		t.Fatalf("order.created tidak boleh melepas stok: %+v", r.releaseOnce)
	}
}

func TestHandleOrderEventBadPayload(t *testing.T) {
	s, _ := newService(t)

	ev := kafkax.Event{EventID: "ev-1", Type: "order.cancelled", Payload: json.RawMessage(`not-json`)}
	if err := s.HandleOrderEvent(context.Background(), ev); err == nil {
		t.Fatal("payload rusak harus menghasilkan error")
	}
}

func TestHandleOrderEventMissingOrderID(t *testing.T) {
	s, _ := newService(t)

	ev := kafkax.Event{EventID: "ev-1", Type: "order.expired", Payload: json.RawMessage(`{}`)}
	if err := s.HandleOrderEvent(context.Background(), ev); err == nil {
		t.Fatal("order_id kosong harus menghasilkan error")
	}
}

func TestReleaseStockForOrderValidates(t *testing.T) {
	s, _ := newService(t)

	if err := s.ReleaseStockForOrder(context.Background(), "", "o-1"); err == nil {
		t.Fatal("event_id kosong harus error")
	}
	if err := s.ReleaseStockForOrder(context.Background(), "ev-1", ""); err == nil {
		t.Fatal("order_id kosong harus error")
	}
	var ve *domain.ValidationError
	if err := s.ReleaseStockForOrder(context.Background(), "", "o-1"); !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}
