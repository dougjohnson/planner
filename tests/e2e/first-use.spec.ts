/**
 * E2E test: First-use experience — project creation and navigation.
 *
 * These tests verify the critical user paths work end-to-end.
 * Tests are resilient to existing data (don't assume empty database).
 */

import { test, expect } from '@playwright/test';

test.describe('First-use experience', () => {

  test('projects page loads without error', async ({ page }) => {
    await page.goto('/projects');

    // Should NOT show an error state.
    await expect(page.getByRole('alert')).not.toBeVisible();
    await expect(page.getByText('Failed')).not.toBeVisible();

    // Should show either the empty state OR the project list — both are valid.
    const heading = page.getByRole('heading', { name: /Projects/i });
    await expect(heading).toBeVisible();
  });

  test('create project via UI and land on foundations', async ({ page }) => {
    await page.goto('/projects');

    // Click the create/new project button (text varies by state).
    const newButton = page.getByRole('link', { name: /New Project|Create Project/i });
    await expect(newButton).toBeVisible({ timeout: 5000 });
    await newButton.click();
    await expect(page).toHaveURL(/\/projects\/new/);

    // Fill required field.
    await page.getByLabel(/Project Name/i).fill('Playwright Test Project');

    // Submit.
    await page.getByRole('button', { name: /Create Project/i }).click();

    // Should navigate to the project (foundations page).
    await expect(page).toHaveURL(/\/projects\/[a-f0-9-]+/, { timeout: 5000 });

    // Should NOT show any error.
    await expect(page.getByRole('alert')).not.toBeVisible();
    await expect(page.getByText('Failed')).not.toBeVisible();
  });

  test('created project appears in project list', async ({ page, request }) => {
    // Create a project via API.
    const resp = await request.post('/api/projects', {
      data: { name: 'E2E Visibility Test' },
    });
    expect(resp.ok()).toBe(true);
    const body = await resp.json();
    expect(body.data.id).toBeTruthy();

    // Navigate to projects list.
    await page.goto('/projects');

    // The project should appear (not error state). Use .first() since
    // previous test runs may have created projects with the same name.
    await expect(page.getByText('E2E Visibility Test').first()).toBeVisible({ timeout: 5000 });
  });

  test('project API returns properly cased JSON fields', async ({ request }) => {
    // Create project.
    const createResp = await request.post('/api/projects', {
      data: { name: 'JSON Case Test', description: 'Checking casing' },
    });
    expect(createResp.ok()).toBe(true);
    const { data } = await createResp.json();

    // Verify snake_case fields (not PascalCase).
    expect(data.id).toBeTruthy();
    expect(data.name).toBe('JSON Case Test');
    expect(data.description).toBe('Checking casing');
    expect(data.status).toBe('active');
    expect(data.created_at).toBeTruthy();
    expect(data.updated_at).toBeTruthy();

    // These PascalCase fields should NOT exist.
    expect(data.ID).toBeUndefined();
    expect(data.Name).toBeUndefined();
    expect(data.Status).toBeUndefined();
  });

  test('workflow status endpoint returns 17 stages', async ({ request }) => {
    const createResp = await request.post('/api/projects', {
      data: { name: 'Workflow Stages Test' },
    });
    const { data: project } = await createResp.json();

    const statusResp = await request.get(
      `/api/projects/${project.id}/workflow`
    );
    expect(statusResp.ok()).toBe(true);
    const { data: status } = await statusResp.json();

    expect(status.project_id).toBe(project.id);
    expect(status.stages).toBeDefined();
    expect(Array.isArray(status.stages)).toBe(true);
    expect(status.stages.length).toBe(17);

    const firstStage = status.stages[0];
    expect(firstStage.id).toBe('foundations');
    expect(firstStage.name).toBeTruthy();
    expect(firstStage.number).toBe(1);
  });
});
