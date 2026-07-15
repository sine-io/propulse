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
export type CreateDataSourceRequest = components["schemas"]["CreateDataSourceRequest"];
export type DataSource = components["schemas"]["DataSource"];
export type CreateNeighborhoodRequest = components["schemas"]["CreateNeighborhoodRequest"];
export type Neighborhood = components["schemas"]["NeighborhoodResponse"];
export type NeighborhoodSearchResponse = components["schemas"]["NeighborhoodSearchResponse"];
export type NeighborhoodMetricResponse = components["schemas"]["NeighborhoodMetricResponse"];
export type MetricHistoryResponse = components["schemas"]["MetricHistoryResponse"];
export type MetricHistoryPoint = components["schemas"]["MetricHistoryPoint"];
export type MetricComparison = components["schemas"]["MetricComparison"];
export type MetricChangeValue = components["schemas"]["MetricChangeValue"];
export type AddWatchlistItemRequest = components["schemas"]["AddWatchlistItemRequest"];
export type AddWatchlistItemResponse = components["schemas"]["AddWatchlistItemResponse"];
export type ImportJSONRequest = components["schemas"]["ImportJSONRequest"];
export type ImportJSONRecord = components["schemas"]["ImportJSONRecord"];
export type ImportCollectionRunResponse = components["schemas"]["ImportCollectionRunResponse"];
export type CollectionRunDetail = components["schemas"]["CollectionRunDetail"];
export type ValidationIssue = components["schemas"]["ValidationIssue"];
export type ImportMetadata = Omit<ImportJSONRequest, "records">;

export interface NeighborhoodSearchQuery {
  area?: string;
  city?: string;
  page?: number;
  pageSize?: number;
  q?: string;
  targetLayout?: string;
}

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
    public readonly details: ValidationIssue[] = [],
    public readonly acceptedRecordCount?: number,
    public readonly rejectedRecordCount?: number,
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

export async function addWatchlistItem(
  input: AddWatchlistItemRequest,
  signal?: AbortSignal,
): Promise<AddWatchlistItemResponse> {
  return request<AddWatchlistItemResponse>("/api/v1/watchlist/items", {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "POST",
    signal,
  });
}

export async function getActionWindow(
  neighborhoodId: string,
  signal?: AbortSignal,
): Promise<ActionWindowResponse> {
  const params = new URLSearchParams({ neighborhoodId });
  return request<ActionWindowResponse>(
    `/api/v1/decision/action-window?${params.toString()}`,
    signal ? { signal } : undefined,
  );
}

