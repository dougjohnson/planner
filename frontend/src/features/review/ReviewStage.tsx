import { useState } from "react";
import { useParams } from "react-router-dom";
import { useReviewItems, useSubmitDecisions } from "../../hooks/useApi";
import { Badge, Button, Card, CardBody, CardHeader, StatusChip } from "../../components/ui";

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
    return <div role="status" aria-label="Loading review items">Loading review items...</div>;
  }

  if (error) {
    return <div role="alert">Error loading review items: {error.message}</div>;
  }

  if (!pendingItems || pendingItems.length === 0) {
    return (
      <div>
        <h1>Review: Stage {stage}</h1>
        <p>No disagreements to review. The integration pass produced full agreement.</p>
        <p>Proceeding to the review loop.</p>
      </div>
    );
  }

  return (
    <div>
      <header>
        <h1>Review Disagreements: Stage {stage}</h1>
        <p>
          {pendingItems.length} disputed change{pendingItems.length !== 1 ? "s" : ""} to resolve.
        </p>
      </header>

      <div style={{ display: "flex", gap: "8px", margin: "16px 0" }}>
        <Button onClick={() => handleBulkAction("accepted")}>Accept All</Button>
        <Button onClick={() => handleBulkAction("rejected")}>Reject All</Button>
      </div>

      <div role="list" aria-label="Review items">
        {pendingItems.map((item) => (
          <Card key={item.id} style={{ marginBottom: "16px" }}>
            <CardHeader>
              <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <Badge variant={severityVariant(item.severity)}>{item.severity}</Badge>
                <span>{item.summary}</span>
                {item.fragment_id && (
                  <code style={{ fontSize: "0.85em", color: "#666" }}>{item.fragment_id}</code>
                )}
              </div>
            </CardHeader>
            <CardBody>
              {item.rationale && (
                <p style={{ marginBottom: "8px", fontStyle: "italic" }}>{item.rationale}</p>
              )}
              {item.suggested_change && (
                <div
                  style={{
                    background: "#f0fdf4",
                    padding: "8px",
                    borderRadius: "4px",
                    marginBottom: "8px",
                  }}
                >
                  <strong>Suggested change:</strong>
                  <p>{item.suggested_change}</p>
                </div>
              )}

              <div
                role="group"
                aria-label={`Decision for ${item.summary}`}
                style={{ display: "flex", gap: "8px", marginTop: "8px" }}
              >
                <Button
                  onClick={() => setDecision(item.id, "accepted")}
                  aria-pressed={decisions[item.id]?.decision === "accepted"}
                  style={{
                    background: decisions[item.id]?.decision === "accepted" ? "#22c55e" : undefined,
                    color: decisions[item.id]?.decision === "accepted" ? "white" : undefined,
                  }}
                >
                  Accept
                </Button>
                <Button
                  onClick={() => setDecision(item.id, "rejected")}
                  aria-pressed={decisions[item.id]?.decision === "rejected"}
                  style={{
                    background: decisions[item.id]?.decision === "rejected" ? "#ef4444" : undefined,
                    color: decisions[item.id]?.decision === "rejected" ? "white" : undefined,
                  }}
                >
                  Reject
                </Button>
              </div>

              {decisions[item.id] && (
                <div style={{ marginTop: "8px" }}>
                  <StatusChip
                    status={decisions[item.id].decision === "accepted" ? "completed" : "failed"}
                  />
                  <label style={{ display: "block", marginTop: "4px" }}>
                    Note (optional):
                    <textarea
                      value={decisions[item.id]?.userNote ?? ""}
                      onChange={(e) => setNote(item.id, e.target.value)}
                      rows={2}
                      style={{ width: "100%", marginTop: "4px" }}
                      aria-label={`Note for ${item.summary}`}
                    />
                  </label>
                </div>
              )}
            </CardBody>
          </Card>
        ))}
      </div>

      <Card style={{ marginTop: "24px" }}>
        <CardHeader>User Guidance</CardHeader>
        <CardBody>
          <p>Optionally provide guidance for the next workflow step.</p>
          <div role="radiogroup" aria-label="Guidance mode" style={{ margin: "8px 0" }}>
            <label style={{ marginRight: "16px" }}>
              <input
                type="radio"
                name="guidanceMode"
                value="advisory_only"
                checked={guidanceMode === "advisory_only"}
                onChange={() => setGuidanceMode("advisory_only")}
              />{" "}
              <Badge variant="info">Advisory</Badge> Carried into next step
            </label>
            <label>
              <input
                type="radio"
                name="guidanceMode"
                value="decision_record"
                checked={guidanceMode === "decision_record"}
                onChange={() => setGuidanceMode("decision_record")}
              />{" "}
              <Badge variant="default">Decision Record</Badge> Explains your decisions
            </label>
          </div>
          <textarea
            value={guidanceText}
            onChange={(e) => setGuidanceText(e.target.value)}
            rows={4}
            placeholder="Enter guidance for the model in the next stage..."
            style={{ width: "100%" }}
            aria-label="Guidance text"
          />
        </CardBody>
      </Card>

      <div style={{ marginTop: "24px" }}>
        <Button onClick={handleSubmit} disabled={!allDecided || submitting}>
          {submitting ? "Submitting..." : `Submit ${Object.keys(decisions).length} Decisions`}
        </Button>
        {!allDecided && (
          <p style={{ color: "#b91c1c", marginTop: "8px" }}>
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
