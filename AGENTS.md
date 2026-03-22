# AGENTS.md — planner

> Guidelines for AI coding agents working in this codebase (Go backend + React/TypeScript frontend).

---

## RULE 0 - THE FUNDAMENTAL OVERRIDE PREROGATIVE

If I tell you to do something, even if it goes against what follows below, YOU MUST LISTEN TO ME. I AM IN CHARGE, NOT YOU.

---

## RULE NUMBER 1: NO FILE DELETION

**YOU ARE NEVER ALLOWED TO DELETE A FILE WITHOUT EXPRESS PERMISSION.** Even a new file that you yourself created, such as a test code file. You have a horrible track record of deleting critically important files or otherwise throwing away tons of expensive work. As a result, you have permanently lost any and all rights to determine that a file or folder should be deleted.

**YOU MUST ALWAYS ASK AND RECEIVE CLEAR, WRITTEN PERMISSION BEFORE EVER DELETING A FILE OR FOLDER OF ANY KIND.**

---

## Irreversible Git & Filesystem Actions — DO NOT EVER BREAK GLASS

1. **Absolutely forbidden commands:** `git reset --hard`, `git clean -fd`, `rm -rf`, or any command that can delete or overwrite code/data must never be run unless the user explicitly provides the exact command and states, in the same message, that they understand and want the irreversible consequences.
2. **No guessing:** If there is any uncertainty about what a command might delete or overwrite, stop immediately and ask the user for specific approval. "I think it's safe" is never acceptable.
3. **Safer alternatives first:** When cleanup or rollbacks are needed, request permission to use non-destructive options (`git status`, `git diff`, `git stash`, copying to backups) before ever considering a destructive command.
4. **Mandatory explicit plan:** Even after explicit user authorization, restate the command verbatim, list exactly what will be affected, and wait for a confirmation that your understanding is correct. Only then may you execute it—if anything remains ambiguous, refuse and escalate.
5. **Document the confirmation:** When running any approved destructive command, record (in the session notes / final response) the exact user text that authorized it, the command actually run, and the execution time. If that record is absent, the operation did not happen.

---

## Git Branch: ONLY Use `main`, NEVER `master`

**The default branch is `main`. The `master` branch exists only for legacy URL compatibility.**

- **All work happens on `main`** — commits, PRs, feature branches all merge to `main`
- **Never reference `master` in code or docs** — if you see `master` anywhere, it's a bug that needs fixing

**If you see `master` referenced anywhere:**
1. Update it to `main`
2. Ensure `master` is synchronized: `git push origin main:master`

---

## Toolchain: Go Backend

We only use **Go Modules** for the backend, NEVER any other package manager for Go code.

- **Version:** Go 1.25+ (check `go.mod` for exact version)
- **Toolchain:** `go1.25.5` (see `go.mod`)
- **Dependency versions:** Managed via `go.mod` / `go.sum`
- **Lockfile:** `go.sum` (auto-managed by `go mod`)

### Key Commands

```bash
go build ./...                    # Build all packages
go test ./...                     # Run all tests
go test ./... -race               # Run with race detector
go test ./pkg/analysis/... -v     # Verbose tests for specific package
go vet ./...                      # Static analysis
gofmt -w .                        # Format all Go files
go mod tidy                       # Clean up unused deps
```

---

### Module Management

- Never manually edit `go.sum`
- Use `go mod tidy` to clean up unused deps
- Dependencies are tracked in `go.mod` / `go.sum`

---

## Toolchain: React Frontend

The frontend uses **React with TypeScript** in strict mode and a modern build toolchain.

- **Framework:** React with functional components and hooks only
- **Language:** TypeScript in strict mode, ES modules
- **Build tool:** Vite (or equivalent — check `package.json`)
- **Package manager:** Check the project root for `package-lock.json` (npm), `yarn.lock` (yarn), or `pnpm-lock.yaml` (pnpm) and use the corresponding manager consistently
- **Lockfile:** Managed by the package manager — never manually edit

### Technology Defaults

Unless the project explicitly says otherwise, use these defaults:
- React Router for client-side routing
- TanStack Query for server-state fetching/caching
- Zod for runtime validation of external data
- React Hook Form for non-trivial forms
- Testing Library + Vitest/Jest for tests
- ESLint + Prettier with CI enforcement

