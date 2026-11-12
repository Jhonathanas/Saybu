package kafkax

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer mengirim event ke Kafka dengan idempotent write dan acks=all.
type Producer struct {
	client *kgo.Client
}

// NewProducer membuat producer.
func NewProducer(brokers []string) (*Producer, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka: brokers kosong")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ProducerLinger(10*time.Millisecond),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	return &Producer{client: client}, nil
}

// Publish mengirim Event ke topic dengan key tertentu.
func (p *Producer) Publish(ctx context.Context, topic, key string, ev Event) error {
	value, err := ev.Marshal()
	if err != nil {
		return err
	}
	return p.PublishRaw(ctx, topic, key, value, map[string]string{"event-type": ev.Type})
}

// PublishRaw mengirim payload mentah (dipakai juga untuk DLQ).
func (p *Producer) PublishRaw(ctx context.Context, topic, key string, value []byte, headers map[string]string) error {
	rec := &kgo.Record{Topic: topic, Key: []byte(key), Value: value}
	for k, v := range headers {
		rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	if err := p.client.ProduceSync(ctx, rec).FirstErr(); err != nil {
		return fmt.Errorf("kafka publish %s: %w", topic, err)
	}
	return nil
}

// Ping memeriksa koneksi broker.
func (p *Producer) Ping(ctx context.Context) error {
	return p.client.Ping(ctx)
}

// Close menutup producer.
func (p *Producer) Close() { p.client.Close() }
