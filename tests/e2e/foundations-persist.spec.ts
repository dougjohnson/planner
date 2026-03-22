/**
 * Regression test: Foundations must persist after locking, and
 * the seed PRD textarea must appear inline below the locked foundations.
 */
import { test, expect } from '@playwright/test';
import { APIHelper } from './helpers/api';

test.describe('Foundations persistence', () => {
  test('files visible after locking via UI flow', async ({ page, request }) => {
    const api = new APIHelper(request);
    const name = `Persist Test ${Date.now()}`;

    const project = await api.createProject(name);

    await page.goto(`/projects/${project.id}/foundations`);
    await expect(page.getByRole('alert')).not.toBeVisible();

    // Fill in foundations fields.
    await page.getByLabel(/Project Name/i).fill(name);
    await page.getByLabel(/Tech Stack/i).fill('Go, React, TypeScript');
    await page.getByLabel(/Architecture Direction/i).fill('Modular monolith');

    // Save & Preview.
    await page.getByRole('button', { name: /Save/i }).click();
    await expect(page.getByRole('button', { name: /Lock/i })).toBeVisible({ timeout: 5000 });

    // Lock foundations.
    await page.getByRole('button', { name: /Lock/i }).click();

    // Locked badge visible.
    await expect(page.getByText('Foundations Locked')).toBeVisible({ timeout: 5000 });

    // Files still visible after locking.
    await expect(page.getByText('TECH_STACK.md').first()).toBeVisible();
    await expect(page.getByText('ARCHITECTURE.md').first()).toBeVisible();

    // Seed PRD textarea appears inline (not a separate page).
    await expect(page.getByLabel(/Paste your PRD/i)).toBeVisible();

    // Navigate away and come back — files must persist.
    await page.goto(`/projects/${project.id}`);
    await page.goto(`/projects/${project.id}/foundations`);

    await expect(page.getByText('Foundations Locked')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('TECH_STACK.md').first()).toBeVisible();
  });

  test('locked foundations show inline seed PRD textarea', async ({ page, request }) => {
    const api = new APIHelper(request);
    const name = `PRD Inline Test ${Date.now()}`;

    const project = await api.createProject(name);
    await api.submitFoundations(project.id, {
      project_name: name,
      tech_stack: ['Go'],
      architecture_direction: 'Mono',
    });
    await request.post(`/api/projects/${project.id}/foundations/lock`, { data: {} });

    await page.goto(`/projects/${project.id}/foundations`);
    await expect(page.getByRole('alert')).not.toBeVisible();

    // Locked state visible.
    await expect(page.getByText('Foundations Locked')).toBeVisible({ timeout: 5000 });

    // Foundation files visible.
    await expect(page.getByText('AGENTS.md').first()).toBeVisible();

    // Seed PRD section appears inline below foundations.
    await expect(page.getByRole('heading', { name: 'Seed PRD', exact: true })).toBeVisible();
    await expect(page.getByLabel(/Paste your PRD/i)).toBeVisible();
  });
});
