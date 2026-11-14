// Package repo mengakses database saybu_shipping dengan pgx.
package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/services/shipping/internal/domain"
)

// Repo adalah implementasi akses data ongkir.
type Repo struct {
	pool *pgxpool.Pool
}

// New membuat Repo.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const zoneColumns = `id, name, areas, fee, free_min_subtotal, is_active, created_at, updated_at`

func scanZone(row pgx.Row) (domain.Zone, error) {
	var z domain.Zone
	err := row.Scan(&z.ID, &z.Name, &z.Areas, &z.Fee, &z.FreeMinSubtotal,
		&z.IsActive, &z.CreatedAt, &z.UpdatedAt)
	if err != nil {
		return domain.Zone{}, err
	}
	if z.Areas == nil {
		z.Areas = []string{}
	}
	return z, nil
}

// ListZones mengambil daftar zona; includeInactive menyertakan yang nonaktif.
func (r *Repo) ListZones(ctx context.Context, includeInactive bool) ([]domain.Zone, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+zoneColumns+`
		FROM zones
		WHERE $1 OR is_active
		ORDER BY name
	`, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("repo: list zones: %w", err)
	}
	defer rows.Close()

	var out []domain.Zone
	for rows.Next() {
		z, err := scanZone(rows)
		if err != nil {
			return nil, fmt.Errorf("repo: scan zone: %w", err)
		}
		out = append(out, z)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: rows zones: %w", err)
	}
	return out, nil
}

// GetZone mengambil zona berdasarkan id.
func (r *Repo) GetZone(ctx context.Context, id string) (domain.Zone, error) {
	z, err := scanZone(r.pool.QueryRow(ctx, `SELECT `+zoneColumns+` FROM zones WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return domain.Zone{}, domain.ErrZoneNotFound
	}
	if err != nil {
		return domain.Zone{}, fmt.Errorf("repo: get zone: %w", err)
	}
	return z, nil
}

// CreateZone menyimpan zona baru.
func (r *Repo) CreateZone(ctx context.Context, z domain.Zone) (domain.Zone, error) {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO zones (id, name, areas, fee, free_min_subtotal, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, z.ID, z.Name, areasOrEmpty(z.Areas), z.Fee, z.FreeMinSubtotal, z.IsActive); err != nil {
		return domain.Zone{}, mapWriteErr("create zone", err)
	}
	return r.GetZone(ctx, z.ID)
}

// UpdateZone memperbarui zona yang ada.
func (r *Repo) UpdateZone(ctx context.Context, z domain.Zone) (domain.Zone, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE zones SET
			name = $2, areas = $3, fee = $4, free_min_subtotal = $5,
			is_active = $6, updated_at = now()
		WHERE id = $1
	`, z.ID, z.Name, areasOrEmpty(z.Areas), z.Fee, z.FreeMinSubtotal, z.IsActive)
	if err != nil {
		return domain.Zone{}, mapWriteErr("update zone", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Zone{}, domain.ErrZoneNotFound
	}
	return r.GetZone(ctx, z.ID)
}

// DeleteZone menghapus zona. Pesanan lama tetap aman karena menyimpan snapshot.
func (r *Repo) DeleteZone(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM zones WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("repo: delete zone: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrZoneNotFound
	}
	return nil
}

// areasOrEmpty memastikan slice tidak nil agar kolom text[] tidak menjadi NULL.
func areasOrEmpty(areas []string) []string {
	if areas == nil {
		return []string{}
	}
	return areas
}

func mapWriteErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrZoneExists
		case "22P02": // uuid tidak valid
			return domain.ErrZoneNotFound
		}
	}
	return fmt.Errorf("repo: %s: %w", op, err)
}

// isInvalidUUID melaporkan apakah error berasal dari uuid yang tidak valid.
func isInvalidUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
