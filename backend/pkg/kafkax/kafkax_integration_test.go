//go:build integration

package kafkax

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

// TestProducerConsumerIntegration membutuhkan Kafka yang berjalan.
//
//	make test-integration
func TestProducerConsumerIntegration(t *testing.T) {
	raw := os.Getenv("KAFKA_BROKERS")
	if raw == "" {
		t.Skip("KAFKA_BROKERS tidak diset")
	}
	brokers := strings.Split(raw, ",")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	topic := fmt.Sprintf("saybu.test.%d", time.Now().UnixNano())
	group := topic + ".group"

	producer, err := NewProducer(brokers)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()

	received := make(chan Event, 1)
	consumer, err := NewConsumer(brokers, group, []string{topic}, func(_ context.Context, ev Event) error {
		received <- ev
		return nil
	}, log)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() { _ = consumer.Run(ctx) }()

	ev, err := NewEvent("test.event", "agg-1", map[string]string{"hello": "world"}, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Beri waktu consumer join group, lalu publish.
	time.Sleep(2 * time.Second)
	if err := producer.Publish(ctx, topic, "agg-1", ev); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-received:
		if got.EventID != ev.EventID {
			t.Fatalf("event id = %s, want %s", got.EventID, ev.EventID)
		}
	case <-ctx.Done():
		t.Fatal("timeout menunggu event")
	}
}
