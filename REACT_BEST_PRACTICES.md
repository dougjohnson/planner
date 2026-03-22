React Best Practices

This document defines the engineering standards for building a client-side React application. It is intended for both humans and coding agents. Treat these rules as defaults unless there is a documented reason to do otherwise.

1. Core principles
	•	Optimize for clarity, maintainability, correctness, and user experience over cleverness.
	•	Prefer simple, boring, composable solutions.
	•	Build for production quality from the start: accessibility, error states, loading states, empty states, observability, and testability are not optional.
	•	Keep the codebase easy to navigate. A new engineer should be able to understand major flows quickly.
	•	Make the right thing the easy thing through consistent patterns.
	•	Minimize hidden behavior, side effects, and magic abstractions.
	•	Avoid premature optimization, but do not ignore obvious performance risks.

2. Technology defaults

Unless the project explicitly says otherwise, use these defaults:
	•	React with functional components and hooks only.
	•	TypeScript in strict mode.
	•	ES modules.
	•	Vite or equivalent modern build tooling.
	•	React Router for client-side routing when routing is needed.
	•	TanStack Query for server-state fetching/caching if the app talks to APIs.
	•	Zod for runtime validation of external data and form inputs where appropriate.
	•	React Hook Form for non-trivial forms.
	•	Testing Library + Vitest/Jest for tests.
	•	ESLint + Prettier (or equivalent formatting) with CI enforcement.

Do not introduce extra libraries without a clear reason. Every dependency adds cost.

3. Architecture

3.1 General structure

Organize by feature/domain first, not by file type alone.

Preferred shape:

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
      routes/
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

Guidelines:
	•	Put code close to where it is used.
	•	Shared code should be truly shared; do not move code to a global folder too early.
	•	Separate UI components, state logic, API logic, and pure utilities.
	•	Avoid a giant utils/ dumping ground. Name utilities by domain or purpose.

3.2 Layering

Keep a clear direction of dependencies:
	•	app can depend on feature modules.
	•	feature modules can depend on shared UI/components/lib.
	•	shared modules should not depend on feature modules.

Avoid circular dependencies. If two modules depend on each other, the design is wrong.

4. Component design

4.1 Component rules
	•	Keep components small and focused.
	•	One component should do one job well.
	•	If a component becomes difficult to scan, split it.
	•	Prefer composition over inheritance or giant prop surfaces.
	•	Keep rendering logic readable; extract helpers when JSX becomes dense.

4.2 Presentational vs container logic

Use a soft separation:
	•	Presentational components: mostly UI, minimal logic, easy to reuse and test.
	•	Feature/container components: compose hooks, orchestrate data flow, wire actions.

Do not bury network calls, routing, analytics, and business logic deep inside low-level UI components.

4.3 Props
	•	Keep props explicit and minimal.
	•	Prefer booleans only when they clearly describe a state.
	•	Avoid “god components” with large numbers of props.
	•	Prefer passing domain data and well-named callbacks over raw implementation details.
	•	Avoid prop drilling across many layers; use composition, context, or state colocated at the right boundary.

Bad:

<MyComponent isLoading isDisabled hasBorder isCompact isInteractive mode="x" variant="y" />

Better:

<UserCard variant="compact" disabled={isSaving} onSelect={handleSelect} />

4.4 Keys
	•	Never use array index as a key for dynamic/reorderable lists.
	•	Use stable, unique identifiers from the data.

5. State management

Choose the smallest state solution that fits.

5.1 State hierarchy

Use this order of preference:
	1.	Local component state for local concerns.
	2.	Lift state to the nearest common parent when needed.
	3.	Context for stable cross-tree concerns.
	4.	Dedicated client state store only for genuine shared client state complexity.
	5.	TanStack Query (or equivalent) for server state.

5.2 Rules for state
	•	Keep state as minimal as possible.
	•	Do not store values that can be derived cheaply from props/state.
	•	Keep state close to where it is used.
	•	Normalize shape when handling complex nested data.
	•	Avoid duplicated sources of truth.

Bad:

const [firstName, setFirstName] = useState('');
const [lastName, setLastName] = useState('');
const [fullName, setFullName] = useState('');

