import type { ApiError as ApiErrorBody } from "@staykey/api-client";

/** A failed API call, with the server's error code and message when it sent one. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly field?: string;
  readonly retryAfter?: number;

  constructor(status: number, body?: Partial<ApiErrorBody>) {
    super(body?.message ?? fallbackMessage(status));
    this.name = "ApiError";
    this.status = status;
    this.code = body?.code ?? `http_${status}`;
    this.field = body?.field;
    this.retryAfter = body?.retryAfter;
  }
}

/** Thrown when the API can't be reached at all. */
export class OfflineError extends Error {
  constructor() {
    super("Can't reach StayKey right now. Check your connection and try again.");
    this.name = "OfflineError";
  }
}

function fallbackMessage(status: number) {
  if (status === 401) return "Your sign-in has expired. Sign in again.";
  if (status >= 500) return "Something went wrong on our side. Please try again.";
  return "That didn't work. Please try again.";
}

type Result<T> = { data?: T; error?: unknown; response: Response };

/**
 * Runs an openapi-fetch call and returns its data, or throws ApiError or OfflineError, so query
 * and mutation functions read as plain async code.
 */
export async function unwrap<T>(call: Promise<Result<T>>): Promise<T> {
  let result: Result<T>;
  try {
    result = await call;
  } catch {
    throw new OfflineError();
  }
  if (result.response.ok) return result.data as T;
  throw new ApiError(result.response.status, result.error as Partial<ApiErrorBody> | undefined);
}

/** A message fit to show the owner for any error a call can throw. */
export function messageFor(error: unknown): string {
  if (error instanceof ApiError || error instanceof OfflineError) return error.message;
  return "Something went wrong. Please try again.";
}