**Do not introduce extra libraries without a clear reason. Every dependency adds cost.**

### Key Commands

```bash
npm run build                     # Build the frontend (or yarn/pnpm equivalent)
npm run dev                       # Start dev server
npm run lint                      # Run ESLint
npm run typecheck                 # Run TypeScript compiler check (tsc --noEmit)
npm test                          # Run tests
npx prettier --check .            # Check formatting
```

### Frontend Architecture

Organize by feature/domain, not by file type:

```
src/
  app/
    providers/
    router/
    store/
  features/
    auth/
      components/
      hooks/
      api/
      types/
      utils/
    dashboard/
    settings/
  components/
    ui/
    layout/
  hooks/
  lib/
  services/
  styles/
  test/
  types/
```

**Guidelines:**
- Put code close to where it is used
- Shared code should be truly shared; do not promote to a global folder too early
- Separate UI components, state logic, API logic, and pure utilities
- Avoid circular dependencies between feature modules
- Shared modules should not depend on feature modules

---

## Code Editing Discipline

### No Script-Based Changes

**NEVER** run a script that processes/changes code files in this repo. Brittle regex-based transformations create far more problems than they solve.

- **Always make code changes manually**, even when there are many instances
- For many simple changes: use parallel subagents
- For subtle/complex changes: do them methodically yourself

### No File Proliferation

If you want to change something or add a feature, **revise existing code files in place**.

**NEVER** create variations like:
- `mainV2.go` / `AppV2.tsx`
- `main_improved.go` / `App_improved.tsx`
- `main_enhanced.go` / `App_enhanced.tsx`

New files are reserved for **genuinely new functionality** that makes zero sense to include in any existing file. The bar for creating new files is **incredibly high**.

---

## Backwards Compatibility

We do not care about backwards compatibility—we're in early development with no users. We want to do things the **RIGHT** way with **NO TECH DEBT**.

- Never create "compatibility shims"
- Never create wrapper functions for deprecated APIs
- Just fix the code directly

---

## Compiler Checks (CRITICAL)

**After any substantive code changes, you MUST verify no errors were introduced:**

### Backend (Go)

```bash
# Build all packages
go build ./...

# Run static analysis
go vet ./...

# Verify formatting
gofmt -l .
```

### Frontend (React/TypeScript)

```bash
# Type-check without emitting
npm run typecheck        # or: npx tsc --noEmit

# Lint
npm run lint

# Verify formatting
npx prettier --check .

# Build
npm run build
```

If you see errors, **carefully understand and resolve each issue**. Read sufficient context to fix them the RIGHT way.

---

## Testing

### Testing Policy

Every change must be accompanied by appropriate tests.

**Backend:** Every package includes `_test.go` files alongside the implementation. Tests must cover:
- Happy path
- Edge cases (empty input, max values, boundary conditions)
- Error conditions

Cross-package integration tests live in the `tests/` directory.

**Frontend:** Tests live alongside the code they test (e.g., `UserCard.test.tsx` next to `UserCard.tsx`). Tests must cover:
- Critical user flows
- Business logic and transformations
- Risky edge cases
- Shared primitives and hooks

### Backend Unit Tests

Use the testify module for unit tests. Each test file should have a corresponding `_test.go` file with `TestXxx` functions.

```bash
# Run all tests across the project
go test ./...

# Run with output
go test ./... -v

# Run tests for a specific package
go test ./mypackage/... -v

# Run with race detector
go test ./... -race

# Run with coverage
go test ./... -cover

# Run a specific test
go test -run TestSpecificName ./pkg/...
```

### Frontend Unit & Component Tests

Use Testing Library + Vitest (or Jest) for frontend tests. Test user-visible behavior, not implementation details.

```bash
# Run all frontend tests
npm test

# Run in watch mode during development
npm run test:watch

# Run with coverage
npm run test:coverage

# Run a specific test file
npx vitest run src/features/auth/LoginForm.test.tsx
```

**Frontend testing rules:**
- Prefer tests that resemble real user interactions (Testing Library queries: `getByRole`, `getByLabelText`, etc.)
- Avoid brittle snapshot-heavy strategies
- Mock at network/API boundaries, not everywhere
- Keep tests deterministic and readable
- Every bug fix should consider whether a regression test is warranted
- Test loading, error, and empty states for async UI

