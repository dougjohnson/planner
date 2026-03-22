# flywheel-planner

A local-first, single-user planning workbench that turns rough markdown PRDs into world-class implementation plans through a fixed, inspectable, multi-model workflow. Models interact with the system through structured tool calls rather than free-form text parsing, and documents are stored as versioned fragments rather than monolithic files.

## Why This Exists

Writing a thorough implementation plan is one of the highest-leverage activities in software engineering, yet most teams either skip it or produce plans that are vague, incomplete, or quickly outdated. The problem compounds when you involve LLMs: a single model produces blind spots that compound silently, and free-form text output is fragile to parse, diff, and version.

Flywheel Planner addresses this by orchestrating multiple model families (GPT-family and Opus-family) through a locked, multi-stage workflow where each model checks and improves the other's work. Every change is a validated tool call on an identified fragment, every version is recoverable, and every decision is inspectable. The result is a planning process that behaves like a rigorous editorial workbench rather than a loose prompt runner.

## Key Design Principles

- **Artifacts over chats.** The system centers on versioned documents, fragments, runs, prompts, decisions, and lineage -- not conversation threads.
- **Fixed workflow, flexible models.** The 17-stage workflow is locked and deterministic. Provider adapters are extensible.
- **Fragment-based storage.** PRDs and plans are stored as versioned fragments (one per `##` heading). Markdown is a derived view composed on demand. This eliminates the entire class of problems associated with text-matching edit application.
- **Tool-based model interaction.** Models submit all work through stage-specific tool calls (`submit_document`, `update_fragment`, `report_disagreement`, etc.) validated against schemas. No output-format parsing.
- **Human judgment stays in the loop.** Review checkpoints are explicit state transitions, not ad hoc interruptions. Every disagreement surfaces for user resolution.
- **Local-first, no cloud.** Everything persists locally. Only model API calls leave the machine.

## Architecture

```
Local User in Browser
       |
  React + TypeScript Frontend
       |  HTTP JSON + SSE
  Local Go API (127.0.0.1:7432)
       |
  +-----------+-----------+-----------+
  |           |           |           |
Workflow   Prompt      Document    Credential
Engine     Manager     Composer    Service
  |           |           |           |
  +-----+-----+     +----+----+     |
        |            |         |     |
  Model Provider   SQLite   Filesystem
  Layer (GPT +     (metadata  (artifacts,
   Opus adapters)  + fragments) raw payloads)
```

**Go backend** handles orchestration, durable state transitions, fragment management, file I/O, and provider integration. **React frontend** provides the dashboard, intake forms, artifact viewer, diff review, and prompt inspection. In production the frontend is served as embedded static assets from the Go binary, making the app behave like a single local product.

### Tech Stack

**Backend:** Go 1.25+, chi/v5 (HTTP router), modernc.org/sqlite (pure-Go driver), goldmark (markdown parsing), slog (structured logging with credential redaction)

**Frontend:** React 19, TypeScript strict mode, Vite, React Router, TanStack Query, React Hook Form, Zod, CSS Modules, Vitest + MSW + Testing Library

**Storage:** SQLite with WAL journaling, foreign keys, busy timeout. Metadata and fragments in the database; raw model payloads and foundation files on the filesystem.

## The 17-Stage Workflow

The workflow is a fixed, inspectable pipeline that takes a rough PRD through parallel multi-model generation, cross-model synthesis, integration review, iterative improvement loops, and final export. The same pattern repeats for implementation plan generation.

| # | Stage | Category | Models | Description |
|---|-------|----------|--------|-------------|
| 1 | Foundations | foundations | -- | Project creation, stack selection, guide intake, AGENTS.md assembly |
| 2 | PRD Intake | intake | -- | Upload/paste seed PRD, structural quality assessment |
| 3 | Parallel PRD Generation | parallel | GPT + Opus | Each model generates a full expanded PRD from the seed |
| 4 | PRD Synthesis | synthesis | GPT | Extended-reasoning synthesis of competing PRDs |
| 5 | PRD Integration | integration | Opus | Reviews GPT synthesis, reports agreements/disagreements |
| 6 | Disagreement Review | review | -- | User resolves disputed changes with fragment-level context |
| 7 | PRD Review Pass | review_loop | GPT/Opus | Model reviews canonical PRD, proposes fragment operations |
| 8 | Commit Revisions | commit | -- | Applies validated fragment operations to canonical artifact |
| 9 | Loop Control | loop_control | -- | Iteration management, convergence detection, model rotation |
| 10-16 | Plan Pipeline | (mirrors 3-9) | GPT + Opus | Same pattern for implementation plan generation |
| 17 | Final Export | export | -- | Stabilization checks, bundle assembly, manifest generation |

