import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "../../test/utils";
import CreateProject from "./CreateProject";

describe("CreateProject", () => {
  it("renders all form fields", () => {
    renderWithProviders(<CreateProject />, {
      initialEntries: ["/projects/new"],
    });

    expect(screen.getByLabelText(/project name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/tech stack/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/architecture direction/i)).toBeInTheDocument();
  });

  it("shows validation error for empty name", async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateProject />, {
      initialEntries: ["/projects/new"],
    });

    await user.click(screen.getByRole("button", { name: /create project/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
  });

  it("renders tech stack suggestion chips", () => {
    renderWithProviders(<CreateProject />, {
      initialEntries: ["/projects/new"],
    });

    expect(screen.getByText("Go")).toBeInTheDocument();
    expect(screen.getByText("React")).toBeInTheDocument();
    expect(screen.getByText("TypeScript")).toBeInTheDocument();
  });

  it("has cancel button", () => {
    renderWithProviders(<CreateProject />, {
      initialEntries: ["/projects/new"],
    });

    expect(screen.getByRole("button", { name: /cancel/i })).toBeInTheDocument();
  });
});