export async function getMetricHistory(
  neighborhoodId: string,
  targetLayout: string,
  window: { from?: string; to?: string } = {},
  signal?: AbortSignal,
): Promise<MetricHistoryResponse> {
  const params = new URLSearchParams({ targetLayout });
  if (window.from) params.set("from", window.from);
  if (window.to) params.set("to", window.to);
  return request<MetricHistoryResponse>(
    `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}/metrics/history?${params.toString()}`,
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

export async function listDataSources(signal?: AbortSignal): Promise<DataSource[]> {
  const response = await request<{ items: DataSource[] }>(
    "/admin/api/data-sources",
    signal ? { signal } : undefined,
  );
  return response.items;
}

export async function createDataSource(
  input: CreateDataSourceRequest,
  signal?: AbortSignal,
): Promise<DataSource> {
  return request<DataSource>("/admin/api/data-sources", {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "POST",
    signal,
  });
}

export async function searchNeighborhoods(
  query: string | NeighborhoodSearchQuery = "",
  signal?: AbortSignal,
): Promise<NeighborhoodSearchResponse> {
  const filters: NeighborhoodSearchQuery = typeof query === "string" ? { q: query } : query;
  const params = new URLSearchParams({
    page: String(filters.page ?? 1),
    pageSize: String(filters.pageSize ?? 50),
  });
  for (const [key, value] of [
    ["q", filters.q],
    ["city", filters.city],
    ["area", filters.area],
    ["targetLayout", filters.targetLayout],
  ] as const) {
    if (value?.trim()) params.set(key, value.trim());
  }
  return request<NeighborhoodSearchResponse>(
    `/api/v1/neighborhoods?${params.toString()}`,
    signal ? { signal } : undefined,
  );
}

export async function getNeighborhood(
  neighborhoodId: string,
  signal?: AbortSignal,
): Promise<Neighborhood> {
  return request<Neighborhood>(
    `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}`,
    signal ? { signal } : undefined,
  );
}

export async function getNeighborhoodMetrics(
  neighborhoodId: string,
  targetLayout: string,
  signal?: AbortSignal,
): Promise<NeighborhoodMetricResponse> {
  const params = new URLSearchParams({ targetLayout });
  return request<NeighborhoodMetricResponse>(
    `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}/metrics?${params.toString()}`,
    signal ? { signal } : undefined,
  );
}

export async function createNeighborhood(
  input: CreateNeighborhoodRequest,
  signal?: AbortSignal,
): Promise<Neighborhood> {
  return request<Neighborhood>("/api/v1/neighborhoods", {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "POST",
    signal,
  });
}

export async function importJSONCollectionRun(
  input: ImportJSONRequest,
  signal?: AbortSignal,
): Promise<ImportCollectionRunResponse> {
  return request<ImportCollectionRunResponse>("/admin/api/imports/json", {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "POST",
    signal,
  });
}

export async function importCSVCollectionRun(
  metadata: ImportMetadata,
  file: File,
  signal?: AbortSignal,
): Promise<ImportCollectionRunResponse> {
  const body = new FormData();
  body.set("dataSourceId", metadata.dataSourceId);
  body.set("neighborhoodId", metadata.neighborhoodId);
  body.set("sourceRef", metadata.sourceRef);
  body.set("collectedAt", metadata.collectedAt);
  body.set("coverage", metadata.coverage);
  body.set("file", file);
  return request<ImportCollectionRunResponse>("/admin/api/imports/csv", {
    body,
    method: "POST",
    signal,
  });
}

export async function getCollectionRunDetail(
  id: string,
  signal?: AbortSignal,
): Promise<CollectionRunDetail> {
  return request<CollectionRunDetail>(
    `/admin/api/imports/${encodeURIComponent(id)}`,
    signal ? { signal } : undefined,
  );
}

export async function getCSVImportTemplate(signal?: AbortSignal): Promise<Blob> {
  const response = await authorizedResponse(
    "/admin/api/imports/csv/template",
    signal ? { signal } : undefined,
  );
  return response.blob();
}

async function request<T>(
  url: string,
  init?: RequestInit,
  explicitToken?: string,
): Promise<T> {
  const response = await authorizedResponse(url, init, explicitToken);
  return response.json() as Promise<T>;
}

async function authorizedResponse(
  url: string,
  init?: RequestInit,
  explicitToken?: string,
): Promise<Response> {
  const storedToken = getAccessToken();
  const token = explicitToken?.trim() || storedToken;
  let requestInit = init;
  if (token) {
    const headers = new Headers(init?.headers);
    headers.set("Authorization", `Bearer ${token}`);
    requestInit = { ...init, headers };
  }

  const response = await fetch(url, requestInit);
  if (!response.ok) {
    const data = await response.json().catch(() => undefined);
    const error = isAPIErrorPayload(data)
      ? data.error
      : {
          code: `http_${response.status}`,
          message: response.statusText || "API request failed",
        };

    if (response.status === 401 && storedToken && token === storedToken) {
      clearAccessToken();
    }
    throw new ApiError(
      error.code,
      error.message,
      response.status,
      isAPIErrorPayload(data) ? data.error.details ?? [] : [],
      isAPIErrorPayload(data) ? data.acceptedRecordCount : undefined,
      isAPIErrorPayload(data) ? data.rejectedRecordCount : undefined,
    );
  }

  return response;
}

interface APIErrorPayload {
  error: {
    code: string;
    message: string;
    details?: ValidationIssue[];
  };
  acceptedRecordCount?: number;
  rejectedRecordCount?: number;
}

function isAPIErrorPayload(value: unknown): value is APIErrorPayload {
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
