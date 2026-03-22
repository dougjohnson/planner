import { Page, Locator } from '@playwright/test';

/** Page object model for the project dashboard. */
export class ProjectDashboardPage {
  readonly page: Page;
  readonly heading: Locator;
  readonly stageCards: Locator;
  readonly timeline: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole('heading', { level: 1 });
    this.stageCards = page.locator('[data-testid="stage-card"]');
    this.timeline = page.locator('[data-testid="workflow-timeline"]');
  }

  async goto(projectId: string) {
    await this.page.goto(`/projects/${projectId}`);
  }
}
