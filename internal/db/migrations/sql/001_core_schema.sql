-- Migration 001: Core schema — projects, project_inputs, model_configs, project_model_settings.
-- Ref: §9.3, §9.3.2.

-- projects: top-level project metadata and current state.
CREATE TABLE projects (
    id                         TEXT PRIMARY KEY NOT NULL,
    name                       TEXT NOT NULL,
    description                TEXT NOT NULL DEFAULT '',
    status                     TEXT NOT NULL DEFAULT 'active',
    workflow_definition_version TEXT NOT NULL DEFAULT '1',
    current_stage              TEXT NOT NULL DEFAULT '',
    created_at                 TEXT NOT NULL,
    updated_at                 TEXT NOT NULL,
    archived_at                TEXT
);

-- project_inputs: raw uploaded/pasted inputs tagged by role.
CREATE TABLE project_inputs (
    id                   TEXT PRIMARY KEY NOT NULL,
    project_id           TEXT NOT NULL REFERENCES projects(id),
    role                 TEXT NOT NULL,
    source_type          TEXT NOT NULL,
    content_path         TEXT NOT NULL,
    original_filename    TEXT NOT NULL DEFAULT '',
    detected_mime        TEXT NOT NULL DEFAULT '',
    encoding             TEXT NOT NULL DEFAULT 'utf-8',
    normalization_status TEXT NOT NULL DEFAULT 'pending',
    warning_flags        TEXT NOT NULL DEFAULT '',
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);

CREATE INDEX idx_project_inputs_project_id ON project_inputs(project_id);

-- model_configs: global provider/model definitions.
CREATE TABLE model_configs (
    id                TEXT PRIMARY KEY NOT NULL,
    provider          TEXT NOT NULL,
    model_name        TEXT NOT NULL,
    reasoning_mode    TEXT NOT NULL DEFAULT 'standard',
    credential_source TEXT NOT NULL DEFAULT '',
    validation_status TEXT NOT NULL DEFAULT 'unchecked',
    enabled_global    INTEGER NOT NULL DEFAULT 1 CHECK (enabled_global IN (0, 1)),
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

-- project_model_settings: per-project enable/disable state for models.
CREATE TABLE project_model_settings (
    id              TEXT PRIMARY KEY NOT NULL,
    project_id      TEXT NOT NULL REFERENCES projects(id),
    model_config_id TEXT NOT NULL REFERENCES model_configs(id),
    enabled         INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    priority_order  INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE(project_id, model_config_id)
);

CREATE INDEX idx_project_model_settings_project_id ON project_model_settings(project_id);
