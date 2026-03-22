import { useState } from "react";
import { useParams } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import { useArtifactFragments } from "../../hooks/useApi";
import {
  Card,
  CardHeader,
  CardBody,
  Badge,
  Button,
  LoadingState,
  ErrorState,
} from "../../components/ui";
import styles from "./ArtifactDetail.module.css";

type ViewMode = "composed" | "fragments";

export default function ArtifactDetail() {
  const { artifactId } = useParams<{
    projectId: string;
    artifactId: string;
  }>();
  const [view, setView] = useState<ViewMode>("composed");

  const {
    data: fragmentDetail,
    isLoading,
    error,
    refetch,
  } = useArtifactFragments(artifactId!);

  if (isLoading) return <LoadingState message="Loading artifact..." />;
  if (error)
    return (
      <ErrorState
        message="Failed to load artifact."
        onRetry={() => refetch()}
      />
    );
  if (!fragmentDetail) return <ErrorState message="Artifact not found." />;

  // Compose markdown from fragments.
  const composedMarkdown = fragmentDetail.fragments
    .map((f) => {
      if (f.heading) {
        const prefix = "#".repeat(f.depth);
        return `${prefix} ${f.heading}\n\n${f.current_version.content}`;
      }
      return f.current_version.content;
    })
    .join("\n\n");

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Artifact: {artifactId}</h1>
        <div className={styles.viewToggle}>
          <Button
            variant={view === "composed" ? "primary" : "secondary"}
            size="sm"
            onClick={() => setView("composed")}
          >
            Composed
          </Button>
          <Button
            variant={view === "fragments" ? "primary" : "secondary"}
            size="sm"
            onClick={() => setView("fragments")}
          >
            Fragments ({fragmentDetail.fragments.length})
          </Button>
        </div>
      </div>

      {view === "composed" ? (
        <Card>
          <CardBody>
            <div className={styles.markdown}>
              <ReactMarkdown>{composedMarkdown}</ReactMarkdown>
            </div>
          </CardBody>
        </Card>
      ) : (
        <div className={styles.fragmentList}>
          {fragmentDetail.fragments.map((fragment) => (
            <Card key={fragment.fragment_id}>
              <CardHeader>
                <div className={styles.fragmentHeader}>
                  <span className={styles.fragmentHeading}>
                    {fragment.heading || "(preamble)"}
                  </span>
                  <div className={styles.fragmentMeta}>
                    <Badge>{fragment.fragment_id}</Badge>
                    <Badge variant="info">
                      {fragment.version_count} version
                      {fragment.version_count !== 1 ? "s" : ""}
                    </Badge>
                    {fragment.current_version.source_stage && (
                      <Badge variant="default">
                        {fragment.current_version.source_stage}
                      </Badge>
                    )}
                  </div>
                </div>
              </CardHeader>
              <CardBody>
                <div className={styles.fragmentContent}>
                  <ReactMarkdown>
                    {fragment.current_version.content}
                  </ReactMarkdown>
                </div>
                <div className={styles.versionInfo}>
                  <span className={styles.checksum}>
                    {fragment.current_version.checksum.slice(0, 8)}...
                  </span>
                  <span className={styles.timestamp}>
                    {fragment.current_version.created_at}
                  </span>
                </div>
              </CardBody>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
