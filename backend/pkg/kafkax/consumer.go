package kafkax

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"saybu/pkg/telemetry"
)

// Handler memproses satu event. Error yang dikembalikan akan di-retry.
type Handler func(ctx context.Context, ev Event) error

// Consumer membaca topic dengan consumer group, retry 3x, lalu DLQ.
type Consumer struct {
	client      *kgo.Client
	log         *slog.Logger
	handler     Handler
	group       string
	maxAttempts int
}

// NewConsumer membuat consumer. Handler harus idempotent.
func NewConsumer(brokers []string, group string, topics []string, handler Handler, log *slog.Logger) (*Consumer, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka: brokers kosong")
	}
	if group == "" {
		return nil, fmt.Errorf("kafka: consumer group kosong")
	}
	if len(topics) == 0 {
		return nil, fmt.Errorf("kafka: topics kosong")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}
	return &Consumer{client: client, log: log, handler: handler, group: group, maxAttempts: 3}, nil
}

// Run membaca event sampai context dibatalkan.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if err := ctx.Err(); err != nil {
			return nil
		}
		for _, fe := range fetches.Errors() {
			c.log.Error("kafka fetch error", "topic", fe.Topic, "error", fe.Err)
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()
			if err := c.process(ctx, rec); err != nil {
				if ctx.Err() != nil {
					// Context berhenti: jangan commit, biar diproses ulang setelah restart.
					return nil
				}
				return err
			}
			if err := c.client.CommitRecords(ctx, rec); err != nil && ctx.Err() == nil {
				c.log.Error("kafka commit error", "topic", rec.Topic, "offset", rec.Offset, "error", err)
			}
		}
	}
}

func (c *Consumer) process(ctx context.Context, rec *kgo.Record) error {
	ev, err := Unmarshal(rec.Value)
	if err != nil {
		c.log.Error("kafka invalid event", "topic", rec.Topic, "error", err)
		return c.toDLQ(ctx, rec, err)
	}

	// Lanjutkan jejak request: pakai correlation_id dari producer, atau
	// event_id sebagai cadangan bila event lama belum memilikinya.
	corr := ev.CorrelationID
	if corr == "" {
		corr = ev.EventID
	}
	ctx = telemetry.WithRequestID(ctx, corr)

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		lastErr = c.handler(ctx, ev)
		if lastErr == nil {
			return nil
		}
		c.log.Warn("kafka handler gagal",
			"request_id", telemetry.RequestID(ctx),
			"topic", rec.Topic, "event_id", ev.EventID, "type", ev.Type,
			"attempt", attempt, "error", lastErr)

		if attempt == c.maxAttempts {
			break
		}
		backoff := time.Duration(1<<(attempt-1)) * time.Second
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return c.toDLQ(ctx, rec, lastErr)
}

func (c *Consumer) toDLQ(ctx context.Context, rec *kgo.Record, cause error) error {
	dlq := rec.Topic + ".dlq"
	headers := append([]kgo.RecordHeader{}, rec.Headers...)
	headers = append(headers,
		kgo.RecordHeader{Key: "x-error", Value: []byte(cause.Error())},
		kgo.RecordHeader{Key: "x-original-topic", Value: []byte(rec.Topic)},
	)
	dlqRec := &kgo.Record{Topic: dlq, Key: rec.Key, Value: rec.Value, Headers: headers}
	if err := c.client.ProduceSync(ctx, dlqRec).FirstErr(); err != nil {
		c.log.Error("kafka DLQ publish gagal", "topic", dlq, "error", err)
		return err
	}
	c.log.Error("kafka event masuk DLQ", "topic", dlq, "event_id", eventID(rec.Value), "error", cause)
	return nil
}

func eventID(b []byte) string {
	ev, err := Unmarshal(b)
	if err != nil {
		return ""
	}
	return ev.EventID
}

// Close menutup consumer.
func (c *Consumer) Close() { c.client.Close() }

// Ping memeriksa koneksi ke broker (untuk readiness).
func (c *Consumer) Ping(ctx context.Context) error { return c.client.Ping(ctx) }

// Lag mengembalikan lag (jumlah pesan tertinggal) per topic untuk consumer group ini.
func (c *Consumer) Lag(ctx context.Context) (map[string]int64, error) {
	adm := kadm.NewClient(c.client)
	lags, err := adm.Lag(ctx, c.group)
	if err != nil {
		return nil, err
	}
	dl, ok := lags[c.group]
	if !ok {
		return nil, fmt.Errorf("kafka: grup %s tidak ditemukan", c.group)
	}
	if err := dl.Error(); err != nil {
		return nil, err
	}
	out := make(map[string]int64)
	for topic, tl := range dl.Lag.TotalByTopic() {
		out[topic] = tl.Lag
	}
	return out, nil
}

// LagReporter menerima lag consumer (dipenuhi *telemetry.Metrics).
type LagReporter interface {
	SetConsumerLag(group, topic string, lag int64)
}

// ReportLagLoop menghitung lag consumer secara berkala dan memperbarui metrik
// sampai context dibatalkan.
func (c *Consumer) ReportLagLoop(ctx context.Context, reporter LagReporter, interval time.Duration) {
	if reporter == nil {
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		c.reportLag(ctx, reporter)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *Consumer) reportLag(ctx context.Context, reporter LagReporter) {
	lags, err := c.Lag(ctx)
	if err != nil {
		if ctx.Err() == nil {
			c.log.Warn("kafka lag tidak dapat dihitung", "group", c.group, "error", err)
		}
		return
	}
	for topic, lag := range lags {
		reporter.SetConsumerLag(c.group, topic, lag)
	}
}
