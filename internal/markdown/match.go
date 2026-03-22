package markdown

// MatchResult describes how a submitted section maps to an existing fragment.
type MatchResult struct {
	// Section is the submitted section.
	Section Section
	// FragmentID is the ID of the matched existing fragment ("" if new).
	FragmentID string
	// MatchType describes how the match was determined.
	MatchType MatchType
}

// MatchType classifies how a section was matched.
type MatchType string

const (
	// MatchExact means the heading text matched exactly.
	MatchExact MatchType = "exact"
	// MatchPositional means the heading matched by position among duplicates.
	MatchPositional MatchType = "positional"
	// MatchNew means no matching fragment exists — this is a new section.
	MatchNew MatchType = "new"
)

// FragmentRef is a lightweight reference to an existing fragment for matching.
type FragmentRef struct {
	ID      string
	Heading string
}

// MatchSections maps submitted sections to existing fragments using heading text.
//
// Matching rules:
//  1. Exact heading match (case-sensitive) — if unique, direct match.
//  2. Duplicate headings — matched by position (first occurrence matches first
//     existing fragment with that heading, second matches second, etc.).
//  3. No match — section is treated as new (MatchNew).
//
// Preamble sections (empty heading) are matched by heading="" if an existing
// fragment with empty heading exists.
func MatchSections(sections []Section, existingFragments []FragmentRef) []MatchResult {
	// Build a map of heading → list of fragment IDs (preserving order).
	headingToFrags := make(map[string][]string)
	for _, f := range existingFragments {
		headingToFrags[f.Heading] = append(headingToFrags[f.Heading], f.ID)
	}

	// Track consumption of each heading's fragment list.
	headingConsumed := make(map[string]int)

	results := make([]MatchResult, len(sections))
	for i, sec := range sections {
		frags, ok := headingToFrags[sec.Heading]
		if !ok || len(frags) == 0 {
			// No matching fragment exists.
			results[i] = MatchResult{
				Section:   sec,
				MatchType: MatchNew,
			}
			continue
		}

		consumed := headingConsumed[sec.Heading]
		if consumed >= len(frags) {
			// All fragments for this heading are consumed — new section.
			results[i] = MatchResult{
				Section:   sec,
				MatchType: MatchNew,
			}
			continue
		}

		matchType := MatchExact
		if len(frags) > 1 {
			matchType = MatchPositional
		}

		results[i] = MatchResult{
			Section:    sec,
			FragmentID: frags[consumed],
			MatchType:  matchType,
		}
		headingConsumed[sec.Heading] = consumed + 1
	}

	return results
}

// UnmatchedFragments returns fragment IDs that were not matched by any section.
// These represent sections that were removed or had their headings changed.
func UnmatchedFragments(results []MatchResult, existingFragments []FragmentRef) []string {
	matched := make(map[string]bool)
	for _, r := range results {
		if r.FragmentID != "" {
			matched[r.FragmentID] = true
		}
	}

	var unmatched []string
	for _, f := range existingFragments {
		if !matched[f.ID] {
			unmatched = append(unmatched, f.ID)
		}
	}
	return unmatched
}
