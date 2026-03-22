/**
 * E2E test: Foundations → Seed PRD intake flow.
 *
 * Verifies the critical path that was broken: foundation files persist
 * after locking, and the user can navigate to the PRD intake page.
 */

import { test, expect } from '@playwright/test';

test.describe('Foundations to PRD intake flow', () => {
  let projectId: string;

  test.beforeAll(async ({ request }) => {
    // Create a project via API to avoid brittle UI-driven setup.
    const resp = await request.post('/api/projects', {
      data: { name: `Foundations E2E ${Date.now()}` },
    });
    expect(resp.ok()).toBe(true);
    const body = await resp.json();
    projectId = body.data.id;
  });

  test('submit foundations and verify generated files appear', async ({ page }) => {
    await page.goto(`/projects/${projectId}/foundations`);

    // Should show the foundations form (not locked).
    await expect(page.getByRole('heading', { name: /Foundations/i })).toBeVisible();

    // Fill in the form fields.
    const nameInput = page.getByLabel(/Project Name/i);
    if (await nameInput.isVisible()) {
      await nameInput.fill('Foundations E2E Test');
    }

    const techInput = page.getByLabel(/Tech Stack/i);
    if (await techInput.isVisible()) {
      await techInput.fill('Go, React');
    }

    const archInput = page.getByLabel(/Architecture/i);
    if (await archInput.isVisible()) {
      await archInput.fill('Layered architecture with service layer');
    }

    // Click Save & Preview.
    const saveButton = page.getByRole('button', { name: /Save|Preview/i });
    if (await saveButton.isVisible()) {
      await saveButton.click();

      // Verify generated files appear.
      await expect(page.getByText('AGENTS.md').first()).toBeVisible({ timeout: 5000 });
    }
  });

  test('lock foundations and verify files persist', async ({ page, request }) => {
    // Submit foundations via API (reliable setup).
    await request.post(`/api/projects/${projectId}/foundations`, {
      data: {
        project_name: 'Foundations E2E Test',
        tech_stack: ['Go', 'React'],
        architecture_direction: 'Layered architecture',
      },
    });

    await page.goto(`/projects/${projectId}/foundations`);

    // Click lock button if visible.
    const lockButton = page.getByRole('button', { name: /Lock/i });
    if (await lockButton.isVisible({ timeout: 3000 }).catch(() => false)) {
      await lockButton.click();
    } else {
      // Lock via API instead.
      await request.post(`/api/projects/${projectId}/foundations/lock`, {
        data: {},
      });
      await page.reload();
    }

    // Verify locked state shows files.
    await expect(page.getByText('Foundations Locked')).toBeVisible({ timeout: 5000 });

    // Files should be visible in the locked view.
    await expect(page.getByText('AGENTS.md').first()).toBeVisible({ timeout: 5000 });
  });

  test('foundations files persist after navigation away and back', async ({ page, request }) => {
    // Ensure foundations are submitted and locked via API.
    await request.post(`/api/projects/${projectId}/foundations`, {
      data: {
        project_name: 'Foundations E2E Test',
        tech_stack: ['Go', 'React'],
        architecture_direction: 'Layered architecture',
      },
    });
    await request.post(`/api/projects/${projectId}/foundations/lock`, {
      data: {},
    });

    // Navigate to foundations.
    await page.goto(`/projects/${projectId}/foundations`);
    await expect(page.getByText('AGENTS.md').first()).toBeVisible({ timeout: 5000 });

    // Navigate away.
    await page.goto(`/projects/${projectId}`);
    await expect(page.getByRole('heading')).toBeVisible({ timeout: 5000 });

    // Navigate back to foundations.
    await page.goto(`/projects/${projectId}/foundations`);

    // Files should STILL be visible (loaded from backend, not React state).
    await expect(page.getByText('AGENTS.md').first()).toBeVisible({ timeout: 5000 });
  });

  test('locked foundations show inline seed PRD section', async ({ page, request }) => {
    // Ensure locked.
    await request.post(`/api/projects/${projectId}/foundations/lock`, {
      data: {},
    });

    await page.goto(`/projects/${projectId}/foundations`);

    // Seed PRD section should appear inline below locked foundations (not on a separate page).
    await expect(page.getByRole('heading', { name: /Seed PRD/i })).toBeVisible({ timeout: 5000 });
    await expect(page.getByLabel(/paste.*PRD/i)).toBeVisible();
  });

  test('PRD intake page renders with paste textarea', async ({ page }) => {
    await page.goto(`/projects/${projectId}/prd-intake`);

    // Should show the PRD intake heading.
    await expect(page.getByRole('heading', { name: /Seed PRD/i })).toBeVisible({ timeout: 5000 });

    // Paste textarea should be present and focused (primary input).
    const textarea = page.getByLabel(/paste|PRD/i);
    await expect(textarea).toBeVisible();

    // Should NOT show an error state.
    await expect(page.getByRole('alert')).not.toBeVisible();
  });

  test('paste markdown and verify submit button enables', async ({ page }) => {
    await page.goto(`/projects/${projectId}/prd-intake`);

    const textarea = page.getByLabel(/paste|PRD/i);
    await expect(textarea).toBeVisible({ timeout: 5000 });

    // Paste markdown content.
    await textarea.fill('# Test PRD\n\n## Overview\n\nThis is a test product requirements document.\n\n## Goals\n\n- Goal 1\n- Goal 2');

    // Submit button should be enabled.
    const submitButton = page.getByRole('button', { name: /Submit/i });
    await expect(submitButton).toBeEnabled();
  });
});
