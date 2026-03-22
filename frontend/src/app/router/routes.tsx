import { lazy, Suspense, type ReactNode } from "react";
import { Navigate, type RouteObject } from "react-router-dom";
import { RouteErrorBoundary } from "./ErrorBoundary";
import { AppShell } from "../../components/layout/AppShell";
import { LoadingState } from "../../components/ui";

// Route-level code splitting with lazy loading (§13.1.1).
const ProjectList = lazy(() => import("../../features/projects/ProjectList"));
const ProjectDashboard = lazy(
  () => import("../../features/projects/ProjectDashboard"),
);
const Foundations = lazy(
  () => import("../../features/foundations/Foundations"),
);
const ReviewStage = lazy(() => import("../../features/review/ReviewStage"));
const ArtifactDetail = lazy(
  () => import("../../features/artifacts/ArtifactDetail"),
);
const Prompts = lazy(() => import("../../features/prompts/Prompts"));
const ProjectExport = lazy(() => import("../../features/export/ProjectExport"));
const ModelsConfig = lazy(() => import("../../features/models/ModelsConfig"));
const PromptLibrary = lazy(
  () => import("../../features/prompts/PromptLibrary"),
);
const Settings = lazy(() => import("../../features/settings/Settings"));
const CreateProject = lazy(
  () => import("../../features/projects/CreateProject"),
);

function SuspenseWrapper({ children }: { children: ReactNode }) {
  return (
    <Suspense fallback={<LoadingState message="Loading..." />}>
      {children}
    </Suspense>
  );
}

function LazyRoute({ children }: { children: ReactNode }) {
  return (
    <RouteErrorBoundary>
      <SuspenseWrapper>{children}</SuspenseWrapper>
    </RouteErrorBoundary>
  );
}

function NotFound() {
  return (
    <div style={{ padding: "2rem" }}>
      <h2>Page not found</h2>
      <p style={{ color: "#78716c", marginTop: "0.5rem" }}>
        The page you are looking for does not exist.
      </p>
    </div>
  );
}

/**
 * Application route configuration matching §13.1.1.
 */
export const routes: RouteObject[] = [
  {
    path: "/",
    element: <AppShell />,
    children: [
      // Root redirects to project list.
      { index: true, element: <Navigate to="/projects" replace /> },

      // Project listing.
      {
        path: "projects",
        element: (
          <LazyRoute>
            <ProjectList />
          </LazyRoute>
        ),
      },

      // Project creation.
      {
        path: "projects/new",
        element: (
          <LazyRoute>
            <CreateProject />
          </LazyRoute>
        ),
      },

      // Project-scoped routes.
      {
        path: "projects/:projectId",
        children: [
          {
            index: true,
            element: (
              <LazyRoute>
                <ProjectDashboard />
              </LazyRoute>
            ),
          },
          {
            path: "foundations",
            element: (
              <LazyRoute>
                <Foundations />
              </LazyRoute>
            ),
          },
          {
            path: "review/:stage",
            element: (
              <LazyRoute>
                <ReviewStage />
              </LazyRoute>
            ),
          },
          {
            path: "artifacts/:artifactId",
            element: (
              <LazyRoute>
                <ArtifactDetail />
              </LazyRoute>
            ),
          },
          {
            path: "prompts",
            element: (
              <LazyRoute>
                <Prompts />
              </LazyRoute>
            ),
          },
          {
            path: "export",
            element: (
              <LazyRoute>
                <ProjectExport />
              </LazyRoute>
            ),
          },
        ],
      },

      // Global screens.
      {
        path: "models",
        element: (
          <LazyRoute>
            <ModelsConfig />
          </LazyRoute>
        ),
      },
      {
        path: "prompts",
        element: (
          <LazyRoute>
            <PromptLibrary />
          </LazyRoute>
        ),
      },
      {
        path: "settings",
        element: (
          <LazyRoute>
            <Settings />
          </LazyRoute>
        ),
      },

      // Catch-all not-found.
      { path: "*", element: <NotFound /> },
    ],
  },
];
