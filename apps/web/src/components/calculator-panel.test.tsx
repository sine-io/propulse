import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { setAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createCapacityCalculation,
  getCapacityAssumptions,
  getCapacityCalculation,
  getMarketListingDetail,
  getMarketListings,
  listAssets,
  listCapacityCalculations,
  searchNeighborhoods,
  type Asset,
  type CalculationResponse,
  type CapacityAssumptionsResponse,
  type MarketListingDetail,
  type Neighborhood,
} from "@/lib/api-client";

import { CalculatorPanel } from "./calculator-panel";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    createCapacityCalculation: vi.fn(),
    getCapacityAssumptions: vi.fn(),
    getCapacityCalculation: vi.fn(),
    getMarketListingDetail: vi.fn(),
    getMarketListings: vi.fn(),
    listAssets: vi.fn(),
    listCapacityCalculations: vi.fn(),
    searchNeighborhoods: vi.fn(),
  };
});

const neighborhood: Neighborhood = {
  id: "22222222-2222-4222-8222-222222222222",
  name: "海河花园",
  city: "天津",
  area: "河西区",
  availableLayouts: ["2室1厅"],
};

const asset: Asset = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "现住房",
  property: {
    neighborhoodId: "33333333-3333-4333-8333-333333333333",
    neighborhoodName: "梅江家园",
    city: "天津",
    district: "西青区",
    layout: "2室1厅",
    areaSqm: 82,
    floorBand: "中楼层",
    floorDescription: "中楼层/18层",
    orientation: "南北",
    currentListingPriceWan: 320,
  },
  originalPurchasePriceWan: 180,
  purchasedOn: "2020-08-20",
  currentLoanBalanceWan: 60,
  sourceKind: "manual",
  listingSource: null,
  createdAt: "2026-07-01T00:00:00Z",
  updatedAt: "2026-07-16T08:00:00Z",
};

const listing: MarketListingDetail = {
  roomId: "room-1",
  layout: "3室2厅",
  areaSqm: 118,
  listingTotalPriceWan: 500,
  listingUnitPrice: 42373,
  listedAt: "2026-06-20T00:00:00Z",
  daysOnMarket: 27,
  floorBand: "中楼层",
  floorDescription: "中楼层/20层",
  orientation: "南北",
  adjustmentCount: 1,
  followCount: 8,
  lookCount30Days: 3,
  neighborhoodId: neighborhood.id,
  neighborhoodName: neighborhood.name,
  city: "天津",
  district: "河西区",
  status: "active",
  snapshotId: "44444444-4444-4444-8444-444444444444",
  collectionRunId: "55555555-5555-4555-8555-555555555555",
  collectedAt: "2026-07-08T08:00:00Z",
  source: {
    dataSourceId: "66666666-6666-4666-8666-666666666666",
    dataSourceName: "房鉴",
    dataSourceType: "fangjian",
    sourceRef: "batch-20260708",
  },
  qualityStatus: "complete",
  freshness: "stale",
};

