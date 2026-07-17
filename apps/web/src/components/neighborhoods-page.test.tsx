import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { setAccessToken } from "@/lib/access-token";
import {
  addWatchlistItem,
  ApiError,
  compareCommunityMarkets,
  getListingAdjustments,
  getMarketListings,
  getMarketTransactions,
  getMetricHistory,
  getCommunityMarketSnapshot,
  getNeighborhood,
  getNeighborhoodMetrics,
  searchNeighborhoods,
  type CommunityMarketSnapshot,
  type CommunityMarketComparison,
  type MarketListingsPage,
  type MarketTransactionsPage,
  type MetricHistoryPoint,
  type MetricHistoryResponse,
  type Neighborhood,
  type NeighborhoodMetricResponse,
  type NeighborhoodSearchResponse,
} from "@/lib/api-client";

import { NeighborhoodsPage } from "./neighborhoods-page";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    addWatchlistItem: vi.fn(),
    compareCommunityMarkets: vi.fn(),
    getListingAdjustments: vi.fn(),
    getMarketListings: vi.fn(),
    getMarketTransactions: vi.fn(),
    getMetricHistory: vi.fn(),
    getCommunityMarketSnapshot: vi.fn(),
    getNeighborhood: vi.fn(),
    getNeighborhoodMetrics: vi.fn(),
    searchNeighborhoods: vi.fn(),
  };
});

const neighborhoodID = "11111111-1111-4111-8111-111111111111";
const collectionRunID = "33333333-3333-4333-8333-333333333333";
const dataSourceID = "44444444-4444-4444-8444-444444444444";

const neighborhoodFixture: Neighborhood = {
  id: neighborhoodID,
  name: "接口花园",
  city: "杭州",
  area: "滨江",
  availableLayouts: ["两房", "三房"],
};

const secondNeighborhoodFixture: Neighborhood = {
  id: "22222222-2222-4222-8222-222222222222",
  name: "海岸公寓",
  city: "上海",
  area: "浦东",
  availableLayouts: ["一房"],
};

const mingquanFixture: Neighborhood = {
  id: "99999999-9999-4999-8999-999999999999",
  name: "鸣泉花园",
  city: "天津",
  area: "梅江",
  availableLayouts: ["87㎡", "88㎡"],
};

const transactionEvidence = {
  windowStart: "2026-04-15",
  windowEnd: "2026-07-14",
  sampleCount: 6,
  recent30DayTransactionCount: 2,
  preceding60DayTransactionCount: 4,
  recent30DayMonthlyFrequency: 2,
  preceding60DayMonthlyFrequency: 2,
};

const metricFixture: NeighborhoodMetricResponse = {
  id: "55555555-5555-4555-8555-555555555555",
  neighborhoodId: neighborhoodID,
  collectionRunId: collectionRunID,
  sourceIds: [dataSourceID],
  latestObservedAt: "2026-07-14T08:00:00Z",
  collectedAt: "2026-07-14T08:00:00Z",
  algorithmVersion: "market-metrics/test.1",
  listedHomes: 18,
  priceCutHomes: 5,
  avgDaysOnMarket: 62,
  listingPriceMin: 520,
  listingPriceMax: 610,
  transactionPriceMin: 490,
  transactionPriceMax: 545,
  transactionMomentum: "weak",
  transactionEvidence,
  targetLayout: "两房",
  targetLayoutSupply: 7,
  listingSampleCount: 18,
  transactionSampleCount: 6,
  coverage: "full",
  freshness: "current",
  qualityState: "sufficient",
  qualityWarnings: [],
  status: "适合砍价",
  supplyPressure: "high",
  priceCutShare: 0.278,
  priceGapPct: 0.08,
  targetLayoutScarcity: "medium",
  advice: "使用真实成交区间校准报价。",
  reasons: ["真实挂牌和成交证据支持当前状态。"],
  calculatedAt: "2026-07-14T08:05:00Z",
};

