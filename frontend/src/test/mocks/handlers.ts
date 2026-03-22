/**
 * MSW request handlers for the flywheel-planner API.
 * Provides mock responses for all API endpoints used by TanStack Query hooks.
 */
import { http, HttpResponse } from "msw";

function envelope<T>(data: T) {
  return { data, error: null, meta: {} };
}

const mockProject = {
  id: "proj_test1",
  name: "Test Project",
  status: "active" as const,
  workflow_state: "foundations",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

export const handlers = [
  // Projects
  http.get("/api/projects", () => {
    return HttpResponse.json(envelope([mockProject]));
  }),

  http.get("/api/projects/:projectId", ({ params }) => {
    return HttpResponse.json(
      envelope({ ...mockProject, id: params.projectId as string }),
    );
  }),

  http.post("/api/projects", async ({ request }) => {
    const body = (await request.json()) as { name: string };
    return HttpResponse.json(
      envelope({ ...mockProject, id: "proj_new", name: body.name }),
      { status: 201 },
    );
  }),

  // Workflow
  http.get("/api/projects/:projectId/workflow", () => {
    return HttpResponse.json(
      envelope({
        project: mockProject,
        stages: [],
        runs: [],
        pending_reviews: [],
      }),
    );
  }),

  // Artifacts
  http.get("/api/projects/:projectId/artifacts", () => {
    return HttpResponse.json(envelope([]));
  }),

  http.get("/api/artifacts/:artifactId/fragments", ({ params }) => {
    return HttpResponse.json(
      envelope({
        artifact_id: params.artifactId as string,
        fragments: [],
      }),
    );
  }),

  // Review items
  http.get("/api/projects/:projectId/review-items", () => {
    return HttpResponse.json(envelope([]));
  }),

  http.post("/api/projects/:projectId/reviews/bulk-decision", () => {
    return HttpResponse.json(envelope({ accepted: 0, rejected: 0 }));
  }),

  // Prompts
  http.get("/api/prompts", () => {
    return HttpResponse.json(envelope([]));
  }),

  // Models
  http.get("/api/models", () => {
    return HttpResponse.json(envelope([]));
  }),

  // Stage actions
  http.post("/api/projects/:projectId/stages/:stage/start", () => {
    return HttpResponse.json(envelope({ status: "started" }));
  }),

  // Exports
  http.post("/api/projects/:projectId/exports", () => {
    return HttpResponse.json(
      envelope({
        id: "exp_test1",
        status: "pending",
        created_at: "2026-01-01T00:00:00Z",
      }),
    );
  }),

  // Health
  http.get("/api/health", () => {
    return HttpResponse.json({ status: "ok" });
  }),
];