const assumptions = {
  ruleVersion: "tianjin-2026.01.01",
  effectiveDate: "2026-01-01",
  ruleSource: "天津住房交易测算政策",
  downPaymentRate: 0.15,
  loan: { annualInterestRate: 0.0305, loanTermMonths: 360, repaymentMethod: "equal_installment" },
  loanSource: "policy",
  loanOrigin: "configured_default",
  cityPolicy: {
    city: "天津",
    policyName: "天津住房交易测算政策",
    downPaymentRate: 0.15,
    effectiveDate: "2026-01-01",
    source: "policy",
    origin: "configured_default",
  },
  reserveMonths: 6,
  pressureThresholds: { safeRatio: 0.35, strainedRatio: 0.45, dangerRatio: 0.55, dangerMultiplier: 1.15 },
  oldHomeShareThreshold: 0.5,
  policyVersion: {
    id: "77777777-7777-4777-8777-777777777777",
    city: "天津",
    version: "tianjin-2026.01.01",
    name: "天津住房交易测算政策",
    effectiveFrom: "2026-01-01",
    enabled: true,
    createdAt: "2026-01-01T00:00:00Z",
    rules: {
      downPayment: { commercialFirst: 0.15, commercialSecond: 0.15, providentFirst: 0.2, providentSecond: 0.2, combinedFirst: 0.2, combinedSecond: 0.2 },
      interest: { commercialFirst: 0.0305, commercialSecond: 0.0305, providentFirstUpToFiveYears: 0.021, providentFirstOverFiveYears: 0.026, providentSecondUpToFiveYears: 0.02525, providentSecondOverFiveYears: 0.03075 },
      tax: { deedFirstUpToAreaRate: 0.01, deedFirstOverAreaRate: 0.015, deedSecondUpToAreaRate: 0.01, deedSecondOverAreaRate: 0.02, deedAreaThresholdSqm: 140, vatRate: 0.03, vatExemptHoldingYears: 2, vatSurchargeRate: 0.06, incomeTaxGainRate: 0.2, incomeTaxAssessedRate: 0.01, incomeTaxExemptHoldingYears: 5 },
    },
    sources: [{ code: "official", title: "政策来源", issuer: "主管部门", url: "https://example.com", effectiveDate: "2026-01-01" }],
  },
  sources: [],
  loanOptions: [
    { type: "commercial", downPaymentRate: 0.15, commercialAnnualInterestRate: 0.0305 },
    { type: "provident_fund", downPaymentRate: 0.2, providentAnnualInterestRate: 0.026 },
    { type: "combined", downPaymentRate: 0.2, commercialAnnualInterestRate: 0.0305, providentAnnualInterestRate: 0.026 },
  ],
  homePurchaseOrder: "first",
  loanTermMonths: 360,
  disclaimer: "仅供预算估算。",
} satisfies CapacityAssumptionsResponse;

const report = {
  id: "calc-1",
  input: {
    cashOnHand: 150,
    oldHomeValue: 320,
    oldLoanBalance: 60,
    monthlyIncome: 3.5,
    currentMonthlyMortgage: 0,
    acceptableMonthlyMortgage: 1.5,
    targetTotalPrice: 480,
    renovationBudget: 30,
    transitionRentCost: 5,
  },
  result: {
    netOldHomeProceeds: 260,
    deployableCash: 350,
    safeTotalPrice: 520,
    strainedTotalPrice: 600,
    dangerTotalPrice: 680,
    downPaymentGap: 0,
    monthlyPayment: 1.2,
    monthlyPaymentRatio: 0.34,
    pressureLevel: "safe",
    minimumSafeOldHomeSalePrice: 280,
    strategy: "可以同步推进",
    reasons: ["现金流处于安全区间。"],
    ruleVersion: "tianjin-2026.01.01",
    effectiveDate: "2026-01-01",
    traceabilityStatus: "complete",
    appliedAssumptions: null,
  },
  selectionContext: {
    oldHome: {
      mode: "asset",
      assetId: asset.id,
      assetName: asset.name,
      property: { ...asset.property, referenceListingPriceWan: 320 },
      originalPurchasePriceWan: 180,
      purchasedOn: "2020-08-20",
      holdingYears: 5,
      confirmedSalePriceWan: 320,
      confirmedLoanBalanceWan: 60,
      priceDifferenceWan: 0,
      assetUpdatedAt: asset.updatedAt,
      marketReference: null,
      confirmedAt: "2026-07-17T08:00:00Z",
    },
    targetHome: {
      property: {
        neighborhoodId: neighborhood.id,
        neighborhoodName: neighborhood.name,
        city: "天津",
        district: "河西区",
        layout: listing.layout,
        areaSqm: listing.areaSqm,
        floorBand: listing.floorBand,
        floorDescription: listing.floorDescription,
        orientation: listing.orientation,
        referenceListingPriceWan: 500,
      },
      confirmedPurchasePriceWan: 480,
      priceDifferenceWan: -20,
      marketReference: {
        sourceListingId: listing.roomId,
        dataSourceId: listing.source.dataSourceId,
        dataSourceName: listing.source.dataSourceName,
        dataSourceType: listing.source.dataSourceType,
        sourceRef: listing.source.sourceRef,
        collectionRunId: listing.collectionRunId,
        snapshotId: listing.snapshotId,
        collectedAt: listing.collectedAt,
        listedAt: listing.listedAt,
        qualityStatus: "complete",
        freshness: "stale",
      },
      confirmedAt: "2026-07-17T08:00:00Z",
    },
  },
  createdAt: "2026-07-17T08:00:00Z",
} satisfies CalculationResponse;

