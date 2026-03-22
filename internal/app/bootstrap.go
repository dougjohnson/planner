package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/api/handlers"
	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
	"github.com/dougflynn/flywheel-planner/internal/db/queries"
	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/events"
	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/providers"
	"github.com/dougflynn/flywheel-planner/internal/models/registry"
	"github.com/dougflynn/flywheel-planner/internal/prompts/canonical"
	"github.com/dougflynn/flywheel-planner/internal/prompts/rendering"
	"github.com/dougflynn/flywheel-planner/internal/security/credentials"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/dougflynn/flywheel-planner/internal/workflow/engine"
	"github.com/dougflynn/flywheel-planner/internal/workflow/stages"
)

// Services holds all initialized application services.
type Services struct {
	DB              *sql.DB
	ProjectRepo     *queries.ProjectRepo
	FragmentStore   *fragments.Store
	Composer        *composer.Composer
	ArtifactStore   *artifacts.Store
	Assembler       *rendering.Assembler
	SSEHub          *sse.Hub
	EventPublisher  *events.Publisher
	ProjectHandler      *handlers.ProjectHandler
	WorkflowHandler     *handlers.WorkflowHandler
	FoundationsHandler  *handlers.FoundationsHandler
	Logger              *slog.Logger
}

// Bootstrap initializes all application services in the correct order per §6.5.
// All migrations must complete before any query or write operation.
func Bootstrap(ctx context.Context, cfg *Config, db *sql.DB, logger *slog.Logger) (*Services, error) {
	// Step 4: Run migrations — MUST complete before any queries.
	logger.Info("running database migrations")
	if err := migrations.Run(ctx, db, logger); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	logger.Info("migrations complete")

	// Step 6: Initialize core services.
	projectRepo := queries.NewProjectRepo(db)
	fragmentStore := fragments.NewStore(db)
	docComposer := composer.New(db)
	artifactStore := artifacts.NewStore(cfg.DataDir, logger)
	assembler := rendering.NewAssembler(db)

	// Step 8: Initialize workflow engine components.
	sseHub := sse.NewHub(logger)
	eventPublisher := events.NewPublisher(db, sseHub, logger)

	// Step 9: Seed canonical prompts (idempotent).
	logger.Info("seeding canonical prompts")
	if err := canonical.Seed(ctx, db, logger); err != nil {
		return nil, fmt.Errorf("seeding canonical prompts: %w", err)
	}

	// Step 10: Crash recovery — mark any runs left in "running" from a prior
	// crash as "interrupted" BEFORE new workflow actions are accepted (§6.5).
	interrupted, err := workflow.RecoverInterruptedRuns(ctx, db, logger)
	if err != nil {
		return nil, fmt.Errorf("crash recovery: %w", err)
	}
	if interrupted > 0 {
		logger.Warn("recovered interrupted runs from prior crash", "count", interrupted)
	}

	// Step 11: Initialize provider registry and register adapters.
	providerRegistry := registry.New(logger)
	credService := credentials.NewService(cfg.DataDir)

	if providers.IsMockMode() {
		logger.Info("mock provider mode enabled")
		providerRegistry.Register(providers.NewMockProvider(providers.MockConfig{
			Name:   models.ProviderOpenAI,
			Family: "gpt",
		}))
		providerRegistry.Register(providers.NewMockProvider(providers.MockConfig{
			Name:   models.ProviderAnthropic,
			Family: "opus",
		}))
	} else {
		if key, err := credService.Get(models.ProviderOpenAI); err == nil && key != "" {
			providerRegistry.Register(providers.NewOpenAIProvider(key))
		} else {
			logger.Warn("OpenAI provider not configured — skipping")
		}
		if key, err := credService.Get(models.ProviderAnthropic); err == nil && key != "" {
			providerRegistry.Register(providers.NewAnthropicProvider(key))
		} else {
			logger.Warn("Anthropic provider not configured — skipping")
		}
	}

	// Step 12: Create worker pool and parallel orchestrator.
	workerPool := engine.NewPool(providerRegistry, sseHub, logger, 4)
	parallelOrch := engine.NewParallelOrchestrator(workerPool, engine.DefaultQuorumPolicy(), logger)

	// Step 13: Build dispatcher and register stage handlers.
	dispatcher := engine.NewDispatcher(logger)
	snapshotCreator := artifacts.NewSnapshotCreator(db)

	// Register Stage 3 handler (parallel PRD generation).
	stage3Orch := stages.NewStage3Orchestrator(
		fragmentStore, snapshotCreator, artifactStore,
		parallelOrch, providerRegistry, assembler, logger,
	)
	dispatcher.Register("parallel_prd_generation", engine.StageHandlerFunc(
		func(ctx context.Context, projectID, workflowRunID string) error {
			// Load seed PRD content.
			var seedContent string
			err := db.QueryRowContext(ctx,
				`SELECT content_path FROM project_inputs
				 WHERE project_id = ? AND role = 'seed_prd'
				 ORDER BY created_at DESC LIMIT 1`, projectID,
			).Scan(&seedContent)
			if err != nil {
				return fmt.Errorf("loading seed PRD: %w", err)
			}

			// Load foundation context.
			var foundationCtx string
			rows, err := db.QueryContext(ctx,
				`SELECT content_path FROM project_inputs
				 WHERE project_id = ? AND role = 'foundation'
				 ORDER BY created_at ASC`, projectID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var content string
					if rows.Scan(&content) == nil {
						foundationCtx += content + "\n\n"
					}
				}
			}

			result, err := stage3Orch.Execute(ctx, projectID, workflowRunID, seedContent, foundationCtx)
			if err != nil {
				return err
			}
			if !result.QuorumMet {
				return fmt.Errorf("quorum not met: only %d artifacts created", len(result.ArtifactIDs))
			}

			// Advance stage on success.
			db.ExecContext(ctx,
				`UPDATE projects SET current_stage = 'prd_synthesis' WHERE id = ?`, projectID)
			return nil
		},
	))

	// Register Stage 4 handler (PRD synthesis — GPT).
	dispatcher.Register("prd_synthesis", engine.StageHandlerFunc(
		func(ctx context.Context, projectID, _ string) error {
			gptProvider := providerRegistry.Get(models.ProviderOpenAI)
			if gptProvider == nil {
				return fmt.Errorf("GPT provider not registered")
			}
			result, err := stages.ExecuteSynthesis(ctx, db, gptProvider,
				stages.PRDSynthesisConfig(), projectID, "", logger)
			if err != nil {
				return err
			}
			logger.Info("stage 4 complete", "artifact_id", result.ArtifactID)
			// Advance to integration.
			db.ExecContext(ctx,
				`UPDATE projects SET current_stage = 'prd_integration' WHERE id = ?`, projectID)
			return nil
		},
	))

	// Register Stage 5 handler (PRD integration — Opus).
	dispatcher.Register("prd_integration", engine.StageHandlerFunc(
		func(ctx context.Context, projectID, _ string) error {
			opusProvider := providerRegistry.Get(models.ProviderAnthropic)
			if opusProvider == nil {
				return fmt.Errorf("Opus provider not registered")
			}
			result, err := stages.ExecuteIntegration(ctx, db, opusProvider,
				stages.PRDIntegrationConfig(), projectID, "", logger)
			if err != nil {
				return err
			}
			logger.Info("stage 5 complete",
				"artifact_id", result.ArtifactID,
				"disagreements", result.DisagreementCount)
			// Advance based on disagreements.
			nextStage := "prd_review" // Stage 7 — skip Stage 6 if no disagreements
			if result.HasDisagreements {
				nextStage = "prd_disagreement_review" // Stage 6 — user review
			}
			db.ExecContext(ctx,
				`UPDATE projects SET current_stage = ? WHERE id = ?`, nextStage, projectID)
			return nil
		},
	))

	// Register Stage 7 handler (PRD review + commit + loop control).
	// This handler runs one review iteration: review → commit → check convergence.
	dispatcher.Register("prd_review", engine.StageHandlerFunc(
		func(ctx context.Context, projectID, _ string) error {
			// Determine which provider to use (model rotation at midpoint).
			// For simplicity in V1, use GPT for all review iterations.
			provider := providerRegistry.Get(models.ProviderOpenAI)
			if provider == nil {
				return fmt.Errorf("GPT provider not registered")
			}

			// Run review pass.
			reviewResult, err := stages.ExecuteReview(ctx, db, provider,
				stages.PRDReviewConfig("gpt"), projectID, "", logger)
			if err != nil {
				return err
			}

			logger.Info("stage 7 review complete",
				"operations", reviewResult.OperationCount,
				"no_changes", reviewResult.NoChanges)

			// Stage 8: Commit fragment operations.
			if !reviewResult.NoChanges && len(reviewResult.Operations) > 0 {
				// Find canonical artifact.
				var canonicalID string
				db.QueryRowContext(ctx,
					`SELECT id FROM artifacts WHERE project_id = ? AND artifact_type = 'prd' AND is_canonical = 1
					 ORDER BY created_at DESC LIMIT 1`, projectID,
				).Scan(&canonicalID)

				if canonicalID != "" {
					commitResult, err := workflow.CommitFragmentOperations(
						ctx, db, projectID, canonicalID,
						reviewResult.Operations, "prd_review", "", "prd")
					if err != nil {
						logger.Error("commit failed", "error", err)
					} else {
						logger.Info("stage 8 commit complete",
							"artifact_id", commitResult.ArtifactID,
							"updates", commitResult.UpdateCount)
					}
				}
			}

			// Stage 9: Loop control — for mock testing, advance to plan pipeline.
			// In production this would check iteration count and convergence.
			db.ExecContext(ctx,
				`UPDATE projects SET current_stage = 'parallel_plan_generation' WHERE id = ?`, projectID)

			return nil
		},
	))

	// Step 14: Build API handlers.
	projectHandler := handlers.NewProjectHandler(projectRepo, logger)
	workflowHandler := handlers.NewWorkflowHandler(db, eventPublisher, logger)
	workflowHandler.SetDispatcher(dispatcher)
	foundationsHandler := handlers.NewFoundationsHandler(db, logger)

	logger.Info("application bootstrap complete")

	return &Services{
		DB:              db,
		ProjectRepo:     projectRepo,
		FragmentStore:   fragmentStore,
		Composer:        docComposer,
		ArtifactStore:   artifactStore,
		Assembler:       assembler,
		SSEHub:          sseHub,
		EventPublisher:  eventPublisher,
		ProjectHandler:     projectHandler,
		WorkflowHandler:    workflowHandler,
		FoundationsHandler: foundationsHandler,
		Logger:             logger,
	}, nil
}