Better:

const fullName = `${firstName} ${lastName}`.trim();

5.3 Context usage

Use context sparingly.

Good for:
	•	theme
	•	auth session shell state
	•	localization
	•	app-wide configuration

Not good for:
	•	frequently changing granular UI state across large trees
	•	replacing proper state architecture

Split contexts by concern to avoid massive rerenders.

6. Hooks

6.1 General hook rules
	•	Follow the Rules of Hooks without exception.
	•	Use hooks to encapsulate reusable stateful logic.
	•	Keep custom hooks focused on a single concern.
	•	Hook names must start with use.

6.2 Custom hook design

A good custom hook:
	•	has a narrow purpose
	•	exposes a stable, clear API
	•	hides implementation detail
	•	avoids leaking unrelated state/actions

Example:

function useUserPreferences() {
  // fetch, update, and expose domain-specific state
}

Not:

function useEverythingPageNeeds() {
  // mixed routing, toasts, API calls, modal state, analytics, filters...
}

6.3 Effects

Treat useEffect as an escape hatch, not the default tool.

Use effects for:
	•	syncing with external systems
	•	subscriptions
	•	timers
	•	imperative browser APIs
	•	effects caused by rendering, not user events

Do not use effects for:
	•	deriving display values
	•	handling straightforward event-driven logic
	•	mirroring props into state without a real need
	•	papering over poor architecture

Before writing an effect, ask: can this be computed during render, handled in an event, or moved to a dedicated library?

6.4 Effect safety
	•	Include correct dependencies.
	•	Do not suppress lint warnings unless there is a real, documented reason.
	•	Clean up subscriptions/timers/listeners.
	•	Guard against race conditions in async flows.
	•	Prefer framework/data-layer tools over ad hoc fetch-in-effect patterns.

7. Data fetching and API integration

7.1 General rules
	•	Keep API calls out of low-level presentational components.
	•	Centralize API access patterns by feature or service.
	•	Validate external data at the boundary.
	•	Handle loading, empty, error, retry, and stale states deliberately.

7.2 Server state

Use TanStack Query or equivalent for remote data.

Guidelines:
	•	Use stable query keys.
	•	Keep fetchers pure.
	•	Co-locate query hooks with the owning feature.
	•	Invalidate/refetch deliberately after mutations.
	•	Use optimistic updates only when the UX gain justifies the complexity.

7.3 API boundaries
	•	Convert raw transport data into app-safe shapes near the boundary.
	•	Do not leak inconsistent backend naming conventions across the app if avoidable.
	•	Prefer typed API modules over scattered fetch() calls.

Example:

export async function getUser(id: string): Promise<User> {
  const response = await http.get(`/users/${id}`);
  return UserSchema.parse(response.data);
}

8. Forms and user input
	•	Use controlled or well-managed form state consistently.
	•	Prefer React Hook Form for complex forms.
	•	Validate on both client and server when applicable.
	•	Keep validation schemas explicit and reusable.
	•	Show actionable inline errors.
	•	Disable submit only when appropriate; avoid trapping users.
	•	Preserve user input during recoverable failures.
	•	Support keyboard interaction throughout.

9. Error handling
	•	Anticipate failure.
	•	Every async flow should define what happens on failure.
	•	Use error boundaries for rendering failures at sensible boundaries.
	•	Show user-friendly messages, but keep detailed diagnostics available for logs/observability.
	•	Do not swallow errors silently.
	•	Prefer recoverable UI over blank screens.

Minimum expectations:
	•	route-level error boundary
	•	component-level fallback for risky sections if justified
	•	API failure state
	•	retry path where sensible

10. Accessibility

Accessibility is a baseline requirement, not a polish step.

10.1 Core rules
	•	Use semantic HTML first.
	•	Prefer native controls over div/button impersonation.
	•	Every interactive element must be keyboard accessible.
	•	Ensure visible focus states.
	•	Label inputs correctly.
	•	Provide alt text where needed.
	•	Use ARIA only when native HTML is insufficient.
	•	Ensure sufficient color contrast.
	•	Announce dynamic status changes when relevant.

