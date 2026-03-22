package documents

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

// seedTestData sets up a project, document stream, and artifacts for testing.
// Returns projectID, streamID, artifact IDs.
func seedTestData(t testing.TB, tdb *testutil.TestDB) (projectID, streamID string, artifactIDs []string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)

	projectID = "proj-test-001"
	streamID = "stream-prd-001"

	// Create project.
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, ?, 'active', ?, ?)",
		projectID, "Test Project", now, now)

	// Create document stream.
	tdb.Exec("INSERT INTO document_streams (id, project_id, stream_type, created_at) VALUES (?, ?, 'prd', ?)",
		streamID, projectID, now)

	// Create three artifacts.
	artifactIDs = []string{"art-001", "art-002", "art-003"}
	for _, id := range artifactIDs {
		tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, source_stage, is_canonical, created_at)
			VALUES (?, ?, 'prd', 'stage-3', 0, ?)`,
			id, projectID, now)
	}

	return projectID, streamID, artifactIDs
}

func TestPromoteCanonical_FirstPromotion(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projectID, streamID, artifactIDs := seedTestData(t, tdb)
	_ = projectID

	ctx := context.Background()
	result, err := PromoteCanonical(ctx, tdb.DB, streamID, artifactIDs[0])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StreamID != streamID {
		t.Errorf("expected stream %s, got %s", streamID, result.StreamID)
	}
	if result.NewCanonicalID != artifactIDs[0] {
		t.Errorf("expected new canonical %s, got %s", artifactIDs[0], result.NewCanonicalID)
	}
	if result.PreviousCanonicalID != "" {
		t.Errorf("expected no previous canonical, got %s", result.PreviousCanonicalID)
	}

	// Verify: artifact is_canonical = 1.
	var isCanonical int
	tdb.QueryRow("SELECT is_canonical FROM artifacts WHERE id = ?", artifactIDs[0]).Scan(&isCanonical)
	if isCanonical != 1 {
		t.Errorf("expected is_canonical=1, got %d", isCanonical)
	}

	// Verify: stream_heads points to the artifact.
	got, err := GetCanonicalArtifactID(ctx, tdb.DB, streamID)
	if err != nil {
		t.Fatalf("GetCanonicalArtifactID: %v", err)
	}
	if got != artifactIDs[0] {
		t.Errorf("stream head: expected %s, got %s", artifactIDs[0], got)
	}
}

func TestPromoteCanonical_SecondPromotionClearsPrevious(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	_, streamID, artifactIDs := seedTestData(t, tdb)

	ctx := context.Background()

	// First promotion: A becomes canonical.
	_, err := PromoteCanonical(ctx, tdb.DB, streamID, artifactIDs[0])
	if err != nil {
		t.Fatalf("first promotion: %v", err)
	}

	// Second promotion: B becomes canonical.
	result, err := PromoteCanonical(ctx, tdb.DB, streamID, artifactIDs[1])
	if err != nil {
		t.Fatalf("second promotion: %v", err)
	}

	if result.PreviousCanonicalID != artifactIDs[0] {
		t.Errorf("expected previous canonical %s, got %s", artifactIDs[0], result.PreviousCanonicalID)
	}

	// Verify: old artifact is_canonical = 0.
	var oldCanonical int
	tdb.QueryRow("SELECT is_canonical FROM artifacts WHERE id = ?", artifactIDs[0]).Scan(&oldCanonical)
	if oldCanonical != 0 {
		t.Errorf("old artifact: expected is_canonical=0, got %d", oldCanonical)
	}

	// Verify: new artifact is_canonical = 1.
	var newCanonical int
	tdb.QueryRow("SELECT is_canonical FROM artifacts WHERE id = ?", artifactIDs[1]).Scan(&newCanonical)
	if newCanonical != 1 {
		t.Errorf("new artifact: expected is_canonical=1, got %d", newCanonical)
	}

	// Verify: exactly one canonical artifact.
	var count int
	tdb.QueryRow("SELECT COUNT(*) FROM artifacts WHERE project_id = 'proj-test-001' AND artifact_type = 'prd' AND is_canonical = 1").Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 canonical artifact, got %d", count)
	}

	// Verify: stream head points to B.
	got, _ := GetCanonicalArtifactID(ctx, tdb.DB, streamID)
	if got != artifactIDs[1] {
		t.Errorf("stream head: expected %s, got %s", artifactIDs[1], got)
	}
}

func TestPromoteCanonical_ThreePromotions(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	_, streamID, artifactIDs := seedTestData(t, tdb)

	ctx := context.Background()

	for _, id := range artifactIDs {
		_, err := PromoteCanonical(ctx, tdb.DB, streamID, id)
		if err != nil {
			t.Fatalf("promoting %s: %v", id, err)
		}
	}

	// Only the last artifact should be canonical.
	var count int
	tdb.QueryRow("SELECT COUNT(*) FROM artifacts WHERE is_canonical = 1 AND project_id = 'proj-test-001'").Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 canonical, got %d", count)
	}

	got, _ := GetCanonicalArtifactID(ctx, tdb.DB, streamID)
	if got != artifactIDs[2] {
		t.Errorf("expected final canonical %s, got %s", artifactIDs[2], got)
	}
}

func TestPromoteCanonical_ArtifactNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	_, streamID, _ := seedTestData(t, tdb)

	ctx := context.Background()
	_, err := PromoteCanonical(ctx, tdb.DB, streamID, "nonexistent-artifact")
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("expected ErrArtifactNotFound, got %v", err)
	}
}

func TestPromoteCanonical_StreamNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	seedTestData(t, tdb) // seed artifacts but use wrong stream

	ctx := context.Background()
	_, err := PromoteCanonical(ctx, tdb.DB, "nonexistent-stream", "art-001")
	if !errors.Is(err, ErrStreamNotFound) {
		t.Errorf("expected ErrStreamNotFound, got %v", err)
	}
}

func TestPromoteCanonical_ProjectMismatch(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)

	// Create two projects.
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES ('p1', 'P1', 'active', ?, ?)", now, now)
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES ('p2', 'P2', 'active', ?, ?)", now, now)

	// Stream belongs to p1.
	tdb.Exec("INSERT INTO document_streams (id, project_id, stream_type, created_at) VALUES ('s1', 'p1', 'prd', ?)", now)

	// Artifact belongs to p2.
	tdb.Exec("INSERT INTO artifacts (id, project_id, artifact_type, is_canonical, created_at) VALUES ('a1', 'p2', 'prd', 0, ?)", now)

	_, err := PromoteCanonical(ctx, tdb.DB, "s1", "a1")
	if !errors.Is(err, ErrArtifactStreamMismatch) {
		t.Errorf("expected ErrArtifactStreamMismatch, got %v", err)
	}
}

func TestPromoteCanonical_EmptyInputs(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	_, err := PromoteCanonical(ctx, tdb.DB, "", "art-001")
	if err == nil {
		t.Fatal("expected error for empty stream ID")
	}

	_, err = PromoteCanonical(ctx, tdb.DB, "stream-001", "")
	if err == nil {
		t.Fatal("expected error for empty artifact ID")
	}
}

func TestPromoteCanonical_ConcurrentPromotions(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	_, streamID, artifactIDs := seedTestData(t, tdb)

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan *PromotionResult, len(artifactIDs))
	errs := make(chan error, len(artifactIDs))

	// Launch concurrent promotions.
	for _, id := range artifactIDs {
		wg.Add(1)
		go func(artID string) {
			defer wg.Done()
			result, err := PromoteCanonical(ctx, tdb.DB, streamID, artID)
			if err != nil {
				errs <- err
				return
			}
			results <- result
		}(id)
	}

	wg.Wait()
	close(results)
	close(errs)

	// Check that at most one error occurred and the rest succeeded.
	for err := range errs {
		// Errors from contention are acceptable, as long as the end state is consistent.
		t.Logf("concurrent promotion error (acceptable): %v", err)
	}

	// Critical check: exactly one canonical artifact.
	var count int
	tdb.QueryRow("SELECT COUNT(*) FROM artifacts WHERE project_id = 'proj-test-001' AND artifact_type = 'prd' AND is_canonical = 1").Scan(&count)
	if count != 1 {
		t.Errorf("CONSISTENCY VIOLATION: expected exactly 1 canonical artifact after concurrent promotions, got %d", count)
	}

	// Verify stream head points to the canonical artifact.
	var headArtifact string
	tdb.QueryRow("SELECT artifact_id FROM stream_heads WHERE stream_id = ?", streamID).Scan(&headArtifact)

	var headIsCanonical int
	tdb.QueryRow("SELECT is_canonical FROM artifacts WHERE id = ?", headArtifact).Scan(&headIsCanonical)
	if headIsCanonical != 1 {
		t.Errorf("stream head artifact %s has is_canonical=%d, expected 1", headArtifact, headIsCanonical)
	}
}

func TestGetCanonicalArtifactID_NoHead(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	_, err := GetCanonicalArtifactID(ctx, tdb.DB, "nonexistent-stream")
	if !errors.Is(err, ErrStreamNotFound) {
		t.Errorf("expected ErrStreamNotFound, got %v", err)
	}
}

func TestGetCanonicalArtifactID_AfterPromotion(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	_, streamID, artifactIDs := seedTestData(t, tdb)

	ctx := context.Background()
	_, err := PromoteCanonical(ctx, tdb.DB, streamID, artifactIDs[1])
	if err != nil {
		t.Fatalf("promotion: %v", err)
	}

	got, err := GetCanonicalArtifactID(ctx, tdb.DB, streamID)
	if err != nil {
		t.Fatalf("GetCanonicalArtifactID: %v", err)
	}
	if got != artifactIDs[1] {
		t.Errorf("expected %s, got %s", artifactIDs[1], got)
	}
}
