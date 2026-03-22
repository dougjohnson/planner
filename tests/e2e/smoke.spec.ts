import { test, expect } from '@playwright/test';
import { APIHelper } from './helpers/api';

test.describe('Smoke tests', () => {
  let api: APIHelper;

  test.beforeEach(async ({ request }) => {
    api = new APIHelper(request);
  });

  test('health endpoint returns ok', async ({ request }) => {
    const resp = await request.get('/api/health');
    expect(resp.ok()).toBe(true);
    const body = await resp.json();
    expect(body.status).toBe('ok');
  });

  test('create and retrieve project via API', async () => {
    const project = await api.createProject('E2E Test Project', 'Created by Playwright');
    expect(project).toBeDefined();
    expect(project.ID || project.id).toBeTruthy();

    const retrieved = await api.getProject(project.ID || project.id);
    expect(retrieved).toBeDefined();
  });

  test('list projects returns array', async () => {
    const projects = await api.listProjects();
    expect(Array.isArray(projects)).toBe(true);
  });

  test('workflow status returns stages', async () => {
    const project = await api.createProject('Workflow Test');
    const id = project.ID || project.id;

    const status = await api.getWorkflowStatus(id);
    expect(status).toBeDefined();
    expect(status.stages).toBeDefined();
  });
});
