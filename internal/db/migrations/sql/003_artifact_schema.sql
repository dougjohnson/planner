-- Migration 003: Artifact schema — artifacts, artifact_relations, artifact_fragments.
-- Artifacts are immutable versioned snapshots composed from fragment versions.
-- Ref: §9.3, §9.3.2, §9.6.

-- artifacts: immutable versioned artifacts.
CREATE TABLE artifacts (
    id               TEXT PRIMARY KEY NOT NULL,
    project_id       TEXT NOT NULL REFERENCES projects(id),
    artifact_type    TEXT NOT NULL,
    version_label    TEXT NOT NULL DEFAULT '',
    source_stage     TEXT NOT NULL DEFAULT '',
    source_model     TEXT NOT NULL DEFAULT '',
    content_path     TEXT,
    raw_payload_path TEXT NOT NULL DEFAULT '',
    checksum         TEXT NOT NULL DEFAULT '',
    is_canonical     INTEGER NOT NULL DEFAULT 0 CHECK (is_canonical IN (0, 1)),
    created_at       TEXT NOT NULL
);

-- Index for canonical artifact lookup (§9.3.2).
CREATE INDEX idx_artifacts_canonical ON artifacts(project_id, artifact_type, is_canonical);

-- artifact_relations: many-to-many lineage/provenance tracking.
CREATE TABLE artifact_relations (
    id                  TEXT PRIMARY KEY NOT NULL,
    artifact_id         TEXT NOT NULL REFERENCES artifacts(id),
    related_artifact_id TEXT NOT NULL REFERENCES artifacts(id),
    relation_type       TEXT NOT NULL,
    created_at          TEXT NOT NULL
);

CREATE INDEX idx_artifact_relations_artifact_id ON artifact_relations(artifact_id);

-- artifact_fragments: junction defining which fragment versions compose an artifact.
-- Two artifacts can share unchanged fragment versions while differing in modified ones.
CREATE TABLE artifact_fragments (
    artifact_id         TEXT NOT NULL REFERENCES artifacts(id),
    fragment_version_id TEXT NOT NULL REFERENCES fragment_versions(id),
    position            INTEGER NOT NULL,
    PRIMARY KEY (artifact_id, fragment_version_id)
);

CREATE INDEX idx_artifact_fragments_artifact_id ON artifact_fragments(artifact_id);
