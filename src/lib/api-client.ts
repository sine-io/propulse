import type { components } from "./generated-api";

export type HousingCapacityInput = components["schemas"]["HousingCapacityInput"];
export type CapacityCalculationResponse =
  components["schemas"]["CreateCalculationResponse"];
export type WatchlistResponse = components["schemas"]["WatchlistResponse"];
export type WatchlistItem = components["schemas"]["WatchlistItem"];
export type ActionWindowResponse =
  components["schemas"]["ActionWindowResponse"];
export type ErrorResponse = components["schemas"]["ErrorResponse"];

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function createCapacityCalculation(
  input: HousingCapacityInput,
  signal?: AbortSignal,
): Promise<CapacityCalculationResponse> {
  return request<CapacityCalculationResponse>("/api/v1/capacity/calculations", {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "POST",
    signal,
  });
}

export async function getWatchlist(
  signal?: AbortSignal,
): Promise<WatchlistResponse> {
  return request<WatchlistResponse>(
    "/api/v1/watchlist",
    signal ? { signal } : undefined,
  );
}

export async function getActionWindow(
  signal?: AbortSignal,
): Promise<ActionWindowResponse> {
  return request<ActionWindowResponse>(
    "/api/v1/decision/action-window",
    signal ? { signal } : undefined,
  );
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, init);
  const data = (await response.json().catch(() => undefined)) as
    | T
    | ErrorResponse
    | undefined;

  if (!response.ok) {
    const error = isErrorResponse(data)
      ? data.error
      : {
          code: `http_${response.status}`,
          message: response.statusText || "API request failed",
        };

    throw new ApiError(error.code, error.message, response.status);
  }

  return data as T;
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  if (!value || typeof value !== "object" || !("error" in value)) {
    return false;
  }

  const error = (value as { error: unknown }).error;

  return (
    !!error &&
    typeof error === "object" &&
    typeof (error as { code?: unknown }).code === "string" &&
    typeof (error as { message?: unknown }).message === "string"
  );
}
