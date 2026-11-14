-- +goose Up
-- Pesanan: pesanan, item (snapshot), riwayat status, idempotensi checkout,
-- penomoran harian, outbox, dan deduplikasi consumer.
--
-- Semua nominal uang BIGINT rupiah. qty dalam satuan terkecil (gram untuk kg).
-- Nama, harga, satuan, dan alamat disimpan sebagai snapshot agar perubahan
-- katalog tidak mengubah pesanan lama.

CREATE TABLE orders (
    id             UUID        PRIMARY KEY,
    code           TEXT        NOT NULL UNIQUE,
    user_id        UUID,
    status         TEXT        NOT NULL
                               CHECK (status IN ('PENDING_PAYMENT', 'WAITING_VERIFICATION', 'PAID',
                                                 'PREPARING', 'SHIPPED', 'READY_PICKUP',
                                                 'COMPLETED', 'CANCELLED', 'EXPIRED')),
    fulfillment    TEXT        NOT NULL CHECK (fulfillment IN ('DELIVERY', 'PICKUP')),
    payment_method TEXT        NOT NULL CHECK (payment_method IN ('TRANSFER', 'COD')),

    customer_name  TEXT        NOT NULL,
    customer_phone TEXT        NOT NULL,
    customer_email TEXT        NOT NULL DEFAULT '',
    note           TEXT        NOT NULL DEFAULT '',

    zone_id        UUID,
    zone_name      TEXT        NOT NULL DEFAULT '',
    shipping_fee   BIGINT      NOT NULL DEFAULT 0 CHECK (shipping_fee >= 0),

    address_line        TEXT NOT NULL DEFAULT '',
    address_rt_rw       TEXT NOT NULL DEFAULT '',
    address_kelurahan   TEXT NOT NULL DEFAULT '',
    address_kecamatan   TEXT NOT NULL DEFAULT '',
    address_city        TEXT NOT NULL DEFAULT '',
    address_postal_code TEXT NOT NULL DEFAULT '',
    address_landmark    TEXT NOT NULL DEFAULT '',

    subtotal       BIGINT      NOT NULL CHECK (subtotal >= 0),
    total          BIGINT      NOT NULL CHECK (total >= 0),
    cancel_reason  TEXT        NOT NULL DEFAULT '',

    expires_at     TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX orders_user_idx ON orders (user_id, created_at DESC);
CREATE INDEX orders_phone_idx ON orders (customer_phone);
CREATE INDEX orders_status_idx ON orders (status, created_at DESC);
CREATE INDEX orders_expiry_idx ON orders (expires_at) WHERE status = 'PENDING_PAYMENT';

CREATE TABLE order_items (
    id         UUID   PRIMARY KEY,
    order_id   UUID   NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id UUID   NOT NULL,
    name       TEXT   NOT NULL,
    unit       TEXT   NOT NULL,
    qty        BIGINT NOT NULL CHECK (qty > 0),
    unit_price BIGINT NOT NULL CHECK (unit_price >= 0),
    line_total BIGINT NOT NULL CHECK (line_total >= 0),
    sort       INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX order_items_order_idx ON order_items (order_id, sort);

CREATE TABLE order_status_history (
    id         UUID        PRIMARY KEY,
    order_id   UUID        NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    status     TEXT        NOT NULL,
    note       TEXT        NOT NULL DEFAULT '',
    actor      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX order_status_history_order_idx ON order_status_history (order_id, created_at);

-- Idempotensi checkout: kunci yang sama + body sama mengembalikan pesanan yang
-- sama; body berbeda ditolak.
CREATE TABLE idempotency_keys (
    key          TEXT        PRIMARY KEY,
    request_hash TEXT        NOT NULL,
    order_id     UUID        REFERENCES orders (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Penomoran pesanan harian: SB-YYYYMMDD-NNNN (zona Asia/Jakarta).
CREATE TABLE order_code_counters (
    day      DATE   PRIMARY KEY,
    last_seq BIGINT NOT NULL DEFAULT 0
);

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
    event_type   TEXT        NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outbox;
DROP TABLE order_code_counters;
DROP TABLE idempotency_keys;
DROP TABLE order_status_history;
DROP TABLE order_items;
DROP TABLE orders;
