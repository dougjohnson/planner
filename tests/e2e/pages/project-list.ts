import { Page, Locator } from '@playwright/test';

/** Page object model for the project list screen. */
export class ProjectListPage {
  readonly page: Page;
  readonly heading: Locator;
  readonly createButton: Locator;
  readonly projectCards: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole('heading', { name: /projects/i });
    this.createButton = page.getByRole('button', { name: /create|new/i });
    this.projectCards = page.locator('[data-testid="project-card"]');
  }

  async goto() {
    await this.page.goto('/');
  }

  async projectCount(): Promise<number> {
    return this.projectCards.count();
  }
}