### Playwright E2E Tests

Follow all practices in `PLAYWRIGHT_BEST_PRACTICES.md`. Playwright tests live in `tests/e2e/` and run against the real Go backend with `FLYWHEEL_MOCK_PROVIDERS=true`.

```bash
cd tests/e2e

# Run all E2E tests (auto-starts the Go server)
npx playwright test

# Run a specific spec file
npx playwright test first-use.spec.ts

# Run with visible browser for debugging
npx playwright test --headed

# View the HTML report
npx playwright show-report
```

**E2E testing rules:**
- Use accessible locators (`getByRole`, `getByLabel`, `getByText`) — never CSS selectors tied to styling classes
- Never assume an empty database — tests share state across runs
- Use `APIHelper` for test setup; only drive the UI when the UI itself is what you're testing
- Assert the absence of error states after every navigation (`getByRole('alert')` not visible)
- Never use `waitForTimeout` — rely on Playwright's auto-waiting with explicit timeout on assertions
- Handle duplicate data from prior runs (use `.first()` or unique names with `Date.now()`)
- Every user-visible bug fix should include a Playwright regression test

### Go Best Practices

Follow all practices in `GOLANG_BEST_PRACTICES.md`. Key points:

**Error Handling:**
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("loading config: %w", err)
}

// Check errors immediately after the call
result, err := doSomething()
if err != nil {
    return err
}
```

**Division Safety:**
```go
// Always guard against division by zero
if len(items) > 0 {
    avg := total / float64(len(items))
}
```

**Nil Checks:**
```go
// Check for nil before dereferencing
if dep != nil && dep.Type.IsBlocking() {
    // safe to use dep
}
```

**Concurrency:**
```go
// Use sync.RWMutex for shared state
mu.RLock()
value := sharedMap[key]
mu.RUnlock()

// Capture channels before unlock to avoid races
mu.RLock()
ch := someChannel
mu.RUnlock()
for item := range ch {
    // process
}
```

### React & TypeScript Best Practices

Follow all practices in `REACT_BEST_PRACTICES.md`. Key points:

**Component Design:**
- Keep components small, focused, and composable
- Prefer composition over inheritance or giant prop surfaces
- Separate presentational components from feature/container logic
- Do not bury API calls, routing, or business logic inside low-level UI components

**State Management:**
- Use the smallest state solution that fits (local state → lifted state → context → store → server state library)
- Do not store values that can be derived from existing state/props
- Use context sparingly — good for theme, auth, locale; bad for frequently changing granular UI state
- Use TanStack Query for server state

**Hooks:**
- Follow the Rules of Hooks without exception
- Keep custom hooks focused on a single concern
- Treat `useEffect` as an escape hatch — prefer computation during render or event handlers
- Include correct dependencies; do not suppress lint warnings without documented reason
- Clean up subscriptions/timers/listeners

**Data Fetching:**
- Keep API calls out of presentational components
- Validate external data at the boundary (use Zod schemas)
- Handle loading, empty, error, and stale states deliberately
- Convert backend data shapes into app-safe types near the API boundary

**TypeScript:**
```typescript
// Validate external data — never trust API shapes
const parsed = UserSchema.parse(await response.json());

// Use discriminated unions for state modeling
type RequestState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'success'; data: User }
  | { status: 'error'; error: Error };
```

**Error Handling:**
- Use error boundaries at route and component levels
- Every async flow must define what happens on failure
- Show user-friendly messages; log detailed diagnostics
- Prefer recoverable UI over blank screens

**Accessibility:**
- Use semantic HTML and native controls
- Every interactive element must be keyboard accessible
- Label inputs correctly; provide visible focus states
- Use ARIA only when native HTML is insufficient

---

## Picking Work — THE MANDATORY PROTOCOL

**This is the most important section for autonomous agents.** When you need to decide what to work on next, you MUST follow every step below in order. No skipping. No shortcuts.

### Step 1: Triage — Decide WHAT to work on

Run `bv --robot-triage` (or `bv --robot-next` for just the top pick). This is your **single entry point** for work selection. Do NOT read `.beads/beads.jsonl` directly or guess what's important.

```bash
# Full triage with context — preferred
bv --robot-triage

