package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// maxSeen membatasi jumlah event id yang diingat oleh dedup in-memory.
const maxSeen = 2000

// DedupStore menyimpan status pemrosesan event konsumer notifikasi.
//
// Efek pengiriman Telegram bersifat eksternal sehingga tidak bisa berada dalam
// transaksi DB yang sama. Pola yang dipakai: Claim (insert pending) → kirim →
// MarkSent / Release. Bila proses crash setelah Claim tetapi sebelum MarkSent,
// event nyangkut sebagai pending; operator bisa menghapus baris pending tersebut
// agar dikonsumsi ulang (lihat RUNBOOK).
type DedupStore interface {
	// Claim mengembalikan true bila event ini baru pertama kali diklaim.
	Claim(ctx context.Context, eventID, eventType string) (bool, error)
	// MarkSent menandai event berhasil dinotifikasi.
	MarkSent(ctx context.Context, eventID string) error
	// Release membatalkan klaim agar retry Kafka bisa memproses ulang.
	Release(ctx context.Context, eventID string) error
}

// NewMemoryDedup menyediakan dedup untuk pengembangan tanpa database.
// State hilang saat restart; pakai NewPostgresDedup di produksi.
func NewMemoryDedup() DedupStore {
	return &memoryDedup{seen: make(map[string]struct{})}
}

type memoryDedup struct {
	mu   sync.Mutex
	seen map[string]struct{}
	ring []string
}

func (d *memoryDedup) Claim(_ context.Context, id, _ string) (bool, error) {
	if id == "" {
		return true, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.seen[id]; ok {
		return false, nil
	}
	d.seen[id] = struct{}{}
	d.ring = append(d.ring, id)
	if len(d.ring) > maxSeen {
		old := d.ring[0]
		d.ring = d.ring[1:]
		delete(d.seen, old)
	}
	return true, nil
}

func (d *memoryDedup) MarkSent(context.Context, string) error { return nil }

func (d *memoryDedup) Release(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.seen, id)
	for i, seen := range d.ring {
		if seen == id {
			d.ring = append(d.ring[:i], d.ring[i+1:]...)
			break
		}
	}
	return nil
}

// PostgresDedup menyimpan processed_events di database notification.
type PostgresDedup struct {
	pool *pgxpool.Pool
}

// NewPostgresDedup membuat dedup persisten.
func NewPostgresDedup(pool *pgxpool.Pool) *PostgresDedup {
	return &PostgresDedup{pool: pool}
}

// Claim melakukan INSERT ... ON CONFLICT DO NOTHING, sehingga dua konsumer
// yang menerima event sama hanya satu yang berhak mengirim notifikasi.
func (d *PostgresDedup) Claim(ctx context.Context, eventID, eventType string) (bool, error) {
	tag, err := d.pool.Exec(ctx, `
		INSERT INTO processed_events (event_id, event_type, status)
		VALUES ($1, $2, 'pending')
		ON CONFLICT (event_id) DO NOTHING
	`, eventID, eventType)
	if err != nil {
		return false, fmt.Errorf("dedup: claim %s: %w", eventID, err)
	}
	return tag.RowsAffected() == 1, nil
}

// MarkSent menandai event sebagai sent.
func (d *PostgresDedup) MarkSent(ctx context.Context, eventID string) error {
	if _, err := d.pool.Exec(ctx, `
		UPDATE processed_events SET status = 'sent', processed_at = now()
		WHERE event_id = $1 AND status = 'pending'
	`, eventID); err != nil {
		return fmt.Errorf("dedup: mark sent %s: %w", eventID, err)
	}
	return nil
}

// Release menghapus klaim gagal agar event bisa dikirim ulang.
func (d *PostgresDedup) Release(ctx context.Context, eventID string) error {
	if _, err := d.pool.Exec(ctx, `
		DELETE FROM processed_events WHERE event_id = $1 AND status = 'pending'
	`, eventID); err != nil {
		return fmt.Errorf("dedup: release %s: %w", eventID, err)
	}
	return nil
}
