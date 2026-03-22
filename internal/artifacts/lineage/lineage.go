// Package lineage tracks artifact provenance relationships via the
// artifact_relations table. Every artifact creation path must record
// its source artifacts and relation type for full lineage traceability.
package lineage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Relation types per §9.6.
const (
	SynthesizedFrom RelationType = "synthesized_from" // Stage 4/11 → input artifacts
	IntegratedFrom  RelationType = "integrated_from"  // Stage 5/12 → synthesis artifact
	ResolvedFrom    RelationType = "resolved_from"    // Stage 6/13 → integration artifact
	RevisedFrom     RelationType = "revised_from"     // Stage 8/15 → prior canonical
	RollbackOf      RelationType = "rollback_of"      // rollback → original artifact
	DiffTarget      RelationType = "diff_target"       // diff → both artifacts
	ExportIncludes  RelationType = "export_includes"   // export bundle → included artifacts
	DecomposedFrom  RelationType = "decomposed_from"   // fragment-backed → source seed/submitted doc
)

// RelationType is a named artifact relationship.
type RelationType string

// Relation represents a link between two artifacts.
type Relation struct {
	ID                string       `json:"id"`
	ArtifactID        string       `json:"artifact_id"`
	RelatedArtifactID string       `json:"related_artifact_id"`
	RelationType      RelationType `json:"relation_type"`
	CreatedAt         string       `json:"created_at"`
}

// Service manages artifact lineage relationships.
type Service struct {
	db *sql.DB
}

// NewService creates a new lineage Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Record creates a lineage relation between two artifacts.
func (s *Service) Record(ctx context.Context, artifactID, relatedArtifactID string, relType RelationType) error {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO artifact_relations (id, artifact_id, related_artifact_id, relation_type, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, artifactID, relatedArtifactID, string(relType), now)
	if err != nil {
		return fmt.Errorf("recording lineage: %w", err)
	}
	return nil
}

// RecordMultiple creates lineage relations from one artifact to multiple sources.
func (s *Service) RecordMultiple(ctx context.Context, artifactID string, relatedIDs []string, relType RelationType) error {
	for _, relID := range relatedIDs {
		if err := s.Record(ctx, artifactID, relID, relType); err != nil {
			return err
		}
	}
	return nil
}

// GetRelations returns all relations for an artifact (both directions).
func (s *Service) GetRelations(ctx context.Context, artifactID string) ([]Relation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, artifact_id, related_artifact_id, relation_type, created_at
		FROM artifact_relations
		WHERE artifact_id = ? OR related_artifact_id = ?
		ORDER BY created_at ASC
	`, artifactID, artifactID)
	if err != nil {
		return nil, fmt.Errorf("querying relations: %w", err)
	}
	defer rows.Close()

	var relations []Relation
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.ID, &r.ArtifactID, &r.RelatedArtifactID, &r.RelationType, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning relation: %w", err)
		}
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// GetSources returns artifacts that this artifact was derived from.
func (s *Service) GetSources(ctx context.Context, artifactID string) ([]Relation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, artifact_id, related_artifact_id, relation_type, created_at
		FROM artifact_relations
		WHERE artifact_id = ?
		ORDER BY created_at ASC
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("querying sources: %w", err)
	}
	defer rows.Close()

	var relations []Relation
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.ID, &r.ArtifactID, &r.RelatedArtifactID, &r.RelationType, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning relation: %w", err)
		}
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// GetDerived returns artifacts that were derived from this artifact.
func (s *Service) GetDerived(ctx context.Context, artifactID string) ([]Relation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, artifact_id, related_artifact_id, relation_type, created_at
		FROM artifact_relations
		WHERE related_artifact_id = ?
		ORDER BY created_at ASC
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("querying derived: %w", err)
	}
	defer rows.Close()

	var relations []Relation
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.ID, &r.ArtifactID, &r.RelatedArtifactID, &r.RelationType, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning relation: %w", err)
		}
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// TraceLineage walks the lineage graph backwards from an artifact to its roots.
// Returns all ancestors in breadth-first order.
func (s *Service) TraceLineage(ctx context.Context, artifactID string) ([]Relation, error) {
	var all []Relation
	visited := map[string]bool{artifactID: true}
	queue := []string{artifactID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		sources, err := s.GetSources(ctx, current)
		if err != nil {
			return nil, err
		}

		for _, r := range sources {
			all = append(all, r)
			if !visited[r.RelatedArtifactID] {
				visited[r.RelatedArtifactID] = true
				queue = append(queue, r.RelatedArtifactID)
			}
		}
	}

	return all, nil
}
