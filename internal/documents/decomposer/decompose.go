// Package decomposer provides the full decomposition pipeline that converts
// submitted markdown documents into versioned fragments stored in the database.
package decomposer

import (
	"context"
	"fmt"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/markdown"
)

// DecomposeResult contains the outcome of decomposing a markdown document.
type DecomposeResult struct {
	// Fragments lists the fragment records (new or existing) in document order.
	Fragments []*fragments.Fragment
	// Versions lists the fragment versions used (new or reused) in document order.
	Versions []*fragments.FragmentVersion
	// NewFragments is the count of newly created fragments.
	NewFragments int
	// NewVersions is the count of newly created versions (content changed).
	NewVersions int
	// ReusedVersions is the count of reused versions (content identical).
	ReusedVersions int
	// ExcludedFragmentIDs lists existing fragments not present in the submitted document.
	ExcludedFragmentIDs []string
}

// Decomposer orchestrates the full decomposition pipeline: parse → match → persist.
type Decomposer struct {
	store *fragments.Store
}

// New creates a new Decomposer backed by the given fragment store.
func New(store *fragments.Store) *Decomposer {
	return &Decomposer{store: store}
}

// Decompose parses a markdown document and persists its sections as versioned
// fragments. For each section:
//  1. Match to existing fragment by heading text (exact, case-sensitive)
//  2. If matched and content differs → create new fragment_version
//  3. If matched and content identical → reuse existing version (checksum)
//  4. If no match → create new fragment + initial fragment_version
//  5. Existing fragments not in document → recorded as excluded
//
// The sourceStage and sourceRunID are recorded on new fragment versions for traceability.
func (d *Decomposer) Decompose(ctx context.Context, projectID, documentType string, source []byte, sourceStage, sourceRunID string) (*DecomposeResult, error) {
	// Step 1: Parse markdown into sections.
	sections := markdown.Decompose(source)

	// Step 2: Load existing fragments for this project/document type.
	existingFrags, err := d.store.ListByProject(ctx, projectID, documentType)
	if err != nil {
		return nil, fmt.Errorf("listing existing fragments: %w", err)
	}

	// Build fragment refs for matching.
	refs := make([]markdown.FragmentRef, len(existingFrags))
	for i, f := range existingFrags {
		refs[i] = markdown.FragmentRef{ID: f.ID, Heading: f.Heading}
	}

	// Step 3: Match sections to existing fragments.
	matchResults := markdown.MatchSections(sections, refs)

	// Step 4: Persist each section.
	result := &DecomposeResult{}

	for _, mr := range matchResults {
		var frag *fragments.Fragment
		var ver *fragments.FragmentVersion

		if mr.FragmentID != "" {
			// Matched existing fragment — check if content changed.
			frag, err = d.store.GetFragment(ctx, mr.FragmentID)
			if err != nil {
				return nil, fmt.Errorf("getting fragment %s: %w", mr.FragmentID, err)
			}

			// Check if content is identical via checksum.
			checksum := fragments.ComputeChecksum(mr.Section.Content)
			existing, findErr := d.store.FindVersionByChecksum(ctx, frag.ID, checksum)
			if findErr == nil {
				// Content identical — reuse existing version.
				ver = existing
				result.ReusedVersions++
			} else {
				// Content changed — create new version.
				ver, err = d.store.CreateVersion(ctx, frag.ID, mr.Section.Content, sourceStage, sourceRunID, "content updated")
				if err != nil {
					return nil, fmt.Errorf("creating version for fragment %s: %w", frag.ID, err)
				}
				result.NewVersions++
			}
		} else {
			// New section — create fragment + initial version.
			depth := mr.Section.Depth
			if depth == 0 {
				depth = 0 // preamble
			}

			frag, err = d.store.CreateFragment(ctx, projectID, documentType, mr.Section.Heading, depth)
			if err != nil {
				return nil, fmt.Errorf("creating fragment for heading %q: %w", mr.Section.Heading, err)
			}
			result.NewFragments++

			ver, err = d.store.CreateVersion(ctx, frag.ID, mr.Section.Content, sourceStage, sourceRunID, "initial version")
			if err != nil {
				return nil, fmt.Errorf("creating initial version for fragment %s: %w", frag.ID, err)
			}
			result.NewVersions++
		}

		result.Fragments = append(result.Fragments, frag)
		result.Versions = append(result.Versions, ver)
	}

	// Step 5: Find excluded fragments (existing but not in submitted doc).
	result.ExcludedFragmentIDs = markdown.UnmatchedFragments(matchResults, refs)

	return result, nil
}
