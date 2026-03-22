// Package fragments provides the data access layer for fragment and
// fragment_version records. Fragments are the stable, addressable document
// sections (one per ## heading). Fragment versions are immutable content
// snapshots — every document change creates new versions for modified
// fragments while reusing existing versions for unchanged ones.
package fragments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/google/uuid"
)

var (
	// ErrNotFound is returned when a fragment or version does not exist.
	ErrNotFound = errors.New("fragment not found")
)

// Fragment represents a stable, addressable document section.
type Fragment struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	DocumentType string `json:"document_type"` // "prd" or "plan"
	Heading      string `json:"heading"`
	Depth        int    `json:"depth"`
	CreatedAt    string `json:"created_at"`
}

// FragmentVersion is an immutable content snapshot of a fragment.
type FragmentVersion struct {
	ID              string `json:"id"`
	FragmentID      string `json:"fragment_id"`
	Content         string `json:"content"`
	SourceStage     string `json:"source_stage"`
	SourceRunID     string `json:"source_run_id"`
	ChangeRationale string `json:"change_rationale"`
	Checksum        string `json:"checksum"`
	CreatedAt       string `json:"created_at"`
}

// Store provides data access operations for fragments and fragment versions.
type Store struct {
	db *sql.DB
}

// NewStore creates a new fragment store backed by the given database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateFragment inserts a new fragment record and returns it.
func (s *Store) CreateFragment(ctx context.Context, projectID, documentType, heading string, depth int) (*Fragment, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if documentType != "prd" && documentType != "plan" {
		return nil, fmt.Errorf("document_type must be 'prd' or 'plan', got %q", documentType)
	}
	// Empty heading is allowed for preamble fragments (content before first ## heading).
	// Preamble fragments have heading="" and depth=0.

	f := &Fragment{
		ID:           uuid.NewString(),
		ProjectID:    projectID,
		DocumentType: documentType,
		Heading:      heading,
		Depth:        depth,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		f.ID, f.ProjectID, f.DocumentType, f.Heading, f.Depth, f.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting fragment: %w", err)
	}

	return f, nil
}

// GetFragment returns a fragment by its ID.
func (s *Store) GetFragment(ctx context.Context, id string) (*Fragment, error) {
	f := &Fragment{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, document_type, heading, depth, created_at
		 FROM fragments WHERE id = ?`, id,
	).Scan(&f.ID, &f.ProjectID, &f.DocumentType, &f.Heading, &f.Depth, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying fragment: %w", err)
	}
	return f, nil
}

// FindByHeading finds a fragment by heading within a project and document type.
func (s *Store) FindByHeading(ctx context.Context, projectID, documentType, heading string) (*Fragment, error) {
	f := &Fragment{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, document_type, heading, depth, created_at
		 FROM fragments
		 WHERE project_id = ? AND document_type = ? AND heading = ?`,
		projectID, documentType, heading,
	).Scan(&f.ID, &f.ProjectID, &f.DocumentType, &f.Heading, &f.Depth, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: heading %q in %s/%s", ErrNotFound, heading, projectID, documentType)
	}
	if err != nil {
		return nil, fmt.Errorf("querying fragment by heading: %w", err)
	}
	return f, nil
}

// ListByProject returns all fragments for a project and document type,
// ordered by creation time.
func (s *Store) ListByProject(ctx context.Context, projectID, documentType string) ([]*Fragment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, document_type, heading, depth, created_at
		 FROM fragments
		 WHERE project_id = ? AND document_type = ?
		 ORDER BY created_at ASC`,
		projectID, documentType,
	)
	if err != nil {
		return nil, fmt.Errorf("listing fragments: %w", err)
	}
	defer rows.Close()

	var fragments []*Fragment
	for rows.Next() {
		f := &Fragment{}
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.DocumentType, &f.Heading, &f.Depth, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning fragment: %w", err)
		}
		fragments = append(fragments, f)
	}
	return fragments, rows.Err()
}

// CreateVersion inserts a new immutable fragment version. The checksum is
// computed automatically from the content using SHA-256.
func (s *Store) CreateVersion(ctx context.Context, fragmentID, content, sourceStage, sourceRunID, changeRationale string) (*FragmentVersion, error) {
	if fragmentID == "" {
		return nil, fmt.Errorf("fragment_id is required")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	checksum := artifacts.Checksum([]byte(content))

	v := &FragmentVersion{
		ID:              uuid.NewString(),
		FragmentID:      fragmentID,
		Content:         content,
		SourceStage:     sourceStage,
		SourceRunID:     sourceRunID,
		ChangeRationale: changeRationale,
		Checksum:        checksum,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO fragment_versions (id, fragment_id, content, source_stage, source_run_id, change_rationale, checksum, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.FragmentID, v.Content, v.SourceStage, v.SourceRunID, v.ChangeRationale, v.Checksum, v.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting fragment version: %w", err)
	}

	return v, nil
}

// GetVersion returns a fragment version by its ID.
func (s *Store) GetVersion(ctx context.Context, id string) (*FragmentVersion, error) {
	v := &FragmentVersion{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, fragment_id, content, source_stage, source_run_id, change_rationale, checksum, created_at
		 FROM fragment_versions WHERE id = ?`, id,
	).Scan(&v.ID, &v.FragmentID, &v.Content, &v.SourceStage, &v.SourceRunID, &v.ChangeRationale, &v.Checksum, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: version %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying fragment version: %w", err)
	}
	return v, nil
}

