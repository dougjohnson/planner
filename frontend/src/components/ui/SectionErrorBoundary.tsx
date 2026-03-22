import { Component, type ErrorInfo, type ReactNode } from "react";
import { ErrorState } from "./StateViews";

interface Props {
  children: ReactNode;
  /** Name of the section for error logging. */
  section?: string;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

/**
 * Section-level error boundary for high-risk panels (§13.6).
 * Unlike RouteErrorBoundary (which catches entire page failures), this wraps
 * individual sections like live event streams, diff rendering, or large previews.
 */
export class SectionErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error(
      `[SectionErrorBoundary:${this.props.section ?? "unknown"}]`,
      error,
      errorInfo,
    );
  }

  render() {
    if (this.state.hasError) {
      return (
        <ErrorState
          message={
            this.state.error?.message ?? "This section encountered an error."
          }
          onRetry={() => this.setState({ hasError: false, error: null })}
        />
      );
    }
    return this.props.children;
  }
}
