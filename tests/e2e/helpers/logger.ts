/**
 * Structured E2E test logging utility.
 * Each test step is logged as a JSON object with timing and outcome.
 */

import { test } from '@playwright/test';

interface StepLog {
  timestamp: string;
  step: string;
  action: string;
  duration_ms: number;
  result: 'pass' | 'fail' | 'skip';
  details?: Record<string, unknown>;
}

/**
 * Runs a named test step with structured logging.
 * Captures timing, result, and optional details.
 */
export async function loggedStep<T>(
  stepName: string,
  action: string,
  fn: () => Promise<T>,
  details?: Record<string, unknown>,
): Promise<T> {
  const start = Date.now();
  const log: StepLog = {
    timestamp: new Date().toISOString(),
    step: stepName,
    action,
    duration_ms: 0,
    result: 'pass',
    details,
  };

  try {
    const result = await test.step(stepName, async () => {
      return fn();
    });
    log.duration_ms = Date.now() - start;
    console.log(JSON.stringify(log));
    return result;
  } catch (error) {
    log.duration_ms = Date.now() - start;
    log.result = 'fail';
    log.details = {
      ...log.details,
      error: error instanceof Error ? error.message : String(error),
    };
    console.log(JSON.stringify(log));
    throw error;
  }
}

/**
 * Logs an API call with request/response details.
 */
export function logApiCall(
  method: string,
  path: string,
  status: number,
  duration_ms: number,
) {
  console.log(
    JSON.stringify({
      timestamp: new Date().toISOString(),
      type: 'api_call',
      method,
      path,
      status,
      duration_ms,
    }),
  );
}
