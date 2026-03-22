/**
 * Smoke tests verifying the test harness (MSW, providers, utilities).
 */
import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders, createMockSSE } from "./utils";
import { useProjects } from "../hooks/useApi";

function ProjectCount() {
  const { data, isLoading, error } = useProjects();
  if (isLoading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;
  return <div>Projects: {data?.length ?? 0}</div>;
}

describe("test harness", () => {
  it("renderWithProviders renders with router and query client", () => {
    renderWithProviders(<div>hello harness</div>);
    expect(screen.getByText("hello harness")).toBeInTheDocument();
  });

  it("MSW handlers serve mock data through TanStack Query hooks", async () => {
    renderWithProviders(<ProjectCount />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Projects: 1")).toBeInTheDocument();
    });
  });

  it("createMockSSE emits and receives events", () => {
    const sse = createMockSSE();
    const received: unknown[] = [];

    sse.addEventListener("message", (e: MessageEvent) => {
      received.push(JSON.parse(e.data));
    });

    sse.emit("workflow:stage_started", { stage: 3 });

    expect(received).toHaveLength(1);
    expect((received[0] as { type: string }).type).toBe(
      "workflow:stage_started",
    );
  });
});