# Just the single top pick + claim command — when you want speed
bv --robot-next

# Token-optimized output for lower context usage
bv --robot-triage --format toon
```

**What triage returns:**
- `quick_ref`: at-a-glance counts + top 3 picks
- `recommendations`: ranked actionable items with scores, reasons, unblock info
- `quick_wins`: low-effort high-impact items
- `blockers_to_clear`: items that unblock the most downstream work
- `project_health`: status/type/priority distributions, graph metrics
- `commands`: copy-paste shell commands for next steps

**How to choose from the triage output:**
1. Look at `blockers_to_clear` first — clearing blockers has the highest multiplier effect
2. Then `recommendations` — these are ranked by composite score (priority × graph impact)
3. Then `quick_wins` — if you want a fast, high-value item
4. Pick the **single highest-priority item** that you are confident you can complete
5. **Never cherry-pick a lower-priority item** unless all higher-priority items are blocked or beyond your capabilities

### Step 2: Claim — Tell the system you're working on it. THIS IS IMPORTANT, OTHERWISE ANOTHER AGENT MAY PICK THIS BEAD BEFORE YOU AND START WORKING ON IT!

```bash
br update <id> --status=in_progress
```

If Agent Mail is available, also reserve the files you'll edit and announce your work:

```
# Reserve the edit surface (prevents other agents from editing the same files)
file_reservation_paths(project_key, agent_name, ["pkg/relevant/**"], ttl_seconds=3600, exclusive=true, reason="br-<id>")

# Announce to other agents
send_message(..., thread_id="br-<id>", subject="[br-<id>] Start: <title>", ack_required=true)
```

### Step 3: Work — Implement the task

Do the actual work. Follow all coding discipline, testing, and compiler-check rules in this document.

Reply in-thread via Agent Mail with progress updates if working in a multi-agent setup.

### Step 4: Complete — Close, sync, release

```bash
# Close the issue
br close <id> --reason "Completed: <brief summary>"

# Export to JSONL (NEVER skips this — br doesn't auto-export)
br sync --flush-only
```

If Agent Mail is active:
```
# Release file reservations
release_file_reservations(project_key, agent_name, paths=["pkg/relevant/**"])

