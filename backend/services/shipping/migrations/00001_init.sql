-- +goose Up
-- Ongkir: zona pengiriman dengan tarif tetap per wilayah.
-- Zona bisa dinonaktifkan (soft) atau dihapus; pesanan menyimpan snapshot
-- nama & tarif zona sehingga perubahan di sini tidak mengubah order lama.

CREATE TABLE zones (
    id                UUID        PRIMARY KEY,
    name              TEXT        NOT NULL UNIQUE,
    areas             TEXT[]      NOT NULL DEFAULT '{}',
    fee               BIGINT      NOT NULL CHECK (fee >= 0),
    free_min_subtotal BIGINT      NOT NULL DEFAULT 0 CHECK (free_min_subtotal >= 0),
    is_active         BOOLEAN     NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX zones_active_idx ON zones (name) WHERE is_active;

-- +goose Down
DROP TABLE zones;
