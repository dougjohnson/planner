/**
 * E2E test: First-use experience — new user creates their first project.
 *
 * This tests the critical path that every new user hits. It would have
 * caught the PascalCase/snake_case JSON mismatch that caused "Failed to
 * load projects" on the Projects page.
 */

import { test, expect } from '@playwright/test';

test.describe('First-use experience', () => {

  test('projects page loads without error on empty database', async ({ page }) => {
    await page.goto('/projects');

    // Should NOT show an error state.
    await expect(page.getByRole('alert')).not.toBeVisible();
    await expect(page.getByText('Failed')).not.toBeVisible();

    // Should show empty state with create CTA.
    await expect(page.getByText('No projects yet')).toBeVisible();
    await expect(page.getByRole('link', { name: /Create Project/i })).toBeVisible();
  });

  test('create first project and land on dashboard', async ({ page }) => {
    // Start at projects list.
    await page.goto('/projects');

    // Click create.
    await page.getByRole('link', { name: /Create Project/i }).click();
    await expect(page).toHaveURL(/\/projects\/new/);

    // Fill required field.
    await page.getByLabel(/Project Name/i).fill('My First Plan');

    // Optionally fill description.
    await page.getByLabel(/Description/i).fill('Testing the first-use flow');

    // Submit.
    await page.getByRole('button', { name: /Create Project/i }).click();

    // Should navigate to the project (foundations or dashboard).
    await expect(page).toHaveURL(/\/projects\/[a-f0-9-]+/);

    // Should NOT show any error.
    await expect(page.getByRole('alert')).not.toBeVisible();
    await expect(page.getByText('Failed')).not.toBeVisible();
  });

  test('created project appears in project list', async ({ page, request }) => {
    // Create a project via API.
    const resp = await request.post('/api/projects', {
      data: { name: 'E2E List Test' },
    });
    expect(resp.ok()).toBe(true);
    const body = await resp.json();
    expect(body.data.id).toBeTruthy();

    // Navigate to projects list.
    await page.goto('/projects');

    // The project should appear (not error state).
    await expect(page.getByText('E2E List Test')).toBeVisible({ timeout: 5000 });
  });

  test('project API returns properly cased JSON fields', async ({ request }) => {
    // Create project.
    const createResp = await request.post('/api/projects', {
      data: { name: 'JSON Field Test', description: 'Checking casing' },
    });
    expect(createResp.ok()).toBe(true);
    const { data } = await createResp.json();

    // Verify snake_case fields (not PascalCase).
    expect(data.id).toBeTruthy();
    expect(data.name).toBe('JSON Field Test');
    expect(data.description).toBe('Checking casing');
    expect(data.status).toBe('active');
    expect(data.created_at).toBeTruthy();
    expect(data.updated_at).toBeTruthy();

    // These PascalCase fields should NOT exist.
    expect(data.ID).toBeUndefined();
    expect(data.Name).toBeUndefined();
    expect(data.Status).toBeUndefined();

    // List projects.
    const listResp = await request.get('/api/projects');
    expect(listResp.ok()).toBe(true);
    const listBody = await listResp.json();
    expect(Array.isArray(listBody.data)).toBe(true);

    if (listBody.data.length > 0) {
      const p = listBody.data[0];
      expect(p.id).toBeTruthy();
      expect(p.name).toBeTruthy();
      expect(p.ID).toBeUndefined();
    }
  });

  test('workflow status endpoint returns stage data', async ({ request }) => {
    // Create project.
    const createResp = await request.post('/api/projects', {
      data: { name: 'Workflow Status Test' },
    });
    const { data: project } = await createResp.json();

    // Get workflow status.
    const statusResp = await request.get(
      `/api/projects/${project.id}/workflow`
    );
    expect(statusResp.ok()).toBe(true);
    const { data: status } = await statusResp.json();

    expect(status.project_id).toBe(project.id);
    expect(status.stages).toBeDefined();
    expect(Array.isArray(status.stages)).toBe(true);
    expect(status.stages.length).toBe(17);

    // First stage should be ready (new project).
    const firstStage = status.stages[0];
    expect(firstStage.id).toBe('foundations');
    expect(firstStage.name).toBeTruthy();
    expect(firstStage.number).toBe(1);
  });
});
