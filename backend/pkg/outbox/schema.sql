-- Tabel outbox. Sertakan migrasi ini di service yang memproduksi event.
CREATE TABLE IF NOT EXISTS outbox (
    id           BIGSERIAL PRIMARY KEY,
    event_id     UUID        NOT NULL UNIQUE,
    topic        TEXT        NOT NULL,
    key          TEXT        NOT NULL DEFAULT '',
    payload      JSONB       NOT NULL,
    headers      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    attempts     INT         NOT NULL DEFAULT 0,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_pending_idx
    ON outbox (id)
    WHERE published_at IS NULL;
