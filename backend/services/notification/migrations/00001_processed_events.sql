-- +goose Up
-- Dedup persisten untuk konsumer notifikasi. Claim (pending) dilakukan sebelum
-- mengirim Telegram; row dilepas bila pengiriman gagal agar retry Kafka bisa
-- mencoba lagi, dan ditandai sent setelah berhasil.
CREATE TABLE processed_events (
    event_id     TEXT        PRIMARY KEY,
    event_type   TEXT        NOT NULL DEFAULT '',
    status       TEXT        NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('pending', 'sent')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX processed_events_status_idx ON processed_events (status);

-- +goose Down
DROP TABLE processed_events;
