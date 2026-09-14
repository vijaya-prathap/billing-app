import type { ApiErrorBody, HealthResponse } from "../types";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly requestId?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { Accept: "application/json", ...init?.headers },
  });

  const body: unknown = await res.json().catch(() => null);

  if (!res.ok) {
    const err = (body as ApiErrorBody | null)?.error;
    throw new ApiError(res.status, err?.message ?? `HTTP ${res.status}`, err?.request_id);
  }
  return body as T;
}

export const api = {
  health: () => request<HealthResponse>("/health"),
  ready: () => request<HealthResponse>("/ready"),
};
