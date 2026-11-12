// Package kafkax membungkus Kafka (franz-go): event envelope, producer,
// consumer dengan retry dan dead-letter queue.
package kafkax

import (
	"encoding/json"
	"fmt"
	"time"

	"saybu/pkg/ids"
)

// Event adalah envelope standar semua event Saybu.
type Event struct {
	EventID       string          `json:"event_id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	AggregateID   string          `json:"aggregate_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Producer      string          `json:"producer,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// NewEvent membungkus payload menjadi Event dengan id dan waktu sekarang.
func NewEvent(eventType, aggregateID string, payload any, producer string) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal payload: %w", err)
	}
	return Event{
		EventID:     ids.New(),
		Type:        eventType,
		Version:     1,
		OccurredAt:  time.Now().UTC(),
		AggregateID: aggregateID,
		Producer:    producer,
		Payload:     raw,
	}, nil
}

// Marshal mengubah Event menjadi JSON.
func (e Event) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Unmarshal membaca Event dari JSON.
func Unmarshal(b []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(b, &e); err != nil {
		return Event{}, fmt.Errorf("unmarshal event: %w", err)
	}
	if e.EventID == "" {
		return Event{}, fmt.Errorf("event tanpa event_id")
	}
	return e, nil
}
