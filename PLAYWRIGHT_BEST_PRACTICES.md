# Playwright Best Practices

> Guidelines for writing reliable, maintainable end-to-end tests with Playwright.

## Core Principles

1. **Test user outcomes, not implementation details.** Assert what the user sees ("item name is visible"), not how the page achieves it ("div with class item-card contains span").
2. **Never assume empty state.** Tests may share a database across runs. Every assertion must work whether the data store has 0 records or 500.
3. **Prefer API setup over UI setup.** Create test data programmatically through API calls. Only drive the UI when the UI itself is what you're testing.
4. **Every test must be independently runnable.** No test should depend on another test having run first. No shared mutable state between tests.
5. **Always assert the absence of errors.** After every navigation, verify that no error states are shown — a page that renders without crashing is the minimum bar.

---

## Locator Strategy

**Prefer, in order:**
1. `getByRole('button', { name: /Submit/i })` — accessible, resilient to markup changes
2. `getByLabel('Email Address')` — for form inputs with labels
3. `getByText('...')` — for visible content assertions
4. `getByTestId('...')` — last resort, for elements with no accessible name

**Never use:**
- CSS selectors tied to styling classes (`.card-item`, `.btn-primary`)
- DOM structure assumptions (`div > span:nth-child(2)`)
- XPath

**Resilience patterns:**
```typescript
// BAD — breaks if button text changes depending on page state
await page.click('text=Create Item');

// GOOD — matches either variant the UI might show
await page.getByRole('link', { name: /New Item|Create Item/i }).click();

// BAD — breaks if duplicates exist from prior test runs
await expect(page.getByText('My Item')).toBeVisible();

// GOOD — handles duplicates gracefully
await expect(page.getByText('My Item').first()).toBeVisible();
```

---

## Data Management

**Never assume the database is empty.** Other tests (or prior runs of the same test) may have created data.

```typescript
// BAD — fails if records already exist
await expect(page.getByText('No items yet')).toBeVisible();

// GOOD — verifies the page loaded without error regardless of data state
await expect(page.getByRole('heading', { name: /Items/i })).toBeVisible();
await expect(page.getByRole('alert')).not.toBeVisible();
```

**Use unique identifiers when you need to find your specific test data:**
```typescript
const name = `E2E Test ${Date.now()}`;
const item = await api.createItem(name);
await page.goto('/items');
await expect(page.getByText(name)).toBeVisible();
```

**Clean up when sharing a persistent database.** If the database persists between test runs and accumulating data causes problems, delete or archive test data in `afterEach`. If the database is ephemeral (e.g., created fresh per server start), cleanup is usually unnecessary.

---

## Test Structure

### Separate API tests from UI tests

API tests are fast and stable — they verify data contracts and status codes. UI tests are slower and more fragile — they verify the user experience. Don't mix them unnecessarily.

```typescript
// API test — fast, no browser needed
test('create item returns correct fields', async ({ request }) => {
  const resp = await request.post('/api/items', { data: { name: 'Test' } });
  const body = await resp.json();
  expect(body.data.id).toBeTruthy();
  expect(body.data.name).toBe('Test');
});

// UI test — slower, tests the actual user experience
test('user can create an item through the form', async ({ page }) => {
  await page.goto('/items/new');
  await page.getByLabel('Name').fill('New Item');
  await page.getByRole('button', { name: /Create/i }).click();
  await expect(page).toHaveURL(/\/items\/[a-f0-9-]+/);
});
```

### Use API helpers for setup

Build a helper class that wraps common API calls. Use it to set up preconditions quickly and reliably, then test only the specific UI step you care about.

```typescript
test('detail page shows item metadata', async ({ page, request }) => {
  const api = new APIHelper(request);

  // Setup via API — fast, no UI brittleness
  const item = await api.createItem('Detail Test');
  await api.addMetadata(item.id, { priority: 'high' });

  // Only NOW test the UI
  await page.goto(`/items/${item.id}`);
  await expect(page.getByText('high')).toBeVisible();
});
```

### Name tests descriptively

```typescript
// BAD
test('test1', ...);

// GOOD — describes the user scenario and expected outcome
test('user can archive a completed item and see it in the archive tab', ...);
```

---

## Waiting and Timing

**Never use fixed `sleep` or `waitForTimeout`.** Playwright's auto-waiting handles most cases. Explicit timeouts on assertions are the correct pattern when you need more time.

```typescript
// BAD — arbitrary delay, still flaky
await page.waitForTimeout(2000);
await expect(page.getByText('Done')).toBeVisible();

// GOOD — Playwright retries the assertion until it passes or times out
await expect(page.getByText('Done')).toBeVisible({ timeout: 5000 });
```

**Use `waitForResponse` when an action depends on an API call completing:**
```typescript
const [response] = await Promise.all([
  page.waitForResponse(resp => resp.url().includes('/api/items') && resp.status() === 201),
  page.getByRole('button', { name: /Create/i }).click(),
]);
expect(response.ok()).toBe(true);
```

**Use `waitForURL` after navigation actions:**
```typescript
await page.getByRole('link', { name: /Dashboard/i }).click();
await page.waitForURL(/\/dashboard/);
```

---

## Assertions

**Assert the absence of errors, not just the presence of content:**
```typescript
// After every navigation or action, check for error states
await page.goto('/items');
await expect(page.getByRole('alert')).not.toBeVisible();
await expect(page.getByText(/Failed|Error/i)).not.toBeVisible();
```

**Assert on accessible roles where possible:**
```typescript
// Prefer role-based assertions — resilient to tag changes
await expect(page.getByRole('heading', { level: 1 })).toHaveText('Dashboard');

// Over element-based assertions — breaks if heading tag changes
await expect(page.locator('h1')).toHaveText('Dashboard');
```

