CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE blue_sheet_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticker        TEXT        NOT NULL,
    start_time    TIMESTAMPTZ NOT NULL,
    end_time      TIMESTAMPTZ NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'queued',
    error_message TEXT,
    s3_key        TEXT,
    etag          TEXT,
    row_count     BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT valid_status CHECK (status IN (
        'queued','running','succeeded','failed'
    ))
);

CREATE INDEX idx_requests_status ON blue_sheet_requests (status);