### Workflow Engine Design

The engine enforces correctness through several interlocking mechanisms:

- **Transition table.** 22 legal stage-to-stage transitions with named guard conditions. No route, UI action, or backend job can bypass it.
- **Guard conditions.** 11 named checks (`foundationsApproved`, `parallelQuorumSatisfied`, `loopConverged`, etc.) that query current state and return pass/fail with reasons.
- **Stage preflight.** Before any model-backed stage begins, a deterministic check verifies required artifacts exist, providers are enabled, prompt templates are present, and no blocking review items remain. Failures produce actionable remediation guidance.
- **Status state machines.** Stage statuses (8 states) and run statuses (8 states) with explicit transition tables that reject illegal moves.
- **Idempotent commands.** Every mutating request accepts an idempotency key. Duplicate submissions return the original outcome.
- **Crash recovery.** On startup, all runs left in "running" status from a prior crash are marked "interrupted" before any new work is accepted.

### Tool-Based Model Interaction

Models never produce free-form text that must be parsed. Instead, each stage provides specific tools:

| Stage Type | Tools |
|-----------|-------|
| Generation (3, 10) | `submit_document` |
| Synthesis (4, 11) | `submit_document`, `submit_change_rationale` |
| Integration (5, 12) | `submit_document`, `report_agreement`, `report_disagreement` |
| Review (7, 14) | `update_fragment`, `add_fragment`, `remove_fragment`, `submit_review_summary` |

Tool-call arguments are validated against JSON schemas. Fragment operations use simple ID lookups (does `fragment_id` exist in the canonical set?) rather than fragile text-matching. A bounded recovery ladder handles validation failures: targeted retry with specific error feedback, partial acceptance of valid calls, then user-guided retry with all raw attempts preserved.

### Provider Abstraction

The `Provider` interface normalizes both GPT-family and Opus-family providers behind a consistent contract:

- Capability flags (session continuity, file attachments, reasoning modes, context limits)
- Normalized error types (retryable vs. permanent, retry-after hints, provider identification)
- Tool-definition translation (canonical schemas to OpenAI function-calling or Anthropic tool-use format)
- Mock provider mode (`FLYWHEEL_MOCK_PROVIDERS=true`) with fixture-backed deterministic responses for full pipeline testing without API keys

## Fragment-Based Document Storage

Documents are not stored as monolithic markdown files. Instead, each `##` heading becomes a **fragment** -- an addressable, independently versioned section. A document is the set of fragment versions at their positions.

```
Markdown Document          Fragment Storage
==================         ================
## Introduction     -->    fragment "frag_001" v3
## Architecture     -->    fragment "frag_002" v5
## API Design       -->    fragment "frag_003" v2
## Testing          -->    fragment "frag_004" v1
```

**Decomposition** parses submitted markdown by `##` headings using goldmark's AST (correctly ignoring headings inside code blocks). **Composition** queries the `artifact_fragments` junction table and concatenates heading + content in position order. For review stages, composition adds `<!-- fragment:frag_XXX -->` annotations so models can reference fragment IDs directly.

This design eliminates the entire class of problems associated with text-matching edit application (`old_text` anchor validation, offset drift, conflict resolution). A fragment ID either exists or it doesn't -- validation is a simple lookup.

## Artifact Lineage and Versioning

Every artifact in the system participates in a directed acyclic lineage graph. When Stage 4 synthesizes multiple Stage 3 PRDs into a single document, the resulting artifact records `synthesized_from` relations pointing back to each input. When Stage 5 integrates the synthesis, it records `integrated_from`. When Stage 8 commits review changes, it records `revised_from`. This lineage is fully traversable: given any artifact, you can walk backward through the graph to the original seed document and see every intermediate version along the way.

Relation types capture the semantic intent of each derivation:

