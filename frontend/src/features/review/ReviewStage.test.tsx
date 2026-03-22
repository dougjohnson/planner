import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "../../test/utils";
import ReviewStage from "./ReviewStage";

// Mock useParams to provide route params.
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ projectId: "p-1", stage: "prd_disagreement_review" }),
  };
});

// Mock the API hooks.
const mockSubmitDecisions = vi.fn();
vi.mock("../../hooks/useApi", () => ({
  useReviewItems: () => ({
    data: [
      {
        id: "ri-1",
        fragment_id: "frag_001",
        severity: "major",
        summary: "Missing error handling",
        rationale: "Architecture needs failure modes",
        suggested_change: "Add error handling section",
        status: "pending",
      },
      {
        id: "ri-2",
        fragment_id: "frag_002",
        severity: "minor",
        summary: "Typo in terminology",
        rationale: "Consistency",
        suggested_change: "Fix the term",
        status: "pending",
      },
    ],
    isLoading: false,
    error: null,
  }),
  useSubmitDecisions: () => ({
    mutateAsync: mockSubmitDecisions,
  }),
}));

describe("ReviewStage", () => {
  it("renders review items with severity badges", () => {
    renderWithProviders(<ReviewStage />);

    expect(screen.getByText("Missing error handling")).toBeInTheDocument();
    expect(screen.getByText("major")).toBeInTheDocument();
    expect(screen.getByText("Typo in terminology")).toBeInTheDocument();
    expect(screen.getByText("minor")).toBeInTheDocument();
  });

  it("renders fragment IDs", () => {
    renderWithProviders(<ReviewStage />);

    expect(screen.getByText("frag_001")).toBeInTheDocument();
    expect(screen.getByText("frag_002")).toBeInTheDocument();
  });

  it("shows suggested changes", () => {
    renderWithProviders(<ReviewStage />);

    expect(screen.getByText("Add error handling section")).toBeInTheDocument();
  });

  it("has accept and reject buttons for each item", () => {
    renderWithProviders(<ReviewStage />);

    const acceptButtons = screen.getAllByText("Accept");
    const rejectButtons = screen.getAllByText("Reject");
    expect(acceptButtons).toHaveLength(2);
    expect(rejectButtons).toHaveLength(2);
  });

  it("shows bulk action buttons", () => {
    renderWithProviders(<ReviewStage />);

    expect(screen.getByText("Accept All")).toBeInTheDocument();
    expect(screen.getByText("Reject All")).toBeInTheDocument();
  });

  it("has guidance mode radio buttons", () => {
    renderWithProviders(<ReviewStage />);

    expect(screen.getByText(/Advisory/)).toBeInTheDocument();
    expect(screen.getByText(/Decision Record/)).toBeInTheDocument();
  });

  it("has a guidance textarea", () => {
    renderWithProviders(<ReviewStage />);

    const textarea = screen.getByLabelText("Guidance text");
    expect(textarea).toBeInTheDocument();
  });

  it("disables submit until all decisions are made", () => {
    renderWithProviders(<ReviewStage />);

    const submitButton = screen.getByText(/Submit.*Decisions/);
    expect(submitButton).toBeDisabled();
  });

  it("enables submit after all items are decided", async () => {
    const user = userEvent.setup();
    renderWithProviders(<ReviewStage />);

    // Click Accept on both items.
    const acceptButtons = screen.getAllByText("Accept");
    await user.click(acceptButtons[0]);
    await user.click(acceptButtons[1]);

    const submitButton = screen.getByText(/Submit 2 Decisions/);
    expect(submitButton).toBeEnabled();
  });

  it("shows note textareas after decisions", async () => {
    const user = userEvent.setup();
    renderWithProviders(<ReviewStage />);

    const acceptButtons = screen.getAllByText("Accept");
    await user.click(acceptButtons[0]);

    expect(screen.getByLabelText(/Note for Missing error handling/)).toBeInTheDocument();
  });

  it("shows count of disputed changes in header", () => {
    renderWithProviders(<ReviewStage />);

    expect(screen.getByText(/2 disputed changes/)).toBeInTheDocument();
  });
});
