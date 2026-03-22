/**
 * Zod schemas for API response DTOs (§14.5).
 * Types are derived via z.infer<> so compile-time and runtime types stay in sync.
 */
import { z } from "zod";

// --- Project ---

export const ProjectSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  status: z.enum(["active", "archived"]),
  workflow_state: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});

export type Project = z.infer<typeof ProjectSchema>;

// --- Stage ---

export const StageSchema = z.object({
  stage: z.number(),
  key: z.string(),
  status: z.string(),
  loop_iteration: z.number(),
  pending_review_count: z.number(),
});

export type Stage = z.infer<typeof StageSchema>;

// --- Workflow Run ---

export const WorkflowRunSchema = z.object({
  id: z.string(),
  stage: z.number(),
  stage_key: z.string(),
  status: z.string(),
  model: z.string().optional(),
  attempt: z.number(),
  started_at: z.string().optional(),
  completed_at: z.string().optional(),
  error_message: z.string().optional(),
});

export type WorkflowRun = z.infer<typeof WorkflowRunSchema>;

// --- Review Item ---

export const ReviewItemSchema = z.object({
  id: z.string(),
  fragment_id: z.string(),
  severity: z.enum(["minor", "moderate", "major"]),
  summary: z.string(),
  rationale: z.string(),
  suggested_change: z.string(),
  status: z.enum(["pending", "accepted", "rejected"]),
  decision_notes: z.string().optional(),
});

export type ReviewItem = z.infer<typeof ReviewItemSchema>;

// --- Workflow Status (§14.5) ---

export const WorkflowStatusSchema = z.object({
  project: ProjectSchema,
  stages: z.array(StageSchema),
  runs: z.array(WorkflowRunSchema),
  pending_reviews: z.array(ReviewItemSchema),
});

export type WorkflowStatus = z.infer<typeof WorkflowStatusSchema>;

// --- Fragment ---

export const FragmentVersionSchema = z.object({
  version_id: z.string(),
  content: z.string(),
  source_stage: z.string().optional(),
  source_run_id: z.string().optional(),
  checksum: z.string(),
  created_at: z.string(),
});

export type FragmentVersion = z.infer<typeof FragmentVersionSchema>;

export const FragmentSchema = z.object({
  fragment_id: z.string(),
  heading: z.string(),
  depth: z.number(),
  position: z.number(),
  current_version: FragmentVersionSchema,
  version_count: z.number(),
});

export type Fragment = z.infer<typeof FragmentSchema>;

export const FragmentDetailSchema = z.object({
  artifact_id: z.string(),
  fragments: z.array(FragmentSchema),
});

export type FragmentDetail = z.infer<typeof FragmentDetailSchema>;

// --- Artifact ---

export const ArtifactSchema = z.object({
  id: z.string(),
  version_label: z.string(),
  source_stage: z.string(),
  source_model: z.string().optional(),
  is_canonical: z.boolean(),
  created_at: z.string(),
});

export type Artifact = z.infer<typeof ArtifactSchema>;

// --- Prompt Template ---

export const PromptTemplateSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  stage: z.number(),
  locked: z.boolean(),
  created_at: z.string(),
  updated_at: z.string(),
});

export type PromptTemplate = z.infer<typeof PromptTemplateSchema>;

// --- Model Config ---

export const ModelConfigSchema = z.object({
  id: z.string(),
  provider: z.string(),
  model_name: z.string(),
  reasoning_mode: z.string().optional(),
  validation_status: z.string(),
  enabled: z.boolean(),
});

export type ModelConfig = z.infer<typeof ModelConfigSchema>;

// --- Export ---

export const ExportSchema = z.object({
  id: z.string(),
  status: z.enum(["pending", "completed", "failed"]),
  download_url: z.string().optional(),
  created_at: z.string(),
});

export type ExportRecord = z.infer<typeof ExportSchema>;

// --- SSE Event ---

export const SSEEventPayloadSchema = z.object({
  type: z.string(),
  data: z.record(z.string(), z.unknown()),
  timestamp: z.string(),
});

export type SSEEventPayload = z.infer<typeof SSEEventPayloadSchema>;