// ListVersions returns the version history for a fragment, ordered by creation
// time (oldest first).
func (s *Store) ListVersions(ctx context.Context, fragmentID string) ([]*FragmentVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, fragment_id, content, source_stage, source_run_id, change_rationale, checksum, created_at
		 FROM fragment_versions
		 WHERE fragment_id = ?
		 ORDER BY created_at ASC, rowid ASC`,
		fragmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing fragment versions: %w", err)
	}
	defer rows.Close()

	var versions []*FragmentVersion
	for rows.Next() {
		v := &FragmentVersion{}
		if err := rows.Scan(&v.ID, &v.FragmentID, &v.Content, &v.SourceStage, &v.SourceRunID, &v.ChangeRationale, &v.Checksum, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning fragment version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// LatestVersion returns the most recent version of a fragment.
func (s *Store) LatestVersion(ctx context.Context, fragmentID string) (*FragmentVersion, error) {
	v := &FragmentVersion{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, fragment_id, content, source_stage, source_run_id, change_rationale, checksum, created_at
		 FROM fragment_versions
		 WHERE fragment_id = ?
		 ORDER BY created_at DESC, rowid DESC
		 LIMIT 1`,
		fragmentID,
	).Scan(&v.ID, &v.FragmentID, &v.Content, &v.SourceStage, &v.SourceRunID, &v.ChangeRationale, &v.Checksum, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: no versions for fragment %s", ErrNotFound, fragmentID)
	}
	if err != nil {
		return nil, fmt.Errorf("querying latest version: %w", err)
	}
	return v, nil
}

// FindVersionByChecksum finds an existing version of a fragment with the given
// content checksum. Returns ErrNotFound if no match exists. Useful for
// deduplication — if content hasn't changed, the existing version can be reused.
func (s *Store) FindVersionByChecksum(ctx context.Context, fragmentID, checksum string) (*FragmentVersion, error) {
	v := &FragmentVersion{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, fragment_id, content, source_stage, source_run_id, change_rationale, checksum, created_at
		 FROM fragment_versions
		 WHERE fragment_id = ? AND checksum = ?
		 LIMIT 1`,
		fragmentID, checksum,
	).Scan(&v.ID, &v.FragmentID, &v.Content, &v.SourceStage, &v.SourceRunID, &v.ChangeRationale, &v.Checksum, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: no version with checksum %s for fragment %s", ErrNotFound, checksum, fragmentID)
	}
	if err != nil {
		return nil, fmt.Errorf("querying version by checksum: %w", err)
	}
	return v, nil
}

// ComputeChecksum returns the SHA-256 hex digest of content.
// Exported so callers can pre-compute checksums for deduplication checks.
// Uses the same algorithm as artifacts.Checksum for consistency.
func ComputeChecksum(content string) string {
	return artifacts.Checksum([]byte(content))
}

// NewRepo is an alias for NewStore, for compatibility.
func NewRepo(db *sql.DB) *Store {
	return NewStore(db)
}

// ListFragments is an alias for ListByProject.
func (s *Store) ListFragments(ctx context.Context, projectID, documentType string) ([]*Fragment, error) {
	return s.ListByProject(ctx, projectID, documentType)
}

// VersionHistory returns the version history for a fragment, ordered by
// creation time (newest first). This is the reverse of ListVersions.
func (s *Store) VersionHistory(ctx context.Context, fragmentID string) ([]*FragmentVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, fragment_id, content, source_stage, source_run_id, change_rationale, checksum, created_at
		 FROM fragment_versions
		 WHERE fragment_id = ?
		 ORDER BY created_at DESC, rowid DESC`,
		fragmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing fragment version history: %w", err)
	}
	defer rows.Close()

	var versions []*FragmentVersion
	for rows.Next() {
		v := &FragmentVersion{}
		if err := rows.Scan(&v.ID, &v.FragmentID, &v.Content, &v.SourceStage, &v.SourceRunID, &v.ChangeRationale, &v.Checksum, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning fragment version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}
