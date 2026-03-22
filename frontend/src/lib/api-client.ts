/**
 * Typed API client for the flywheel-planner backend.
 *
 * Wraps fetch with: /api/ prefix, JSON handling, consistent error envelope
 * parsing (§14.1), and automatic idempotency-key generation for mutating
 * requests (§15.1).
 */

const BASE_URL = "/api";

/** Response envelope from the backend (§14.1). */
export interface ApiEnvelope<T> {
  data: T;
  error: ApiError | null;
  meta: Record<string, unknown>;
}

/** Machine-readable error from the backend. */
export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

/** Thrown when an API call fails. */
export class ApiClientError extends Error {
  constructor(
    public readonly status: number,
    public readonly apiError: ApiError | null,
    message: string,
  ) {
    super(message);
    this.name = "ApiClientError";
  }
}

/** Generate a UUID v4 for idempotency keys. */
function generateIdempotencyKey(): string {
  return crypto.randomUUID();
}

interface RequestOptions {
  /** Skip the idempotency key header (for GET/HEAD). */
  skipIdempotency?: boolean;
  /** Additional headers. */
  headers?: Record<string, string>;
  /** AbortSignal for cancellation. */
  signal?: AbortSignal;
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options: RequestOptions = {},
): Promise<T> {
  const url = `${BASE_URL}${path}`;
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...options.headers,
  };

  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  // Attach idempotency key to all mutating requests (§15.1).
  const isMutating = !["GET", "HEAD"].includes(method.toUpperCase());
  if (isMutating && !options.skipIdempotency) {
    headers["Idempotency-Key"] = generateIdempotencyKey();
  }

  const response = await fetch(url, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal: options.signal,
  });

  // Handle non-JSON responses (e.g., file downloads).
  const contentType = response.headers.get("Content-Type") ?? "";
  if (!contentType.includes("application/json")) {
    if (!response.ok) {
      throw new ApiClientError(
        response.status,
        null,
        `HTTP ${response.status}: ${response.statusText}`,
      );
    }
    // Return the response body as-is for non-JSON (cast is intentional).
    return (await response.text()) as unknown as T;
  }

  const envelope: ApiEnvelope<T> = await response.json();

  if (!response.ok || envelope.error) {
    throw new ApiClientError(
      response.status,
      envelope.error,
      envelope.error?.message ?? `HTTP ${response.status}`,
    );
  }

  return envelope.data;
}

/** GET a resource. */
export function get<T>(
  path: string,
  options?: RequestOptions,
): Promise<T> {
  return request<T>("GET", path, undefined, {
    ...options,
    skipIdempotency: true,
  });
}

/** POST a resource. */
export function post<T>(
  path: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<T> {
  return request<T>("POST", path, body, options);
}

/** PUT a resource. */
export function put<T>(
  path: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<T> {
  return request<T>("PUT", path, body, options);
}

/** PATCH a resource. */
export function patch<T>(
  path: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<T> {
  return request<T>("PATCH", path, body, options);
}

/** DELETE a resource. */
export function del<T>(
  path: string,
  options?: RequestOptions,
): Promise<T> {
  return request<T>("DELETE", path, undefined, options);
}
