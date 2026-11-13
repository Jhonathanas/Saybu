-- +goose Up
-- Katalog: kategori, produk, gambar, dan reservasi stok.
-- Stok disimpan dalam satuan terkecil: gram untuk unit 'kg', lainnya satuan.

CREATE TABLE categories (
    id         UUID        PRIMARY KEY,
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE,
    sort       INTEGER     NOT NULL DEFAULT 0,
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE products (
    id                  UUID        PRIMARY KEY,
    category_id         UUID        REFERENCES categories (id) ON DELETE SET NULL,
    name                TEXT        NOT NULL,
    slug                TEXT        NOT NULL UNIQUE,
    description         TEXT        NOT NULL DEFAULT '',
    unit                TEXT        NOT NULL DEFAULT 'pcs'
                                    CHECK (unit IN ('kg', 'ikat', 'pcs', 'buah', 'paket')),
    price               BIGINT      NOT NULL CHECK (price >= 0),
    stock               BIGINT      NOT NULL DEFAULT 0 CHECK (stock >= 0),
    min_qty             BIGINT      NOT NULL DEFAULT 1 CHECK (min_qty > 0),
    step_qty            BIGINT      NOT NULL DEFAULT 1 CHECK (step_qty > 0),
    low_stock_threshold BIGINT      NOT NULL DEFAULT 5 CHECK (low_stock_threshold >= 0),
    is_active           BOOLEAN     NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX products_category_idx ON products (category_id);
CREATE INDEX products_active_idx ON products (created_at DESC) WHERE is_active;
CREATE INDEX products_name_lower_idx ON products (lower(name));

CREATE TABLE product_images (
    id         UUID        PRIMARY KEY,
    product_id UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    url        TEXT        NOT NULL,
    sort       INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX product_images_product_idx ON product_images (product_id, sort);

CREATE TABLE stock_reservations (
    id         UUID        PRIMARY KEY,
    order_id   UUID        NOT NULL,
    product_id UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    qty        BIGINT      NOT NULL CHECK (qty > 0),
    status     TEXT        NOT NULL DEFAULT 'RESERVED'
                           CHECK (status IN ('RESERVED', 'CONFIRMED', 'RELEASED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX stock_reservations_order_idx ON stock_reservations (order_id);
CREATE INDEX stock_reservations_product_idx ON stock_reservations (product_id);

-- Transactional outbox: event ditulis di transaksi yang sama dengan data bisnis.
CREATE TABLE outbox (
    id           BIGSERIAL   PRIMARY KEY,
    event_id     UUID        NOT NULL UNIQUE,
    topic        TEXT        NOT NULL,
    key          TEXT        NOT NULL DEFAULT '',
    payload      JSONB       NOT NULL,
    headers      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    attempts     INTEGER     NOT NULL DEFAULT 0,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;

-- Idempotensi consumer.
CREATE TABLE processed_events (
    event_id     UUID        PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outbox;
DROP TABLE stock_reservations;
DROP TABLE product_images;
DROP TABLE products;
DROP TABLE categories;
