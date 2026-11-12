// Package outbox mengimplementasikan transactional outbox:
// event ditulis di transaksi DB yang sama dengan data bisnis, lalu direlay ke Kafka.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Record adalah satu baris outbox.
type Record struct {
	ID          int64
	EventID     string
	Topic       string
	Key         string
	Payload     []byte
	Headers     map[string]string
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	PublishedAt *time.Time
}

// Store mengakses tabel outbox.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore membuat Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Insert menulis event ke outbox. Wajib dipanggil dalam transaksi yang sama
// dengan perubahan data bisnis.
func (s *Store) Insert(ctx context.Context, tx pgx.Tx, rec Record) error {
	headers, err := json.Marshal(rec.Headers)
	if err != nil {
		return fmt.Errorf("outbox: marshal headers: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox (event_id, topic, key, payload, headers)
		VALUES ($1, $2, $3, $4, $5)
	`, rec.EventID, rec.Topic, rec.Key, string(rec.Payload), string(headers))
	if err != nil {
		return fmt.Errorf("outbox: insert: %w", err)
	}
	return nil
}

// FetchPending mengambil event yang belum terkirim.
// At-least-once: duplikasi setelah crash ditangani idempotensi consumer.
func (s *Store) FetchPending(ctx context.Context, limit int) ([]Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, event_id, topic, key, payload, headers, attempts,
		       COALESCE(last_error, ''), created_at, published_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox: query pending: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		var headers []byte
		if err := rows.Scan(&r.ID, &r.EventID, &r.Topic, &r.Key, &r.Payload, &headers,
			&r.Attempts, &r.LastError, &r.CreatedAt, &r.PublishedAt); err != nil {
			return nil, fmt.Errorf("outbox: scan: %w", err)
		}
		if len(headers) > 0 {
			_ = json.Unmarshal(headers, &r.Headers)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox: rows: %w", err)
	}
	return out, nil
}

// MarkPublished menandai event sudah terkirim.
func (s *Store) MarkPublished(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE outbox SET published_at = now(), last_error = NULL WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("outbox: mark published: %w", err)
	}
	return nil
}

// MarkFailed mencatat kegagalan pengiriman.
func (s *Store) MarkFailed(ctx context.Context, id int64, cause error) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = $1`,
		id, cause.Error())
	if err != nil {
		return fmt.Errorf("outbox: mark failed: %w", err)
	}
	return nil
}

// PendingStore adalah bagian Store yang dibutuhkan relay (memudahkan testing).
type PendingStore interface {
	FetchPending(ctx context.Context, limit int) ([]Record, error)
	MarkPublished(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, cause error) error
}

// Publisher adalah bagian producer Kafka yang dibutuhkan relay.
type Publisher interface {
	PublishRaw(ctx context.Context, topic, key string, value []byte, headers map[string]string) error
}

// Relay memindahkan baris outbox ke Kafka secara berkala.
type Relay struct {
	store     PendingStore
	publisher Publisher
	log       *slog.Logger
	interval  time.Duration
	batch     int
}

// NewRelay membuat Relay.
func NewRelay(store PendingStore, publisher Publisher, log *slog.Logger) *Relay {
	return &Relay{store: store, publisher: publisher, log: log, interval: 200 * time.Millisecond, batch: 100}
}

// Run menjalankan relay sampai context dibatalkan.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		if n, err := r.RunOnce(ctx); err != nil {
			r.log.Error("outbox relay gagal", "error", err)
		} else if n > 0 {
			r.log.Debug("outbox relay terkirim", "count", n)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunOnce mengirim satu batch. Mengembalikan jumlah event terkirim.
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	records, err := r.store.FetchPending(ctx, r.batch)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, rec := range records {
		if err := r.publisher.PublishRaw(ctx, rec.Topic, rec.Key, rec.Payload, rec.Headers); err != nil {
			_ = r.store.MarkFailed(ctx, rec.ID, err)
			continue
		}
		if err := r.store.MarkPublished(ctx, rec.ID); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}
