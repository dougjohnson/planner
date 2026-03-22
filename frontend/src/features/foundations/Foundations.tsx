import { useCallback, useState } from "react";
import { useParams } from "react-router-dom";
import { Button, Badge, Card, CardHeader, CardBody } from "../../components/ui";
import { LoadingState } from "../../components/ui";
import styles from "./Foundations.module.css";

interface FoundationFile {
  name: string;
  content: string;
  source: "built_in" | "generated" | "uploaded";
}

export default function Foundations() {
  const { projectId } = useParams<{ projectId: string }>();
  const [projectName, setProjectName] = useState("");
  const [description, setDescription] = useState("");
  const [techStack, setTechStack] = useState("");
  const [archDirection, setArchDirection] = useState("");
  const [customGuide, setCustomGuide] = useState<File | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [locked, setLocked] = useState(false);
  const [files, setFiles] = useState<FoundationFile[]>([]);

  const handleGeneratePreview = useCallback(() => {
    setPreviewing(true);
    // Simulate generation — in production this calls the backend.
    const generated: FoundationFile[] = [
      {
        name: "AGENTS.md",
        content: `# AGENTS.md — ${projectName || "Untitled"}\n\n> AI coding agent guidelines.\n\n## Tech Stack\n${techStack || "(not specified)"}\n`,
        source: "generated",
      },
      {
        name: "TECH_STACK.md",
        content: `# Tech Stack\n\n${techStack ? techStack.split(",").map((t) => `- ${t.trim()}`).join("\n") : "(not specified)"}\n`,
        source: "generated",
      },
      {
        name: "ARCHITECTURE.md",
        content: `# Architecture Direction\n\n${archDirection || "(not specified)"}\n`,
        source: "generated",
      },
    ];
    setFiles(generated);
    setPreviewing(false);
  }, [projectName, techStack, archDirection]);

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

  if (locked) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Foundations</h1>
        <Badge variant="success">Locked</Badge>
        <p className={styles.lockedMsg}>
          Foundations are locked for project {projectId}. Proceed to the next stage.
        </p>
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
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Foundations</h1>
      <p className={styles.subtitle}>
        Configure project foundations for {projectId}. All fields are editable until locked.
      </p>

      <div className={styles.form}>
        <div className={styles.field}>
          <label htmlFor="projectName" className={styles.label}>Project Name</label>
          <input
            id="projectName"
            type="text"
            className={styles.input}
            value={projectName}
            onChange={(e) => setProjectName(e.target.value)}
            placeholder="e.g., flywheel-planner"
          />
        </div>

        <div className={styles.field}>
          <label htmlFor="description" className={styles.label}>Description</label>
          <textarea
            id="description"
            className={styles.textarea}
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Brief project description..."
          />
        </div>

        <div className={styles.field}>
          <label htmlFor="techStack" className={styles.label}>Tech Stack</label>
          <input
            id="techStack"
            type="text"
            className={styles.input}
            value={techStack}
            onChange={(e) => setTechStack(e.target.value)}
            placeholder="Go, React, TypeScript, SQLite"
          />
        </div>

        <div className={styles.field}>
          <label htmlFor="archDirection" className={styles.label}>Architecture Direction</label>
          <textarea
            id="archDirection"
            className={styles.textarea}
            rows={4}
            value={archDirection}
            onChange={(e) => setArchDirection(e.target.value)}
            placeholder="Modular monolith, local-first, single binary..."
          />
        </div>

        <div
          className={styles.dropZone}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          role="button"
          tabIndex={0}
          aria-label="Drop a .md guide file here or click to upload"
        >
          <p>Drop a .md best-practice guide here</p>
          <input
            type="file"
            accept=".md"
            className={styles.fileInput}
            onChange={(e) => setCustomGuide(e.target.files?.[0] ?? null)}
          />
          {customGuide && (
            <Badge variant="info">{customGuide.name}</Badge>
          )}
        </div>

        <div className={styles.actions}>
          <Button variant="secondary" onClick={handleGeneratePreview}>
            Generate Preview
          </Button>
        </div>
      </div>

      {previewing && <LoadingState message="Generating foundation artifacts..." />}

      {files.length > 0 && (
        <div className={styles.previewSection}>
          <h2 className={styles.previewHeading}>Preview</h2>
          <div className={styles.fileList}>
            {files.map((f) => (
              <Card key={f.name}>
                <CardHeader>
                  {f.name}{" "}
                  <Badge variant={f.source === "uploaded" ? "info" : "default"}>
                    {f.source}
                  </Badge>
                </CardHeader>
                <CardBody>
                  <pre className={styles.preview}>{f.content}</pre>
                </CardBody>
              </Card>
            ))}
          </div>
          <div className={styles.actions}>
            <Button onClick={() => setLocked(true)}>
              Lock Foundations
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