function historyPoint(id: string, collectedAt: string, listedHomes: number): MetricHistoryPoint {
  const batch = {
    collectionRunId: id,
    dataSourceId: dataSourceID,
    sourceRef: `source-${id.slice(0, 4)}`,
    collectedAt,
    coverage: "full" as const,
  };
  return {
    id: `metric-${id}`,
    neighborhoodId: neighborhoodID,
    algorithmVersion: "market-metrics/test.1",
    collectedAt,
    calculatedAt: collectedAt,
    latestObservedAt: collectedAt,
    batch,
    sourceIds: [dataSourceID],
    listedHomes,
    priceCutHomes: 2,
    transactionMomentum: "stable",
    transactionEvidence,
    targetLayoutSupply: 7,
    listingSampleCount: listedHomes,
    transactionSampleCount: 6,
    coverage: "full",
    freshness: "current",
    qualityState: "sufficient",
    qualityWarnings: [],
    weeklyComparison: { status: "unavailable", reason: "full_baseline_not_found", currentBatch: batch },
    monthlyComparison: { status: "unavailable", reason: "full_baseline_not_found", currentBatch: batch },
  };
}

const historyFixture: MetricHistoryResponse = {
  status: "ready",
  neighborhoodId: neighborhoodID,
  targetLayout: "两房",
  algorithmVersion: "market-metrics/test.1",
  window: { from: "2026-05-19T08:00:00Z", to: "2026-07-14T08:00:00Z" },
  items: [
    historyPoint("66666666-6666-4666-8666-666666666666", "2026-07-07T08:00:00Z", 15),
    historyPoint(collectionRunID, "2026-07-14T08:00:00Z", 18),
  ],
};

const catalogFixture: NeighborhoodSearchResponse = {
  items: [neighborhoodFixture, secondNeighborhoodFixture],
  total: 2,
  page: 1,
  pageSize: 100,
  filters: {
    cities: ["上海", "杭州"],
    areas: [
      { city: "上海", area: "浦东" },
      { city: "杭州", area: "滨江" },
    ],
  },
};

const communityMarketFixture: CommunityMarketSnapshot = {
  id: "88888888-8888-4888-8888-888888888888",
  dataSourceId: dataSourceID,
  neighborhoodId: neighborhoodID,
  sourceRef: "fangjian-mingquan-2026-07-16T125857Z",
  collectedAt: "2026-07-16T12:58:57Z",
  contentChecksum: "a".repeat(64),
  collectionRunId: null,
  qualityStatus: "aggregate_only",
  sourceCommunityId: "a2d56505411446cfe70fd3960beb19c7",
  communityName: "富力津门湖鸣泉花园",
  formerName: "鸣泉花园",
  provinceCode: "120000",
  provinceName: "天津市",
  cityCode: "120100",
  cityName: "天津市",
  districtCode: "120111",
  districtName: "西青区",
  blockCode: "BK2022112435579",
  blockName: "梅江南",
  propertyType: "普通住宅",
  propertyTags: ["商品房", "私产"],
  buildingCount: 11,
  buildingType: "板楼",
  buildingYear: 2012,
  developer: "天津耀华投资发展有限公司",
  householdCount: 1089,
  closedManagement: "是",
  plotRatio: 1.8,
  greenAreaSqm: 14271,
  greeningRatePercent: 40,
  propertyManagementCompany: "天津碧桂园物业有限公司",
  propertyFee: "2.3-2.9",
  fixedParkingSpaces: 1550,
  parkingRatio: "1:0.7",
  parkingFee: "500",
  heatingType: "集中供暖",
  waterType: "民水",
  electricityType: "民电",
  gasCost: "2.5-2.61",
  manCarSeparation: "否",
  latitude: 39.057089,
  longitude: 117.203624,
  latestListingDate: "2026-06-29",
  listingAvgUnitPrice: 22741,
  listingCount: 46,
  listingAreaSqm: 5491.78,
  listingAvgTotalPriceWan: 272,
  listingAvgUnitPrice6Months: 22884.52,
  newListingCount3Months: 9,
  newListingAvgTotalPrice3MonthsWan: 270,
  newListingUnitPrice3Months: 20425,
  latestTradeDate: "2026-06-12",
  latestTradeAvgUnitPrice: 16461,
  tradeCount3Months: 5,
  tradeArea3MonthsSqm: 639.37,
  tradeAvgTotalPrice3MonthsWan: 215,
  tradeUnitPrice3Months: 16780,
  tradeAvgUnitPrice6Months: 17371,
  tradeCountPerMonth6Months: 1,
  takeLookCount: 125,
  takeLookConversionRatePercent: 3.25,
  onSaleAreaRangeSqm: "84-229",
  onSalePriceRangeWan: "149-479",
  onSaleRoomTypes: ["五室", "四室", "二室", "三室"],
  analysis: {},
  surroundings: {},
  cityContext: {},
  createdAt: "2026-07-16T13:00:00Z",
};

