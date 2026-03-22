-- Migration 002: Fragment schema — fragments, fragment_versions, document_streams, stream_heads.
-- Fragments are the addressable document sections (one per ## heading).
-- Ref: §9.3, §9.3.1.

-- fragments: addressable document sections.
CREATE TABLE fragments (
    id            TEXT PRIMARY KEY NOT NULL,
    project_id    TEXT NOT NULL REFERENCES projects(id),
    document_type TEXT NOT NULL CHECK (document_type IN ('prd', 'plan')),
    heading       TEXT NOT NULL,
    depth         INTEGER NOT NULL DEFAULT 2,
    created_at    TEXT NOT NULL
);

CREATE INDEX idx_fragments_project_id ON fragments(project_id);

-- fragment_versions: immutable content versions per fragment.
CREATE TABLE fragment_versions (
    id               TEXT PRIMARY KEY NOT NULL,
    fragment_id      TEXT NOT NULL REFERENCES fragments(id),
    content          TEXT NOT NULL,
    source_stage     TEXT NOT NULL DEFAULT '',
    source_run_id    TEXT NOT NULL DEFAULT '',
    change_rationale TEXT NOT NULL DEFAULT '',
    checksum         TEXT NOT NULL,
    created_at       TEXT NOT NULL
);

CREATE INDEX idx_fragment_versions_fragment_id ON fragment_versions(fragment_id);

-- document_streams: logical document lanes (PRD, PLAN).
CREATE TABLE document_streams (
    id          TEXT PRIMARY KEY NOT NULL,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    stream_type TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE INDEX idx_document_streams_project_id ON document_streams(project_id);

-- stream_heads: current canonical artifact pointer per stream.
CREATE TABLE stream_heads (
    stream_id   TEXT NOT NULL UNIQUE REFERENCES document_streams(id),
    artifact_id TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
