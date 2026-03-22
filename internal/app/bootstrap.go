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
	"github.com/dougflynn/flywheel-planner/internal/prompts/canonical"
	"github.com/dougflynn/flywheel-planner/internal/prompts/rendering"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
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
	ProjectHandler  *handlers.ProjectHandler
	WorkflowHandler *handlers.WorkflowHandler
	Logger          *slog.Logger
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

	// Step 14: Build API handlers.
	projectHandler := handlers.NewProjectHandler(projectRepo, logger)
	workflowHandler := handlers.NewWorkflowHandler(db, eventPublisher, logger)

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
		ProjectHandler:  projectHandler,
		WorkflowHandler: workflowHandler,
		Logger:          logger,
	}, nil
}
