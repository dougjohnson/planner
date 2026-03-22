# Playwright Best Practices

> Guidelines for writing reliable, maintainable E2E tests in this project.

## Core Principles

1. **Test user outcomes, not implementation details.** Assert what the user sees ("project name is visible"), not how the page achieves it ("div with class project-card contains span").
2. **Never assume empty state.** Tests share a database across runs. Every assertion must work whether the database has 0 projects or 500.
3. **Prefer API setup over UI setup.** Use the `APIHelper` to create test data programmatically. Only drive the UI when the UI itself is what you're testing.
4. **Every test must be independently runnable.** No test should depend on another test having run first. No shared mutable state between tests.

## Locator Strategy

**Prefer, in order:**
1. `getByRole('button', { name: /Submit/i })` — accessible, resilient to markup changes
2. `getByLabel('Project Name')` — for form inputs
3. `getByText('...')` — for visible content assertions
4. `getByTestId('...')` — last resort, for elements with no accessible name

**Never use:**
- CSS selectors tied to styling classes (`.project-card`, `.btn-primary`)
- DOM structure assumptions (`div > span:nth-child(2)`)
- XPath

**Resilience patterns:**
```typescript
// BAD — breaks if button text changes by state
await page.click('text=Create Project');

// GOOD — matches either variant
await page.getByRole('link', { name: /New Project|Create Project/i }).click();

// BAD — breaks if duplicates exist from prior runs
await expect(page.getByText('My Project')).toBeVisible();

// GOOD — handles duplicates
await expect(page.getByText('My Project').first()).toBeVisible();
```

## Data Management

**Never assume the database is empty.** Other tests (or prior runs of the same test) may have created data.

```typescript
// BAD — fails if projects already exist
await expect(page.getByText('No projects yet')).toBeVisible();

// GOOD — verifies the page loaded without error regardless of state
await expect(page.getByRole('heading', { name: /Projects/i })).toBeVisible();
await expect(page.getByRole('alert')).not.toBeVisible();
```

**Use unique identifiers when you need to find your data:**
```typescript
// Generate a unique name so this test's project is distinguishable
const name = `E2E Test ${Date.now()}`;
await api.createProject(name);
await page.goto('/projects');
await expect(page.getByText(name)).toBeVisible();
```

**Clean up after yourself only when necessary.** For this project, the SQLite database is ephemeral per server start (stored in a temp directory), so cleanup is usually not needed. If tests do share a persistent database, archive or delete test data in `afterEach`.

## Test Structure

### Separate API tests from UI tests

API tests are fast and stable. UI tests are slower and more fragile. Don't mix them unnecessarily.

```typescript
// API test — fast, no browser needed
test('create project returns correct fields', async ({ request }) => {
  const resp = await request.post('/api/projects', { data: { name: 'Test' } });
  const { data } = await resp.json();
  expect(data.id).toBeTruthy();
  expect(data.name).toBe('Test');
});

// UI test — slower, tests the actual user experience
test('user can create a project through the form', async ({ page }) => {
  await page.goto('/projects');
  // ... drive the UI
});
```

### Use the APIHelper for setup

```typescript
test('review page shows disagreements', async ({ page, request }) => {
  const api = new APIHelper(request);

  // Setup via API — fast, no UI brittleness
  const project = await api.createProject('Review Test');
  await api.submitFoundations(project.id, { ... });
  await api.startStage(project.id, 'parallel_prd_generation');

  // Only NOW test the UI
  await page.goto(`/projects/${project.id}/review/prd_disagreement_review`);
  await expect(page.getByText('disputed')).toBeVisible();
});
```

### Name tests descriptively

```typescript
// BAD
test('test1', ...);

// GOOD — describes the user scenario
test('user can lock foundations and proceed to PRD intake', ...);
```

## Waiting and Timing

**Never use fixed `sleep` or `waitForTimeout`.** Playwright's auto-waiting handles most cases.