10.2 Practical checks
	•	Can the page be used with keyboard only?
	•	Can screen readers identify controls and their purpose?
	•	Are forms properly labeled and errors associated with inputs?
	•	Are dialogs, menus, and popovers focus-managed correctly?

11. Styling and design system

11.1 Styling approach

Use a consistent styling strategy across the app. Do not mix multiple styling paradigms without reason.

Preferred traits:
	•	reusable design tokens
	•	predictable spacing scale
	•	typography scale
	•	color roles instead of random hex values
	•	shared UI primitives

11.2 Component styling rules
	•	Keep styles close enough to the component to maintain clarity.
	•	Use variants for supported visual states.
	•	Do not hardcode one-off magic numbers unless justified.
	•	Avoid deep selector chains and fragile overrides.
	•	Prefer systematized spacing/layout over ad hoc tweaks.

11.3 Design system mindset

For apps of any meaningful size:
	•	define base UI primitives
	•	standardize buttons, inputs, modals, cards, tables, alerts, empty states
	•	document usage expectations

12. Routing and navigation
	•	Define routes clearly and consistently.
	•	Use route-based code splitting for larger screens/features.
	•	Keep route params typed/validated where possible.
	•	Do not scatter navigation rules everywhere; centralize route helpers when useful.
	•	Handle not-found and unauthorized states explicitly.
	•	Preserve back-button expectations.

13. Performance

Do not optimize blindly. Optimize where it matters.

13.1 Default mindset
	•	Prevent unnecessary rerenders through good state boundaries.
	•	Keep render functions pure and cheap.
	•	Memoize only when it solves a measured or obvious problem.
	•	Use list virtualization for large collections.
	•	Use code splitting for large routes/components.
	•	Load heavy dependencies lazily when appropriate.

13.2 Avoid common mistakes
	•	do not use useMemo/useCallback everywhere by reflex
	•	do not keep huge objects/functions changing every render when it impacts children
	•	do not fetch the same data repeatedly without caching strategy
	•	do not render massive lists naively

13.3 Measure first

Use profiling and real observations before making code harder to read for micro-optimizations.

14. Security and privacy
	•	Treat all external data as untrusted.
	•	Validate and sanitize where appropriate.
	•	Never use dangerouslySetInnerHTML unless there is no viable alternative and content is sanitized.
	•	Do not store secrets in client code.
	•	Assume client-side authorization checks are cosmetic; enforce real permissions server-side.
	•	Minimize exposure of sensitive user data in logs, analytics, and local storage.
	•	Avoid persistent storage for sensitive data unless absolutely necessary.

15. Testing strategy

Test behavior, not implementation trivia.

15.1 Testing priorities
	1.	Critical user flows
	2.	Business logic and transformations
	3.	Risky edge cases
	4.	Shared primitives and hooks

15.2 Recommended test mix
	•	Unit tests for pure logic and utilities.
	•	Component tests for user-visible behavior.
	•	Integration tests for feature flows.
	•	End-to-end tests for the most critical paths.

15.3 Testing rules
	•	Prefer tests that resemble real user interactions.
	•	Avoid brittle snapshot-heavy strategies.
	•	Mock at network/process boundaries, not everywhere.
	•	Keep tests deterministic and readable.
	•	Every bug fix should consider whether a regression test is warranted.

16. TypeScript standards
	•	Use strict mode.
	•	Prefer precise types over any.
	•	Avoid as casts unless there is a well-understood reason.
	•	Use discriminated unions for state machines and variant modeling.
	•	Model domain concepts explicitly.
	•	Keep types near the owning domain unless broadly shared.
	•	Generate or derive types from schemas/contracts when possible.

Bad:

const data: any = await response.json();

Better:

const parsed = UserSchema.parse(await response.json());

17. Naming and code style

17.1 Naming
	•	Use clear, descriptive names.
	•	Prefer domain language over vague technical names.
	•	Components: PascalCase
	•	hooks: useSomething
	•	utilities/functions: camelCase
	•	constants: UPPER_SNAKE_CASE only for true constants
	•	files should follow a consistent convention across the repo

