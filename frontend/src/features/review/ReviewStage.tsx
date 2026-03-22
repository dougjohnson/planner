import { useState } from "react";
import { useParams } from "react-router-dom";
import { useReviewItems, useSubmitDecisions } from "../../hooks/useApi";
import {
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  LoadingState,
  ErrorState,
  StatusChip,
} from "../../components/ui";
import styles from "./ReviewStage.module.css";

type Decision = "accepted" | "rejected";

interface ReviewDecisionState {
  reviewItemId: string;
  decision: Decision;
  userNote: string;
}

type GuidanceMode = "advisory_only" | "decision_record";

export default function ReviewStage() {
  const { projectId = "", stage = "" } = useParams<{ projectId: string; stage: string }>();
  const { data: reviewItems, isLoading, error } = useReviewItems(projectId);
  const submitDecisions = useSubmitDecisions(projectId);

  const [decisions, setDecisions] = useState<Record<string, ReviewDecisionState>>({});
  const [guidanceText, setGuidanceText] = useState("");
  const [guidanceMode, setGuidanceMode] = useState<GuidanceMode>("advisory_only");
  const [submitting, setSubmitting] = useState(false);

  const pendingItems = reviewItems?.filter((item) => item.status === "pending");

  const setDecision = (itemId: string, decision: Decision) => {
    setDecisions((prev) => ({
      ...prev,
      [itemId]: { reviewItemId: itemId, decision, userNote: prev[itemId]?.userNote ?? "" },
    }));
  };

  const setNote = (itemId: string, note: string) => {
    setDecisions((prev) => ({
      ...prev,
      [itemId]: { ...prev[itemId], reviewItemId: itemId, userNote: note },
    }));
  };

  const allDecided = pendingItems?.every((item) => decisions[item.id]) ?? false;

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      await submitDecisions.mutateAsync(
        Object.values(decisions).map((d) => ({
          review_item_id: d.reviewItemId,
          action: d.decision,
          notes: d.userNote || undefined,
        })),
      );
    } catch {
      // Guidance text preserved on failure for recovery.
    } finally {
      setSubmitting(false);
    }
  };

  const handleBulkAction = (action: Decision) => {
    if (!pendingItems) return;
    const bulk: Record<string, ReviewDecisionState> = {};
    for (const item of pendingItems) {
      bulk[item.id] = { reviewItemId: item.id, decision: action, userNote: "" };
    }
    setDecisions(bulk);
  };

  if (isLoading) {
    return <LoadingState message="Loading review items..." />;
  }

  if (error) {
    return <ErrorState message={`Error loading review items: ${error.message}`} />;
  }

  if (!pendingItems || pendingItems.length === 0) {
    return (
      <div className={styles.emptyState}>
        <h1>Review: Stage {stage}</h1>
        <p>No disagreements to review. The integration pass produced full agreement.</p>
        <p>Proceeding to the review loop.</p>
      </div>
    );
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1>Review Disagreements</h1>
        <p>
          {pendingItems.length} disputed change{pendingItems.length !== 1 ? "s" : ""} to resolve.
          Review each item and accept or reject the proposed change.
        </p>
      </header>

      <div className={styles.bulkActions}>
        <Button size="sm" variant="secondary" onClick={() => handleBulkAction("accepted")}>
          Accept All
        </Button>
        <Button size="sm" variant="secondary" onClick={() => handleBulkAction("rejected")}>
          Reject All
        </Button>
      </div>

      <div className={styles.itemList} role="list" aria-label="Review items">
        {pendingItems.map((item) => (
          <Card key={item.id}>
            <CardHeader>
              <div className={styles.itemHeader}>
                <Badge variant={severityVariant(item.severity)}>{item.severity}</Badge>
                <span>{item.summary}</span>
                {item.fragment_id && (
                  <code className={styles.fragmentId}>{item.fragment_id}</code>
                )}
              </div>
            </CardHeader>
            <CardBody>
              {item.rationale && <p className={styles.rationale}>{item.rationale}</p>}

              {item.suggested_change && (
                <div className={styles.suggestedChange}>
                  <strong>Suggested change</strong>
                  <p>{item.suggested_change}</p>
                </div>
              )}

              <div
                className={styles.decisionGroup}
                role="group"
                aria-label={`Decision for ${item.summary}`}
              >
                <button
                  className={styles.acceptBtn}
                  onClick={() => setDecision(item.id, "accepted")}
                  aria-pressed={decisions[item.id]?.decision === "accepted"}
                >
                  Accept
                </button>
                <button
                  className={styles.rejectBtn}
                  onClick={() => setDecision(item.id, "rejected")}
                  aria-pressed={decisions[item.id]?.decision === "rejected"}
                >
                  Reject
                </button>
              </div>

              {decisions[item.id] && (
                <div className={styles.decisionConfirm}>
                  <StatusChip
                    status={decisions[item.id].decision === "accepted" ? "completed" : "failed"}
                  />
                  <label className={styles.noteLabel}>
                    Note (optional)
                    <textarea
                      className={styles.noteTextarea}
                      value={decisions[item.id]?.userNote ?? ""}
                      onChange={(e) => setNote(item.id, e.target.value)}
                      rows={2}
                      aria-label={`Note for ${item.summary}`}
                    />
                  </label>
                </div>
              )}
            </CardBody>
          </Card>
        ))}
      </div>

      <Card className={styles.guidanceSection}>
        <CardHeader>User Guidance</CardHeader>
        <CardBody>
          <p>Optionally provide guidance for the next workflow step.</p>
          <div className={styles.guidanceModes} role="radiogroup" aria-label="Guidance mode">
            <label>
              <input
                type="radio"
                name="guidanceMode"
                value="advisory_only"
                checked={guidanceMode === "advisory_only"}
                onChange={() => setGuidanceMode("advisory_only")}
              />
              <Badge variant="info">Advisory</Badge>
              Carried into next step
            </label>
            <label>
              <input
                type="radio"
                name="guidanceMode"
                value="decision_record"
                checked={guidanceMode === "decision_record"}
                onChange={() => setGuidanceMode("decision_record")}
              />
              <Badge variant="default">Decision Record</Badge>
              Explains your decisions
            </label>
          </div>
          <textarea
            className={styles.guidanceTextarea}
            value={guidanceText}
            onChange={(e) => setGuidanceText(e.target.value)}
            rows={4}
            placeholder="Enter guidance for the model in the next stage..."
            aria-label="Guidance text"
          />
        </CardBody>
      </Card>

      <div className={styles.submitBar}>
        <Button onClick={handleSubmit} disabled={!allDecided || submitting}>
          {submitting ? "Submitting..." : `Submit ${Object.keys(decisions).length} Decisions`}
        </Button>
        {!allDecided && (
          <p className={styles.submitWarning}>
            All items must have a decision before submitting.
          </p>
        )}
      </div>
    </div>
  );
}

function severityVariant(severity: string): "warning" | "error" | "info" {
  switch (severity) {
    case "major":
      return "error";
    case "moderate":
      return "warning";
    default:
      return "info";
  }
}
