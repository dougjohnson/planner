/**
 * ProjectExport — Stage 17 final review and export workspace.
 *
 * Shows final artifacts, stabilization check results, export options,
 * and individual file download.
 */

import { useState, useCallback } from "react";
import { useParams } from "react-router-dom";

/** Stabilization finding from the backend. */
interface Finding {
  code: string;
  severity: "error" | "warning";
  message: string;
  location?: string;
}

interface StabilizationReport {
  findings: Finding[];
  error_count: number;
  warning_count: number;
  export_ready: boolean;
}

/** Export bundle options. */
interface ExportOptions {
  canonical_only: boolean;
  include_intermediates: boolean;
  include_raw: boolean;
}

type ExportState = "review" | "exporting" | "complete" | "error";

export default function ProjectExport() {
  const { projectId } = useParams<{ projectId: string }>();
  const [state, setState] = useState<ExportState>("review");
  const [report, setReport] = useState<StabilizationReport | null>(null);
  const [options, setOptions] = useState<ExportOptions>({
    canonical_only: true,
    include_intermediates: false,
    include_raw: false,
  });
  const [exportError, setExportError] = useState<string | null>(null);
  const [bundlePath, setBundlePath] = useState<string | null>(null);

  const runStabilizationChecks = useCallback(async () => {
    try {
      const response = await fetch(
        `/api/projects/${encodeURIComponent(projectId!)}/stabilize`,
        { method: "POST" },
      );
      if (!response.ok) throw new Error("Stabilization check failed");
      const data: StabilizationReport = await response.json();
      setReport(data);
    } catch (err) {
      setExportError(
        err instanceof Error ? err.message : "Check failed",
      );
    }
  }, [projectId]);

  const handleExport = useCallback(async () => {
    if (report && !report.export_ready) return;

    setState("exporting");
    setExportError(null);

    try {
      const response = await fetch(
        `/api/projects/${encodeURIComponent(projectId!)}/exports`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(options),
        },
      );
      if (!response.ok) {
        const err = await response.json().catch(() => null);
        throw new Error(err?.error?.message || "Export failed");
      }
      const data = await response.json();
      setBundlePath(data.bundle_path || "export.zip");
      setState("complete");
    } catch (err) {
      setExportError(err instanceof Error ? err.message : "Export failed");
      setState("error");
    }
  }, [projectId, options, report]);

  return (
    <div className="project-export">
      <h1>Export Project</h1>
      <p>Review stabilization results and create an export bundle.</p>

      {/* Stabilization checks */}
      <section className="stabilization-section">
        <h2>Stabilization Checks</h2>
        <button onClick={runStabilizationChecks} className="check-button">
          Run Checks
        </button>

        {report && (
          <div className="stabilization-report">
            <div
              className={`report-status ${report.export_ready ? "report-status--ready" : "report-status--blocked"}`}
            >
              {report.export_ready
                ? "Export Ready"
                : `Blocked (${report.error_count} errors)`}
              {report.warning_count > 0 &&
                ` — ${report.warning_count} warnings`}
            </div>

            {report.findings.length > 0 && (
              <ul className="findings-list">
                {report.findings.map((f, i) => (
                  <li
                    key={`${f.code}-${i}`}
                    className={`finding finding--${f.severity}`}
                  >
                    <span className="finding-severity">{f.severity}</span>
                    <span className="finding-message">{f.message}</span>
                    {f.location && (
                      <span className="finding-location">{f.location}</span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </section>

      {/* Export options */}
      <section className="export-options">
        <h2>Export Options</h2>
        <label>
          <input
            type="checkbox"
            checked={options.canonical_only}
            onChange={(e) =>
              setOptions((o) => ({
                ...o,
                canonical_only: e.target.checked,
                include_intermediates: e.target.checked
                  ? false
                  : o.include_intermediates,
              }))
            }
          />
          Canonical artifacts only
        </label>
        <label>
          <input
            type="checkbox"
            checked={options.include_intermediates}
            disabled={options.canonical_only}
            onChange={(e) =>
              setOptions((o) => ({
                ...o,
                include_intermediates: e.target.checked,
              }))
            }
          />
          Include intermediate versions
        </label>
        <label>
          <input
            type="checkbox"
            checked={options.include_raw}
            onChange={(e) =>
              setOptions((o) => ({ ...o, include_raw: e.target.checked }))
            }
          />
          Include raw model outputs
        </label>
      </section>

      {/* Export action */}
      <section className="export-action">
        <button
          onClick={handleExport}
          disabled={
            state === "exporting" || (report !== null && !report.export_ready)
          }
          className="export-button"
        >
          {state === "exporting" ? "Creating bundle..." : "Create Export Bundle"}
        </button>
      </section>

      {/* Error */}
      {exportError && (
        <div className="export-error" role="alert">
          {exportError}
        </div>
      )}

      {/* Complete */}
      {state === "complete" && bundlePath && (
        <div className="export-complete">
          <h3>Export Complete</h3>
          <p>Bundle created: {bundlePath}</p>
          <a
            href={`/api/projects/${encodeURIComponent(projectId!)}/exports/latest/download`}
            download
            className="download-link"
          >
            Download Bundle
          </a>
        </div>
      )}
    </div>
  );
}
