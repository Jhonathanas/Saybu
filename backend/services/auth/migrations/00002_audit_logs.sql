-- +goose Up
-- Audit mutasi admin disimpan persisten di auth service (pemilik identitas).
-- Gateway tidak punya database dan hanya mengirim lewat gRPC RecordAudit.
CREATE TABLE audit_logs (
    id          UUID        PRIMARY KEY,
    actor_id    TEXT        NOT NULL,
    actor_role  TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    path        TEXT        NOT NULL,
    status      INTEGER     NOT NULL CHECK (status BETWEEN 100 AND 599),
    request_id  TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at DESC);
-- request_id kosong tidak dianggap sebagai kunci idempotensi.
CREATE UNIQUE INDEX audit_logs_request_id_idx ON audit_logs (request_id)
    WHERE request_id <> '';

-- +goose Down
DROP TABLE audit_logs;