| Relation | Created By | Meaning |
|----------|-----------|---------|
| `decomposed_from` | Stages 3, 10 | Fragment-backed artifact derived from a submitted document |
| `synthesized_from` | Stages 4, 11 | Synthesis output merged from multiple parallel inputs |
| `integrated_from` | Stages 5, 12 | Integration output derived from a synthesis artifact |
| `resolved_from` | Stages 6, 13 | Artifact after user-resolved disagreements |
| `revised_from` | Stages 8, 15 | Artifact after review loop fragment operations |
| `rollback_of` | Rollback action | Artifact created by reverting to an earlier version |
| `diff_target` | Diff computation | Links both sides of a comparison |
| `export_includes` | Export bundle | Links export to all included canonical artifacts |

Artifacts are immutable -- once created, their content never changes. The `stream_heads` table tracks which artifact is the "current canonical" for each document stream (PRD stream, Plan stream). Promoting a new canonical is an atomic transaction that updates both the stream head pointer and the `is_canonical` flag in a single `BEGIN/COMMIT` block, preventing any state where two artifacts are simultaneously canonical for the same stream.

Auto-incrementing version labels (`prd.v01.seed`, `prd.v02.generated`, `prd.v03.synthesized`, `plan.v01.generated`, etc.) provide human-readable version identification alongside the opaque UUIDs.

## Prompt Assembly Pipeline

Every model invocation is built from a deterministic 6-segment prompt, assembled in the exact order specified by the architecture:

1. **System instructions** -- role definition, structured-output guidance, and stage-specific tool schemas
2. **Foundational context** -- project metadata, AGENTS.md content, tech stack, architecture direction
3. **Active prompt text** -- the canonical prompt template for this stage (versioned, locked after seeding)
4. **Artifact context** -- composed documents with `<!-- fragment:ID -->` annotations, enabling models to reference sections by ID in their tool calls
5. **Loop change history** -- a structured summary of what changed in prior iterations (empty for the first iteration; progressively richer for subsequent ones)
6. **User guidance injections** -- advisory text or decision records the user has submitted to steer the model's behavior

Each segment is preserved as a distinct section in the rendered prompt snapshot, so users can inspect exactly how every model invocation was assembled. The rendered snapshot links back to the prompt template version and references all artifact IDs included in the context.

A context budget estimator runs before each model call, computing per-segment token estimates using character-based heuristics and accounting for tool-definition overhead and response reserves. If the assembled prompt exceeds the target model's context window, the system surfaces diagnostic output identifying which segments contribute the most tokens and blocks the stage rather than silently truncating content.

## Parallel Orchestration and Quorum

Stages 3 and 10 (parallel generation) use a shared orchestration layer that prevents code duplication between the PRD and plan pipelines. The orchestrator:

1. Queries the provider registry for all enabled providers
2. Assembles a provider-specific prompt for each (using the prompt assembly pipeline)
3. Translates tool definitions to each provider's native format (OpenAI function-calling or Anthropic tool-use)
4. Submits all requests to a bounded worker pool for concurrent execution
5. Collects results as they complete via channel-based aggregation
6. Evaluates quorum once all providers have finished or timed out

**Quorum enforcement** ensures model diversity. The default policy requires at least one GPT-family success AND one Opus-family success. This is the core value proposition -- cross-model comparison catches the blind spots that any single model produces. If quorum cannot be met (e.g., both Anthropic runs fail), the orchestrator records all failures and surfaces them to the user rather than proceeding with incomplete data.

The worker pool uses a semaphore pattern for bounded concurrency (default 4 simultaneous model executions). Each execution publishes SSE events (`run_started`, `run_completed`, `run_failed`) so the dashboard timeline updates in real time. Per-provider timeouts (default 5 minutes) and an overall orchestration timeout (default 15 minutes) prevent indefinite hangs.

Partial failures are handled gracefully: if 3 of 4 providers succeed and quorum is met, the orchestrator returns success with the 3 submissions plus a recorded failure for the 4th. The user can see what failed and decide whether to retry.

## Review Loop Mechanics

Stages 7-9 (PRD review loop) and 14-16 (plan review loop) implement an iterative improvement cycle. The loop engine is parameterized by document type, so the same infrastructure handles both PRD and plan review without code duplication.

**Model rotation.** By default, GPT handles review iterations, but the loop engine schedules one Opus review pass at the midpoint to reintroduce model diversity. For a 4-iteration loop: GPT, GPT, Opus, GPT. This prevents single-model bias from compounding across iterations while keeping API costs manageable.

**Convergence detection.** If a review pass produces zero fragment operations (the model finds nothing to change), the system offers an early exit rather than forcing the remaining iterations. Convergence is tracked per-loop, and the user can accept the early exit or inject additional guidance and continue. This prevents late-loop regressions where models start tinkering with already-good prose.