const communityMarketWithoutProfile: CommunityMarketSnapshot = {
  ...communityMarketFixture,
  provinceCode: null,
  provinceName: null,
  propertyType: null,
  propertyTags: null,
  buildingCount: null,
  buildingType: null,
  buildingYear: null,
  developer: null,
  householdCount: null,
  closedManagement: null,
  plotRatio: null,
  greenAreaSqm: null,
  greeningRatePercent: null,
  propertyManagementCompany: null,
  propertyFee: null,
  fixedParkingSpaces: null,
  parkingRatio: null,
  parkingFee: null,
  heatingType: null,
  waterType: null,
  electricityType: null,
  gasCost: null,
  manCarSeparation: null,
};

const completeCommunityMarket: CommunityMarketSnapshot = {
  ...communityMarketFixture,
  collectionRunId: collectionRunID,
  qualityStatus: "complete",
  analysis: {
    tradeTrends: { tradeTrends: [{ tradeDate: "2026-06", avgTradePriceCommunity: 16780, avgTradePriceDistrict: 22000 }] },
    supplyTrend: { supplyTrend: [{ listingDate: "2026-06", num: 46, takeLook: 125, supplyDemandRatio: 2.1 }] },
    tradeCycle: { tradeCycle6: [{ tradeDate: "2026-06", avgDealCycle: 91 }] },
    hotIndex: { hotIndex: [{ listingDate: "2026-06", hot: 121 }] },
    confidenceIndex: { confidenceIndex: [{ tradeDate: "2026-06", confidenceIndex: 57 }] },
    roomType: { roomTypes: [{ tradeDate: "2026-06", roomTypeFilter: "二室", tradeNum: 4, avgTradePrice: 16000 }] },
  },
  surroundings: {
    poi: [{ bizType: "交通", itemPageDate: { total: 1, rows: [{ poiName: "梅江会展中心站", distance: 850 }] } }],
  },
};

const peerMarket: CommunityMarketSnapshot = {
  ...completeCommunityMarket,
  id: "99999999-9999-4999-8999-999999999999",
  neighborhoodId: secondNeighborhoodFixture.id,
  collectionRunId: "55555555-5555-4555-8555-555555555555",
  communityName: "亲和美园",
  sourceCommunityId: "0a5b87b0d81dadbb50fb85df01489a13",
  listingAvgUnitPrice: 8000,
  listingCount: 71,
  tradeCount3Months: 12,
};

const marketComparison: CommunityMarketComparison = {
  primary: completeCommunityMarket,
  peer: peerMarket,
  listingUnitPrice: { primary: 22741, peer: 8000, delta: 14741 },
  supply: { primary: 46, peer: 71, delta: -25 },
  recentTrades: { primary: 5, peer: 12, delta: -7 },
  listingTradeGap: { primary: 5961, peer: 2000, delta: 3961 },
  averageTradeCycle: { primary: 91, peer: 75, delta: 16 },
};

