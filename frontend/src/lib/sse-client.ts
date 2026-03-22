/**
 * SSE client for real-time workflow events (§14.3, §14.4).
 *
 * Manages EventSource lifecycle with automatic reconnection and typed event
 * callbacks. Surfaces reconnect status for the UI.
 */

const BASE_URL = "/api";

/** SSE event types from §14.3. */
export type SSEEventType =
  | "workflow:stage_started"
  | "workflow:stage_completed"
  | "workflow:stage_failed"
  | "workflow:stage_blocked"
  | "workflow:run_started"
  | "workflow:run_retrying"
  | "workflow:run_failed"
  | "workflow:run_completed"
  | "workflow:run_progress"
  | "workflow:review_ready"
  | "workflow:loop_tick"
  | "workflow:state_changed"
  | "workflow:artifact_created"
  | "workflow:export_completed";

/** Parsed SSE event payload. */
export interface SSEEvent {
  type: SSEEventType;
  data: Record<string, unknown>;
  timestamp: string;
}

export type SSEEventHandler = (event: SSEEvent) => void;
export type SSEStatusHandler = (status: "connected" | "reconnecting" | "closed") => void;

/** Maximum reconnect delay in ms. */
const MAX_BACKOFF_MS = 30_000;

/**
 * Creates an SSE connection for a project's live events.
 * Returns a cleanup function to close the connection.
 */
export function connectSSE(
  projectId: string,
  onEvent: SSEEventHandler,
  onStatus?: SSEStatusHandler,
): () => void {
  let eventSource: EventSource | null = null;
  let reconnectAttempt = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let closed = false;

  function connect() {
    if (closed) return;

    const url = `${BASE_URL}/projects/${encodeURIComponent(projectId)}/events`;
    eventSource = new EventSource(url);

    eventSource.onopen = () => {
      reconnectAttempt = 0;
      onStatus?.("connected");
    };

    eventSource.onmessage = (e) => {
      try {
        const parsed: SSEEvent = JSON.parse(e.data);
        onEvent(parsed);
      } catch {
        // Ignore heartbeat comments or malformed events.
      }
    };

    eventSource.onerror = () => {
      eventSource?.close();
      eventSource = null;

      if (closed) return;

      onStatus?.("reconnecting");

      // Progressive backoff: 1s, 2s, 4s, 8s, ... up to MAX_BACKOFF_MS.
      const delay = Math.min(1000 * 2 ** reconnectAttempt, MAX_BACKOFF_MS);
      reconnectAttempt++;
      reconnectTimer = setTimeout(connect, delay);
    };
  }

  connect();

  // Cleanup function.
  return () => {
    closed = true;
    if (reconnectTimer) clearTimeout(reconnectTimer);
    eventSource?.close();
    eventSource = null;
    onStatus?.("closed");
  };
}