**Change history injection.** Each iteration after the first receives a structured summary of what changed in prior iterations. This prevents the model from undoing previous improvements or re-proposing rejected changes. The change history is formatted as markdown and included as a distinct segment in the prompt.

**Fragment commit pipeline.** After each review pass, validated fragment operations (update, add, remove) are applied to the canonical artifact in a single transaction. A new artifact is created from the updated fragment set with correct position ordering, and it is promoted to canonical. If zero operations were proposed, the prior canonical is preserved unchanged and the stage classifies as `no_changes_proposed`.

## Real-Time Event System

The backend publishes structured events for every meaningful state change. These events serve two purposes: real-time UI updates via Server-Sent Events (SSE), and a persistent audit trail via the `workflow_events` table.

14 event types cover the full lifecycle:

```
workflow:stage_started      workflow:run_started
workflow:stage_completed    workflow:run_retrying
workflow:stage_failed       workflow:run_failed
workflow:stage_blocked      workflow:run_completed
workflow:review_ready       workflow:run_progress
workflow:loop_tick          workflow:state_changed
workflow:artifact_created   workflow:export_completed
```

The SSE hub is keyed by project ID -- events for one project never reach subscribers of another. Multiple browser tabs can subscribe to the same project's event stream simultaneously. Heartbeat comments every 30 seconds keep idle connections healthy. Dead clients are auto-unregistered on write error or context cancellation.

On the frontend, SSE events selectively invalidate TanStack Query caches, so the dashboard, artifact viewer, and review workspace update automatically without polling. A `ConnectionStatus` indicator shows whether the SSE connection is active.

## Application Bootstrap

The application follows a strict startup sequence where each phase completes before the next begins:

1. **Configuration** -- parse environment variables and config file; determine data directory
2. **Logger** -- initialize structured JSON logging with credential-redaction pipeline (active before any credential access)
3. **Database** -- open SQLite with WAL pragmas; single-connection mode ensures all per-connection pragmas persist
4. **Migrations** -- execute all pending migrations; no queries are permitted until this completes
5. **Services** -- initialize artifact store, project repository, fragment store, composer, prompt repository
6. **Providers** -- create GPT and Opus adapters; register in provider registry; skip adapters with missing credentials (log warning, continue)
7. **Workflow engine** -- SSE hub, event publisher, stage definitions, transition table, guard conditions
8. **Prompt seeding** -- idempotent canonical prompt insertion (skips already-seeded templates)
9. **Crash recovery** -- mark any runs left in "running" status from a prior crash as "interrupted"
10. **HTTP server** -- mount all API endpoints, SSE endpoint, and embedded frontend; begin accepting requests
11. **Shutdown hooks** -- on SIGINT/SIGTERM, drain connections, stop worker pool, close database

This ordering ensures the credential-redacting logger is active before any credentials are loaded, migrations complete before any database queries, and interrupted runs are cleaned up before new workflow actions are accepted.

## Database Schema

SQLite stores all structured metadata and document fragments across 9 migrations:

| Migration | Tables | Purpose |
|-----------|--------|---------|
| 001 | projects, project_inputs, model_configs | Core project metadata |
| 002 | fragments, fragment_versions, document_streams, stream_heads | Fragment-based document storage |
| 003 | artifacts, artifact_relations, artifact_fragments | Immutable versioned artifacts with lineage |
| 004 | workflow_runs, workflow_events | Stage execution tracking and event timeline |
| 005 | review_items, review_decisions, guidance_injections | Review workflow and user steering |
| 006 | prompt_templates, prompt_renders | Versioned prompts with rendered snapshots |
| 007 | loop_configs, usage_records, credentials, exports | Supporting operational tables |
| 008 | idempotency_keys | Duplicate request prevention |

