-- Migration 005: Review schema — review_items, review_decisions, guidance_injections.
-- Created from report_disagreement tool calls. Links to fragments via fragment_id.
-- Ref: §9.3, §9.5, §12.4.

-- review_items: disputed or user-reviewable changes.
CREATE TABLE review_items (
    id                 TEXT PRIMARY KEY NOT NULL,
    project_id         TEXT NOT NULL REFERENCES projects(id),
    stage              TEXT NOT NULL,
    change_id          TEXT NOT NULL DEFAULT '',
    fragment_id        TEXT REFERENCES fragments(id),
    classification     TEXT NOT NULL DEFAULT '',
    summary            TEXT NOT NULL DEFAULT '',
    diff_ref           TEXT NOT NULL DEFAULT '',
    status             TEXT NOT NULL DEFAULT 'pending',
    group_key          TEXT NOT NULL DEFAULT '',
    conflict_group_key TEXT NOT NULL DEFAULT '',
    created_at         TEXT NOT NULL
);

CREATE INDEX idx_review_items_project_id ON review_items(project_id);
CREATE INDEX idx_review_items_fragment_id ON review_items(fragment_id);

-- review_decisions: accept/reject decisions and notes.
CREATE TABLE review_decisions (
    id             TEXT PRIMARY KEY NOT NULL,
    review_item_id TEXT NOT NULL REFERENCES review_items(id),
    decision       TEXT NOT NULL,
    decision_group TEXT NOT NULL DEFAULT '',
    user_note      TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

CREATE INDEX idx_review_decisions_item_id ON review_decisions(review_item_id);

-- guidance_injections: user steering text fed into later prompts.
CREATE TABLE guidance_injections (
    id            TEXT PRIMARY KEY NOT NULL,
    project_id    TEXT NOT NULL REFERENCES projects(id),
    stage         TEXT NOT NULL,
    guidance_mode TEXT NOT NULL CHECK (guidance_mode IN ('advisory_only', 'decision_record')),
    content       TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE INDEX idx_guidance_injections_project_id ON guidance_injections(project_id);
