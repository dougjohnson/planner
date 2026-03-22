import { useParams, Link } from "react-router-dom";
import { useProject, useWorkflowStatus } from "../../hooks/useApi";
import {
  Card,
  CardHeader,
  CardBody,
  StatusChip,
  Badge,
  Button,
  LoadingState,
  ErrorState,
} from "../../components/ui";
import styles from "./ProjectDashboard.module.css";

const STAGE_LABELS: Record<string, string> = {
  foundations: "Foundations",
  prd_seed: "PRD Seed",
  parallel_prd_generation: "PRD Generation",
  prd_synthesis: "PRD Synthesis",
  prd_integration: "PRD Integration",
  prd_review_checkpoint: "PRD Review",
  prd_review: "PRD Review Loop",
  prd_loop_control: "PRD Loop Control",
  parallel_plan_generation: "Plan Generation",
  plan_synthesis: "Plan Synthesis",
  plan_integration: "Plan Integration",
  plan_review_checkpoint: "Plan Review",
  plan_review: "Plan Review Loop",
  plan_loop_control: "Plan Loop Control",
  final_stabilization: "Final Stabilization",
  export: "Export",
};

function stageLabel(key: string): string {
  return STAGE_LABELS[key] ?? key;
}

type ChipStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "blocked"
  | "cancelled"
  | "interrupted";

function mapStatus(status: string): ChipStatus {
  const mapping: Record<string, ChipStatus> = {
    not_started: "pending",
    ready: "pending",
    running: "running",
    completed: "completed",
    failed: "failed",
    blocked: "blocked",
    cancelled: "cancelled",
    interrupted: "interrupted",
    paused: "blocked",
    awaiting_user: "blocked",
    retryable_failure: "failed",
  };
  return mapping[status] ?? "pending";
}

interface StageActionProps {
  stageId: string;
  status: string;
  projectId: string;
}

function StageAction({ stageId, status, projectId }: StageActionProps) {
  const isReady = status === "ready";
  const isAwaitingUser = status === "awaiting_user";
  const isFailed = status === "failed" || status === "retryable_failure";

  if (stageId === "foundations" && isReady) {
    return (
      <Link to={`/projects/${projectId}/foundations`}>
        <Button size="sm">Set Up Foundations</Button>
      </Link>
    );
  }

  if (stageId === "prd_intake" && isReady) {
    return (
      <Link to={`/projects/${projectId}/prd-intake`}>
        <Button size="sm">Enter Seed PRD</Button>
      </Link>
    );
  }

  if ((stageId === "prd_disagreement_review" || stageId === "plan_disagreement_review") && isAwaitingUser) {
    return (
      <Link to={`/projects/${projectId}/review/${stageId}`}>
        <Button size="sm">Review Disagreements</Button>
      </Link>
    );
  }

  if (stageId === "final_export" && isReady) {
    return (
      <Link to={`/projects/${projectId}/export`}>
        <Button size="sm">Review & Export</Button>
      </Link>
    );
  }

  if (isFailed) {
    return (
      <Button size="sm" variant="secondary" onClick={() => {
        fetch(`/api/projects/${projectId}/workflow/stages/${stageId}/retry`, { method: "POST" });
      }}>
        Retry
      </Button>
    );
  }

  if (isReady) {
    return (
      <Button size="sm" onClick={() => {
        fetch(`/api/projects/${projectId}/workflow/stages/${stageId}/start`, { method: "POST" });
        window.location.reload();
      }}>
        Start Stage
      </Button>
    );
  }

  return null;
}

export default function ProjectDashboard() {
  const { projectId } = useParams<{ projectId: string }>();
  const { data: project, isLoading: loadingProject, error: projectError } = useProject(projectId!);
  const { data: workflow, isLoading: loadingWorkflow } = useWorkflowStatus(projectId!);

  if (loadingProject) return <LoadingState message="Loading project..." />;
  if (projectError) return <ErrorState message="Failed to load project." />;
  if (!project) return <ErrorState message="Project not found." />;

  return (
    <div className={styles.dashboard}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>{project.name}</h1>
          <div className={styles.meta}>
            <Badge variant={project.status === "active" ? "success" : "default"}>
              {project.status}
            </Badge>
            <span className={styles.id}>{project.id}</span>
          </div>
        </div>
        <div className={styles.actions}>
          <Link to={`/projects/${projectId}/foundations`}>
            <Button variant="secondary" size="sm">Foundations</Button>
          </Link>
          <Link to={`/projects/${projectId}/export`}>
            <Button variant="secondary" size="sm">Export</Button>
          </Link>
        </div>
      </div>

      <Card>
        <CardHeader>Workflow Timeline</CardHeader>
        <CardBody>
          {loadingWorkflow ? (
            <LoadingState message="Loading workflow..." />
          ) : !workflow?.stages?.length ? (
            <p className={styles.empty}>No stages started yet. Begin with Foundations.</p>
          ) : (
            <div className={styles.timeline}>
              {workflow.stages.map((stage) => (
                <div key={stage.key} className={styles.timelineItem}>
                  <StatusChip status={mapStatus(stage.status)} />
                  <div className={styles.stageInfo}>
                    <span className={styles.stageName}>
                      Stage {stage.stage}: {stageLabel(stage.key)}
                    </span>
                    {stage.loop_iteration > 0 && (
                      <Badge variant="info">Loop {stage.loop_iteration}</Badge>
                    )}
                    {stage.pending_review_count > 0 && (
                      <Link to={`/projects/${projectId}/review/${stage.key}`}>
                        <Badge variant="warning">
                          {stage.pending_review_count} reviews pending
                        </Badge>
                      </Link>
                    )}
                  </div>
                  <StageAction stageId={stage.key} status={stage.status} projectId={projectId!} />
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

      {workflow?.runs && workflow.runs.length > 0 && (
        <Card>
          <CardHeader>Recent Runs</CardHeader>
          <CardBody>
            <div className={styles.runList}>
              {workflow.runs.slice(0, 10).map((run) => (
                <div key={run.id} className={styles.runItem}>
                  <StatusChip status={mapStatus(run.status)} />
                  <span className={styles.runStage}>{stageLabel(run.stage_key)}</span>
                  {run.model && <Badge>{run.model}</Badge>}
                  <span className={styles.runMeta}>Attempt {run.attempt}</span>
                  {run.error_message && (
                    <span className={styles.runError}>{run.error_message}</span>
                  )}
                </div>
              ))}
            </div>
          </CardBody>
        </Card>
      )}

      {workflow?.pending_reviews && workflow.pending_reviews.length > 0 && (
        <Card>
          <CardHeader>
            Pending Reviews{" "}
            <Badge variant="warning">{workflow.pending_reviews.length}</Badge>
          </CardHeader>
          <CardBody>
            <div className={styles.reviewList}>
              {workflow.pending_reviews.map((item) => (
                <div key={item.id} className={styles.reviewItem}>
                  <Badge variant={
                    item.severity === "major" ? "error" :
                    item.severity === "moderate" ? "warning" : "default"
                  }>
                    {item.severity}
                  </Badge>
                  <span>{item.summary}</span>
                </div>
              ))}
            </div>
          </CardBody>
        </Card>
      )}
    </div>
  );
}
