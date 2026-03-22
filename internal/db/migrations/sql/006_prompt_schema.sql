-- Migration 006: Prompt schema — prompt_templates, prompt_renders.
-- Stores canonical and wrapper prompts with rendered snapshots per run.
-- Ref: §9.3, §11.2, §11.3.

-- prompt_templates: canonical and wrapper prompts.
CREATE TABLE prompt_templates (
    id                         TEXT PRIMARY KEY NOT NULL,
    name                       TEXT NOT NULL,
    stage                      TEXT NOT NULL,
    version                    INTEGER NOT NULL DEFAULT 1,
    baseline_text              TEXT NOT NULL DEFAULT '',
    wrapper_text               TEXT NOT NULL DEFAULT '',
    output_contract_json       TEXT NOT NULL DEFAULT '{}',
    locked_status              TEXT NOT NULL DEFAULT 'unlocked',
    original_prd_baseline_text TEXT,
    created_at                 TEXT NOT NULL,
    updated_at                 TEXT NOT NULL,
    deprecated_at              TEXT,
    UNIQUE(name, version)
);

-- prompt_renders: redacted rendered prompt payloads linked to runs.
CREATE TABLE prompt_renders (
    id                 TEXT PRIMARY KEY NOT NULL,
    workflow_run_id    TEXT NOT NULL REFERENCES workflow_runs(id),
    prompt_template_id TEXT NOT NULL REFERENCES prompt_templates(id),
    rendered_prompt_path TEXT NOT NULL,
    redaction_status   TEXT NOT NULL DEFAULT 'pending',
    created_at         TEXT NOT NULL
);

CREATE INDEX idx_prompt_renders_run_id ON prompt_renders(workflow_run_id);
