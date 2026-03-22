-- Migration 004: Workflow schema — workflow_runs, workflow_events.
-- Tracks every model execution and provides the timeline for dashboard/SSE.
-- Ref: §9.3, §9.3.2.

-- workflow_runs: stage executions with provider attempt details.
CREATE TABLE workflow_runs (
    id                  TEXT PRIMARY KEY NOT NULL,
    project_id          TEXT NOT NULL REFERENCES projects(id),
    stage               TEXT NOT NULL,
    model_config_id     TEXT REFERENCES model_configs(id),
    status              TEXT NOT NULL DEFAULT 'pending',
    attempt             INTEGER NOT NULL DEFAULT 1,
    session_handle      TEXT NOT NULL DEFAULT '',
    continuity_mode     TEXT NOT NULL DEFAULT '',
    timeout_ms          INTEGER NOT NULL DEFAULT 0,
    provider_request_id TEXT NOT NULL DEFAULT '',
    started_at          TEXT,
    completed_at        TEXT,
    error_message       TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL
);

-- Index for dashboard and resume queries (§9.3.2).
CREATE INDEX idx_workflow_runs_project_stage_status ON workflow_runs(project_id, stage, status);

-- workflow_events: timeline and live event history.
CREATE TABLE workflow_events (
    id              TEXT PRIMARY KEY NOT NULL,
    project_id      TEXT NOT NULL REFERENCES projects(id),
    workflow_run_id TEXT REFERENCES workflow_runs(id),
    event_type      TEXT NOT NULL,
    payload_json    TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL
);

CREATE INDEX idx_workflow_events_run_id ON workflow_events(workflow_run_id);
CREATE INDEX idx_workflow_events_project_id ON workflow_events(project_id, created_at);
