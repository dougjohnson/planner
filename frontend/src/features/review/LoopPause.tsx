/**
 * LoopPause — inter-loop review UI shown when the review loop pauses.
 * Displays iteration progress, changes summary, guidance input, and continue/finish actions.
 */

import { useState, useCallback } from "react";

interface LoopPauseProps {
  projectId: string;
  iteration: number;
  totalIterations: number;
  documentType: "prd" | "plan";
  converged: boolean;
  changes: {
    updated: number;
    added: number;
    removed: number;
  };
  onContinue?: () => void;
  onFinish?: () => void;
}

export default function LoopPause({
  projectId,
  iteration,
  totalIterations,
  documentType,
  converged,
  changes,
  onContinue,
  onFinish,
}: LoopPauseProps) {
  const [guidance, setGuidance] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [guidanceSubmitted, setGuidanceSubmitted] = useState(false);

  const totalChanges = changes.updated + changes.added + changes.removed;
  const progress = (iteration / totalIterations) * 100;
  const docLabel = documentType === "prd" ? "PRD" : "Plan";

  const submitGuidance = useCallback(async () => {
    if (!guidance.trim()) return;

    setSubmitting(true);
    try {
      const nextStage =
        documentType === "prd"
          ? `stage-${iteration <= 2 ? 7 : 8}`
          : `stage-${iteration <= 2 ? 14 : 15}`;

      await fetch(
        `/api/projects/${encodeURIComponent(projectId)}/guidance`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            content: guidance,
            guidance_mode: "advisory_only",
            target_stage: nextStage,
          }),
        },
      );

      setGuidanceSubmitted(true);
    } catch {
      // Best effort — guidance is optional.
    } finally {
      setSubmitting(false);
    }
  }, [guidance, projectId, documentType, iteration]);

  if (converged) {
    return (
      <div className="loop-pause loop-pause--converged" role="status">
        <h3>{docLabel} Review Loop — Converged</h3>
        <p>
          Iteration {iteration} of {totalIterations}: model proposed no changes.
          The document has stabilized.
        </p>
        <div className="loop-actions">
          <button onClick={onFinish} className="action-button action-finish">
            Accept and Proceed
          </button>
          <button onClick={onContinue} className="action-button action-continue">
            Force Another Iteration
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="loop-pause" role="status">
      <h3>{docLabel} Review Loop — Paused</h3>

      {/* Progress */}
      <div className="loop-progress">
        <div className="progress-bar">
          <div
            className="progress-fill"
            style={{ width: `${progress}%` }}
            role="progressbar"
            aria-valuenow={iteration}
            aria-valuemin={1}
            aria-valuemax={totalIterations}
          />
        </div>
        <span className="progress-label">
          Iteration {iteration} of {totalIterations} complete
        </span>
      </div>

      {/* Changes summary */}
      <div className="changes-summary">
        <h4>Changes This Iteration</h4>
        {totalChanges === 0 ? (
          <p>No fragment changes in this iteration.</p>
        ) : (
          <ul>
            {changes.updated > 0 && (
              <li>{changes.updated} fragment(s) updated</li>
            )}
            {changes.added > 0 && <li>{changes.added} fragment(s) added</li>}
            {changes.removed > 0 && (
              <li>{changes.removed} fragment(s) removed</li>
            )}
          </ul>
        )}
      </div>

      {/* Guidance input */}
      <div className="guidance-section">
        <h4>Guidance for Next Iteration (optional)</h4>
        <textarea
          value={guidance}
          onChange={(e) => setGuidance(e.target.value)}
          placeholder="Steer the next review pass — e.g., 'Focus on the security section' or 'The testing strategy needs more specificity.'"
          rows={3}
          disabled={guidanceSubmitted}
        />
        {!guidanceSubmitted && guidance.trim() && (
          <button
            onClick={submitGuidance}
            disabled={submitting}
            className="guidance-submit"
          >
            {submitting ? "Submitting..." : "Submit Guidance"}
          </button>
        )}
        {guidanceSubmitted && (
          <span className="guidance-confirmed">Guidance submitted</span>
        )}
      </div>

      {/* Actions */}
      <div className="loop-actions">
        <button onClick={onContinue} className="action-button action-continue">
          Continue to Iteration {iteration + 1}
        </button>
        <button onClick={onFinish} className="action-button action-finish">
          Finish Early
        </button>
      </div>
    </div>
  );
}
