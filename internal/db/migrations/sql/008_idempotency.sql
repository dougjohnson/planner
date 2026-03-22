-- Migration 008: Idempotency key tracking for duplicate request prevention.
-- Every mutating command accepts an idempotency key; repeated submissions
-- return the original outcome. Ref: §10.1, §15.1.

CREATE TABLE idempotency_keys (
    key         TEXT PRIMARY KEY NOT NULL,
    project_id  TEXT NOT NULL,
    command     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'received',
    result_json TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL,
    completed_at TEXT
);

CREATE INDEX idx_idempotency_project ON idempotency_keys(project_id, command);
