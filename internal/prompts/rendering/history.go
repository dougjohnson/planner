package rendering

import (
	"fmt"
	"strings"
)

// IterationChange describes a single fragment change in one review iteration.
type IterationChange struct {
	FragmentID   string `json:"fragment_id"`
	FragmentName string `json:"fragment_name"` // heading text
	ChangeType   string `json:"change_type"`   // "modified", "added", "removed"
	Summary      string `json:"summary"`       // brief description of what changed
}

// IterationRecord describes one review iteration's changes and context.
type IterationRecord struct {
	Iteration   int               `json:"iteration"`
	ModelFamily string            `json:"model_family"` // "gpt" or "opus"
	Changes     []IterationChange `json:"changes"`
	Guidance    []string          `json:"guidance,omitempty"` // user guidance applied since last iteration
}

// AssembleChangeHistory formats loop iteration records into a concise text
// summary suitable for injection into prompt assembly (step 5 of §11.3.1).
//
// The output is a structured list, not full artifact replay. Its purpose:
// prevent fresh sessions from re-proposing already-made or rejected changes
// and give model awareness of the document's recent trajectory.
func AssembleChangeHistory(iterations []IterationRecord) string {
	if len(iterations) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Review Loop Change History\n\n")
	b.WriteString("The following changes were made in prior review iterations. ")
	b.WriteString("Do NOT re-propose changes that have already been made or rejected.\n\n")

	for _, iter := range iterations {
		b.WriteString(fmt.Sprintf("### Iteration %d (%s)\n\n", iter.Iteration, iter.ModelFamily))

		if len(iter.Changes) == 0 {
			b.WriteString("No fragment changes (convergence reached).\n\n")
			continue
		}

		for _, ch := range iter.Changes {
			icon := changeIcon(ch.ChangeType)
			heading := ch.FragmentName
			if heading == "" {
				heading = ch.FragmentID
			}
			b.WriteString(fmt.Sprintf("- %s **%s** [%s]: %s\n", icon, heading, ch.ChangeType, ch.Summary))
		}
		b.WriteString("\n")

		if len(iter.Guidance) > 0 {
			b.WriteString("**User guidance applied:**\n")
			for _, g := range iter.Guidance {
				b.WriteString(fmt.Sprintf("- %s\n", g))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func changeIcon(changeType string) string {
	switch changeType {
	case "modified":
		return "~"
	case "added":
		return "+"
	case "removed":
		return "-"
	default:
		return "*"
	}
}
