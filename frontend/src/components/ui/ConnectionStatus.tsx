/**
 * ConnectionStatus — shows Live/Reconnecting/Offline indicator.
 * Placed near the workflow status on the project dashboard.
 */

import type { SSEStatus } from "@/hooks/useSSE";

interface ConnectionStatusProps {
  status: SSEStatus;
}

const statusConfig: Record<
  SSEStatus,
  { label: string; className: string; dot: string }
> = {
  connected: {
    label: "Live",
    className: "connection-status--connected",
    dot: "dot--green",
  },
  reconnecting: {
    label: "Reconnecting",
    className: "connection-status--reconnecting",
    dot: "dot--yellow",
  },
  closed: {
    label: "Offline",
    className: "connection-status--closed",
    dot: "dot--red",
  },
  idle: {
    label: "",
    className: "connection-status--idle",
    dot: "",
  },
};

export function ConnectionStatus({ status }: ConnectionStatusProps) {
  if (status === "idle") return null;

  const config = statusConfig[status];

  return (
    <span
      className={`connection-status ${config.className}`}
      role="status"
      aria-live="polite"
    >
      <span className={`dot ${config.dot}`} aria-hidden="true" />
      {config.label}
    </span>
  );
}

export default ConnectionStatus;
