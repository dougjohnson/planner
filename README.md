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

## License

Private. All rights reserved.
