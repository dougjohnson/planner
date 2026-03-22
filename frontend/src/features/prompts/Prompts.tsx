/**
 * Prompts — prompt template and render inspection UI.
 * Shows all prompt templates used in this project's workflow,
 * their versions, lock status, and rendered snapshots.
 */

import { useState } from "react";
import { useParams } from "react-router-dom";

interface PromptTemplate {
  id: string;
  name: string;
  stage: string;
  version: number;
  baseline_text: string;
  locked_status: string;
}

export default function Prompts() {
  const { projectId: _projectId } = useParams<{ projectId: string }>();
  void _projectId; // Available for future API calls.
  const [templates] = useState<PromptTemplate[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const selected = templates.find((t) => t.id === selectedId);

  return (
    <div className="prompts-page">
      <h1>Prompt Templates</h1>
      <p>Inspect the prompt templates used in this project&apos;s workflow.</p>

      <div className="prompts-layout">
        {/* Template list */}
        <nav className="prompts-list" aria-label="Prompt templates">
          {templates.length === 0 ? (
            <p className="prompts-empty">
              No prompt templates loaded. Run canonical prompt seeding first.
            </p>
          ) : (
            <ul>
              {templates.map((t) => (
                <li key={t.id}>
                  <button
                    onClick={() => setSelectedId(t.id)}
                    className={`prompt-item ${selectedId === t.id ? "prompt-item--active" : ""}`}
                    aria-current={selectedId === t.id ? "true" : undefined}
                  >
                    <span className="prompt-name">{t.name}</span>
                    <span className="prompt-meta">
                      v{t.version} · {t.stage}
                    </span>
                    {t.locked_status === "locked" && (
                      <span className="prompt-locked" title="Locked">
                        locked
                      </span>
                    )}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </nav>

        {/* Template detail */}
        <main className="prompts-detail">
          {selected ? (
            <>
              <h2>{selected.name}</h2>
              <div className="prompt-info">
                <span>Version: {selected.version}</span>
                <span>Stage: {selected.stage}</span>
                <span>Status: {selected.locked_status}</span>
              </div>
              <h3>Baseline Text</h3>
              <pre className="prompt-text">{selected.baseline_text}</pre>
            </>
          ) : (
            <p className="prompts-placeholder">
              Select a template to view its details.
            </p>
          )}
        </main>
      </div>
    </div>
  );
}
