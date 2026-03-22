/**
 * Regression test: Foundations must persist after locking.
 * Reproduces the user-reported bug where files disappear after clicking
 * "Lock Foundations & Proceed".
 */
import { test, expect } from '@playwright/test';
import { APIHelper } from './helpers/api';

test.describe('Foundations persistence', () => {
  test('files visible after locking via UI flow', async ({ page, request }) => {
    const api = new APIHelper(request);
    const name = `Persist Test ${Date.now()}`;

    // Create project via API.
    const project = await api.createProject(name);

    // Navigate to foundations page.
    await page.goto(`/projects/${project.id}/foundations`);
    await expect(page.getByRole('alert')).not.toBeVisible();

    // Fill in foundations fields.
    await page.getByLabel(/Project Name/i).fill(name);
    await page.getByLabel(/Tech Stack/i).fill('Go, React, TypeScript');
    await page.getByLabel(/Architecture Direction/i).fill('Modular monolith');

    // Save & Preview.
    await page.getByRole('button', { name: /Save/i }).click();

    // Wait for preview to appear — use CardHeader text which is unique per file.
    await expect(page.getByRole('button', { name: /Lock/i })).toBeVisible({ timeout: 5000 });

    // Lock foundations.
    await page.getByRole('button', { name: /Lock/i }).click();

    // === KEY ASSERTION: Locked badge visible ===
    await expect(page.getByText('Locked', { exact: true })).toBeVisible({ timeout: 5000 });

    // === KEY ASSERTION: Files must STILL be visible after locking ===
    // Check for the file content (unique per file, avoids strict mode issues)
    await expect(page.getByText('TECH_STACK.md').first()).toBeVisible();
    await expect(page.getByText('ARCHITECTURE.md').first()).toBeVisible();

    // Navigate away and come back — files must persist.
    await page.goto(`/projects/${project.id}`); // dashboard
    await page.goto(`/projects/${project.id}/foundations`);

    // Files still visible after reload.
    await expect(page.getByText('Locked', { exact: true })).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('TECH_STACK.md').first()).toBeVisible();
  });

  test('locked foundations show Enter Seed PRD link', async ({ page, request }) => {
    const api = new APIHelper(request);
    const name = `PRD Entry Test ${Date.now()}`;

    // Create project, submit foundations, lock — all via API.
    const project = await api.createProject(name);
    await api.submitFoundations(project.id, {
      project_name: name,
      tech_stack: ['Go'],
      architecture_direction: 'Mono',
    });

    // Lock via API.
    await request.post(`/api/projects/${project.id}/foundations/lock`, { data: {} });

    // Navigate to foundations page.
    await page.goto(`/projects/${project.id}/foundations`);
    await expect(page.getByRole('alert')).not.toBeVisible();

    // Should show locked state.
    await expect(page.getByText('Locked', { exact: true })).toBeVisible({ timeout: 5000 });

    // Should show foundation files.
    await expect(page.getByText('AGENTS.md').first()).toBeVisible();

    // Should show Enter Seed PRD link.
    await expect(page.getByRole('link', { name: /Enter Seed PRD/i })).toBeVisible();

    // Should show Go to Dashboard link.
    await expect(page.getByRole('link', { name: /Go to Dashboard/i })).toBeVisible();
  });
});
