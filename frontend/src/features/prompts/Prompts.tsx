/**
 * Prompts — prompt template inspection UI.
 * Fetches all prompt templates from the backend and displays them
 * with version, stage, lock status, and baseline text.
 */

import { useEffect, useState } from "react";
import { get } from "../../lib/api-client";
import { LoadingState, ErrorState, EmptyState } from "../../components/ui";

interface PromptTemplate {
  id: string;
  name: string;
  stage: string;
  version: number;
  baseline_text: string;
  locked_status: string;
}

export default function Prompts() {
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const data = await get<PromptTemplate[]>("/prompts");
        if (!cancelled) {
          setTemplates(data ?? []);
        }
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "Failed to load prompt templates");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => { cancelled = true; };
  }, []);

  const selected = templates.find((t) => t.id === selectedId);

  if (loading) return <LoadingState message="Loading prompt templates..." />;
  if (error) return <ErrorState message={error} />;

  if (templates.length === 0) {
    return (
      <EmptyState
        title="No prompt templates"
        description="Canonical prompts are seeded automatically when the application starts. If this is empty, check the server logs for seeding errors."
      />
    );
  }

  return (
    <div className="prompts-page">
      <h1>Prompt Templates</h1>
      <p>Inspect the prompt templates used in the workflow. {templates.length} templates loaded.</p>

      <div style={{ display: "flex", gap: "1.5rem", marginTop: "1rem" }}>
        {/* Template list */}
        <nav style={{ width: "16rem", flexShrink: 0 }} aria-label="Prompt templates">
          <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
            {templates.map((t) => (
              <li key={t.id}>
                <button
                  onClick={() => setSelectedId(t.id)}
                  style={{
                    display: "block",
                    width: "100%",
                    textAlign: "left",
                    padding: "0.5rem 0.75rem",
                    marginBottom: "2px",
                    border: "1px solid transparent",
                    borderRadius: "var(--radius-sm)",
                    background: selectedId === t.id ? "var(--color-primary-light)" : "transparent",
                    color: selectedId === t.id ? "var(--color-primary)" : "var(--color-text)",
                    cursor: "pointer",
                    fontSize: "var(--text-sm)",
                    fontFamily: "inherit",
                  }}
                  aria-current={selectedId === t.id ? "true" : undefined}
                >
                  <strong style={{ display: "block" }}>{t.name}</strong>
                  <span style={{ fontSize: "var(--text-xs)", color: "var(--color-text-muted)" }}>
                    v{t.version} · {t.stage} {t.locked_status === "locked" ? " · 🔒" : ""}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </nav>

        {/* Template detail */}
        <main style={{ flex: 1, minWidth: 0 }}>
          {selected ? (
            <>
              <h2>{selected.name}</h2>
              <div style={{ display: "flex", gap: "1rem", marginBottom: "1rem", fontSize: "var(--text-sm)", color: "var(--color-text-secondary)" }}>
                <span>Version {selected.version}</span>
                <span>Stage: {selected.stage}</span>
                <span>Status: {selected.locked_status}</span>
              </div>
              <h3>Baseline Text</h3>
              <pre style={{
                background: "var(--color-bg-subtle)",
                padding: "1rem",
                borderRadius: "var(--radius-md)",
                fontSize: "var(--text-sm)",
                fontFamily: "var(--font-mono)",
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
                lineHeight: 1.6,
                maxHeight: "30rem",
                overflow: "auto",
              }}>
                {selected.baseline_text || "(empty)"}
              </pre>
            </>
          ) : (
            <p style={{ color: "var(--color-text-muted)", padding: "2rem 0" }}>
              Select a template from the list to view its details.
            </p>
          )}
        </main>
      </div>
    </div>
  );
}
