/**
 * SSE event listener utility for E2E tests.
 * Subscribes to project events and waits for specific event types.
 */
export class SSEListener {
  private events: Array<{ type: string; data: unknown }> = [];
  private eventSource: EventSource | null = null;

  constructor(
    private baseURL: string,
    private projectId: string,
  ) {}

  start(): void {
    const url = `${this.baseURL}/api/projects/${this.projectId}/events`;
    this.eventSource = new EventSource(url);

    this.eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.events.push({ type: data.event_type || event.type, data });
      } catch {
        this.events.push({ type: event.type, data: event.data });
      }
    };
  }

  async waitForEvent(eventType: string, timeoutMs = 10000): Promise<unknown> {
    const start = Date.now();
    while (Date.now() - start < timeoutMs) {
      const found = this.events.find((e) => e.type === eventType);
      if (found) return found.data;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error(`Timeout waiting for event: ${eventType}`);
  }

  getEvents(): Array<{ type: string; data: unknown }> {
    return [...this.events];
  }

  stop(): void {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
  }
}
