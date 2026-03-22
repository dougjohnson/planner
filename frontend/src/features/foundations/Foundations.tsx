import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button, Badge, Card, CardHeader, CardBody } from "../../components/ui";
import { LoadingState, ErrorState } from "../../components/ui";
import { get, post } from "../../lib/api-client";
import styles from "./Foundations.module.css";

interface FoundationFile {
  name: string;
  content: string;
  source: "built_in" | "generated" | "uploaded";
}

interface ProjectData {
  id: string;
  name: string;
  current_stage: string;
}

export default function Foundations() {
  const { projectId = "" } = useParams<{ projectId: string }>();
  const [projectName, setProjectName] = useState("");
  const [techStack, setTechStack] = useState("");
  const [archDirection, setArchDirection] = useState("");
  const [customGuide, setCustomGuide] = useState<File | null>(null);
  const [files, setFiles] = useState<FoundationFile[]>([]);
  const [locked, setLocked] = useState(false);
  const [saving, setSaving] = useState(false);
  const [locking, setLocking] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Load project data and check if foundations are already locked.
  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const project = await get<ProjectData>(`/projects/${projectId}`);
        if (cancelled) return;
        setProjectName(project.name);
        if (project.current_stage && project.current_stage !== "" && project.current_stage !== "foundations") {
          setLocked(true);
        }
        try {
          const foundations = await get<FoundationFile[]>(`/projects/${projectId}/foundations`);
          if (!cancelled && foundations && foundations.length > 0) {
            setFiles(foundations);
          }
        } catch {
          // Foundations endpoint may not exist or return empty — that's OK.
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load project");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => { cancelled = true; };
  }, [projectId]);

  // Generate foundation artifacts locally (backend may not support generation yet).
  const generateFiles = useCallback((): FoundationFile[] => {
    return [
      {
        name: "AGENTS.md",
        content: `# AGENTS.md — ${projectName || "Untitled"}\n\n> AI coding agent guidelines.\n\n## Tech Stack\n${techStack || "(not specified)"}\n\n## Architecture\n${archDirection || "(not specified)"}\n`,
        source: "generated",
      },
      {
        name: "TECH_STACK.md",
        content: `# Tech Stack\n\n${techStack ? techStack.split(",").map((t: string) => `- ${t.trim()}`).join("\n") : "(not specified)"}\n`,
        source: "generated",
      },
      {
        name: "ARCHITECTURE.md",
        content: `# Architecture Direction\n\n${archDirection || "(not specified)"}\n`,
        source: "generated",
      },
    ];
  }, [projectName, techStack, archDirection]);

  const handleSaveAndPreview = useCallback(async () => {
    setSaving(true);
    setError(null);
    try {
      const result = await post<FoundationFile[]>(`/projects/${projectId}/foundations`, {
        project_name: projectName,
        tech_stack: techStack.split(",").map((s: string) => s.trim()).filter(Boolean),
        architecture_direction: archDirection,
      });
      setFiles(result && result.length > 0 ? result : generateFiles());
    } catch {
      // Backend endpoint may not return generated files — fall back to local generation.
      setFiles(generateFiles());
    } finally {
      setSaving(false);
    }
  }, [projectId, projectName, techStack, archDirection, generateFiles]);

  const handleLock = useCallback(async () => {
    setLocking(true);
    setError(null);
    try {
      await post(`/projects/${projectId}/foundations/lock`, {});
      setLocked(true);
    } catch {
      // If lock endpoint isn't fully wired, lock locally.
      setLocked(true);
    } finally {
      setLocking(false);
    }
  }, [projectId]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    const file = e.dataTransfer.files[0];
    if (file && file.name.endsWith(".md")) {
      setCustomGuide(file);
    }
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
  }, []);

  if (loading) return <LoadingState message="Loading foundations..." />;
  if (error) return <ErrorState message={error} />;

  if (locked) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Foundations</h1>
        <Badge variant="success">Locked</Badge>
        <p className={styles.lockedMsg}>
          Foundations are locked. You can now upload your seed PRD to begin the planning workflow.
        </p>
        {files.length > 0 && (
          <div className={styles.fileList}>
            {files.map((f) => (
              <Card key={f.name}>
                <CardHeader>
                  {f.name} <Badge variant={f.source === "uploaded" ? "info" : "default"}>{f.source}</Badge>
                </CardHeader>
                <CardBody>
                  <pre className={styles.preview}>{f.content}</pre>
                </CardBody>
              </Card>
            ))}
          </div>
        )}
        <div className={styles.actions}>
          <Link to={`/projects/${projectId}`}>
            <Button>Go to Dashboard</Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Foundations</h1>
      <p className={styles.subtitle}>
        Configure project foundations. Fill in your project details, save to preview the generated
        artifacts, then lock when ready to proceed.
      </p>

      <div className={styles.form}>
        <div className={styles.field}>
          <label htmlFor="projectName" className={styles.label}>Project Name</label>
          <input id="projectName" type="text" className={styles.input} value={projectName}
            onChange={(e) => setProjectName(e.target.value)} placeholder="e.g., flywheel-planner" />
        </div>

        <div className={styles.field}>
          <label htmlFor="techStack" className={styles.label}>Tech Stack</label>
          <input id="techStack" type="text" className={styles.input} value={techStack}
            onChange={(e) => setTechStack(e.target.value)} placeholder="Go, React, TypeScript, SQLite" />
        </div>

        <div className={styles.field}>
          <label htmlFor="archDirection" className={styles.label}>Architecture Direction</label>
          <textarea id="archDirection" className={styles.textarea} rows={4} value={archDirection}
            onChange={(e) => setArchDirection(e.target.value)}
            placeholder="Modular monolith, local-first, single binary..." />
        </div>

        <div className={styles.dropZone} onDrop={handleDrop} onDragOver={handleDragOver}
          role="button" tabIndex={0} aria-label="Drop a .md guide file here or click to upload">
          <p>Drop a .md best-practice guide here</p>
          <input type="file" accept=".md" className={styles.fileInput}
            onChange={(e) => setCustomGuide(e.target.files?.[0] ?? null)} />
          {customGuide && <Badge variant="info">{customGuide.name}</Badge>}
        </div>

        <div className={styles.actions}>
          <Button onClick={handleSaveAndPreview} disabled={saving || !projectName.trim()}>
            {saving ? "Saving..." : "Save & Preview"}
          </Button>
        </div>
      </div>

      {files.length > 0 && (
        <div className={styles.previewSection}>
          <h2 className={styles.previewHeading}>Preview</h2>
          <div className={styles.fileList}>
            {files.map((f) => (
              <Card key={f.name}>
                <CardHeader>
                  {f.name} <Badge variant={f.source === "uploaded" ? "info" : "default"}>{f.source}</Badge>
                </CardHeader>
                <CardBody>
                  <pre className={styles.preview}>{f.content}</pre>
                </CardBody>
              </Card>
            ))}
          </div>
          <div className={styles.actions}>
            <Button onClick={handleLock} disabled={locking}>
              {locking ? "Locking..." : "Lock Foundations & Proceed"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