```typescript
// BAD
await page.waitForTimeout(2000);
await expect(page.getByText('Done')).toBeVisible();

// GOOD — Playwright waits automatically
await expect(page.getByText('Done')).toBeVisible({ timeout: 5000 });
```

**Use `waitForResponse` for operations that depend on API calls:**
```typescript
const [response] = await Promise.all([
  page.waitForResponse(resp => resp.url().includes('/api/projects') && resp.status() === 201),
  page.getByRole('button', { name: /Create/i }).click(),
]);
```

## Assertions

**Assert the absence of errors, not just the presence of content:**
```typescript
// Always check for error states after navigation
await page.goto('/projects');
await expect(page.getByRole('alert')).not.toBeVisible();
await expect(page.getByText(/Failed|Error/i)).not.toBeVisible();
```

**Assert on accessible roles where possible:**
```typescript
// Prefer role-based assertions
await expect(page.getByRole('heading', { level: 1 })).toHaveText('Projects');

// Over text-based assertions
await expect(page.locator('h1')).toHaveText('Projects');
```

## Configuration

### Playwright config (`playwright.config.ts`)

- `webServer` starts the Go backend automatically with `FLYWHEEL_MOCK_PROVIDERS=true`
- Screenshots are captured on failure (`screenshot: 'only-on-failure'`)
- Video is retained on failure (`video: 'retain-on-failure'`)
- HTML, JSON, and JUnit reporters are configured for CI
- Base URL defaults to `http://127.0.0.1:7432`

### Running tests

```bash
cd tests/e2e

# Run all tests (starts server automatically)
npx playwright test

# Run a specific test file
npx playwright test first-use.spec.ts

# Run with visible browser for debugging
npx playwright test --headed

# Run with Playwright Inspector for step-by-step debugging
npx playwright test --debug

# View the HTML report from the last run
npx playwright show-report
```

## Anti-Patterns

| Anti-Pattern | Why It Breaks | Better Approach |
|-------------|---------------|-----------------|
| `page.click('.btn-primary')` | CSS classes change during styling updates | `page.getByRole('button', { name: ... })` |
| Assuming empty database | Fails on second run | Check for content resilient to existing data |
| `page.waitForTimeout(3000)` | Flaky timing | `expect(...).toBeVisible({ timeout: 5000 })` |
| Long test chains (create → foundations → lock → intake → ...) | One failure cascades to every subsequent step | Use API setup, test only the UI step you care about |
| Hardcoded URLs with IDs | IDs change between runs | Navigate via UI or construct from API responses |
| `page.locator('div:nth-child(3) > span')` | Any DOM change breaks it | Accessible locators or test IDs |
| Testing the same thing in API and UI tests | Wasted time, double maintenance | API tests for data contracts, UI tests for user flows |
| Not checking for error states | Silently passes when the page shows an error | Always assert `getByRole('alert')` is not visible |

## File Organization

```
tests/e2e/
  helpers/
    api.ts              # APIHelper for programmatic setup
    logger.ts           # Structured test logging
    sse.ts              # SSE event helpers
  pages/                # Page Object Models (if needed)
  smoke.spec.ts         # Health check, basic API contract
  first-use.spec.ts     # First-use experience flows
  foundations.spec.ts    # Foundations workflow
  prd-pipeline.spec.ts  # PRD pipeline stages
  plan-pipeline.spec.ts # Plan pipeline stages
  playwright.config.ts  # Playwright configuration
  package.json          # Dependencies (@playwright/test)
```

## When to Write a Playwright Test

Write a Playwright test when:
- A user-visible flow broke in production and you want to prevent regression
- The flow involves multiple pages or navigation
- The flow involves real API calls that need to be verified end-to-end
- You need to verify the frontend correctly handles backend response shapes (like JSON field casing)

Don't write a Playwright test when:
- A unit test or component test would cover the same thing faster
- You're testing pure business logic with no UI
- You're testing API contracts (use Go integration tests instead)