Runtime hardening: WAL journaling, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`. Write paths are kept atomic and narrow.

## Repository Layout

```
cmd/flywheel-planner/         Entry point
internal/
  api/                        HTTP server, middleware, SSE hub, response helpers
    handlers/                 Route handlers (projects, models, workflow)
    middleware/               Structured logging, security headers
    response/                 Standard JSON envelope
    sse/                      Server-sent events hub
  app/                        Config, bootstrap, data directory management
  artifacts/                  Filesystem artifact store with checksums
  db/                         SQLite connection, migration runner, queries
    migrations/sql/           Numbered SQL migration files
  documents/
    composer/                 Fragment-to-markdown composition
    decomposer/               Markdown-to-fragment decomposition
    fragments/                Fragment and version CRUD
  events/                     Domain event types
  export/                     Bundle assembly
  foundations/                Foundation artifact management
  logging/                    Credential-redacting slog handler
  markdown/                   Goldmark heading-level parser
  models/                     Provider interface, tool schemas, translation layer
    providers/                GPT, Opus, and mock provider adapters
    registry/                 Provider registry with health tracking
  prompts/                    Template repository, canonical seeding, assembly
  review/                     Review decisions, user guidance service
  security/
    credentials/              Env var + config file credential loading
    headers/                  Security header middleware
  testutil/                   Test DB factory, logger, fixtures, assertions
  workflow/
    engine/                   Stage orchestrator, worker pool, dispatch
    tools/                    Tool-call handlers (submit_document, fragment ops)
    guards.go                 Named guard conditions
    idempotency.go            Idempotent command store
    preflight.go              Stage preflight checks
    recovery.go               Startup crash recovery
    recovery_ladder.go        Tool-call validation and bounded retry
    run.go                    Workflow run persistence
    stages.go                 17 stage definitions
    status.go                 Stage/run status state machines
    transitions.go            Legal transition table
frontend/
  src/
    app/router/               React Router with code splitting
    components/ui/            Shared primitives (Button, Badge, Card, StatusChip, etc.)
    components/layout/        App shell layout
    features/                 Feature pages (projects, foundations, review, artifacts, etc.)
    hooks/                    TanStack Query hooks for all API endpoints
    lib/                      API client, SSE client
    services/                 Zod schemas for all DTOs
    test/                     Vitest + MSW test infrastructure
guides/canonical/             Built-in best-practice guides (Go, React)
prompts/                      Canonical and wrapper prompt text
templates/agents/             AGENTS.md assembly templates
migrations/                   Database migration files
tests/
  integration/                Cross-package integration tests
  e2e/                        End-to-end tests
  fixtures/                   Test data (seeds, fragments, mock responses)
```

## Getting Started

### Prerequisites

- Go 1.25+ with toolchain go1.25.5
- Node.js 18+ with npm
- No external database required (SQLite is embedded)

### Build and Run

```bash
# Backend
go build ./...
go test ./...

# Frontend
cd frontend
npm install
npm run dev          # Dev server with hot reload
npm run build        # Production build
npm run typecheck    # TypeScript validation
npm test             # Vitest test suite

# Run the application
go run ./cmd/flywheel-planner
# Opens at http://127.0.0.1:7432
```

### Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `FLYWHEEL_DATA_DIR` | `~/.flywheel-planner` | Data directory for database and artifacts |
| `FLYWHEEL_LISTEN_ADDR` | `127.0.0.1:7432` | HTTP server listen address |
| `FLYWHEEL_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `FLYWHEEL_OPENAI_API_KEY` | -- | OpenAI API key (or set in credentials.json) |
| `FLYWHEEL_ANTHROPIC_API_KEY` | -- | Anthropic API key (or set in credentials.json) |
| `FLYWHEEL_MOCK_PROVIDERS` | `false` | Use mock providers for testing |
| `FLYWHEEL_MOCK_SCENARIO` | `happy-path-prd` | Mock provider scenario selection |

Credentials are loaded from environment variables first, then from `~/.flywheel-planner/credentials.json` (created with `0600` permissions). The UI can also set credentials via the Settings screen. Keys are held in memory only and never written to the database, logs, or exports.

## Security

- Server binds to `127.0.0.1` only (loopback). No network exposure.
- Security headers on all responses: `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`.
- Structured logging with automatic credential redaction (API keys, tokens, passwords are scrubbed from all log output).
- Credential fields are never included in API responses, artifacts, prompt renders, or export bundles.
- Path traversal prevention on all file operations. Symlinks outside the managed data directory are rejected.
- Markdown upload limits with extension, MIME type, and size validation.

## Testing Strategy

The test infrastructure is designed to make backend tests fast, isolated, and comprehensive without requiring any external services.

**Test database factory.** `testutil.NewTestDB(t)` creates a fresh, migrated SQLite database per test with automatic cleanup. Every test gets an isolated schema -- no shared state, no ordering dependencies, no cleanup logic. The factory uses the same migration runner as production, so test schemas always match.