17.2 Readability
	•	Prefer explicit code over compressed clever code.
	•	Keep nesting shallow.
	•	Use early returns.
	•	Extract well-named helpers for complex conditions.
	•	Delete dead code promptly.

18. Logging, analytics, and observability
	•	Instrument meaningful events, not noise.
	•	Keep analytics out of presentational components when possible.
	•	Centralize telemetry wrappers/helpers.
	•	Log enough for diagnosis without leaking sensitive data.
	•	Capture frontend errors with stack/context in production.

19. Internationalization and formatting

If the app may ever be multilingual, avoid painting the codebase into a corner.
	•	Do not hardcode user-facing strings deep inside reusable components if i18n is expected.
	•	Be aware of date, time, number, and currency formatting.
	•	Avoid concatenating user-facing strings in fragile ways.

20. Offline, resilience, and network reality

Client apps run in unreliable environments.
	•	Assume slow, flaky, or interrupted networks.
	•	Provide loading feedback for non-instant operations.
	•	Avoid UI jumps during async transitions.
	•	Handle retry and recovery deliberately.
	•	Make destructive actions explicit and ideally reversible.

21. Agent-specific implementation rules

These rules are specifically for coding agents operating in this repository.

21.1 Before coding
	•	Inspect existing patterns and follow them unless they are clearly harmful.
	•	Prefer consistency with the codebase over introducing a new pattern.
	•	Do not add dependencies casually.
	•	Do not rewrite unrelated code while solving a focused task.

21.2 While coding
	•	Make the smallest change that cleanly solves the problem.
	•	Preserve backward compatibility unless the task explicitly allows breaking changes.
	•	Add or update tests when behavior changes.
	•	Handle loading, empty, and error states for new async UI.
	•	Keep accessibility intact.
	•	Keep TypeScript types accurate.

21.3 When creating components
	•	Prefer reusing existing primitives.
	•	Do not duplicate similar components without checking for extension/composition options.
	•	Expose a minimal public API.
	•	Avoid over-generalizing too early.

21.4 When handling data
	•	Validate external inputs.
	•	Do not trust API response shapes blindly.
	•	Keep transformation logic out of JSX.
	•	Avoid mixing server state and local UI state carelessly.

21.5 When finishing work

Agents should verify:
	•	code builds
	•	lint passes
	•	tests pass or are updated appropriately
	•	no obvious accessibility regressions
	•	no obvious performance regressions
	•	no dead code or debug leftovers remain

22. Pull request / change expectations

Each change should be:
	•	small enough to review
	•	scoped to a clear goal
	•	accompanied by tests when warranted
	•	documented when introducing a new pattern

PR descriptions should explain:
	•	what changed
	•	why it changed
	•	tradeoffs or notable decisions
	•	follow-up work, if any

23. Anti-patterns to avoid

Avoid these unless there is a strong documented reason:
	•	massive components with mixed concerns
	•	effect-heavy code that should be derived or event-driven
	•	duplicated sources of truth
	•	prop drilling across half the app
	•	global state for everything
	•	widespread use of any
	•	index keys in dynamic lists
	•	unvalidated API data
	•	scattered raw fetch calls throughout the UI
	•	silent catch blocks
	•	inaccessible custom controls
	•	premature abstraction
	•	premature memoization
	•	giant shared utility files
	•	CSS overrides wars
	•	leaking backend shapes directly into every component

24. Definition of done

A feature or change is not done unless:
	•	the code is understandable and maintainable
	•	types are sound
	•	user-facing states are handled
	•	accessibility has been considered
	•	tests are added or updated as appropriate
	•	lint/format/build pass
	•	the solution matches existing architecture or improves it intentionally
	•	no obvious loose ends are left behind

25. Default decision framework

When multiple valid approaches exist, prefer the option that is:
	1.	easiest to understand
	2.	easiest to maintain
	3.	easiest to test
	4.	most consistent with the existing codebase
	5.	sufficiently performant for the actual use case

26. Final note

Build React applications that are boring in the best way: predictable, robust, readable, and easy to evolve. Clever code ages badly. Clear code scales.