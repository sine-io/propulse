import type { components } from "./generated-api";
import { clearAccessToken, getAccessToken } from "./access-token";

export type HousingCapacityInput = components["schemas"]["HousingCapacityInput"];
export type LoanParams = components["schemas"]["LoanParams"];
export type CityPolicyOverride = components["schemas"]["CityPolicyOverride"];
export type TransactionScenario = components["schemas"]["TransactionScenario"];
export type LoanPlan = components["schemas"]["LoanPlan"];
export type CalculationOverrides = components["schemas"]["CalculationOverrides"];
export type HousingPolicyVersion = components["schemas"]["HousingPolicyVersion"];
export type CreateHousingPolicyVersionRequest = components["schemas"]["CreateHousingPolicyVersionRequest"];
export type AppliedAssumptions = components["schemas"]["AppliedAssumptions"];
export type HousingCapacityResult = components["schemas"]["HousingCapacityResult"];
export type CapacityAssumptionsResponse =
  components["schemas"]["CapacityAssumptionsResponse"];
export type CalculationResponse = components["schemas"]["CalculationResponse"];
export type PropertySelectionContext = components["schemas"]["PropertySelectionContext"];
export type Asset = components["schemas"]["AssetResponse"];
export type AssetsPage = components["schemas"]["AssetsPageResponse"];
export type CreateAssetInput = components["schemas"]["CreateAssetRequest"];
export type UpdateAssetInput = components["schemas"]["UpdateAssetRequest"];
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
export type CommunityMarketSnapshot = components["schemas"]["CommunityMarketSnapshot"];
export type MarketListing = components["schemas"]["MarketListing"];
export type MarketListingDetail = components["schemas"]["MarketListingDetail"];
export type MarketTransaction = components["schemas"]["MarketTransaction"];
export type ListingAdjustment = components["schemas"]["ListingAdjustment"];
export type MarketListingsPage = components["schemas"]["MarketListingsPage"];
export type MarketTransactionsPage = components["schemas"]["MarketTransactionsPage"];
export type CommunityMarketComparison = components["schemas"]["CommunityMarketComparison"];
export type MetricHistoryPoint = components["schemas"]["MetricHistoryPoint"];
export type MetricComparison = components["schemas"]["MetricComparison"];
export type MetricChangeValue = components["schemas"]["MetricChangeValue"];
export type AddWatchlistItemRequest = components["schemas"]["AddWatchlistItemRequest"];
export type AddWatchlistItemResponse = components["schemas"]["AddWatchlistItemResponse"];
export type ImportJSONRequest = components["schemas"]["ImportJSONRequest"];
export type ImportJSONRecord = components["schemas"]["ImportJSONRecord"];
export type ImportCollectionRunResponse = components["schemas"]["ImportCollectionRunResponse"];
export type CollectionRunDetail = components["schemas"]["CollectionRunDetail"];
export type CollectionRunSummary = components["schemas"]["CollectionRunSummary"];
export type CollectionRunsPage = components["schemas"]["CollectionRunsPage"];
export type ValidationIssue = components["schemas"]["ValidationIssue"];
export type ImportMetadata = Omit<ImportJSONRequest, "records">;
export type ReviewNoteKind = components["schemas"]["ReviewNoteKind"];
export type CreateReviewNoteInput = components["schemas"]["CreateReviewNoteRequest"];
export type UpdateReviewNoteInput = components["schemas"]["UpdateReviewNoteRequest"];
export type ReviewNote = components["schemas"]["ReviewNoteResponse"];
export type ReviewNotesPage = components["schemas"]["ReviewNotesPageResponse"];

export interface NeighborhoodSearchQuery {
  area?: string;
  city?: string;
  page?: number;
  pageSize?: number;
  q?: string;
  targetLayout?: string;
}

export interface CapacityAssumptionsQuery {
  city?: string;
  homePurchaseOrder?: "first" | "second";
  loanTermMonths?: number;
}

export interface CollectionRunListQuery {
  dataSourceId?: string;
  neighborhoodId?: string;
  status?: "completed";
  metricStatus?: "pending" | "completed" | "failed";
  from?: string;
  to?: string;
  page?: number;
  pageSize?: number;
}

