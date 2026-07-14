import { fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  ApiError,
  getMetricHistory,
  getNeighborhood,
  getNeighborhoodMetrics,
  searchNeighborhoods,
  type MetricHistoryPoint,
  type MetricHistoryResponse,
  type Neighborhood,
  type NeighborhoodMetricResponse,
} from "@/lib/api-client";

import { NeighborhoodsPage } from "./neighborhoods-page";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    getMetricHistory: vi.fn(),
    getNeighborhood: vi.fn(),
    getNeighborhoodMetrics: vi.fn(),
    searchNeighborhoods: vi.fn(),
  };
});

const neighborhoodID = "11111111-1111-4111-8111-111111111111";
const secondNeighborhoodID = "22222222-2222-4222-8222-222222222222";
const collectionRunID = "33333333-3333-4333-8333-333333333333";
const dataSourceID = "44444444-4444-4444-8444-444444444444";

const neighborhoodFixture: Neighborhood = {
  id: neighborhoodID,
  name: "接口花园",
  area: "南城",
  targetLayout: "两房",
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
  algorithmVersion: "market-metrics/test.1",
  window: { from: "2026-05-19T08:00:00Z", to: "2026-07-14T08:00:00Z" },
  items: [
    historyPoint("66666666-6666-4666-8666-666666666666", "2026-07-07T08:00:00Z", 15),
    historyPoint(collectionRunID, "2026-07-14T08:00:00Z", 18),
  ],
};

describe("NeighborhoodsPage", () => {
  beforeEach(() => {
    vi.mocked(getMetricHistory).mockReset();
    vi.mocked(getNeighborhood).mockReset();
    vi.mocked(getNeighborhoodMetrics).mockReset();
    vi.mocked(searchNeighborhoods).mockReset();
  });

  it("shows a real search selector when no neighborhood ID is present", async () => {
    vi.mocked(searchNeighborhoods).mockResolvedValueOnce({
      items: [neighborhoodFixture], total: 1, page: 1, pageSize: 50,
    });

    render(createElement(NeighborhoodsPage, { initialNeighborhoodId: "" }));

    expect(await screen.findByRole("link", { name: /接口花园/ })).toHaveAttribute(
      "href",
      `/neighborhoods?id=${neighborhoodID}`,
    );
    expect(getNeighborhood).not.toHaveBeenCalled();
  });

  it("renders identity, latest metrics, backend advice, and real history", async () => {
    mockReadyNeighborhood();

    render(createElement(NeighborhoodsPage, { initialNeighborhoodId: neighborhoodID }));

    expect(await screen.findByRole("heading", { name: "接口花园" })).toBeInTheDocument();
    expect(screen.getByText("南城")).toBeInTheDocument();
    expect(screen.getByText("520-610 万")).toBeInTheDocument();
    expect(screen.getByText("使用真实成交区间校准报价。")).toBeInTheDocument();
    expect(screen.getByLabelText("真实挂牌与降价批次趋势图")).toBeInTheDocument();
    expect(screen.queryByText("带看转定率")).not.toBeInTheDocument();
    expect(screen.queryByText("更新时间: 今天 10:30")).not.toBeInTheDocument();
  });

  it("shows no metric without rendering zero values or a recommendation", async () => {
    vi.mocked(getNeighborhood).mockResolvedValueOnce(neighborhoodFixture);
    vi.mocked(getNeighborhoodMetrics).mockRejectedValueOnce(new ApiError("not_found", "missing", 404));
    vi.mocked(getMetricHistory).mockResolvedValueOnce({ ...historyFixture, status: "empty", items: [] });

    render(createElement(NeighborhoodsPage, { initialNeighborhoodId: neighborhoodID }));

    expect(await screen.findByText("该小区暂无市场指标")).toBeInTheDocument();
    expect(screen.queryByText("0 套")).not.toBeInTheDocument();
    expect(screen.queryByText("适合砍价")).not.toBeInTheDocument();
  });

  it("marks stale and insufficient metrics without opening a buyer window", async () => {
    mockReadyNeighborhood({
      freshness: "stale",
      qualityState: "low_confidence",
      transactionMomentum: "unknown",
      status: "数据不足",
      advice: "等待补充数据。",
      qualityWarnings: ["stale_data", "insufficient_transaction_samples"],
    });

    render(createElement(NeighborhoodsPage, { initialNeighborhoodId: neighborhoodID }));

    expect(await screen.findByRole("heading", { name: "市场数据已陈旧" })).toBeInTheDocument();
    expect(screen.getByText("数据不足")).toBeInTheDocument();
    expect(screen.queryByText("买方窗口开启")).not.toBeInTheDocument();
  });

  it("keeps current metrics when only history fails and offers retry", async () => {
    vi.mocked(getNeighborhood).mockResolvedValueOnce(neighborhoodFixture);
    vi.mocked(getNeighborhoodMetrics).mockResolvedValueOnce(metricFixture);
    vi.mocked(getMetricHistory).mockRejectedValueOnce(new Error("history offline"));

    render(createElement(NeighborhoodsPage, { initialNeighborhoodId: neighborhoodID }));

    expect(await screen.findByText("历史趋势读取失败")).toBeInTheDocument();
    expect(screen.getByText("520-610 万")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
  });

  it("retries a failed core request", async () => {
    vi.mocked(getNeighborhood)
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(neighborhoodFixture);
    vi.mocked(getNeighborhoodMetrics)
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(metricFixture);
    vi.mocked(getMetricHistory)
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(historyFixture);

    render(createElement(NeighborhoodsPage, { initialNeighborhoodId: neighborhoodID }));
    fireEvent.click(await screen.findByRole("button", { name: "重试" }));

    expect(await screen.findByRole("heading", { name: "接口花园" })).toBeInTheDocument();
    expect(screen.queryByText("小区数据读取失败")).not.toBeInTheDocument();
  });

  it("clears the previous neighborhood while a new ID is loading", async () => {
    mockReadyNeighborhood();
    const { rerender } = render(createElement(NeighborhoodsPage, { initialNeighborhoodId: neighborhoodID }));
    expect(await screen.findByRole("heading", { name: "接口花园" })).toBeInTheDocument();

    vi.mocked(getNeighborhood).mockReturnValueOnce(new Promise(() => undefined));
    vi.mocked(getNeighborhoodMetrics).mockReturnValueOnce(new Promise(() => undefined));
    vi.mocked(getMetricHistory).mockReturnValueOnce(new Promise(() => undefined));
    rerender(createElement(NeighborhoodsPage, { initialNeighborhoodId: secondNeighborhoodID }));

    expect(await screen.findByText("正在加载小区身份、指标与历史")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "接口花园" })).not.toBeInTheDocument();
  });

  it("rejects malformed IDs without making API requests", async () => {
    render(createElement(NeighborhoodsPage, { initialNeighborhoodId: "not-a-uuid" }));

    expect(await screen.findByText("找不到该小区")).toBeInTheDocument();
    expect(getNeighborhood).not.toHaveBeenCalled();
  });
});

function mockReadyNeighborhood(metricOverrides: Partial<NeighborhoodMetricResponse> = {}) {
  vi.mocked(getNeighborhood).mockResolvedValueOnce(neighborhoodFixture);
  vi.mocked(getNeighborhoodMetrics).mockResolvedValueOnce({ ...metricFixture, ...metricOverrides });
  vi.mocked(getMetricHistory).mockResolvedValueOnce(historyFixture);
}
