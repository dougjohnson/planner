/**
 * InterruptedRuns — banner component showing interrupted workflow runs
 * with recovery actions (resume, retry, abandon).
 */

import { useState, useCallback } from "react";

interface InterruptedRun {
  id: string;
  stage: string;
  status: string;
  started_at?: string;
  error_message: string;
  created_at: string;
}

interface InterruptedRunsProps {
  projectId: string;
  runs: InterruptedRun[];
  onAction?: () => void;
}

type ActionState = "idle" | "loading" | "error";

export default function InterruptedRuns({
  projectId,
  runs,
  onAction,
}: InterruptedRunsProps) {
  const [actionState, setActionState] = useState<ActionState>("idle");
  const [actionError, setActionError] = useState<string | null>(null);

  const handleAction = useCallback(
    async (runId: string, action: "resume" | "retry" | "abandon") => {
      setActionState("loading");
      setActionError(null);

      try {
        const response = await fetch(
          `/api/projects/${encodeURIComponent(projectId)}/runs/${encodeURIComponent(runId)}/${action}`,
          { method: "POST" },
        );

        if (!response.ok) {
          const err = await response.json().catch(() => null);
          throw new Error(
            err?.error?.message || `${action} failed (${response.status})`,
          );
        }

        setActionState("idle");
        onAction?.();
      } catch (err) {
        setActionError(err instanceof Error ? err.message : "Action failed");
        setActionState("error");
      }
    },
    [projectId, onAction],
  );

  if (runs.length === 0) return null;

  return (
    <div className="interrupted-runs" role="alert" aria-live="polite">
      <h3>Interrupted Runs Detected</h3>
      <p>
        The following workflow runs were interrupted (server restart, crash, or
        timeout). Choose an action for each:
      </p>

      {actionError && <div className="action-error">{actionError}</div>}

      <ul className="interrupted-list">
        {runs.map((run) => (
          <li key={run.id} className="interrupted-item">
            <div className="run-info">
              <span className="run-stage">Stage: {stageLabel(run.stage)}</span>
              <span className="run-id">Run: {run.id.slice(0, 8)}...</span>
              {run.started_at && (
                <span className="run-time">
                  Started: {new Date(run.started_at).toLocaleString()}
                </span>
              )}
              {run.error_message && (
                <span className="run-error">{run.error_message}</span>
              )}
            </div>
            <div className="run-actions">
              <button
                onClick={() => handleAction(run.id, "resume")}
                disabled={actionState === "loading"}
                className="action-button action-resume"
                title="Resume from last checkpoint"
              >
                Resume
              </button>
              <button
                onClick={() => handleAction(run.id, "retry")}
                disabled={actionState === "loading"}
                className="action-button action-retry"
                title="Restart this stage from scratch"
              >
                Retry
              </button>
              <button
                onClick={() => handleAction(run.id, "abandon")}
                disabled={actionState === "loading"}
                className="action-button action-abandon"
                title="Mark as failed and move on"
              >
                Abandon
              </button>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function stageLabel(stage: string): string {
  const labels: Record<string, string> = {
    "stage-1": "Foundations",
    "stage-2": "PRD Intake",
    "stage-3": "PRD Generation",
    "stage-4": "PRD Synthesis",
    "stage-5": "PRD Integration",
    "stage-6": "PRD Review (User)",
    "stage-7": "PRD Review (GPT)",
    "stage-8": "PRD Review (Opus)",
    "stage-9": "PRD Loop Control",
    "stage-10": "Plan Generation",
    "stage-11": "Plan Synthesis",
    "stage-12": "Plan Integration",
    "stage-13": "Plan Review (User)",
    "stage-14": "Plan Review (GPT)",
    "stage-15": "Plan Review (Opus)",
    "stage-16": "Plan Loop Control",
    "stage-17": "Final Stabilization",
  };
  return labels[stage] || stage;
}
