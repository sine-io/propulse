import type { components } from "./generated-api";
import { clearAccessToken, getAccessToken } from "./access-token";

export type HousingCapacityInput = components["schemas"]["HousingCapacityInput"];
export type LoanParams = components["schemas"]["LoanParams"];
export type CityPolicyOverride = components["schemas"]["CityPolicyOverride"];
export type AppliedAssumptions = components["schemas"]["AppliedAssumptions"];
export type HousingCapacityResult = components["schemas"]["HousingCapacityResult"];
export type CapacityAssumptionsResponse =
  components["schemas"]["CapacityAssumptionsResponse"];
export type CalculationResponse = components["schemas"]["CalculationResponse"];
export type CapacityCalculationResponse = CalculationResponse;
export type WatchlistResponse = components["schemas"]["WatchlistResponse"];
export type WatchlistItem = components["schemas"]["WatchlistItem"];
export type ActionWindowResponse =
  components["schemas"]["ActionWindowResponse"];
export type ErrorResponse = components["schemas"]["ErrorResponse"];
export type AccessStatusResponse = components["schemas"]["AccessStatusResponse"];

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

export async function getCapacityAssumptions(
  signal?: AbortSignal,
): Promise<CapacityAssumptionsResponse> {
  return request<CapacityAssumptionsResponse>(
    "/api/v1/capacity/assumptions",
    signal ? { signal } : undefined,
  );
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

export async function verifyAccessToken(
  token: string,
  signal?: AbortSignal,
): Promise<AccessStatusResponse> {
  return request<AccessStatusResponse>(
    "/api/v1/access",
    signal ? { signal } : undefined,
    token,
  );
}

async function request<T>(
  url: string,
  init?: RequestInit,
  explicitToken?: string,
): Promise<T> {
  const storedToken = getAccessToken();
  const token = explicitToken?.trim() || storedToken;
  let requestInit = init;
  if (token) {
    const headers = new Headers(init?.headers);
    headers.set("Authorization", `Bearer ${token}`);
    requestInit = { ...init, headers };
  }

  const response = await fetch(url, requestInit);
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

    if (response.status === 401 && storedToken && token === storedToken) {
      clearAccessToken();
    }
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