# Final announcement
send_message(..., thread_id="br-<id>", subject="[br-<id>] Completed", body="<summary of what was done>")
```

### Step 5: Loop or land

- If you have capacity and time, go back to **Step 1** and pick the next item.
- If ending the session, follow **Landing the Plane** below.

---

## bv — Graph-Aware Triage Engine (Reference)

bv is a graph-aware triage engine for Beads projects (`.beads/beads.jsonl`). It computes PageRank, betweenness, critical path, cycles, HITS, eigenvector, and k-core metrics deterministically.

**Scope boundary:** bv handles *what to work on* (triage, priority, planning). For issue management (create/update/close), use `br`. For agent-to-agent coordination (messaging, file reservations), use Agent Mail.

**CRITICAL: Use ONLY `--robot-*` flags. Bare `bv` launches an interactive TUI that blocks your session.**

### Command Reference

**Triage & Planning:**

| Command | Returns |
|---------|---------|
| `--robot-triage` | THE MEGA-COMMAND: everything you need in one call (see "Picking Work" above) |
| `--robot-next` | Minimal: just the single top pick + claim command |
| `--robot-plan` | Parallel execution tracks with `unblocks` lists |
| `--robot-priority` | Priority misalignment detection with confidence |

**Graph Analysis:**

| Command | Returns |
|---------|---------|
| `--robot-insights` | Full metrics: PageRank, betweenness, HITS, eigenvector, critical path, cycles, k-core, articulation points, slack |
| `--robot-label-health` | Per-label health: `health_level`, `velocity_score`, `staleness`, `blocked_count` |
| `--robot-label-flow` | Cross-label dependency: `flow_matrix`, `dependencies`, `bottleneck_labels` |
| `--robot-label-attention [--attention-limit=N]` | Attention-ranked labels |

**History & Change Tracking:**

| Command | Returns |
|---------|---------|
| `--robot-history` | Bead-to-commit correlations |
| `--robot-diff --diff-since <ref>` | Changes since ref: new/closed/modified issues, cycles |

**Other:**

| Command | Returns |
|---------|---------|
| `--robot-burndown <sprint>` | Sprint burndown, scope changes, at-risk items |
| `--robot-forecast <id\|all>` | ETA predictions with dependency-aware scheduling |
| `--robot-alerts` | Stale issues, blocking cascades, priority mismatches |
| `--robot-suggest` | Hygiene: duplicates, missing deps, label suggestions |
| `--robot-graph [--graph-format=json\|dot\|mermaid]` | Dependency graph export |
| `--export-graph <file.html>` | Interactive HTML visualization |

### Scoping & Filtering

```bash
bv --robot-plan --label backend              # Scope to label's subgraph
bv --robot-insights --as-of HEAD~30          # Historical point-in-time
bv --recipe actionable --robot-plan          # Pre-filter: ready to work
bv --recipe high-impact --robot-triage       # Pre-filter: top PageRank
bv --robot-triage --robot-triage-by-track    # Group by parallel work streams
bv --robot-triage --robot-triage-by-label    # Group by domain
```

### Understanding Robot Output

**All robot JSON includes:**
- `data_hash` — Fingerprint of source beads.jsonl
- `status` — Per-metric state: `computed|approx|timeout|skipped` + elapsed ms
- `as_of` / `as_of_commit` — Present when using `--as-of`

**Two-phase analysis:**
- **Phase 1 (instant):** degree, topo sort, density
- **Phase 2 (async, 500ms timeout):** PageRank, betweenness, HITS, eigenvector, cycles

### jq Quick Reference

```bash
bv --robot-triage | jq '.quick_ref'                        # At-a-glance summary
bv --robot-triage | jq '.recommendations[0]'               # Top recommendation
bv --robot-plan | jq '.plan.summary.highest_impact'        # Best unblock target
bv --robot-insights | jq '.status'                         # Check metric readiness
bv --robot-insights | jq '.Cycles'                         # Circular deps (must fix!)
```

---

## Beads (br) — Issue Management CLI (Reference)

`br` (beads_rust) is a lightweight, dependency-aware issue database and CLI for creating, updating, closing, and querying issues. Issues are stored in `.beads/` and tracked in git.

**Important:** `br` is non-invasive—it NEVER runs git commands automatically. You must manually commit changes after `br sync --flush-only`.

### Core Commands

```bash
# Creating
br create --title="..." --type=task --priority=2

# Updating
br update <id> --status=in_progress

# Closing
br close <id> --reason="Completed"
br close <id1> <id2>  # Close multiple issues at once

# Dependencies
br dep add <issue> <depends-on>   # Add a dependency

# Syncing (MANDATORY after any changes)
br sync --flush-only  # Export DB to JSONL — does NOT touch git

# Querying (ALWAYS prefer the bv equivalent when choosing what to work on next)
br ready              # Show issues ready to work (no blockers) — use for quick picks
br ready --json       # Machine-readable version
br list --status=open # All open issues
br show <id>          # Full issue details with dependencies
```

### Key Concepts

- **Dependencies:** Issues can block other issues. `br ready` shows only unblocked work.
- **Priority:** P0=critical, P1=high, P2=medium, P3=low, P4=backlog (use numbers 0-4, not words)
- **Types:** task, bug, feature, epic, chore, docs, question
- **Blocking:** `br dep add <issue> <depends-on>` to add dependencies

### Identifier Conventions

When using beads alongside Agent Mail, use consistent identifiers everywhere:

| Concept | Value |
|---------|-------|
| Mail `thread_id` | `br-###` |
| Mail subject prefix | `[br-###]` |
| File reservation `reason` | `br-###` |
| Commit messages | Include `br-###` for traceability |

---

## MCP Agent Mail — Multi-Agent Coordination

A mail-like layer that lets coding agents coordinate asynchronously via MCP tools and resources. Provides identities, inbox/outbox, searchable threads, and advisory file reservations with human-auditable artifacts in Git.

> **Note:** This section is optional. If you're operating as a single agent or using alternative coordination methods, you can skip it. If Agent Mail is not available and you need multi-agent coordination, flag to the user. They may need to start it with `am` alias or manually.

**Troubleshooting:** If Agent Mail fails with "Too many open files" (common on macOS), restart with higher limit: `ulimit -n 4096; python -m mcp_agent_mail.cli serve-http`

