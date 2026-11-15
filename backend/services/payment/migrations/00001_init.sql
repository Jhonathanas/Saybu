-- +goose Up
-- Pembayaran: status pembayaran per pesanan, riwayat bukti transfer, outbox,
-- dan deduplikasi consumer.
--
-- Pembayaran dibuat dari event order.created (snapshot nama, nomor HP, metode,
-- dan nominal) sehingga perubahan di service order tidak mengubah pembayaran
-- lama. Nominal uang BIGINT rupiah.

CREATE TABLE payments (
    id                UUID        PRIMARY KEY,
    order_id          UUID        NOT NULL UNIQUE,
    order_code        TEXT        NOT NULL,
    user_id           UUID,
    method            TEXT        NOT NULL CHECK (method IN ('TRANSFER', 'COD')),
    status            TEXT        NOT NULL
                                  CHECK (status IN ('UNPAID', 'WAITING_VERIFICATION', 'PAID',
                                                    'REJECTED', 'CANCELLED', 'COD_PENDING', 'COD_PAID')),
    amount            BIGINT      NOT NULL CHECK (amount >= 0),

    customer_name     TEXT        NOT NULL DEFAULT '',
    customer_phone    TEXT        NOT NULL DEFAULT '',

    proof_url         TEXT        NOT NULL DEFAULT '',
    proof_uploaded_at TIMESTAMPTZ,
    verified_at       TIMESTAMPTZ,
    verified_by       TEXT        NOT NULL DEFAULT '',
    note              TEXT        NOT NULL DEFAULT '',

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payments_order_code_idx ON payments (order_code);
CREATE INDEX payments_status_idx ON payments (status, created_at DESC);
CREATE INDEX payments_phone_idx ON payments (customer_phone);

-- Riwayat unggahan bukti transfer (satu pembayaran bisa upload ulang setelah
-- ditolak). payments.proof_url menyimpan bukti terbaru.
CREATE TABLE payment_proofs (
    id           UUID        PRIMARY KEY,
    payment_id   UUID        NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    url          TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL DEFAULT 0,
    content_type TEXT        NOT NULL DEFAULT '',
    uploaded_by  TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payment_proofs_payment_idx ON payment_proofs (payment_id, created_at DESC);

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
DROP TABLE payment_proofs;
DROP TABLE payments;
