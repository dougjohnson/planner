import styles from "./StatusChip.module.css";

type StatusType =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "blocked"
  | "cancelled"
  | "interrupted";

interface StatusChipProps {
  status: StatusType;
  label?: string;
}

const statusLabels: Record<StatusType, string> = {
  pending: "Pending",
  running: "Running",
  completed: "Completed",
  failed: "Failed",
  blocked: "Blocked",
  cancelled: "Cancelled",
  interrupted: "Interrupted",
};

export function StatusChip({ status, label }: StatusChipProps) {
  return (
    <span
      className={`${styles.chip} ${styles[status]}`}
      role="status"
      aria-label={label ?? statusLabels[status]}
    >
      <span className={styles.dot} aria-hidden="true" />
      {label ?? statusLabels[status]}
    </span>
  );
}
