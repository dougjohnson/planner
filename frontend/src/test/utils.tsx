/**
 * Test utilities for the flywheel-planner frontend.
 * Provides renderWithProviders and mock SSE helpers.
 */
import { type ReactNode } from "react";
import { render, type RenderOptions } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, type MemoryRouterProps } from "react-router-dom";

interface ProviderOptions {
  /** Initial route entries for MemoryRouter. */
  initialEntries?: MemoryRouterProps["initialEntries"];
  /** Custom QueryClient (creates a fresh one if omitted). */
  queryClient?: QueryClient;
}

/**
 * Renders a component wrapped in all required providers:
 * - MemoryRouter (configurable initial entries)
 * - QueryClientProvider (fresh client per test by default)
 */
export function renderWithProviders(
  ui: ReactNode,
  options: ProviderOptions & Omit<RenderOptions, "wrapper"> = {},
) {
  const { initialEntries = ["/"], queryClient, ...renderOptions } = options;

  const client =
    queryClient ??
    new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: 0 },
        mutations: { retry: false },
      },
    });

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={initialEntries}>
          {children}
        </MemoryRouter>
      </QueryClientProvider>
    );
  }

  return {
    ...render(ui, { wrapper: Wrapper, ...renderOptions }),
    queryClient: client,
  };
}

/**
 * Creates a mock SSE event source for testing real-time updates.
 */
export function createMockSSE() {
  const listeners: Record<string, Array<(e: MessageEvent) => void>> = {};
  let statusCallback: ((status: string) => void) | null = null;

  return {
    /** Emit a typed SSE event. */
    emit(type: string, data: Record<string, unknown>) {
      const event = new MessageEvent("message", {
        data: JSON.stringify({ type, data, timestamp: new Date().toISOString() }),
      });
      (listeners["message"] ?? []).forEach((fn) => fn(event));
    },

    /** Simulate connection close. */
    close() {
      statusCallback?.("closed");
    },

    /** Simulate connection error (triggers reconnect in real client). */
    error() {
      statusCallback?.("reconnecting");
    },

    /** Register a status listener. */
    onStatus(cb: (status: string) => void) {
      statusCallback = cb;
    },

    /** Register an event listener (matches EventSource API). */
    addEventListener(type: string, handler: (e: MessageEvent) => void) {
      if (!listeners[type]) listeners[type] = [];
      listeners[type].push(handler);
    },
  };
}
