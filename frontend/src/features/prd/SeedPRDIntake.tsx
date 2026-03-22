/**
 * SeedPRDIntake — Stage 2 seed PRD upload/paste UI.
 *
 * Accepts markdown via paste or file upload, previews rendered content,
 * shows quality assessment warnings, and submits to backend.
 */

import { useState, useCallback, type DragEvent, type ChangeEvent } from "react";

/** Quality warning from the backend assessment. */
interface QualityWarning {
  code: string;
  message: string;
}

/** Intake result from the backend. */
interface IntakeResult {
  input_id: string;
  detected_mime: string;
  encoding: string;
  normalization_status: string;
  warning_flags: string[];
}

type IntakeState = "empty" | "editing" | "preview" | "submitting" | "locked";

const ALLOWED_EXTENSIONS = [".md", ".markdown", ".txt"];
const MAX_FILE_SIZE = 5 * 1024 * 1024; // 5 MB

export default function SeedPRDIntake({ projectId }: { projectId: string }) {
  const [state, setState] = useState<IntakeState>("empty");
  const [content, setContent] = useState("");
  const [filename, setFilename] = useState("");
  const [warnings, setWarnings] = useState<QualityWarning[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);

  const handleFileSelect = useCallback(
    async (file: File) => {
      setError(null);

      // Validate extension.
      const ext = "." + file.name.split(".").pop()?.toLowerCase();
      if (!ALLOWED_EXTENSIONS.includes(ext)) {
        setError(
          `Invalid file type "${ext}". Accepted: ${ALLOWED_EXTENSIONS.join(", ")}`,
        );
        return;
      }

      // Validate size.
      if (file.size > MAX_FILE_SIZE) {
        setError(
          `File too large (${(file.size / 1024 / 1024).toFixed(1)} MB). Maximum: 5 MB.`,
        );
        return;
      }

      try {
        const text = await file.text();
        setContent(text);
        setFilename(file.name);
        setState("editing");
      } catch {
        setError("Failed to read file.");
      }
    },
    [],
  );

  const handleDrop = useCallback(
    (e: DragEvent<HTMLDivElement>) => {
      e.preventDefault();
      setIsDragOver(false);
      const file = e.dataTransfer.files[0];
      if (file) handleFileSelect(file);
    },
    [handleFileSelect],
  );

  const handleDragOver = useCallback((e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback(() => {
    setIsDragOver(false);
  }, []);

  const handleFileInput = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) handleFileSelect(file);
    },
    [handleFileSelect],
  );

  const handleSubmit = useCallback(async () => {
    if (!content.trim()) {
      setError("Please enter or upload PRD content.");
      return;
    }

    setState("submitting");
    setError(null);

    try {
      const response = await fetch(
        `/api/projects/${encodeURIComponent(projectId)}/prd-seed`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            content,
            source_type: filename ? "upload" : "paste",
            original_filename: filename,
          }),
        },
      );

      if (!response.ok) {
        const err = await response.json().catch(() => null);
        throw new Error(
          err?.error?.message || `Submit failed (${response.status})`,
        );
      }

      const result: IntakeResult = await response.json();

      // Map warning flags to display warnings.
      const displayWarnings: QualityWarning[] = (
        result.warning_flags || []
      ).map((flag) => ({
        code: flag,
        message: warningMessage(flag),
      }));

      setWarnings(displayWarnings);
      setState(displayWarnings.length > 0 ? "preview" : "locked");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Submission failed.");
      setState("editing");
    }
  }, [content, filename, projectId]);

  const handleAcknowledgeWarnings = useCallback(() => {
    setState("locked");
  }, []);

  return (
    <div className="seed-prd-intake">
      <h2>Seed PRD</h2>
      <p>
        Upload or paste your initial PRD. This will be expanded and refined
        through the multi-model workflow.
      </p>

      {error && (
        <div className="intake-error" role="alert">
          {error}
        </div>
      )}

      {(state === "empty" || state === "editing" || state === "submitting") ? (
        <>
          {/* Drop zone */}
          <div
            className={`drop-zone ${isDragOver ? "drop-zone--active" : ""}`}
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            role="button"
            tabIndex={0}
            aria-label="Drop a markdown file here or click to browse"
          >
            <p>Drag and drop a markdown file here, or</p>
            <label className="file-input-label">
              Browse files
              <input
                type="file"
                accept=".md,.markdown,.txt"
                onChange={handleFileInput}
                className="file-input-hidden"
              />
            </label>
            {filename && <p className="filename">Selected: {filename}</p>}
          </div>

          {/* Text area */}
          <div className="paste-area">
            <label htmlFor="prd-content">Or paste markdown directly:</label>
            <textarea
              id="prd-content"
              value={content}
              onChange={(e) => {
                setContent(e.target.value);
                setState("editing");
              }}
              rows={20}
              placeholder="# My Product Requirements Document&#10;&#10;## Overview&#10;&#10;Describe your product here..."
            />
          </div>

          <button
            onClick={handleSubmit}
            disabled={!content.trim() || state === "submitting"}
            className="submit-button"
          >
            {state === "submitting" ? "Submitting..." : "Submit Seed PRD"}
          </button>
        </>
      ) : null}

      {state === "preview" && warnings.length > 0 && (
        <div className="quality-warnings">
          <h3>Quality Assessment</h3>
          <p>
            The following advisory warnings were detected. You can proceed, but
            addressing them may improve results.
          </p>
          <ul>
            {warnings.map((w) => (
              <li key={w.code} className="warning-item">
                <strong>{w.code}</strong>: {w.message}
              </li>
            ))}
          </ul>
          <button onClick={handleAcknowledgeWarnings} className="submit-button">
            Acknowledge and Proceed
          </button>
        </div>
      )}

      {state === "locked" && (
        <div className="intake-locked">
          <p>Seed PRD accepted. Ready to proceed to Stage 3 (PRD Generation).</p>
        </div>
      )}
    </div>
  );
}

/** Maps warning flag codes to human-readable messages. */
function warningMessage(code: string): string {
  const messages: Record<string, string> = {
    few_sections:
      "Document has fewer than 3 sections. More structure helps models produce better output.",
    short_content:
      "Document is shorter than 500 characters. More detail typically yields better results.",
    missing_success_criteria:
      "No mention of success criteria found. Consider adding acceptance criteria.",
    missing_technical_constraints:
      "No mention of technical constraints found. Consider specifying limits and requirements.",
    missing_user_requirements:
      "No mention of user requirements found. Consider adding user stories or personas.",
    missing_scope_boundaries:
      "No mention of scope boundaries found. Consider defining what is in and out of scope.",
    unfilled_placeholders:
      "Template placeholders detected (TODO, TBD, etc.). Fill these in before proceeding.",
    embedded_html: "Embedded HTML detected. Pure markdown works best.",
    no_headings:
      "No markdown headings found. Use ## headings to structure your document.",
    long_lines:
      "Very long lines detected. This may indicate non-text content.",
    encoding_repair_needed:
      "Encoding issues detected. The document may contain non-UTF-8 characters.",
    very_short_content:
      "Document is very short. Consider adding more detail.",
  };
  return messages[code] || code;
}