export interface MarketListQuery {
  layout?: string;
  floor?: "高楼层" | "中楼层" | "低楼层";
  minPriceWan?: number;
  maxPriceWan?: number;
  sortBy?: "date" | "price" | "unitPrice" | "area" | "adjustments";
  sortOrder?: "asc" | "desc";
  page?: number;
  pageSize?: number;
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

export async function listAssets(
  page = 1,
  pageSize = 100,
  signal?: AbortSignal,
): Promise<AssetsPage> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
  return request<AssetsPage>(
    `/api/v1/assets?${params.toString()}`,
    signal ? { signal } : undefined,
  );
}

export async function getAsset(id: string, signal?: AbortSignal): Promise<Asset> {
  return request<Asset>(
    `/api/v1/assets/${encodeURIComponent(id)}`,
    signal ? { signal } : undefined,
  );
}

export async function createAsset(input: CreateAssetInput, signal?: AbortSignal): Promise<Asset> {
  return request<Asset>("/api/v1/assets", {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "POST",
    signal,
  });
}

export async function updateAsset(id: string, input: UpdateAssetInput, signal?: AbortSignal): Promise<Asset> {
  return request<Asset>(`/api/v1/assets/${encodeURIComponent(id)}`, {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "PATCH",
    signal,
  });
}

export async function deleteAsset(id: string, signal?: AbortSignal): Promise<void> {
  await authorizedResponse(
    `/api/v1/assets/${encodeURIComponent(id)}`,
    { method: "DELETE", signal },
  );
}

export async function getCapacityAssumptions(
  queryOrSignal: CapacityAssumptionsQuery | AbortSignal = {},
  signal?: AbortSignal,
): Promise<CapacityAssumptionsResponse> {
  const query = queryOrSignal instanceof AbortSignal ? {} : queryOrSignal;
  const requestSignal = queryOrSignal instanceof AbortSignal ? queryOrSignal : signal;
  const params = new URLSearchParams();
  if (query.city?.trim()) params.set("city", query.city.trim());
  if (query.homePurchaseOrder) params.set("homePurchaseOrder", query.homePurchaseOrder);
  if (query.loanTermMonths) params.set("loanTermMonths", String(query.loanTermMonths));
  const suffix = params.size > 0 ? `?${params.toString()}` : "";
  return request<CapacityAssumptionsResponse>(
    `/api/v1/capacity/assumptions${suffix}`,
    requestSignal ? { signal: requestSignal } : undefined,
  );
}

export async function listCapacityPolicies(
  city = "",
  signal?: AbortSignal,
): Promise<HousingPolicyVersion[]> {
  const params = new URLSearchParams();
  if (city.trim()) params.set("city", city.trim());
  const suffix = params.size > 0 ? `?${params.toString()}` : "";
  const response = await request<{ items: HousingPolicyVersion[] }>(
    `/admin/api/capacity/policies${suffix}`,
    signal ? { signal } : undefined,
  );
  return response.items;
}

export async function createCapacityPolicy(
  input: CreateHousingPolicyVersionRequest,
  signal?: AbortSignal,
): Promise<HousingPolicyVersion> {
  return request<HousingPolicyVersion>("/admin/api/capacity/policies", {
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

export async function listReviewNotes(
  page: number,
  pageSize: number,
  signal?: AbortSignal,
): Promise<ReviewNotesPage> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  });
  return request<ReviewNotesPage>(
    `/api/v1/review-notes?${params.toString()}`,
    signal ? { signal } : undefined,
  );
}

export async function createReviewNote(
  input: CreateReviewNoteInput,
  signal?: AbortSignal,
): Promise<ReviewNote> {
  return request<ReviewNote>("/api/v1/review-notes", {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "POST",
    signal,
  });
}