**Verify API response shapes in API tests, not UI tests:**
```typescript
// API test — verify the contract once
test('API returns snake_case fields', async ({ request }) => {
  const resp = await request.post('/api/items', { data: { name: 'Test' } });
  const { data } = await resp.json();
  expect(data.id).toBeTruthy();        // snake_case ✓
  expect(data.ID).toBeUndefined();     // PascalCase ✗
  expect(data.created_at).toBeTruthy();
  expect(data.CreatedAt).toBeUndefined();
});
```

---

## Page Object Model (Optional)

For large test suites, Page Object Models reduce duplication. For smaller suites, inline locators are fine — don't over-abstract early.

```typescript
// pages/items.ts
export class ItemsPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/items');
  }

  async createItem(name: string) {
    await this.page.getByRole('link', { name: /New Item|Create/i }).click();
    await this.page.getByLabel('Name').fill(name);
    await this.page.getByRole('button', { name: /Create/i }).click();
  }

  get heading() {
    return this.page.getByRole('heading', { level: 1 });
  }

  get errorAlert() {
    return this.page.getByRole('alert');
  }
}
```

Only introduce POM when you find yourself duplicating the same locator logic across 3+ test files.

---

## Configuration

### Recommended `playwright.config.ts` settings

```typescript
export default defineConfig({
  // Capture evidence on failure for debugging
  use: {
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    trace: 'on-first-retry',
  },

  // Multiple output formats for CI and human review
  reporter: [
    ['html', { open: 'never' }],       // Visual report
    ['json', { outputFile: '...' }],    // Machine-readable
    ['junit', { outputFile: '...' }],   // CI integration
  ],

  // Auto-start the backend before tests
  webServer: {
    command: 'your-server-start-command',
    url: 'http://127.0.0.1:PORT/health',
    reuseExistingServer: !process.env.CI,
    timeout: 30000,
  },
});
```

### Key settings explained

| Setting | Recommended | Why |
|---------|-------------|-----|
| `screenshot` | `only-on-failure` | Evidence for debugging without storage bloat |
| `video` | `retain-on-failure` | See exactly what happened when a test fails |
| `trace` | `on-first-retry` | Full trace on flaky tests, no overhead on green runs |
| `retries` | `2` in CI, `0` locally | CI retries catch flakes; locally you want fast feedback |
| `workers` | `1` in CI, default locally | Parallel locally for speed; serial in CI for stability |
| `reuseExistingServer` | `true` locally | Don't restart the server between `npx playwright test` calls during development |

### Running tests

```bash
# Run all tests (auto-starts server if configured)
npx playwright test

# Run a specific test file
npx playwright test smoke.spec.ts

# Run tests matching a pattern
npx playwright test -g "user can create"

# Run with visible browser for debugging
npx playwright test --headed

# Run with Playwright Inspector (step-by-step)
npx playwright test --debug

# View the HTML report from the last run
npx playwright show-report
```

---

## Anti-Patterns

| Anti-Pattern | Why It Breaks | Better Approach |
|-------------|---------------|-----------------|
| `page.click('.btn-primary')` | CSS classes change during styling updates | `page.getByRole('button', { name: ... })` |
| Assuming empty database | Fails on second run | Assert content that's valid in any data state |
| `page.waitForTimeout(3000)` | Flaky — too short or wastefully long | `expect(...).toBeVisible({ timeout: 5000 })` |
| Long multi-step test chains | One failure cascades; hard to diagnose | Use API setup, test one UI step per test |
| Hardcoded URLs with IDs | IDs change between runs | Navigate via UI or construct from API responses |
| `page.locator('div:nth-child(3) > span')` | Any DOM restructuring breaks it | Accessible locators or `data-testid` |
| Testing the same logic in API and UI tests | Double maintenance, wasted CI time | API tests for contracts, UI tests for experience |
| Not checking for error states | Test passes while page shows an error banner | Always assert `getByRole('alert')` is not visible |
| Testing with production data | Unpredictable, privacy risk | Use mock/seed data with deterministic content |
| Shared state between tests | Execution order matters, parallel breaks | Each test sets up its own preconditions |

---

## File Organization

```
tests/e2e/
  helpers/
    api.ts              # API helper for programmatic setup/teardown
    logger.ts           # Structured test logging (optional)
  pages/                # Page Object Models (optional, add as needed)
  smoke.spec.ts         # Health check, basic API contract verification
  first-use.spec.ts     # First-use / onboarding flows
  [feature].spec.ts     # One file per major feature area
  playwright.config.ts  # Configuration
  package.json          # Dependencies (@playwright/test)
  tsconfig.json         # TypeScript config
```

---

## When to Write a Playwright Test

**Write one when:**
- A user-visible bug reached production and you want to prevent regression
- The flow involves multiple pages or navigation
- The flow involves real API calls whose response shape affects the UI (e.g., JSON field casing)
- You need to verify that frontend and backend work together correctly
- The feature has complex state transitions visible in the UI

**Don't write one when:**
- A unit test or component test covers the same thing faster and more reliably
- You're testing pure business logic with no UI interaction
- You're testing API contracts in isolation (use backend integration tests)
- You're testing visual styling (use visual regression tools like Chromatic or Percy instead)

---

## Debugging Failing Tests

1. **Check the HTML report:** `npx playwright show-report` — shows screenshots, video, and trace for failures
2. **Run in headed mode:** `npx playwright test --headed` — watch the browser in real time
3. **Use the Inspector:** `npx playwright test --debug` — step through each action
4. **Check the error context:** Playwright saves a page snapshot (DOM tree) on failure — read it to understand what the page actually showed
5. **Check the server logs:** If the page shows an error, the root cause is usually in the backend logs, not the frontend