**Structured test logger.** `testutil.NewTestLogger(t)` writes JSON logs to `t.Log()` for per-test diagnostics. `NewLogCapture()` captures log entries for assertion, supporting `ContainsMessage()`, `ContainsAttr()`, and `MessagesAtLevel()` queries.

**Mock providers.** The `MockProvider` implements the full `Provider` interface with fixture-backed responses, scenario selection via `FLYWHEEL_MOCK_SCENARIO`, per-stage overrides, and deterministic tool-call outputs. Both GPT and Opus family variants are available, enabling full pipeline testing without API keys.

**Race detection.** All backend tests run under `go test -race`. The SSE hub, worker pool, parallel orchestrator, and provider registry have specific concurrent test scenarios that exercise real goroutine contention patterns.

**Integration tests.** Cross-package tests in `tests/integration/` exercise the full stack: HTTP request → API handler → business logic → SQLite persistence → SSE event publishing. These use the real chi router with `httptest.NewServer`, not mock handlers.

**E2E tests.** Playwright tests in `tests/e2e/` launch the Go backend with mock providers, wait for the health endpoint, and drive the browser. Page object models abstract screen interactions; an API helper enables programmatic setup/teardown.

## Export and Reproducibility

Export bundles are self-documenting zip archives containing:

- Canonical PRD and plan artifacts (composed markdown from fragments)
- Foundation artifacts (AGENTS.md, tech stack file, architecture direction, guides)
- A machine-readable manifest with reproducibility metadata:
  - Workflow definition version
  - Canonical artifact IDs and SHA-256 checksums
  - Prompt template versions used by each stage
  - Model/provider identifiers for each completed run
  - Loop iteration counts and convergence outcomes
  - Review decision totals (accepted/rejected per stage)
  - Execution timing metadata

The manifest makes it possible to understand exactly how the output was produced -- which models ran, which prompts they received, how many iterations of review occurred, and what the user decided at each checkpoint.

By default, exports include only canonical artifacts (the final refined versions). An explicit option adds raw intermediates: every model response, every prompt render, every fragment version, and every review item. This "full lineage" mode produces larger bundles but enables complete auditability.

## Design Rationale

Several design choices in Flywheel Planner are unconventional enough to warrant explanation.

**Why fragments instead of monolithic documents?** The most common failure mode in LLM-driven document editing is the "edit application" problem: a model says "replace this text with that text," and the system has to find and apply that change reliably. This sounds simple but breaks down quickly -- anchors drift, whitespace differs, context windows miss, and concurrent edits conflict. By decomposing documents into independently-addressable fragments at `##` heading boundaries, every model operation becomes a database lookup rather than a text search. The tradeoff is that fragment granularity is fixed (one per `##` heading), but in practice this produces 15-30 fragments per substantial document, which is both manageable for models and meaningful for versioning.

**Why tool calls instead of free-form text output?** When models produce structured output through tool calls, the system can validate every submission against a schema before accepting it. A `submit_document` call either provides a non-empty `content` field or it doesn't. An `update_fragment` call either references an existing fragment ID or it fails validation immediately. This eliminates the entire class of "parsing" bugs where the system tries to extract structure from prose. The bounded recovery ladder (retry → partial acceptance → user-guided retry) handles the remaining edge cases where models produce invalid tool calls.

**Why a fixed workflow instead of a flexible pipeline builder?** Flexibility sounds appealing but creates a massive testing surface and makes reasoning about correctness difficult. The 17-stage workflow is locked: the transition table, guard conditions, and preflight checks are all deterministic and exhaustively testable. The flexibility lives in the model layer (any provider, any model) and the prompt layer (wrapper variants), not in the workflow topology. This means the system behavior is predictable and inspectable even when the model outputs are not.

**Why dual-model orchestration?** Any single LLM has systematic blind spots. GPT and Opus produce characteristically different outputs -- different emphasis, different structural choices, different failure modes. By requiring both families to generate independently and then synthesizing, the system captures a broader range of perspectives. The integration stage (where Opus reviews GPT's synthesis) is where the most valuable feedback emerges: specific, fragment-level disagreements that surface assumptions the synthesis model missed.

**Why local-first?** Planning documents often contain proprietary product details, competitive analysis, and internal architecture decisions. Running the orchestration locally means the only data leaving the machine is the model API calls themselves. There is no account system, no cloud sync, no telemetry. The trade-off is that the application is single-user and single-machine, which is the correct scope for a planning workbench.

## License

Private. All rights reserved.
