import { APIRequestContext } from '@playwright/test';

/**
 * API helper for programmatic setup/teardown in E2E tests.
 * Uses the backend API directly to create test state without brittle UI interaction.
 */
export class APIHelper {
  constructor(private request: APIRequestContext) {}

  async createProject(name: string, description = ''): Promise<{ id: string }> {
    const resp = await this.request.post('/api/projects', {
      data: { name, description },
    });
    const body = await resp.json();
    return body.data;
  }

  async getProject(id: string) {
    const resp = await this.request.get(`/api/projects/${id}`);
    return (await resp.json()).data;
  }

  async listProjects() {
    const resp = await this.request.get('/api/projects');
    return (await resp.json()).data;
  }

  async submitFoundations(projectId: string, data: {
    project_name: string;
    tech_stack?: string[];
    architecture_direction?: string;
  }) {
    const resp = await this.request.post(`/api/projects/${projectId}/foundations`, {
      data,
    });
    return (await resp.json()).data;
  }

  async getWorkflowStatus(projectId: string) {
    const resp = await this.request.get(`/api/projects/${projectId}/workflow`);
    return (await resp.json()).data;
  }

  async startStage(projectId: string, stage: string) {
    const resp = await this.request.post(`/api/projects/${projectId}/workflow/stages/${stage}/start`);
    return (await resp.json()).data;
  }

  async healthCheck(): Promise<boolean> {
    try {
      const resp = await this.request.get('/api/health');
      return resp.ok();
    } catch {
      return false;
    }
  }
}
