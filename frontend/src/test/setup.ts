/**
 * Vitest setup file for the flywheel-planner frontend test suite.
 * Configures Testing Library matchers and MSW server lifecycle.
 */
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./mocks/server";

// Start MSW before all tests.
beforeAll(() => {
  server.listen({ onUnhandledRequest: "warn" });
});

// Reset handlers and cleanup DOM after each test.
afterEach(() => {
  server.resetHandlers();
  cleanup();
});

// Close MSW after all tests.
afterAll(() => {
  server.close();
});
