import type { ReactNode } from "react";
import { Button } from "./Button";
import styles from "./StateViews.module.css";

export function LoadingState({ message = "Loading..." }: { message?: string }) {
  return (
    <div className={styles.state} role="status" aria-label={message}>
      <div className={styles.spinner} aria-hidden="true" />
      <p>{message}</p>
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className={styles.state}>
      <h3 className={styles.title}>{title}</h3>
      {description && <p className={styles.description}>{description}</p>}
      {action && <div className={styles.action}>{action}</div>}
    </div>
  );
}

export function ErrorState({
  message = "Something went wrong.",
  onRetry,
}: {
  message?: string;
  onRetry?: () => void;
}) {
  return (
    <div className={styles.state} role="alert">
      <p className={styles.errorText}>{message}</p>
      {onRetry && (
        <Button variant="secondary" onClick={onRetry}>
          Try again
        </Button>
      )}
    </div>
  );
}
