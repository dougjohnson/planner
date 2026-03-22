import { useState, useCallback } from "react";
import { Link } from "react-router-dom";
import { useProjects } from "../../hooks/useApi";
import { Button } from "../../components/ui";
import { LoadingState, EmptyState, ErrorState } from "../../components/ui";

export default function ProjectList() {
  const { data: projects, isLoading, error, refetch } = useProjects();
  const [search, setSearch] = useState("");
  const [showArchived, setShowArchived] = useState(false);

  const handleArchive = useCallback(
    async (projectId: string) => {
      try {
        await fetch(`/api/projects/${encodeURIComponent(projectId)}/archive`, {
          method: "POST",
        });
        refetch();
      } catch {
        // Best effort.
      }
    },
    [refetch],
  );

  const handleResume = useCallback(
    async (projectId: string) => {
      try {
        await fetch(`/api/projects/${encodeURIComponent(projectId)}/resume`, {
          method: "POST",
        });
        refetch();
      } catch {
        // Best effort.
      }
    },
    [refetch],
  );

  if (isLoading) return <LoadingState message="Loading projects..." />;
  if (error)
    return (
      <ErrorState
        message="Failed to load projects."
        onRetry={() => refetch()}
      />
    );

  if (!projects || projects.length === 0) {
    return (
      <EmptyState
        title="No projects yet"
        description="Create your first project to start planning."
        action={
          <Link to="/projects/new">
            <Button>Create Project</Button>
          </Link>
        }
      />
    );
  }

  // Filter projects.
  const filtered = projects.filter((p) => {
    const matchesSearch =
      !search ||
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      "".toLowerCase().includes(search.toLowerCase());
    const matchesArchive = showArchived || p.status !== "archived";
    return matchesSearch && matchesArchive;
  });

  const archivedCount = projects.filter((p) => p.status === "archived").length;

  return (
    <div className="project-list">
      <div className="project-list-header">
        <h1>Projects</h1>
        <Link to="/projects/new">
          <Button>New Project</Button>
        </Link>
      </div>

      {/* Search and filter */}
      <div className="project-list-controls">
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search projects..."
          className="project-search"
          aria-label="Search projects"
        />
        {archivedCount > 0 && (
          <label className="archive-toggle">
            <input
              type="checkbox"
              checked={showArchived}
              onChange={(e) => setShowArchived(e.target.checked)}
            />
            Show archived ({archivedCount})
          </label>
        )}
      </div>

      {filtered.length === 0 ? (
        <p className="no-results">No projects match your search.</p>
      ) : (
        <ul className="project-cards">
          {filtered.map((project) => (
            <li key={project.id} className="project-card">
              <Link
                to={`/projects/${project.id}`}
                className="project-card-link"
              >
                <div className="project-card-info">
                  <strong className="project-name">{project.name}</strong>
                </div>
                <span
                  className={`project-status project-status--${project.status}`}
                >
                  {project.status}
                </span>
              </Link>
              <div className="project-card-actions">
                {project.status === "archived" ? (
                  <button
                    onClick={() => handleResume(project.id)}
                    className="action-link"
                  >
                    Resume
                  </button>
                ) : (
                  <button
                    onClick={() => handleArchive(project.id)}
                    className="action-link"
                  >
                    Archive
                  </button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