export async function updateReviewNote(
  id: string,
  input: UpdateReviewNoteInput,
  signal?: AbortSignal,
): Promise<ReviewNote> {
  return request<ReviewNote>(`/api/v1/review-notes/${encodeURIComponent(id)}`, {
    body: JSON.stringify(input),
    headers: { "content-type": "application/json" },
    method: "PATCH",
    signal,
  });
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

export async function getCommunityMarketSnapshot(
  neighborhoodId: string,
  signal?: AbortSignal,
): Promise<CommunityMarketSnapshot> {
  return request<CommunityMarketSnapshot>(
    `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}/community-market`,
    signal ? { signal } : undefined,
  );
}

export async function getLatestCommunityMarketSnapshot(
  neighborhoodId: string,
  signal?: AbortSignal,
): Promise<CommunityMarketSnapshot> {
  return request<CommunityMarketSnapshot>(
    `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}/community-market/latest`,
    signal ? { signal } : undefined,
  );
}

export async function getMarketListings(
  neighborhoodId: string,
  query: MarketListQuery = {},
  signal?: AbortSignal,
): Promise<MarketListingsPage> {
  return request<MarketListingsPage>(marketListURL(neighborhoodId, "market-listings", query), signal ? { signal } : undefined);
}

export async function getMarketListingDetail(
  neighborhoodId: string,
  roomId: string,
  signal?: AbortSignal,
): Promise<MarketListingDetail> {
  return request<MarketListingDetail>(
    `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}/market-listings/${encodeURIComponent(roomId)}`,
    signal ? { signal } : undefined,
  );
}

export async function getMarketTransactions(
  neighborhoodId: string,
  query: MarketListQuery = {},
  signal?: AbortSignal,
): Promise<MarketTransactionsPage> {
  return request<MarketTransactionsPage>(marketListURL(neighborhoodId, "market-transactions", query), signal ? { signal } : undefined);
}

export async function getListingAdjustments(
  neighborhoodId: string,
  roomId: string,
  signal?: AbortSignal,
): Promise<{ items: ListingAdjustment[] }> {
  return request<{ items: ListingAdjustment[] }>(
    `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}/market-listings/${encodeURIComponent(roomId)}/adjustments`,
    signal ? { signal } : undefined,
  );
}

export async function compareCommunityMarkets(
  neighborhoodId: string,
  peerNeighborhoodId: string,
  signal?: AbortSignal,
): Promise<CommunityMarketComparison> {
  const params = new URLSearchParams({ neighborhoodId, peerNeighborhoodId });
  return request<CommunityMarketComparison>(`/api/v1/community-market/comparison?${params.toString()}`, signal ? { signal } : undefined);
}

function marketListURL(neighborhoodId: string, resource: "market-listings" | "market-transactions", query: MarketListQuery): string {
  const params = new URLSearchParams();
  if (query.layout?.trim()) params.set("layout", query.layout.trim());
  if (query.floor) params.set("floor", query.floor);
  if (query.minPriceWan != null) params.set("minPriceWan", String(query.minPriceWan));
  if (query.maxPriceWan != null) params.set("maxPriceWan", String(query.maxPriceWan));
  if (query.sortBy) params.set("sortBy", query.sortBy);
  if (query.sortOrder) params.set("sortOrder", query.sortOrder);
  if (query.page) params.set("page", String(query.page));
  if (query.pageSize) params.set("pageSize", String(query.pageSize));
  const suffix = params.size > 0 ? `?${params.toString()}` : "";
  return `/api/v1/neighborhoods/${encodeURIComponent(neighborhoodId)}/${resource}${suffix}`;
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

export async function listCollectionRuns(
  query: CollectionRunListQuery = {},
  signal?: AbortSignal,
): Promise<CollectionRunsPage> {
  const params = new URLSearchParams({
    page: String(query.page ?? 1),
    pageSize: String(query.pageSize ?? 20),
  });
  for (const [key, value] of [
    ["dataSourceId", query.dataSourceId],
    ["neighborhoodId", query.neighborhoodId],
    ["status", query.status],
    ["metricStatus", query.metricStatus],
    ["from", query.from],
    ["to", query.to],
  ] as const) {
    if (value) params.set(key, value);
  }
  return request<CollectionRunsPage>(
    `/admin/api/imports?${params.toString()}`,
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