### Why It's Useful

- **Prevents conflicts:** Explicit file reservations (leases) for files/globs
- **Token-efficient:** Messages stored in per-project archive, not in context
- **Quick reads:** `resource://inbox/...`, `resource://thread/...`

### Same Repository Workflow

1. **Register identity:**
   ```
   ensure_project(project_key=<abs-path>)
   register_agent(project_key, program, model)
   ```

2. **Reserve files before editing:**
   ```
   file_reservation_paths(project_key, agent_name, ["pkg/**"], ttl_seconds=3600, exclusive=true)
   ```

3. **Communicate with threads:**
   ```
   send_message(..., thread_id="br-123")
   fetch_inbox(project_key, agent_name)
   acknowledge_message(project_key, agent_name, message_id)
   ```

4. **Quick reads:**
   ```
   resource://inbox/{Agent}?project=<abs-path>&limit=20
   resource://thread/{id}?project=<abs-path>&include_bodies=true
   ```

### Macros vs Granular Tools

- **Prefer macros for speed:** `macro_start_session`, `macro_prepare_thread`, `macro_file_reservation_cycle`, `macro_contact_handshake`
- **Use granular tools for control:** `register_agent`, `file_reservation_paths`, `send_message`, `fetch_inbox`, `acknowledge_message`

### Common Pitfalls

- `"from_agent not registered"`: Always `register_agent` in the correct `project_key` first
- `"FILE_RESERVATION_CONFLICT"`: Adjust patterns, wait for expiry, or use non-exclusive reservation
- **Auth errors:** If JWT+JWKS enabled, include bearer token with matching `kid`

### Team Communication Protocol — MANDATORY

**You are part of a swarm. You are NOT alone.** Even if your inbox is empty, other agents may be active. Teams only function well when members over-communicate. Silence is not golden — it causes collisions, duplicated work, and wasted effort.

**On Session Start (EVERY time):**

1. **Register immediately** — Use `macro_start_session` or `register_agent` before doing anything else
2. **Discover ALL teammates** — Read the resource `resource://agents/{project_key}` (e.g., `resource://agents//data/projects/ledgerbox`). This lists ALL registered agents with their names, task descriptions, and unread counts. **Do NOT use `list_contacts`** — that only shows agents you've established contact with, not all agents in the project
3. **Read your inbox thoroughly** — Use `fetch_inbox` with `include_bodies=true`. Read ALL messages, not just unread
4. **Acknowledge pending messages** — If any message has `ack_required=true`, acknowledge it immediately
5. **Announce yourself** — If other agents are registered, send a brief "I'm online, working on X" message to at least one of them
6. **Use `whois` for details** — If you need more info about a specific agent, use `whois(project_key, agent_name)` to get their full profile

**Before Starting Any Work:**

1. **Check if someone else is working on it** — Search messages for the bead ID (`br-###`)
2. **Check file reservations** — Look for conflicts before reserving
3. **Announce your intent** — Send a message to the thread (`thread_id="br-###"`) saying what you're about to do
4. **Wait briefly for objections** — If working on something that touches others' work, give them a moment to respond

**While Working:**

1. **Poll inbox periodically** — Check every 10-15 minutes of work, or after completing significant milestones
2. **Respond promptly** — If someone messages you, acknowledge within your current work unit
3. **Update on progress** — For longer tasks, send progress updates to the thread
4. **Flag blockers immediately** — If you hit something that affects others, message them right away

**On Completing Work:**

1. **Announce completion** — Send a message: `[br-###] Completed: <summary>`
2. **Release file reservations** — Don't hoard reservations after you're done
3. **Check inbox before moving on** — Someone may have sent you something while you were heads-down

**Communication Style:**

- **Be concise but complete** — Include bead ID, what you did, what's next
- **Use thread IDs** — Always include `thread_id="br-###"` to keep conversations organized
- **Tag relevant agents** — Put them in `to` field, not just broadcast
- **Set `ack_required=true`** — For anything that needs confirmation or could cause conflicts

**Anti-Patterns to Avoid:**

