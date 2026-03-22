/**
 * TanStack Query hooks for flywheel-planner API endpoints.
 *
 * Query hooks use validated Zod schemas to ensure runtime type safety
 * on all API responses.
 */
import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryOptions,
} from "@tanstack/react-query";
import { get, post } from "../lib/api-client";
import {
  ProjectSchema,
  WorkflowStatusSchema,
  ArtifactSchema,
  FragmentDetailSchema,
  ReviewItemSchema,
  PromptTemplateSchema,
  ModelConfigSchema,
  type Project,
  type WorkflowStatus,
  type Artifact,
  type FragmentDetail,
  type ReviewItem,
  type PromptTemplate,
  type ModelConfig,
  type ExportRecord,
} from "../services/schemas";
import { z } from "zod";

// --- Query key factories ---

export const queryKeys = {
  projects: {
    all: ["projects"] as const,
    detail: (id: string) => ["projects", id] as const,
  },
  workflow: {
    status: (projectId: string) => ["workflow", projectId] as const,
  },
  artifacts: {
    list: (projectId: string) => ["artifacts", projectId] as const,
    detail: (artifactId: string) => ["artifacts", "detail", artifactId] as const,
    fragments: (artifactId: string) =>
      ["artifacts", "fragments", artifactId] as const,
  },
  reviews: {
    list: (projectId: string) => ["reviews", projectId] as const,
  },
  prompts: {
    all: ["prompts"] as const,
  },
  models: {
    all: ["models"] as const,
  },
  exports: {
    detail: (exportId: string) => ["exports", exportId] as const,
  },
} as const;

// --- Helper: validate response with Zod schema ---

function validated<T>(schema: z.ZodType<T>, data: unknown): T {
  return schema.parse(data);
}

// --- Query hooks ---

export function useProjects(
  options?: Omit<UseQueryOptions<Project[]>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.projects.all,
    queryFn: async () => {
      const data = await get<unknown[]>("/projects");
      return validated(z.array(ProjectSchema), data);
    },
    ...options,
  });
}

export function useProject(
  projectId: string,
  options?: Omit<UseQueryOptions<Project>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.projects.detail(projectId),
    queryFn: async () => {
      const data = await get<unknown>(`/projects/${projectId}`);
      return validated(ProjectSchema, data);
    },
    enabled: !!projectId,
    ...options,
  });
}

export function useWorkflowStatus(
  projectId: string,
  options?: Omit<UseQueryOptions<WorkflowStatus>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.workflow.status(projectId),
    queryFn: async () => {
      const data = await get<unknown>(`/projects/${projectId}/workflow`);
      return validated(WorkflowStatusSchema, data);
    },
    enabled: !!projectId,
    ...options,
  });
}

export function useArtifacts(
  projectId: string,
  options?: Omit<UseQueryOptions<Artifact[]>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.artifacts.list(projectId),
    queryFn: async () => {
      const data = await get<unknown[]>(`/projects/${projectId}/artifacts`);
      return validated(z.array(ArtifactSchema), data);
    },
    enabled: !!projectId,
    ...options,
  });
}

export function useArtifactFragments(
  artifactId: string,
  options?: Omit<UseQueryOptions<FragmentDetail>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.artifacts.fragments(artifactId),
    queryFn: async () => {
      const data = await get<unknown>(`/artifacts/${artifactId}/fragments`);
      return validated(FragmentDetailSchema, data);
    },
    enabled: !!artifactId,
    ...options,
  });
}

export function useReviewItems(
  projectId: string,
  options?: Omit<UseQueryOptions<ReviewItem[]>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.reviews.list(projectId),
    queryFn: async () => {
      const data = await get<unknown[]>(`/projects/${projectId}/review-items`);
      return validated(z.array(ReviewItemSchema), data);
    },
    enabled: !!projectId,
    ...options,
  });
}

export function usePromptTemplates(
  options?: Omit<UseQueryOptions<PromptTemplate[]>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.prompts.all,
    queryFn: async () => {
      const data = await get<unknown[]>("/prompts");
      return validated(z.array(PromptTemplateSchema), data);
    },
    ...options,
  });
}

export function useModels(
  options?: Omit<UseQueryOptions<ModelConfig[]>, "queryKey" | "queryFn">,
) {
  return useQuery({
    queryKey: queryKeys.models.all,
    queryFn: async () => {
      const data = await get<unknown[]>("/models");
      return validated(z.array(ModelConfigSchema), data);
    },
    ...options,
  });
}

// --- Mutation hooks ---

export function useCreateProject() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string }) =>
      post<Project>("/projects", body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.projects.all });
    },
  });
}

export function useStartStage(projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (stage: string) =>
      post<unknown>(`/projects/${projectId}/stages/${stage}/start`),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.workflow.status(projectId),
      });
    },
  });
}

export function useSubmitDecisions(projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (
      decisions: Array<{
        review_item_id: string;
        action: "accepted" | "rejected";
        notes?: string;
      }>,
    ) => post<unknown>(`/projects/${projectId}/reviews/bulk-decision`, { decisions }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.reviews.list(projectId),
      });
    },
  });
}

export function useCreateExport(projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body?: { include_intermediates?: boolean }) =>
      post<ExportRecord>(`/projects/${projectId}/exports`, body),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.exports.detail(data.id),
      });
    },
  });
}
