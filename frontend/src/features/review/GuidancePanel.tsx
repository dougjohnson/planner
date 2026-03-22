/**
 * GuidancePanel — user guidance submission and management.
 * Embedded in review workspace and dashboard sidebar.
 * Guidance is injected into model prompts during assembly (§11.3.1).
 */

import { useState, useCallback, useEffect } from "react";

interface GuidanceEntry {
  id: string;
  content: string;
  guidance_mode: "advisory_only" | "decision_record";
  stage: string;
  created_at: string;
}

interface GuidancePanelProps {
  projectId: string;
  currentStage?: string;
  showPrompt?: boolean; // "Add guidance for the next iteration?"
}

export default function GuidancePanel({
  projectId,
  currentStage,
  showPrompt,
}: GuidancePanelProps) {
  const [entries, setEntries] = useState<GuidanceEntry[]>([]);
  const [newContent, setNewContent] = useState("");
  const [mode, setMode] = useState<"advisory_only" | "decision_record">(
    "advisory_only",
  );
  const [targetStage, setTargetStage] = useState(currentStage || "");
  const [submitting, setSubmitting] = useState(false);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  // Load existing guidance.
  const loadEntries = useCallback(async () => {
    try {
      const url = new URL(
        `/api/projects/${encodeURIComponent(projectId)}/guidance`,
        window.location.origin,
      );
      if (currentStage) url.searchParams.set("stage", currentStage);

      const response = await fetch(url.toString());
      if (response.ok) {
        const data = await response.json();
        setEntries(Array.isArray(data) ? data : data.data || []);
      }
    } catch {
      // Best effort.
    }
  }, [projectId, currentStage]);

  useEffect(() => {
    loadEntries();
  }, [loadEntries]);

  const handleSubmit = useCallback(async () => {
    if (!newContent.trim()) return;

    setSubmitting(true);
    try {
      const response = await fetch(
        `/api/projects/${encodeURIComponent(projectId)}/guidance`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            content: newContent,
            guidance_mode: mode,
            target_stage: targetStage || currentStage || "all",
          }),
        },
      );

      if (response.ok) {
        setNewContent("");
        loadEntries();
      }
    } catch {
      // Best effort.
    } finally {
      setSubmitting(false);
    }
  }, [newContent, mode, targetStage, currentStage, projectId, loadEntries]);

  const toggleExpand = useCallback((id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  return (
    <div className="guidance-panel">
      <h3>
        Guidance{" "}
        {entries.length > 0 && (
          <span className="guidance-count">{entries.length}</span>
        )}
      </h3>

      {showPrompt && (
        <p className="guidance-prompt">
          Add guidance for the next iteration?
        </p>
      )}

      {/* New guidance input */}
      <div className="guidance-input">
        <textarea
          value={newContent}
          onChange={(e) => setNewContent(e.target.value)}
          placeholder="Steer the model — e.g., 'Focus on security' or 'The API section needs more detail.'"
          rows={3}
        />
        <div className="guidance-options">
          <select
            value={mode}
            onChange={(e) =>
              setMode(e.target.value as "advisory_only" | "decision_record")
            }
            aria-label="Guidance mode"
          >
            <option value="advisory_only">Advisory</option>
            <option value="decision_record">Decision Record</option>
          </select>
          <input
            type="text"
            value={targetStage}
            onChange={(e) => setTargetStage(e.target.value)}
            placeholder={currentStage || "Stage (e.g., stage-7)"}
            aria-label="Target stage"
            className="stage-input"
          />
          <button
            onClick={handleSubmit}
            disabled={submitting || !newContent.trim()}
            className="guidance-submit-btn"
          >
            {submitting ? "..." : "Add"}
          </button>
        </div>
      </div>

      {/* Active guidance list */}
      {entries.length > 0 && (
        <ul className="guidance-list">
          {entries.map((entry) => {
            const isExpanded = expanded.has(entry.id);
            const truncated =
              entry.content.length > 100 && !isExpanded;

            return (
              <li key={entry.id} className="guidance-entry">
                <div className="entry-header">
                  <span
                    className={`mode-badge mode-badge--${entry.guidance_mode}`}
                  >
                    {entry.guidance_mode === "advisory_only"
                      ? "Advisory"
                      : "Decision"}
                  </span>
                  <span className="entry-stage">{entry.stage}</span>
                  <span className="entry-time">
                    {new Date(entry.created_at).toLocaleTimeString()}
                  </span>
                </div>
                <p className="entry-content">
                  {truncated
                    ? entry.content.slice(0, 100) + "..."
                    : entry.content}
                </p>
                {entry.content.length > 100 && (
                  <button
                    onClick={() => toggleExpand(entry.id)}
                    className="expand-btn"
                  >
                    {isExpanded ? "Show less" : "Show more"}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