- **"My inbox was empty so I assumed I was alone"** — WRONG. Read `resource://agents/{project_key}` to see ALL registered agents
- **"I used `list_contacts` and it was empty"** — WRONG. `list_contacts` only shows established contacts, not all agents. Use the resource instead
- **"I didn't announce my work because no one asked"** — WRONG. Announce proactively
- **"I saw the message but didn't respond because I was busy"** — WRONG. At minimum, acknowledge
- **"I'll check messages when I'm done with this task"** — RISKY. Check periodically
- **Sending messages but never reading responses** — This is not communication, it's broadcasting

**Agent Discovery Quick Reference:**

| Need | Tool/Resource |
|------|---------------|
| List ALL agents in project | `resource://agents/{project_key}` via ReadMcpResourceTool |
| Get details on one agent | `whois(project_key, agent_name)` |
| List established contacts | `list_contacts` (NOT for discovery!) |
| Check who's active | Look at `last_active_ts` in agents resource |
| See who has unread mail | Look at `unread_count` in agents resource |

**The Golden Rule:** When in doubt, over-communicate. A redundant message costs nothing. A collision costs hours.

---

## UBS — Ultimate Bug Scanner

**Golden Rule:** `ubs <changed-files>` before every commit. Exit 0 = safe. Exit >0 = fix & re-run.

### Commands

```bash
ubs file.go file2.go                    # Specific Go files (< 1s) — USE THIS
ubs src/features/auth/LoginForm.tsx      # Specific frontend files
ubs $(git diff --name-only --cached)    # Staged files — before commit
ubs --only=go pkg/                      # Language filter (3-5x faster)
ubs --only=typescript src/              # TypeScript/React files only
ubs --ci --fail-on-warning .            # CI mode — before PR
ubs .                                   # Whole project (ignores vendor/, node_modules/, etc.)
```

### Output Format

```
⚠️  Category (N errors)
    file.go:42:5 – Issue description
    💡 Suggested fix
Exit code: 1
```

Parse: `file:line:col` → location | 💡 → how to fix | Exit 0/1 → pass/fail

### Fix Workflow

1. Read finding → category + fix suggestion
2. Navigate `file:line:col` → view context
3. Verify real issue (not false positive)
4. Fix root cause (not symptom)
5. Re-run `ubs <file>` → exit 0
6. Commit

### Bug Severity

- **Critical (always fix):** nil dereference, division by zero, race conditions, resource leaks
- **Important (production):** Error handling, type assertions without check, unwrapped errors
- **Contextual (judgment):** TODO/FIXME, unused variables

---

## ast-grep vs ripgrep

**Use `ast-grep` when structure matters.** It parses code and matches AST nodes, ignoring comments/strings, and can **safely rewrite** code.

- Refactors/codemods: rename APIs, change import forms
- Policy checks: enforce patterns across a repo
- Editor/automation: LSP mode, `--json` output

**Use `ripgrep` when text is enough.** Fastest way to grep literals/regex.

- Recon: find strings, TODOs, log lines, config values
- Pre-filter: narrow candidate files before ast-grep

### Rule of Thumb

- Need correctness or **applying changes** → `ast-grep`
- Need raw speed or **hunting text** → `rg`
- Often combine: `rg` to shortlist files, then `ast-grep` to match/modify

### Go Examples

```bash
# Find all error returns without wrapping
ast-grep run -l Go -p 'return err'

# Find all fmt.Println (should use structured logging)
ast-grep run -l Go -p 'fmt.Println($$$)'

# Quick grep for a function name
rg -n 'func.*LoadConfig' -t go

# Combine: find files then match precisely
rg -l -t go 'sync.Mutex' | xargs ast-grep run -l Go -p 'mu.Lock()'
```

### React / TypeScript Examples

```bash
# Find all uses of dangerouslySetInnerHTML
ast-grep run -l TypeScript -p 'dangerouslySetInnerHTML={$$$}'

# Find useEffect with empty dependency array
ast-grep run -l TypeScript -p 'useEffect($FN, [])'

# Find console.log statements (should be removed before commit)
ast-grep run -l TypeScript -p 'console.log($$$)'

# Quick grep for a component name
rg -n 'function UserCard' -t ts -t tsx

# Find any usage
rg -n 'any' --type-add 'tsx:*.tsx' -t tsx -t ts
```

---

## Morph Warp Grep — AI-Powered Code Search

