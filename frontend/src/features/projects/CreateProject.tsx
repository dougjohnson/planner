import { useForm } from "react-hook-form";
import { useNavigate } from "react-router-dom";
import { useCreateProject } from "../../hooks/useApi";
import { Button } from "../../components/ui";
import styles from "./CreateProject.module.css";

interface CreateProjectForm {
  name: string;
  description: string;
  tech_stack: string;
  architecture_direction: string;
}

const KNOWN_STACKS = ["Go", "React", "TypeScript", "Python", "Rust", "Java"];

export default function CreateProject() {
  const navigate = useNavigate();
  const createProject = useCreateProject();

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<CreateProjectForm>({
    defaultValues: {
      name: "",
      description: "",
      tech_stack: "",
      architecture_direction: "",
    },
  });

  const onSubmit = async (data: CreateProjectForm) => {
    try {
      const project = await createProject.mutateAsync({ name: data.name });
      navigate(`/projects/${project.id}/foundations`);
    } catch {
      // Error is available via createProject.error
    }
  };

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Create Project</h1>

      <form onSubmit={handleSubmit(onSubmit)} className={styles.form}>
        <div className={styles.field}>
          <label htmlFor="name" className={styles.label}>
            Project Name <span className={styles.required}>*</span>
          </label>
          <input
            id="name"
            type="text"
            className={styles.input}
            placeholder="e.g., flywheel-planner"
            {...register("name", {
              required: "Project name is required",
              minLength: { value: 2, message: "Name must be at least 2 characters" },
            })}
          />
          {errors.name && (
            <p className={styles.error} role="alert">
              {errors.name.message}
            </p>
          )}
        </div>

        <div className={styles.field}>
          <label htmlFor="description" className={styles.label}>
            Description
          </label>
          <textarea
            id="description"
            className={styles.textarea}
            rows={3}
            placeholder="Brief description of the project..."
            {...register("description")}
          />
        </div>

        <div className={styles.field}>
          <label htmlFor="tech_stack" className={styles.label}>
            Tech Stack
          </label>
          <input
            id="tech_stack"
            type="text"
            className={styles.input}
            placeholder="e.g., Go, React, TypeScript"
            {...register("tech_stack")}
          />
          <div className={styles.suggestions}>
            {KNOWN_STACKS.map((stack) => (
              <button
                key={stack}
                type="button"
                className={styles.chip}
                onClick={(e) => {
                  const input = (e.target as HTMLElement)
                    .closest("div")
                    ?.parentElement?.querySelector("input");
                  if (input) {
                    const current = input.value;
                    if (!current.includes(stack)) {
                      input.value = current ? `${current}, ${stack}` : stack;
                      input.dispatchEvent(new Event("input", { bubbles: true }));
                    }
                  }
                }}
              >
                {stack}
              </button>
            ))}
          </div>
        </div>

        <div className={styles.field}>
          <label htmlFor="architecture_direction" className={styles.label}>
            Architecture Direction
          </label>
          <textarea
            id="architecture_direction"
            className={styles.textarea}
            rows={4}
            placeholder="Describe the architectural approach, constraints, key patterns..."
            {...register("architecture_direction")}
          />
        </div>

        {createProject.error && (
          <p className={styles.error} role="alert">
            Failed to create project. Please try again.
          </p>
        )}

        <div className={styles.actions}>
          <Button
            type="button"
            variant="secondary"
            onClick={() => navigate("/projects")}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Creating..." : "Create Project"}
          </Button>
        </div>
      </form>
    </div>
  );
}