const reportSummary = {
  id: report.id,
  createdAt: report.createdAt,
  pressureLevel: report.result.pressureLevel,
  targetTotalPrice: report.input.targetTotalPrice,
  targetNeighborhoodName: neighborhood.name,
  targetLayout: listing.layout,
  oldHomeName: asset.name,
} as const;

function fillFamily() {
  for (const [label, value] of [
    ["当前可用现金 (万)", "150"],
    ["家庭月收入 (万)", "3.5"],
    ["当前月供 (元)", "0"],
    ["可接受新月供 (元)", "15000"],
    ["装修预算 (万)", "30"],
    ["过渡成本 (万)", "5"],
  ]) fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

async function chooseTarget() {
  fireEvent.focus(screen.getByRole("combobox", { name: "目标小区" }));
  fireEvent.click(await screen.findByRole("option", { name: /海河花园/ }));
  fireEvent.focus(screen.getByRole("combobox", { name: "目标房源" }));
  fireEvent.click(await screen.findByRole("option", { name: /3室2厅 · 118㎡/ }));
  await screen.findByText(/数据已陈旧/);
}

describe("CalculatorPanel asset and listing workflow", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    setAccessToken("secret");
    vi.mocked(getCapacityAssumptions).mockReset().mockResolvedValue(assumptions);
    vi.mocked(listAssets).mockReset().mockResolvedValue({ items: [asset], total: 1, page: 1, pageSize: 100 });
    vi.mocked(listCapacityCalculations).mockReset().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });
    vi.mocked(getCapacityCalculation).mockReset();
    vi.mocked(searchNeighborhoods).mockReset().mockResolvedValue({ items: [neighborhood], total: 1, page: 1, pageSize: 100, filters: { cities: ["天津"], areas: [{ city: "天津", area: "河西区" }] } });
    vi.mocked(getMarketListings).mockReset().mockResolvedValue({ items: [listing], total: 1, page: 1, pageSize: 100 });
    vi.mocked(getMarketListingDetail).mockReset().mockResolvedValue(listing);
    vi.mocked(createCapacityCalculation).mockReset().mockResolvedValue(report);
  });

  it("selects an owned asset and authoritative target listing, then submits confirmed snapshots", async () => {
    render(createElement(CalculatorPanel));

    fireEvent.change(await screen.findByRole("combobox", { name: "选择旧房产" }), { target: { value: asset.id } });
    expect(screen.getByLabelText("旧房预期售价 (万)")).toHaveValue("320");
    expect(screen.getByLabelText("当前贷款余额 (万)")).toHaveValue("60");
    await chooseTarget();
    fireEvent.change(screen.getByLabelText("预计成交价 (万)"), { target: { value: "480" } });
    fireEvent.click(screen.getByLabelText("确认采用该旧房预期售价"));
    fireEvent.click(screen.getByLabelText("确认采用该目标房成交价"));
    fillFamily();
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));

    await waitFor(() => expect(createCapacityCalculation).toHaveBeenCalled());
    expect(vi.mocked(createCapacityCalculation).mock.calls[0]?.[0]).toEqual(expect.objectContaining({
      oldHomeValue: 320,
      oldLoanBalance: 60,
      targetTotalPrice: 480,
      oldHomeSelection: { mode: "asset", assetId: asset.id, expectedSalePriceWan: 320, priceConfirmed: true },
      targetHomeSelection: { neighborhoodId: neighborhood.id, roomId: listing.roomId, expectedPurchasePriceWan: 480, priceConfirmed: true },
    }));
    expect(await screen.findByText("房屋与价格快照")).toBeInTheDocument();
    expect(screen.getByText("-20 万")).toBeInTheDocument();
    expect(screen.getByText("现金流处于安全区间。")).toBeInTheDocument();
  });

  it("writes zero old-home inputs for the explicit no-old-home path", async () => {
    render(createElement(CalculatorPanel));
    expect(screen.getByRole("combobox", { name: "选择旧房产" })).toHaveValue("none");
    await chooseTarget();
    fireEvent.click(screen.getByLabelText("确认采用该目标房成交价"));
    fillFamily();
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));

    await waitFor(() => expect(createCapacityCalculation).toHaveBeenCalled());
    expect(vi.mocked(createCapacityCalculation).mock.calls[0]?.[0]).toEqual(expect.objectContaining({
      oldHomeValue: 0,
      oldLoanBalance: 0,
      oldHomeSelection: { mode: "none", priceConfirmed: true },
    }));
  });

  it("resets target price confirmation whenever the price changes", async () => {
    render(createElement(CalculatorPanel));
    await chooseTarget();
    const confirmation = screen.getByLabelText("确认采用该目标房成交价");
    fireEvent.click(confirmation);
    expect(confirmation).toBeChecked();
    fireEvent.change(screen.getByLabelText("预计成交价 (万)"), { target: { value: "490" } });
    expect(confirmation).not.toBeChecked();
  });

  it("shows recoverable empty inventory state without accepting a manual target", async () => {
    vi.mocked(getMarketListings).mockResolvedValueOnce({ items: [], total: 0, page: 1, pageSize: 100 });
    render(createElement(CalculatorPanel));
    fireEvent.focus(screen.getByRole("combobox", { name: "目标小区" }));
    fireEvent.click(await screen.findByRole("option", { name: /海河花园/ }));
    expect(await screen.findByText(/暂无当前在售房源/)).toBeInTheDocument();
    expect(screen.queryByLabelText("预计成交价 (万)")).not.toBeInTheDocument();
  });

  it("debounces neighborhood search and locally filters listings by property facts", async () => {
    render(createElement(CalculatorPanel));
    const neighborhoodInput = screen.getByRole("combobox", { name: "目标小区" });
    fireEvent.change(neighborhoodInput, { target: { value: "海河" } });
    await waitFor(() => expect(searchNeighborhoods).toHaveBeenCalledWith(
      expect.objectContaining({ q: "海河", page: 1, pageSize: 100 }),
      expect.any(AbortSignal),
    ));

    fireEvent.click(await screen.findByRole("option", { name: /海河花园/ }));
    const listingInput = screen.getByRole("combobox", { name: "目标房源" });
    fireEvent.focus(listingInput);
    await screen.findByRole("option", { name: /3室2厅 · 118㎡/ });
    fireEvent.change(listingInput, { target: { value: "南北 500" } });
    expect(screen.getByRole("option", { name: /3室2厅 · 118㎡/ })).toBeInTheDocument();
    fireEvent.change(listingInput, { target: { value: "低楼层" } });
    expect(screen.getByText("没有匹配房源")).toBeInTheDocument();
  });

  it("clears the linked listing, price, and confirmation when the neighborhood changes", async () => {
    const anotherNeighborhood = { ...neighborhood, id: "88888888-8888-4888-8888-888888888888", name: "文化中心" };
    vi.mocked(searchNeighborhoods).mockResolvedValue({
      items: [neighborhood, anotherNeighborhood], total: 2, page: 1, pageSize: 100,
      filters: { cities: ["天津"], areas: [{ city: "天津", area: "河西区" }] },
    });
    render(createElement(CalculatorPanel));
    await chooseTarget();
    fireEvent.click(screen.getByLabelText("确认采用该目标房成交价"));

    fireEvent.click(screen.getByRole("button", { name: "清空目标小区" }));
    expect(screen.getByRole("combobox", { name: "目标房源" })).toBeDisabled();
    expect(screen.queryByLabelText("预计成交价 (万)")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("确认采用该目标房成交价")).not.toBeInTheDocument();
  });

  it("loads the latest history snapshot without overwriting the new calculation draft", async () => {
    vi.mocked(listCapacityCalculations).mockResolvedValue({ items: [reportSummary], total: 1, page: 1, pageSize: 20 });
    vi.mocked(getCapacityCalculation).mockResolvedValue(report);
    render(createElement(CalculatorPanel));

    expect(await screen.findByText("现金流处于安全区间。")).toBeInTheDocument();
    expect(screen.getByText(/快照状态：历史快照/)).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "选择旧房产" })).toHaveValue("none");
    expect(screen.getByLabelText("当前可用现金 (万)")).toHaveValue("");
    expect(getCapacityCalculation).toHaveBeenCalledWith(report.id, expect.any(AbortSignal));
  });

  it("searches history, switches the report only after selection, and keeps it while editing the draft", async () => {
    const olderSummary = { ...reportSummary, id: "calc-older", createdAt: "2026-07-16T08:00:00Z", targetNeighborhoodName: "梅江花园" };
    const olderReport = { ...report, id: olderSummary.id, createdAt: olderSummary.createdAt, result: { ...report.result, reasons: ["旧报告仍然可追溯。"] } } satisfies CalculationResponse;
    vi.mocked(listCapacityCalculations).mockResolvedValue({ items: [reportSummary, olderSummary], total: 2, page: 1, pageSize: 20 });
    vi.mocked(getCapacityCalculation).mockImplementation((id) => Promise.resolve(id === olderSummary.id ? olderReport : report));
    render(createElement(CalculatorPanel));
    await screen.findByText("现金流处于安全区间。");

    const historyInput = screen.getByRole("combobox", { name: "诊断历史" });
    fireEvent.focus(historyInput);
    fireEvent.click(screen.getByRole("option", { name: /梅江花园/ }));
    expect(await screen.findByText("旧报告仍然可追溯。")).toBeInTheDocument();
    expect(screen.getByLabelText("当前可用现金 (万)")).toHaveValue("");

    fireEvent.change(screen.getByLabelText("当前可用现金 (万)"), { target: { value: "200" } });
    expect(screen.getByText("旧报告仍然可追溯。")).toBeInTheDocument();
    fireEvent.change(historyInput, { target: { value: "2026-07-16" } });
    await waitFor(() => expect(listCapacityCalculations).toHaveBeenLastCalledWith(
      expect.objectContaining({ q: "2026-07-16" }),
      expect.any(AbortSignal),
    ));
    expect(screen.getByText("旧报告仍然可追溯。")).toBeInTheDocument();
  });

  it("inserts a newly generated report at the front of history", async () => {
    render(createElement(CalculatorPanel));
    await chooseTarget();
    fireEvent.click(screen.getByLabelText("确认采用该目标房成交价"));
    fillFamily();
    fireEvent.click(screen.getByRole("button", { name: "生成诊断报告" }));

    expect(await screen.findByText(/快照状态：刚刚生成/)).toBeInTheDocument();
    expect((screen.getByRole("combobox", { name: "诊断历史" }) as HTMLInputElement).value).toContain("海河花园");
    expect(screen.getByText("1 份报告")).toBeInTheDocument();
  });

  it("shows locked, stale-history, and contextual method states", async () => {
    setAccessToken("");
    const { unmount } = render(createElement(CalculatorPanel));
    expect(await screen.findByText("个人空间尚未解锁")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "诊断历史" })).toBeDisabled();
    unmount();

    setAccessToken("secret");
    vi.mocked(listCapacityCalculations).mockResolvedValue({ items: [reportSummary], total: 1, page: 1, pageSize: 20 });
    vi.mocked(getCapacityCalculation).mockRejectedValue(new ApiError("not_found", "missing", 404));
    render(createElement(CalculatorPanel));
    expect(await screen.findByText("这条历史记录已失效")).toBeInTheDocument();
  });

  it("shows report methods in context and hides the old-home method without an old-home snapshot", async () => {
    const noOldReport = {
      ...report,
      id: "calc-no-old",
      selectionContext: {
        ...report.selectionContext,
        oldHome: {
          mode: "none" as const,
          assetId: null,
          assetName: "",
          property: null,
          originalPurchasePriceWan: 0,
          purchasedOn: "",
          holdingYears: 0,
          confirmedSalePriceWan: 0,
          confirmedLoanBalanceWan: 0,
          priceDifferenceWan: null,
          assetUpdatedAt: null,
          marketReference: null,
          confirmedAt: report.createdAt,
        },
      },
    } satisfies CalculationResponse;
    const noOldSummary = { ...reportSummary, id: noOldReport.id, oldHomeName: "" };
    vi.mocked(listCapacityCalculations).mockResolvedValue({ items: [noOldSummary], total: 1, page: 1, pageSize: 20 });
    vi.mocked(getCapacityCalculation).mockResolvedValue(noOldReport);
    render(createElement(CalculatorPanel));

    expect(await screen.findByRole("link", { name: /了解计算口径/ })).toHaveAttribute("href", "/methods/monthly-payment-safety");
    expect(screen.queryByRole("link", { name: "旧房迟迟卖不掉怎么办？" })).not.toBeInTheDocument();
  });
});
