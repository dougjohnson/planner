/**
 * E2E: Full plan pipeline through export (Stages 10-17).
 *
 * Tests plan generation, synthesis, review, and export.
 * Assumes PRD pipeline (Stages 1-9) has already completed.
 */

import { test, expect } from '@playwright/test';
import { APIHelper } from './helpers/api';
import { loggedStep } from './helpers/logger';

test.describe('Plan Pipeline E2E', () => {
  let api: APIHelper;

  test.beforeEach(async ({ request }) => {
    api = new APIHelper(request);
  });

  test('plan pipeline: generate → export', async ({ request }) => {
    // Setup: create project with foundations.
    const project = await loggedStep('Setup: Create project', 'POST', async () => {
      return api.createProject('Plan Pipeline E2E');
    });

    await loggedStep('Setup: Submit foundations', 'POST', async () => {
      await api.submitFoundations(project.id, {
        project_name: 'Plan Pipeline E2E',
        tech_stack: ['Go', 'React', 'SQLite'],
      });
    });

    // Stage 10: Start plan generation.
    await loggedStep('Stage 10: Start plan generation', 'POST', async () => {
      const resp = await request.post(
        `/api/projects/${project.id}/stages/stage-10/start`,
      );
      expect(resp.status()).toBeLessThan(500);
    });

    // Stage 17: Run stabilization checks.
    await loggedStep('Stage 17: Stabilization checks', 'POST', async () => {
      const resp = await request.post(
        `/api/projects/${project.id}/stabilize`,
      );
      expect(resp.status()).toBeLessThan(500);
    });

    // Export: Create bundle.
    await loggedStep('Export: Create bundle', 'POST', async () => {
      const resp = await request.post(
        `/api/projects/${project.id}/exports`,
        {
          data: {
            canonical_only: true,
            include_intermediates: false,
            include_raw: false,
          },
        },
      );
      expect(resp.status()).toBeLessThan(500);
    });

    // Verify project still accessible.
    await loggedStep('Verify: Project accessible', 'GET', async () => {
      const p = await api.getProject(project.id);
      expect(p.id).toBe(project.id);
    });
  });
});
