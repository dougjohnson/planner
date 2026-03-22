import { describe, it, expect } from "vitest";
import {
  ProjectSchema,
  StageSchema,
  WorkflowRunSchema,
  ReviewItemSchema,
  WorkflowStatusSchema,
  FragmentSchema,
  FragmentDetailSchema,
  ArtifactSchema,
  ModelConfigSchema,
  ExportSchema,
  SSEEventPayloadSchema,
} from "./schemas";

describe("ProjectSchema", () => {
  it("parses a valid project", () => {
    const data = {
      id: "proj_123",
      name: "Test Project",
      status: "active",
      workflow_state: "foundations",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    };
    expect(() => ProjectSchema.parse(data)).not.toThrow();
  });

  it("rejects missing required fields", () => {
    expect(() => ProjectSchema.parse({ id: "p1" })).toThrow();
  });

  it("rejects invalid status enum", () => {
    const data = {
      id: "p1",
      name: "Test",
      status: "deleted",
      workflow_state: "x",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    };
    expect(() => ProjectSchema.parse(data)).toThrow();
  });
});

describe("StageSchema", () => {
  it("parses a valid stage", () => {
    const result = StageSchema.parse({
      stage: 3,
      key: "parallel_prd_generation",
      status: "running",
      loop_iteration: 0,
      pending_review_count: 0,
    });
    expect(result.stage).toBe(3);
    expect(result.key).toBe("parallel_prd_generation");
  });
});

describe("WorkflowRunSchema", () => {
  it("parses a run with optional fields", () => {
    const result = WorkflowRunSchema.parse({
      id: "run_001",
      stage: 3,
      stage_key: "parallel_prd_generation",
      status: "completed",
      attempt: 1,
      model: "gpt-4",
      started_at: "2026-01-01T00:00:00Z",
      completed_at: "2026-01-01T00:01:00Z",
    });
    expect(result.model).toBe("gpt-4");
  });

  it("parses a run without optional fields", () => {
    const result = WorkflowRunSchema.parse({
      id: "run_002",
      stage: 3,
      stage_key: "test",
      status: "pending",
      attempt: 1,
    });
    expect(result.model).toBeUndefined();
  });
});

describe("ReviewItemSchema", () => {
  it("parses valid review item", () => {
    const result = ReviewItemSchema.parse({
      id: "ri_001",
      fragment_id: "frag_042",
      severity: "major",
      summary: "Missing error handling",
      rationale: "No failure modes",
      suggested_change: "Add error section",
      status: "pending",
    });
    expect(result.severity).toBe("major");
  });

  it("rejects invalid severity", () => {
    expect(() =>
      ReviewItemSchema.parse({
        id: "ri_001",
        fragment_id: "f",
        severity: "critical",
        summary: "s",
        rationale: "r",
        suggested_change: "c",
        status: "pending",
      }),
    ).toThrow();
  });
});

describe("WorkflowStatusSchema", () => {
  it("parses a complete workflow status", () => {
    const result = WorkflowStatusSchema.parse({
      project: {
        id: "p1",
        name: "Test",
        status: "active",
        workflow_state: "running",
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
      stages: [
        {
          stage: 1,
          key: "foundations",
          status: "completed",
          loop_iteration: 0,
          pending_review_count: 0,
        },
      ],
      runs: [],
      pending_reviews: [],
    });
    expect(result.stages).toHaveLength(1);
  });
});

describe("FragmentSchema", () => {
  it("parses a fragment with version", () => {
    const result = FragmentSchema.parse({
      fragment_id: "frag_001",
      heading: "Introduction",
      depth: 2,
      position: 0,
      current_version: {
        version_id: "v1",
        content: "Hello world",
        checksum: "abc123",
        created_at: "2026-01-01T00:00:00Z",
      },
      version_count: 3,
    });
    expect(result.heading).toBe("Introduction");
    expect(result.version_count).toBe(3);
  });
});

describe("FragmentDetailSchema", () => {
  it("parses artifact fragments", () => {
    const result = FragmentDetailSchema.parse({
      artifact_id: "art_123",
      fragments: [
        {
          fragment_id: "f1",
          heading: "Intro",
          depth: 2,
          position: 0,
          current_version: {
            version_id: "v1",
            content: "content",
            checksum: "abc",
            created_at: "2026-01-01T00:00:00Z",
          },
          version_count: 1,
        },
      ],
    });
    expect(result.fragments).toHaveLength(1);
  });
});

describe("ArtifactSchema", () => {
  it("parses a valid artifact", () => {
    const result = ArtifactSchema.parse({
      id: "art_001",
      version_label: "v01.seed",
      source_stage: "stage-2",
      is_canonical: true,
      created_at: "2026-01-01T00:00:00Z",
    });
    expect(result.is_canonical).toBe(true);
  });
});

describe("ModelConfigSchema", () => {
  it("parses model config", () => {
    const result = ModelConfigSchema.parse({
      id: "mc_001",
      provider: "openai",
      model_name: "gpt-4",
      validation_status: "valid",
      enabled: true,
    });
    expect(result.provider).toBe("openai");
  });
});

describe("ExportSchema", () => {
  it("parses export record", () => {
    const result = ExportSchema.parse({
      id: "exp_001",
      status: "completed",
      download_url: "/api/exports/exp_001/download",
      created_at: "2026-01-01T00:00:00Z",
    });
    expect(result.status).toBe("completed");
  });

  it("rejects invalid status", () => {
    expect(() =>
      ExportSchema.parse({
        id: "e1",
        status: "processing",
        created_at: "2026-01-01T00:00:00Z",
      }),
    ).toThrow();
  });
});

describe("SSEEventPayloadSchema", () => {
  it("parses SSE event", () => {
    const result = SSEEventPayloadSchema.parse({
      type: "workflow:stage_started",
      data: { stage: 3, project_id: "p1" },
      timestamp: "2026-01-01T00:00:00Z",
    });
    expect(result.type).toBe("workflow:stage_started");
  });
});