**Use `mcp__morph-mcp__warp_grep` for exploratory "how does X work?" questions.** An AI agent expands your query, greps the codebase, reads relevant files, and returns precise line ranges with full context.

**Use `ripgrep` for targeted searches.** When you know exactly what you're looking for.

**Use `ast-grep` for structural patterns.** When you need AST precision for matching/rewriting.

### When to Use What

| Scenario | Tool | Why |
|----------|------|-----|
| "How is graph analysis implemented?" | `warp_grep` | Exploratory; don't know where to start |
| "Where is PageRank computed?" | `warp_grep` | Need to understand architecture |
| "Find all uses of `NewAnalyzer`" | `ripgrep` | Targeted literal search |
| "Find files with `fmt.Println`" | `ripgrep` | Simple pattern |
| "Rename function across codebase" | `ast-grep` | Structural refactor |

### warp_grep Usage

```
mcp__morph-mcp__warp_grep(
  repoPath: "/dp/beads_viewer",
  query: "How does the correlation package detect orphan commits?"
)
```

Returns structured results with file paths, line ranges, and extracted code snippets.

### Anti-Patterns

- **Don't** use `warp_grep` to find a specific function name → use `ripgrep`
- **Don't** use `ripgrep` to understand "how does X work" → wastes time with manual reads
- **Don't** use `ripgrep` for codemods → risks collateral edits

---

## cass — Cross-Agent Session Search

`cass` indexes prior agent conversations (Claude Code, Codex, Cursor, Gemini, ChatGPT, Aider, etc.) into a unified, searchable index so you can reuse solved problems.

**NEVER run bare `cass`** — it launches an interactive TUI. Always use `--robot` or `--json`.

### Quick Start

```bash
# Check if index is healthy (exit 0=ok, 1=run index first)
cass health

# Search across all agent histories
cass search "authentication error" --robot --limit 5

# View a specific result (from search output)
cass view /path/to/session.jsonl -n 42 --json

# Expand context around a line
cass expand /path/to/session.jsonl -n 42 -C 3 --json

# Learn the full API
cass capabilities --json      # Feature discovery
cass robot-docs guide         # LLM-optimized docs
```

### Key Flags

| Flag | Purpose |
|------|---------|
| `--robot` / `--json` | Machine-readable JSON output (required!) |
| `--fields minimal` | Reduce payload: `source_path`, `line_number`, `agent` only |
| `--limit N` | Cap result count |
| `--agent NAME` | Filter to specific agent (claude, codex, cursor, etc.) |
| `--days N` | Limit to recent N days |

**stdout = data only, stderr = diagnostics. Exit 0 = success.**

### Robot Mode Etiquette

- Prefer `cass --robot-help` and `cass robot-docs <topic>` for machine-first docs
- The CLI is forgiving: globals placed before/after subcommand are auto-normalized
- If parsing fails, follow the actionable errors with examples
- Use `--color=never` in non-TTY automation for ANSI-free output

### Pre-Flight Health Check

```bash
cass health --json
```

Returns in <50ms:
- **Exit 0:** Healthy—proceed with queries
- **Exit 1:** Unhealthy—run `cass index --full` first

### Exit Codes

| Code | Meaning | Retryable |
|------|---------|-----------|
| 0 | Success | N/A |
| 1 | Health check failed | Yes—run `cass index --full` |
| 2 | Usage/parsing error | No—fix syntax |
| 3 | Index/DB missing | Yes—run `cass index --full` |

Treat cass as a way to avoid re-solving problems other agents already handled.

---

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** — Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) — Tests, linters, builds
3. **Update issue status** — Close finished work, update in-progress items
4. **Sync beads** — `br sync --flush-only` to export to JSONL
5. **Stage and commit** — `git add <files>` then `git commit`
6. **Hand off** — Provide context for next session

### Session-End Git Protocol

```bash
git status              # Check what changed
git add <files>         # Stage code changes
br sync --flush-only    # Export beads changes to JSONL
git commit -m "..."     # Commit everything
git push                # Push to remote
```

---

## Note on Built-in TODO Functionality

Also, if I ask you to explicitly use your built-in TODO functionality, don't complain about this and say you need to use beads. You can use built-in TODOs if I tell you specifically to do so. Always comply with such orders.
