## Objective

Build a local-first planning workbench that transforms rough PRDs into comprehensive implementation plans through multi-model workflows.

## Target Users

Individual developers and small teams who need structured planning assistance.

## Functional Requirements

- Upload and edit PRD documents in markdown format
- Multi-model parallel document generation (GPT + Opus)
- Fragment-based version tracking for each document section
- Review workflow with accept/reject decisions
- Export bundles with all artifacts

## Non-Functional Requirements

- Single binary deployment with embedded frontend
- SQLite database with WAL for concurrent access
- Response time under 200ms for all non-model API calls
- Maximum 5MB upload size for markdown files

## Technical Constraints

- Go 1.25+ backend with chi router
- React 19 + TypeScript frontend
- Loopback-only HTTP binding (127.0.0.1)
- No cloud dependencies for core functionality

## Success Criteria

- User can create a project and run the full PRD workflow in under 30 minutes
- All model outputs are traceable to specific prompts and inputs
- Fragment-level diffs are accurate for documents up to 50 sections

## Scope Boundaries

In scope: PRD generation, plan generation, review workflow, export
Out of scope: Multi-user collaboration, cloud deployment, direct code generation

## Risks

- Model API rate limits may slow parallel generation
- Large documents (50+ sections) may exceed context windows
