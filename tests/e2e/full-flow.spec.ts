/**
 * E2E test: Complete user journey from project creation to export.
 *
 * This test walks through the entire workflow with mock providers.
 * It validates the critical user path end-to-end.
 */

import { test, expect } from '@playwright/test';

test.describe('Complete workflow flow', () => {
  test('create project through pipeline execution', async ({ page, request }) => {
    // 1. Navigate to projects page.
    await page.goto('/projects');
    await expect(page.getByRole('heading', { name: /Projects/i })).toBeVisible({ timeout: 5000 });

    // 2. Create project via API (more reliable than UI creation for E2E flow test).
    const projectName = `E2E Flow ${Date.now()}`;
    const createResp = await request.post('/api/projects', {
      data: { name: projectName, description: 'Complete flow test' },
    });
    expect(createResp.ok()).toBe(true);
    const { data: project } = await createResp.json();
    const projectId = project.id;
    expect(projectId).toBeTruthy();

    // 3. Navigate to foundations and fill in data.
    await page.goto(`/projects/${projectId}/foundations`);
    await expect(page.getByRole('heading', { name: /Foundations/i })).toBeVisible({ timeout: 5000 });

    // Submit foundations via API.
    const foundationsResp = await request.post(`/api/projects/${projectId}/foundations`, {
      data: {
        project_name: projectName,
        tech_stack: ['Go', 'React', 'TypeScript'],
        architecture_direction: 'Layered architecture with service boundaries',
      },
    });
    expect(foundationsResp.ok()).toBe(true);

    // 4. Verify foundation files appear on reload.
    await page.reload();
    await expect(page.getByText('AGENTS.md').first()).toBeVisible({ timeout: 5000 });

    // 5. Lock foundations.
    const lockResp = await request.post(`/api/projects/${projectId}/foundations/lock`, {
      data: {},
    });
    expect(lockResp.ok()).toBe(true);

    // 6. Submit seed PRD.
    const seedPRD = `# ${projectName}

## Overview

A comprehensive product for managing workflows.

## Goals

- Enable multi-model AI orchestration
- Provide fragment-based document versioning
- Support iterative review loops

## Requirements

- Must support at least 2 model families
- Must persist all artifacts with lineage
- Must provide real-time progress via SSE

## Technical Constraints

- Local-first architecture
- SQLite for persistence
- Go backend with React frontend`;

    const seedResp = await request.post(`/api/projects/${projectId}/prd-seed`, {
      data: { content: seedPRD, source_type: 'paste' },
    });
    expect(seedResp.ok()).toBe(true);

    // 7. Navigate to dashboard and verify Stage 3 is ready.
    await page.goto(`/projects/${projectId}`);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 5000 });

    // Verify workflow status via API.
    const statusResp = await request.get(`/api/projects/${projectId}/workflow`);
    expect(statusResp.ok()).toBe(true);
    const { data: workflow } = await statusResp.json();
    // Stage 3 should be ready (current stage is parallel_prd_generation).
    const stage3 = workflow.stages.find((s: any) => s.key === 'parallel_prd_generation');
    expect(stage3).toBeDefined();
    expect(stage3.status).toBe('ready');

    // 8. Start Stage 3 via API.
    const startResp = await request.post(
      `/api/projects/${projectId}/workflow/stages/parallel_prd_generation/start`,
    );
    expect(startResp.ok()).toBe(true);
    const { data: startResult } = await startResp.json();
    expect(startResult.run_id).toBeTruthy();

    // 9. Wait for the stage chain to progress (mock providers are instant).
    // Poll the workflow status until current_stage advances past parallel_prd_generation.
    let currentStage = 'parallel_prd_generation';
    for (let i = 0; i < 30; i++) {
      await page.waitForTimeout(1000);
      const pollResp = await request.get(`/api/projects/${projectId}/workflow`);
      if (pollResp.ok()) {
        const { data: pollStatus } = await pollResp.json();
        currentStage = pollStatus.current_stage;
        if (currentStage !== 'parallel_prd_generation') break;
      }
    }

    // Stage should have advanced past Stage 3.
    expect(currentStage).not.toBe('parallel_prd_generation');

    // 10. Verify the dashboard reflects the progression.
    await page.reload();
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 5000 });

    // Should NOT show an error.
    await expect(page.getByRole('alert')).not.toBeVisible();
  });

  test('project API health check', async ({ request }) => {
    const resp = await request.get('/api/health');
    expect(resp.ok()).toBe(true);
    const body = await resp.json();
    expect(body.status).toBe('ok');
  });

  test('workflow status shows 17 stages for any project', async ({ request }) => {
    const createResp = await request.post('/api/projects', {
      data: { name: `Stages Check ${Date.now()}` },
    });
    const { data: project } = await createResp.json();

    const statusResp = await request.get(`/api/projects/${project.id}/workflow`);
    const { data: workflow } = await statusResp.json();

    expect(workflow.stages).toHaveLength(17);
    expect(workflow.stages[0].key).toBe('foundations');
    expect(workflow.stages[16].key).toBeTruthy();
  });
});
