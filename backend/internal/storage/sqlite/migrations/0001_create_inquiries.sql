CREATE TABLE IF NOT EXISTS inquiries (
    request_id           TEXT PRIMARY KEY,
    created_at           TEXT    NOT NULL,
    masked_card          TEXT    NOT NULL,
    card_fingerprint     TEXT    NOT NULL DEFAULT '',
    status               TEXT    NOT NULL CHECK (status IN ('success', 'failed')),
    error_code           TEXT    NOT NULL DEFAULT '',
    error_message        TEXT    NOT NULL DEFAULT '',
    upstream_http_status INTEGER NOT NULL DEFAULT 0,
    upstream_track_id    TEXT    NOT NULL DEFAULT '',
    upstream_code        INTEGER,
    upstream_response    TEXT    NOT NULL DEFAULT '',
    masked_iban          TEXT    NOT NULL DEFAULT '',
    bank_name            TEXT    NOT NULL DEFAULT '',
    duration_ms          INTEGER NOT NULL DEFAULT 0,
    client_ip            TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_inquiries_created_at ON inquiries (created_at);
CREATE INDEX IF NOT EXISTS idx_inquiries_status ON inquiries (status, created_at);
CREATE INDEX IF NOT EXISTS idx_inquiries_fingerprint ON inquiries (card_fingerprint);
