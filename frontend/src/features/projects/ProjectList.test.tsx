import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "../../test/utils";
import ProjectList from "./ProjectList";

describe("ProjectList", () => {
  it("shows loading state initially", () => {
    renderWithProviders(<ProjectList />);
    expect(screen.getByText(/loading projects/i)).toBeInTheDocument();
  });

  it("renders project list from MSW mock", async () => {
    renderWithProviders(<ProjectList />);

    await waitFor(() => {
      expect(screen.getByText("Test Project")).toBeInTheDocument();
    });
  });

  it("shows New Project button when projects exist", async () => {
    renderWithProviders(<ProjectList />);

    await waitFor(() => {
      expect(screen.getByRole("link", { name: /new project/i })).toBeInTheDocument();
    });
  });
});
