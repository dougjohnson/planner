-- Migration 007: Support tables — loop_configs, usage_records, credentials, exports.
-- Ref: §9.3.

-- loop_configs: per-project loop settings.
CREATE TABLE loop_configs (
    id                  TEXT PRIMARY KEY NOT NULL,
    project_id          TEXT NOT NULL REFERENCES projects(id),
    loop_type           TEXT NOT NULL,
    iteration_count     INTEGER NOT NULL DEFAULT 4 CHECK (iteration_count >= 1),
    pause_between_loops INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    UNIQUE(project_id, loop_type)
);

-- usage_records: local token/cost accounting per run.
CREATE TABLE usage_records (
    id                  TEXT PRIMARY KEY NOT NULL,
    workflow_run_id     TEXT NOT NULL REFERENCES workflow_runs(id),
    provider            TEXT NOT NULL,
    model_name          TEXT NOT NULL,
    input_tokens        INTEGER NOT NULL DEFAULT 0,
    output_tokens       INTEGER NOT NULL DEFAULT 0,
    cached_tokens       INTEGER NOT NULL DEFAULT 0,
    estimated_cost_minor INTEGER NOT NULL DEFAULT 0,
    recorded_at         TEXT NOT NULL
);

CREATE INDEX idx_usage_records_run_id ON usage_records(workflow_run_id);

-- credentials: credential references and metadata (never the key itself).
CREATE TABLE credentials (
    id         TEXT PRIMARY KEY NOT NULL,
    provider   TEXT NOT NULL,
    label      TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL CHECK (source IN ('env_var', 'config_file', 'ui_entry')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- exports: project bundle records.
CREATE TABLE exports (
    id                    TEXT PRIMARY KEY NOT NULL,
    project_id            TEXT NOT NULL REFERENCES projects(id),
    bundle_path           TEXT NOT NULL,
    include_intermediates INTEGER NOT NULL DEFAULT 0 CHECK (include_intermediates IN (0, 1)),
    manifest_path         TEXT NOT NULL DEFAULT '',
    created_at            TEXT NOT NULL
);

CREATE INDEX idx_exports_project_id ON exports(project_id);