const marketListings: MarketListingsPage = {
  items: [{ roomId: "room-1", layout: "二室", areaSqm: 78.67, listingTotalPriceWan: 63, listingUnitPrice: 8009, listedAt: "2026-06-28T00:00:00Z", daysOnMarket: 19, floorBand: "高楼层", floorDescription: "高楼层(共18层)", orientation: "南 北", adjustmentCount: 2, followCount: 3, lookCount30Days: 1 }],
  total: 1,
  page: 1,
  pageSize: 10,
};

const marketTransactions: MarketTransactionsPage = {
  items: [{ roomId: "trade-1", layout: "二室", areaSqm: 81.43, listingTotalPriceWan: 53, tradeTotalPriceWan: 45, tradeUnitPrice: 5526.22, tradeDate: "2026-06-14T00:00:00Z", negotiationWan: 8, negotiationPercent: 15.09, floorBand: "高楼层", floorDescription: "高楼层/18层", orientation: "南", adjustmentCount: 9 }],
  total: 1,
  page: 1,
  pageSize: 10,
};

describe("NeighborhoodsPage add flow", () => {
  beforeEach(() => {
    vi.mocked(addWatchlistItem).mockReset();
    vi.mocked(getMetricHistory).mockReset();
    vi.mocked(getCommunityMarketSnapshot).mockReset().mockRejectedValue(new ApiError("not_found", "missing", 404));
    vi.mocked(getNeighborhood).mockReset().mockImplementation(async (id) => {
      if (id === neighborhoodID) return neighborhoodFixture;
      if (id === secondNeighborhoodFixture.id) return secondNeighborhoodFixture;
      if (id === mingquanFixture.id) return mingquanFixture;
      throw new ApiError("not_found", "missing", 404);
    });
    vi.mocked(getNeighborhoodMetrics).mockReset();
    vi.mocked(searchNeighborhoods).mockReset().mockResolvedValue(catalogFixture);
    window.sessionStorage.clear();
    window.history.replaceState({}, "", "/neighborhoods");
  });

  it("selects city, area, neighborhood, and layout, persists the URL, and submits the exact target", async () => {
    setAccessToken("secret-token");
    const navigate = vi.fn();
    vi.mocked(addWatchlistItem).mockResolvedValue({
      id: "77777777-7777-4777-8777-777777777777",
      neighborhoodId: neighborhoodID,
      targetLayout: "三房",
      userId: "default-user",
      createdAt: "2026-07-15T00:00:00Z",
    });
    render(<NeighborhoodsPage initialNeighborhoodId="" navigate={navigate} />);

    await selectTarget("杭州", "滨江", neighborhoodID, "三房");
    expect(window.location.search).toContain("city=%E6%9D%AD%E5%B7%9E");
    expect(window.location.search).toContain(`neighborhoodId=${neighborhoodID}`);
    expect(window.location.search).toContain("targetLayout=%E4%B8%89%E6%88%BF");

    fireEvent.click(screen.getByRole("button", { name: "加入观察池" }));
    await waitFor(() => expect(addWatchlistItem).toHaveBeenCalledWith({
      neighborhoodId: neighborhoodID,
      targetLayout: "三房",
    }));
    expect(navigate).toHaveBeenCalledWith("/watchlist");
  });

  it("clears downstream selections when an upstream value changes", async () => {
    render(<NeighborhoodsPage initialNeighborhoodId="" />);
    await selectTarget("杭州", "滨江", neighborhoodID, "两房");

    fireEvent.change(screen.getByLabelText("城市"), { target: { value: "上海" } });
    expect(screen.getByLabelText("板块")).toHaveValue("");
    expect(screen.getByLabelText("小区")).toHaveValue("");
    expect(screen.getByLabelText("目标户型")).toHaveValue("");
    expect(window.location.search).not.toContain("neighborhoodId");
    expect(window.location.search).not.toContain("targetLayout");
  });

  it("restores selection from the URL and follows popstate changes", async () => {
    window.history.replaceState({}, "", `/neighborhoods?city=%E6%9D%AD%E5%B7%9E&area=%E6%BB%A8%E6%B1%9F&q=%E6%8E%A5%E5%8F%A3&neighborhoodId=${neighborhoodID}&targetLayout=%E4%B8%A4%E6%88%BF`);
    render(<NeighborhoodsPage initialNeighborhoodId="" />);

    expect(await screen.findByLabelText("城市")).toHaveValue("杭州");
    await waitFor(() => expect(screen.getByLabelText("目标户型")).toHaveValue("两房"));
    expect(screen.getByLabelText("小区名称")).toHaveValue("接口");

    window.history.pushState({}, "", "/neighborhoods?city=%E4%B8%8A%E6%B5%B7&area=%E6%B5%A6%E4%B8%9C");
    window.dispatchEvent(new PopStateEvent("popstate"));
    await waitFor(() => expect(screen.getByLabelText("城市")).toHaveValue("上海"));
    expect(screen.getByLabelText("板块")).toHaveValue("浦东");
    expect(screen.getByLabelText("小区")).toHaveValue("");
  });

  it("reports and clears a stale layout restored from the URL", async () => {
    window.history.replaceState({}, "", `/neighborhoods?city=%E6%9D%AD%E5%B7%9E&area=%E6%BB%A8%E6%B1%9F&neighborhoodId=${neighborhoodID}&targetLayout=%E4%BA%94%E6%88%BF`);
    render(<NeighborhoodsPage initialNeighborhoodId="" />);

    expect(await screen.findByText("原选择已失效")).toBeInTheDocument();
    expect(screen.getByText("原目标户型已不在该小区目录中。")).toBeInTheDocument();
    expect(screen.getByLabelText("目标户型")).toHaveValue("");
    expect(window.location.search).not.toContain("targetLayout");
  });

  it("cancels without creating an item", async () => {
    const navigate = vi.fn();
    render(<NeighborhoodsPage initialNeighborhoodId="" navigate={navigate} />);
    fireEvent.click(await screen.findByRole("button", { name: "取消" }));

    expect(navigate).toHaveBeenCalledWith("/");
    expect(addWatchlistItem).not.toHaveBeenCalled();
  });

  it("keeps the complete selection while locked and can submit after unlock", async () => {
    const navigate = vi.fn();
    vi.mocked(addWatchlistItem).mockResolvedValue({
      id: "77777777-7777-4777-8777-777777777777",
      neighborhoodId: neighborhoodID,
      targetLayout: "两房",
      userId: "default-user",
      createdAt: "2026-07-15T00:00:00Z",
    });
    render(<NeighborhoodsPage initialNeighborhoodId="" navigate={navigate} />);
    await selectTarget("杭州", "滨江", neighborhoodID, "两房");

    fireEvent.click(screen.getByRole("button", { name: "加入观察池" }));
    expect(await screen.findByText("个人空间尚未解锁，当前选择已保留。")).toBeInTheDocument();
    expect(addWatchlistItem).not.toHaveBeenCalled();
    expect(screen.getByLabelText("目标户型")).toHaveValue("两房");

    setAccessToken("secret-token");
    fireEvent.click(await screen.findByRole("button", { name: "加入观察池" }));
    await waitFor(() => expect(addWatchlistItem).toHaveBeenCalled());
    expect(navigate).toHaveBeenCalledWith("/watchlist");
  });

  it("renders loading, empty, failed, and retry catalog states", async () => {
    vi.mocked(searchNeighborhoods).mockReturnValueOnce(new Promise(() => undefined));
    const { unmount } = render(<NeighborhoodsPage initialNeighborhoodId="" />);
    const loadingText = await screen.findByText("正在加载小区目录");
    expect(loadingText.closest("[role=status]")?.querySelector(".animate-spin")).toBeInTheDocument();
    unmount();

    vi.mocked(searchNeighborhoods).mockReset()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValue(catalogFixture);
    render(<NeighborhoodsPage initialNeighborhoodId="" />);
    fireEvent.click(await screen.findByRole("button", { name: "重试" }));
    await waitFor(() => expect(screen.getByLabelText("城市")).not.toBeDisabled());
    expect(screen.queryByText("小区搜索失败")).not.toBeInTheDocument();
  });

  it("shows an empty result after valid city and area filters", async () => {
    vi.mocked(searchNeighborhoods).mockResolvedValue({ ...catalogFixture, items: [], total: 0 });
    render(<NeighborhoodsPage initialNeighborhoodId="" />);
    fireEvent.change(await screen.findByLabelText("城市"), { target: { value: "杭州" } });
    await waitFor(() => expect(screen.getByLabelText("板块")).not.toBeDisabled());
    fireEvent.change(screen.getByLabelText("板块"), { target: { value: "滨江" } });

    expect(await screen.findByText("没有匹配的小区")).toBeInTheDocument();
  });

  it.each(["鸣泉花园", "鸣泉"])("shows and selects a pure name result for %s", async (keyword) => {
    vi.mocked(searchNeighborhoods).mockImplementation(async (query) => {
      const q = typeof query === "string" ? query : query?.q;
      return {
        items: q ? [mingquanFixture] : [],
        total: q ? 1 : 0,
        page: 1,
        pageSize: 100,
        filters: { cities: ["天津"], areas: [{ city: "天津", area: "梅江" }] },
      };
    });
    render(<NeighborhoodsPage initialNeighborhoodId="" />);

    fireEvent.change(await screen.findByLabelText("小区名称"), { target: { value: keyword } });
    fireEvent.click(screen.getByRole("button", { name: "搜索" }));
    const result = await screen.findByRole("button", { name: /鸣泉花园/ });
    fireEvent.click(result);

    expect(screen.getByLabelText("城市")).toHaveValue("天津");
    expect(screen.getByLabelText("板块")).toHaveValue("梅江");
    expect(screen.getByLabelText("小区")).toHaveValue(mingquanFixture.id);
    expect(screen.getByLabelText("目标户型")).not.toBeDisabled();
  });

  it("shows an empty state for a pure name search without city or area", async () => {
    vi.mocked(searchNeighborhoods).mockResolvedValue({
      ...catalogFixture,
      items: [],
      total: 0,
    });
    render(<NeighborhoodsPage initialNeighborhoodId="" />);

    fireEvent.change(await screen.findByLabelText("小区名称"), { target: { value: "不存在的小区" } });
    fireEvent.click(screen.getByRole("button", { name: "搜索" }));

    expect(await screen.findByText("没有匹配的小区")).toBeInTheDocument();
    expect(screen.getByLabelText("城市")).toHaveValue("");
    expect(screen.getByLabelText("板块")).toHaveValue("");
  });

  it("keeps the target on duplicate and failed submissions, then retries", async () => {
    setAccessToken("secret-token");
    vi.mocked(addWatchlistItem).mockRejectedValueOnce(new ApiError("watchlist_item_exists", "exists", 409));
    const { unmount } = render(<NeighborhoodsPage initialNeighborhoodId="" />);
    await selectTarget("杭州", "滨江", neighborhoodID, "两房");
    fireEvent.click(screen.getByRole("button", { name: "加入观察池" }));
    expect(await screen.findByText("该小区已在观察池中。")).toBeInTheDocument();
    expect(screen.getByLabelText("目标户型")).toHaveValue("两房");
    unmount();

    window.history.replaceState({}, "", "/neighborhoods");
    vi.mocked(addWatchlistItem).mockReset()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce({
        id: "77777777-7777-4777-8777-777777777777",
        neighborhoodId: neighborhoodID,
        targetLayout: "两房",
        userId: "default-user",
        createdAt: "2026-07-15T00:00:00Z",
      });
    const navigate = vi.fn();
    render(<NeighborhoodsPage initialNeighborhoodId="" navigate={navigate} />);
    await selectTarget("杭州", "滨江", neighborhoodID, "两房");
    fireEvent.click(screen.getByRole("button", { name: "加入观察池" }));
    expect(await screen.findByText("加入观察池失败，当前选择已保留。")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重试加入" }));
    await waitFor(() => expect(addWatchlistItem).toHaveBeenCalledTimes(2));
    expect(navigate).toHaveBeenCalledWith("/watchlist");
  });
});

describe("NeighborhoodsPage detail", () => {
  beforeEach(() => {
    vi.mocked(addWatchlistItem).mockReset();
    vi.mocked(getCommunityMarketSnapshot).mockReset().mockRejectedValue(new ApiError("not_found", "missing", 404));
    vi.mocked(getNeighborhood).mockReset().mockResolvedValue(neighborhoodFixture);
    vi.mocked(getNeighborhoodMetrics).mockReset().mockResolvedValue(metricFixture);
    vi.mocked(getMetricHistory).mockReset().mockResolvedValue(historyFixture);
    vi.mocked(searchNeighborhoods).mockReset();
    vi.mocked(compareCommunityMarkets).mockReset();
    vi.mocked(getListingAdjustments).mockReset();
    vi.mocked(getMarketListings).mockReset();
    vi.mocked(getMarketTransactions).mockReset();
    window.sessionStorage.clear();
    window.history.replaceState({}, "", `/neighborhoods?id=${neighborhoodID}&targetLayout=%E4%B8%A4%E6%88%BF`);
  });

  it("requests and renders metrics for the explicit detail layout", async () => {
    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByRole("heading", { name: "接口花园" })).toBeInTheDocument();
    expect(await screen.findByText("520-610 万")).toBeInTheDocument();
    expect(getNeighborhoodMetrics).toHaveBeenCalledWith(neighborhoodID, "两房", expect.any(AbortSignal));
    expect(getMetricHistory).toHaveBeenCalledWith(neighborhoodID, "两房", {}, expect.any(AbortSignal));
    expect(screen.getByLabelText("目标户型")).toHaveValue("两房");
    expect(screen.queryByText("降价提醒")).not.toBeInTheDocument();
  });

  it("renders a complete community profile with source-native values", async () => {
    vi.mocked(getCommunityMarketSnapshot).mockResolvedValueOnce(communityMarketFixture);
    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByLabelText("小区档案")).toBeInTheDocument();
    for (const heading of ["建筑与规模", "物业与停车", "能源与管理"]) {
      expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
    }
    expect(screen.getByText("天津市（120000）")).toBeInTheDocument();
    expect(screen.getByText("天津耀华投资发展有限公司")).toBeInTheDocument();
    expect(screen.getByText("2.3-2.9")).toBeInTheDocument();
    expect(screen.getByText("2.5-2.61")).toBeInTheDocument();
    expect(screen.getByText(/不会替代单套挂牌、成交明细/)).toBeInTheDocument();
  });

  it("supports comparison tabs, real listing adjustments, transactions, trends, and surroundings", async () => {
    vi.mocked(getCommunityMarketSnapshot).mockResolvedValueOnce(completeCommunityMarket);
    vi.mocked(searchNeighborhoods).mockResolvedValueOnce({
      items: [neighborhoodFixture, secondNeighborhoodFixture], total: 2, page: 1, pageSize: 100,
      filters: { cities: ["杭州", "上海"], areas: [{ city: "杭州", area: "滨江" }, { city: "上海", area: "浦东" }] },
    });
    vi.mocked(compareCommunityMarkets).mockResolvedValueOnce(marketComparison);
    vi.mocked(getMarketListings).mockResolvedValue(marketListings);
    vi.mocked(getMarketTransactions).mockResolvedValue(marketTransactions);
    vi.mocked(getListingAdjustments).mockResolvedValueOnce({ items: [
      { id: "66666666-6666-4666-8666-666666666666", roomId: "room-1", adjustedAt: "2026-06-29T00:00:00Z", priceBeforeWan: 65, priceAfterWan: 63, amountWan: -2 },
    ] });

    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByText("完整数据包")).toBeInTheDocument();
    await waitFor(() => expect(compareCommunityMarkets).toHaveBeenCalledWith(neighborhoodID, secondNeighborhoodFixture.id, expect.any(AbortSignal)));
    expect(await screen.findByLabelText("双小区行情比较")).toHaveTextContent("亲和美园");

    fireEvent.click(screen.getByRole("tab", { name: "在售" }));
    expect(await screen.findByText(/高楼层\(共18层\) · 南 北/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "2 次" }));
    expect(await screen.findByLabelText("调价时间线")).toHaveTextContent("-2 万");

    fireEvent.click(screen.getByRole("tab", { name: "成交" }));
    expect(await screen.findByText("53 万 / 45 万")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "趋势与调价" }));
    expect(screen.getByRole("heading", { name: "成交价格趋势" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "市场信心" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "周边" }));
    expect(screen.getByText("梅江会展中心站")).toBeInTheDocument();
  });

  it("renders partial profile values and marks missing values as unavailable", async () => {
    vi.mocked(getCommunityMarketSnapshot).mockResolvedValueOnce({
      ...communityMarketWithoutProfile,
      propertyType: "普通住宅",
      heatingType: "集中供暖",
    });
    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByText("普通住宅")).toBeInTheDocument();
    expect(screen.getByText("集中供暖")).toBeInTheDocument();
    expect(screen.getAllByText("暂无").length).toBeGreaterThan(0);
  });

  it("renders an older market snapshot with no profile as an empty profile", async () => {
    vi.mocked(getCommunityMarketSnapshot).mockResolvedValueOnce(communityMarketWithoutProfile);
    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByLabelText("小区档案")).toBeInTheDocument();
    expect(screen.getAllByText("暂无").length).toBeGreaterThanOrEqual(20);
  });

  it("omits the aggregate and profile panel when the snapshot endpoint returns 404", async () => {
    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByRole("heading", { name: "接口花园" })).toBeInTheDocument();
    await waitFor(() => expect(getCommunityMarketSnapshot).toHaveBeenCalled());
    expect(screen.queryByLabelText("小区档案")).not.toBeInTheDocument();
  });

  it("does not request metrics until a detail layout is selected", async () => {
    window.history.replaceState({}, "", `/neighborhoods?id=${neighborhoodID}`);
    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByText("请选择目标户型")).toBeInTheDocument();
    expect(getNeighborhoodMetrics).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("目标户型"), { target: { value: "三房" } });
    await waitFor(() => expect(getNeighborhoodMetrics).toHaveBeenCalledWith(neighborhoodID, "三房", expect.any(AbortSignal)));
    expect(window.location.search).toContain("targetLayout=%E4%B8%89%E6%88%BF");
  });

  it("shows no metric without synthetic zero values", async () => {
    vi.mocked(getNeighborhoodMetrics).mockRejectedValueOnce(new ApiError("not_found", "missing", 404));
    render(<NeighborhoodsPage initialNeighborhoodId={neighborhoodID} />);

    expect(await screen.findByText("该小区暂无市场指标")).toBeInTheDocument();
    expect(screen.queryByText("0 套")).not.toBeInTheDocument();
  });

  it("rejects malformed IDs without making API requests", async () => {
    render(<NeighborhoodsPage initialNeighborhoodId="not-a-uuid" />);

    expect(await screen.findByText("找不到该小区")).toBeInTheDocument();
    expect(getNeighborhood).not.toHaveBeenCalled();
  });
});

async function selectTarget(city: string, area: string, neighborhoodId: string, targetLayout: string) {
  fireEvent.change(await screen.findByLabelText("城市"), { target: { value: city } });
  await waitFor(() => expect(screen.getByLabelText("板块")).not.toBeDisabled());
  fireEvent.change(screen.getByLabelText("板块"), { target: { value: area } });
  await waitFor(() => expect(screen.getByLabelText("小区")).not.toBeDisabled());
  fireEvent.change(screen.getByLabelText("小区"), { target: { value: neighborhoodId } });
  await waitFor(() => expect(screen.getByLabelText("目标户型")).not.toBeDisabled());
  fireEvent.change(screen.getByLabelText("目标户型"), { target: { value: targetLayout } });
  await waitFor(() => expect(screen.getByLabelText("目标户型")).toHaveValue(targetLayout));
}
