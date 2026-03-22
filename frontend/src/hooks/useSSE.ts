/**
 * useSSE hook — connects SSE events to TanStack Query cache invalidation.
 *
 * Establishes one SSE connection per active project dashboard (§13.3.1).
 * Events trigger selective cache invalidation to keep the UI in sync
 * without polling.
 */

import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { connectSSE, type SSEEvent, type SSEEventType } from "@/lib/sse-client";

/** Connection status for the SSE indicator. */
export type SSEStatus = "connected" | "reconnecting" | "closed" | "idle";

/**
 * Hook that manages an SSE connection for a project and invalidates
 * TanStack Query caches based on received events.
 *
 * @param projectId - The project to subscribe to. Pass undefined to skip.
 * @returns The current connection status.
 */
export function useSSE(projectId: string | undefined): SSEStatus {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<SSEStatus>("idle");

  useEffect(() => {
    if (!projectId) {
      setStatus("idle");
      return;
    }

    const cleanup = connectSSE(
      projectId,
      (event: SSEEvent) => {
        handleEvent(queryClient, projectId, event);
      },
      (newStatus) => {
        setStatus(newStatus);
      },
    );

    return cleanup;
  }, [projectId, queryClient]);

  return status;
}

/**
 * Maps SSE event types to TanStack Query cache invalidation actions.
 * Each event selectively invalidates only the relevant queries.
 */
function handleEvent(
  queryClient: ReturnType<typeof useQueryClient>,
  projectId: string,
  event: SSEEvent,
): void {
  const eventType = event.type as SSEEventType;

  switch (eventType) {
    case "workflow:state_changed":
      // Broad state change — invalidate project summary and workflow status.
      queryClient.invalidateQueries({ queryKey: ["project", projectId] });
      queryClient.invalidateQueries({
        queryKey: ["workflowStatus", projectId],
      });
      break;

    case "workflow:stage_started":
    case "workflow:stage_completed":
    case "workflow:stage_failed":
    case "workflow:stage_blocked":
      // Stage lifecycle — invalidate workflow status and artifact list.
      queryClient.invalidateQueries({
        queryKey: ["workflowStatus", projectId],
      });
      queryClient.invalidateQueries({ queryKey: ["artifacts", projectId] });
      break;

    case "workflow:run_started":
    case "workflow:run_retrying":
    case "workflow:run_failed":
    case "workflow:run_completed":
      // Run lifecycle — invalidate workflow status.
      queryClient.invalidateQueries({
        queryKey: ["workflowStatus", projectId],
      });
      break;

    case "workflow:run_progress":
      // Progress update — just invalidate the workflow status for live state.
      // Don't invalidate heavier queries to avoid flicker.
      queryClient.invalidateQueries({
        queryKey: ["workflowStatus", projectId],
      });
      break;

    case "workflow:review_ready":
      // Review ready — invalidate review items and show callout.
      queryClient.invalidateQueries({
        queryKey: ["reviewItems", projectId],
      });
      queryClient.invalidateQueries({
        queryKey: ["workflowStatus", projectId],
      });
      break;

    case "workflow:loop_tick":
      // Loop iteration — invalidate workflow status.
      queryClient.invalidateQueries({
        queryKey: ["workflowStatus", projectId],
      });
      break;

    case "workflow:artifact_created":
      // New artifact — invalidate artifact list.
      queryClient.invalidateQueries({ queryKey: ["artifacts", projectId] });
      break;

    case "workflow:export_completed":
      // Export done — invalidate project and artifacts.
      queryClient.invalidateQueries({ queryKey: ["project", projectId] });
      queryClient.invalidateQueries({ queryKey: ["artifacts", projectId] });
      break;
  }
}
