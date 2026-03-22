package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/go-chi/chi/v5"
)

// PromptHandler handles prompt template inspection endpoints.
type PromptHandler struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPromptHandler creates a new handler.
func NewPromptHandler(db *sql.DB, logger *slog.Logger) *PromptHandler {
	return &PromptHandler{db: db, logger: logger}
}

// Routes registers prompt endpoints on the router.
func (h *PromptHandler) Routes(r chi.Router) {
	r.Get("/", h.ListPrompts)
	r.Get("/{promptId}", h.GetPrompt)
}

// RunPromptRoutes registers the run-scoped prompt render endpoint.
func (h *PromptHandler) RunPromptRoutes(r chi.Router) {
	r.Get("/{runId}/prompt-render", h.GetPromptRender)
}

type promptResponse struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Stage                 string `json:"stage"`
	Version               int    `json:"version"`
	LockedStatus          string `json:"locked_status"`
	BaselineText          string `json:"baseline_text,omitempty"`
	WrapperText           string `json:"wrapper_text,omitempty"`
	OutputContractJSON    string `json:"output_contract_json,omitempty"`
	OriginalPRDBaseline   string `json:"original_prd_baseline_text,omitempty"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

// ListPrompts handles GET /api/prompts.
func (h *PromptHandler) ListPrompts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, stage, version, locked_status, created_at, updated_at
		 FROM prompt_templates WHERE deprecated_at IS NULL
		 ORDER BY name, version DESC`)
	if err != nil {
		h.logger.Error("listing prompts", "error", err)
		response.InternalError(w, "failed to list prompts")
		return
	}
	defer rows.Close()

	var prompts []promptResponse
	for rows.Next() {
		var p promptResponse
		if err := rows.Scan(&p.ID, &p.Name, &p.Stage, &p.Version,
			&p.LockedStatus, &p.CreatedAt, &p.UpdatedAt); err != nil {
			h.logger.Error("scanning prompt", "error", err)
			response.InternalError(w, "failed to read prompts")
			return
		}
		prompts = append(prompts, p)
	}

	if prompts == nil {
		prompts = []promptResponse{}
	}
	response.JSON(w, http.StatusOK, prompts)
}

// GetPrompt handles GET /api/prompts/:promptId.
func (h *PromptHandler) GetPrompt(w http.ResponseWriter, r *http.Request) {
	promptID := chi.URLParam(r, "promptId")

	var p promptResponse
	var origBaseline sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, name, stage, version, locked_status, baseline_text, wrapper_text,
		 output_contract_json, original_prd_baseline_text, created_at, updated_at
		 FROM prompt_templates WHERE id = ?`, promptID).
		Scan(&p.ID, &p.Name, &p.Stage, &p.Version, &p.LockedStatus,
			&p.BaselineText, &p.WrapperText, &p.OutputContractJSON,
			&origBaseline, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		response.NotFound(w, "prompt template not found")
		return
	}
	if origBaseline.Valid {
		p.OriginalPRDBaseline = origBaseline.String
	}

	response.JSON(w, http.StatusOK, p)
}

type promptRenderResponse struct {
	RunID              string          `json:"run_id"`
	PromptTemplateID   string          `json:"prompt_template_id"`
	RenderedPromptPath string          `json:"rendered_prompt_path"`
	RedactionStatus    string          `json:"redaction_status"`
	TemplateName       string          `json:"template_name"`
	TemplateVersion    int             `json:"template_version"`
	CreatedAt          string          `json:"created_at"`
}

// GetPromptRender handles GET /api/workflow-runs/:runId/prompt-render.
func (h *PromptHandler) GetPromptRender(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT pr.workflow_run_id, pr.prompt_template_id, pr.rendered_prompt_path,
		       pr.redaction_status, pt.name, pt.version, pr.created_at
		FROM prompt_renders pr
		JOIN prompt_templates pt ON pt.id = pr.prompt_template_id
		WHERE pr.workflow_run_id = ?
		ORDER BY pr.created_at ASC`, runID)
	if err != nil {
		h.logger.Error("querying prompt renders", "error", err)
		response.InternalError(w, "failed to query prompt renders")
		return
	}
	defer rows.Close()

	var renders []promptRenderResponse
	for rows.Next() {
		var pr promptRenderResponse
		if err := rows.Scan(&pr.RunID, &pr.PromptTemplateID, &pr.RenderedPromptPath,
			&pr.RedactionStatus, &pr.TemplateName, &pr.TemplateVersion, &pr.CreatedAt); err != nil {
			response.InternalError(w, "failed to read prompt renders")
			return
		}
		renders = append(renders, pr)
	}

	if renders == nil {
		renders = []promptRenderResponse{}
	}

	// Wrap in a consistent envelope.
	result := map[string]any{
		"run_id":  runID,
		"renders": renders,
	}
	response.JSON(w, http.StatusOK, result)
}

// --- JSON helper for raw message fields ---

func rawJSON(s string) json.RawMessage {
	if s == "" {
		return json.RawMessage("{}")
	}
	return json.RawMessage(s)
}
