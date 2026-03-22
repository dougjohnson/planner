import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
    ['json', { outputFile: 'test-results/results.json' }],
    ['junit', { outputFile: 'test-results/junit.xml' }],
    process.env.CI ? ['dot'] : ['list'],
  ],
  outputDir: 'test-results/artifacts',
  use: {
    baseURL: process.env.BASE_URL || 'http://127.0.0.1:7432',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Start the Go backend before tests.
  webServer: {
    command: 'FLYWHEEL_MOCK_PROVIDERS=true go run ../../cmd/flywheel-planner',
    url: 'http://127.0.0.1:7432/api/health',
    reuseExistingServer: !process.env.CI,
    timeout: 30000,
  },
});
