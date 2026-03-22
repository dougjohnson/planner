/**
 * Unit tests for useApi hooks. Uses the global MSW server from test/setup.ts
 * with the mock handlers from test/mocks/handlers.ts.
 */
import { describe, it, expect } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement, type ReactNode } from "react";
import { useProjects, useProject, useWorkflowStatus, useCreateProject } from "./useApi";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

describe("useProjects", () => {
  it("returns typed project data from MSW mock", async () => {
    const { result } = renderHook(() => useProjects(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toHaveLength(1);
    expect(result.current.data![0].name).toBe("Test Project");
    // ID comes from the global mock handler in test/mocks/handlers.ts
    expect(result.current.data![0].id).toBe("proj_test1");
  });
});

describe("useProject", () => {
  it("returns single project", async () => {
    const { result } = renderHook(() => useProject("proj_test1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.name).toBe("Test Project");
  });
});

describe("useWorkflowStatus", () => {
  it("returns workflow status", async () => {
    const { result } = renderHook(() => useWorkflowStatus("proj_test1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    // Global mock returns empty stages — that's the initial project state.
    expect(result.current.data?.stages).toBeDefined();
  });
});

describe("useCreateProject", () => {
  it("calls POST /api/projects", async () => {
    const { result } = renderHook(() => useCreateProject(), {
      wrapper: createWrapper(),
    });

    result.current.mutate({ name: "New Project" });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.name).toBe("New Project");
  });
});
