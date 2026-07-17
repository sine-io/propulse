import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { setAccessToken } from "@/lib/access-token";
import {
  createAsset,
  deleteAsset,
  getMarketListingDetail,
  getMarketListings,
  listAssets,
  searchNeighborhoods,
  updateAsset,
  type Asset,
  type MarketListingDetail,
  type Neighborhood,
} from "@/lib/api-client";

import { AssetsPage } from "./assets-page";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    createAsset: vi.fn(),
    deleteAsset: vi.fn(),
    getMarketListingDetail: vi.fn(),
    getMarketListings: vi.fn(),
    listAssets: vi.fn(),
    searchNeighborhoods: vi.fn(),
    updateAsset: vi.fn(),
  };
});

const neighborhood: Neighborhood = {
  id: "22222222-2222-4222-8222-222222222222",
  name: "海河花园",
  city: "天津",
  area: "河西区",
  availableLayouts: ["3室2厅"],
};

const listing: MarketListingDetail = {
  roomId: "room-1",
  layout: "3室2厅",
  areaSqm: 118,
  listingTotalPriceWan: 500,
  listingUnitPrice: 42373,
  listedAt: "2026-07-01T00:00:00Z",
  daysOnMarket: 16,
  floorBand: "中楼层",
  floorDescription: "中楼层/20层",
  orientation: "南北",
  adjustmentCount: 0,
  followCount: 3,
  lookCount30Days: 2,
  neighborhoodId: neighborhood.id,
  neighborhoodName: neighborhood.name,
  city: "天津",
  district: "河西区",
  status: "active",
  snapshotId: "33333333-3333-4333-8333-333333333333",
  collectionRunId: "44444444-4444-4444-8444-444444444444",
  collectedAt: "2026-07-16T08:00:00Z",
  source: {
    dataSourceId: "55555555-5555-4555-8555-555555555555",
    dataSourceName: "房鉴",
    dataSourceType: "fangjian",
    sourceRef: "batch-1",
  },
  qualityStatus: "complete",
  freshness: "current",
};

const asset: Asset = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "海河花园 3室2厅",
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
    currentListingPriceWan: listing.listingTotalPriceWan,
  },
  originalPurchasePriceWan: 260,
  purchasedOn: "2020-08-20",
  currentLoanBalanceWan: 80,
  sourceKind: "market_listing",
  listingSource: {
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
  },
  createdAt: "2026-07-17T08:00:00Z",
  updatedAt: "2026-07-17T08:00:00Z",
};

describe("AssetsPage", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    vi.mocked(listAssets).mockReset().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 100 });
    vi.mocked(searchNeighborhoods).mockReset().mockResolvedValue({ items: [neighborhood], total: 1, page: 1, pageSize: 100, filters: { cities: ["天津"], areas: [{ city: "天津", area: "河西区" }] } });
    vi.mocked(getMarketListings).mockReset().mockResolvedValue({ items: [listing], total: 1, page: 1, pageSize: 100 });
    vi.mocked(getMarketListingDetail).mockReset().mockResolvedValue(listing);
    vi.mocked(createAsset).mockReset().mockResolvedValue(asset);
    vi.mocked(updateAsset).mockReset().mockResolvedValue(asset);
    vi.mocked(deleteAsset).mockReset().mockResolvedValue(undefined);
  });

  it("keeps private asset data locked without making a request", async () => {
    render(createElement(AssetsPage));
    expect(await screen.findByText("个人资产已锁定")).toBeInTheDocument();
    expect(listAssets).not.toHaveBeenCalled();
  });

  it("creates an asset from a server-authoritative listing selection", async () => {
    setAccessToken("secret");
    render(createElement(AssetsPage));
    expect(await screen.findByText("还没有资产档案")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "新增资产" })[0]);
    fireEvent.click(await screen.findByRole("button", { name: /海河花园/ }));
    fireEvent.click(await screen.findByLabelText(/3室2厅 · 118㎡/));
    await screen.findByText(/采集于/);
    fireEvent.change(screen.getByLabelText("原购入价 (万)"), { target: { value: "260" } });
    fireEvent.change(screen.getByLabelText("购入日期"), { target: { value: "2020-08-20" } });
    fireEvent.change(screen.getByLabelText("当前贷款余额 (万)"), { target: { value: "80" } });
    fireEvent.click(screen.getByRole("button", { name: "建立资产" }));

    await waitFor(() => expect(createAsset).toHaveBeenCalled());
    expect(vi.mocked(createAsset).mock.calls[0]?.[0]).toEqual({
      name: "海河花园 3室2厅",
      neighborhoodId: neighborhood.id,
      originalPurchasePriceWan: 260,
      purchasedOn: "2020-08-20",
      currentLoanBalanceWan: 80,
      propertySelection: { mode: "market_listing", roomId: listing.roomId },
    });
  });

  it("soft-deletes from the list while preserving the explicit history message", async () => {
    setAccessToken("secret");
    vi.mocked(listAssets).mockResolvedValueOnce({ items: [asset], total: 1, page: 1, pageSize: 100 });
    render(createElement(AssetsPage));
    fireEvent.click(await screen.findByRole("button", { name: `删除 ${asset.name}` }));
    expect(screen.getByText(/已有诊断报告仍保留当时快照/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(deleteAsset).toHaveBeenCalledWith(asset.id));
  });
});
