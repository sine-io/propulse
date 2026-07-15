import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { setAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createDataSource,
  createNeighborhood,
  getCollectionRunDetail,
  importCSVCollectionRun,
  importJSONCollectionRun,
  listDataSources,
  searchNeighborhoods,
  type CollectionRunDetail,
  type DataSource,
  type ImportCollectionRunResponse,
  type Neighborhood,
} from "@/lib/api-client";
import { DataManagementPage } from "./data-management-page";
import { ImportDetailPage } from "./import-detail-page";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    createDataSource: vi.fn(),
    createNeighborhood: vi.fn(),
    getCollectionRunDetail: vi.fn(),
    getCSVImportTemplate: vi.fn(),
    importCSVCollectionRun: vi.fn(),
    importJSONCollectionRun: vi.fn(),
    listDataSources: vi.fn(),
    searchNeighborhoods: vi.fn(),
  };
});

const sourceFixture: DataSource = {
  id: "11111111-1111-1111-1111-111111111111",
  name: "市住建委公开数据",
  sourceType: "public_registry",
  city: "杭州",
  notes: "",
  createdAt: "2026-07-14T06:00:00Z",
  updatedAt: "2026-07-14T06:00:00Z",
};

const neighborhoodFixture: Neighborhood = {
  id: "22222222-2222-2222-2222-222222222222",
  name: "真实花园",
  city: "杭州",
  area: "滨江",
  availableLayouts: ["三房", "四房"],
  createdAt: "2026-07-14T06:00:00Z",
};

const importResultFixture: ImportCollectionRunResponse = {
  collectionRunId: "33333333-3333-3333-3333-333333333333",
  acceptedRecordCount: 2,
  rejectedRecordCount: 0,
  collectionRun: {
    id: "33333333-3333-3333-3333-333333333333",
    dataSourceId: sourceFixture.id,
    neighborhoodId: neighborhoodFixture.id,
    sourceRef: "registry-2026-07-14",
    collectedAt: "2026-07-14T06:00:00Z",
    coverage: "full",
    format: "json",
    contentChecksum: "a".repeat(64),
    rawContentType: "application/json",
    validationSummary: {
      recordCount: 2,
      listingCount: 1,
      transactionCount: 1,
      issues: [],
    },
    status: "completed",
    metricStatus: "completed",
    createdAt: "2026-07-14T06:01:00Z",
    updatedAt: "2026-07-14T06:01:00Z",
  },
  listingObservationCount: 1,
  transactionObservationCount: 1,
  idempotentReplay: false,
  metricRefreshStatus: "completed",
};

const detailFixture: CollectionRunDetail = {
  collectionRun: importResultFixture.collectionRun,
  source: sourceFixture,
  listings: [
    {
      id: "listing-observation-1",
      collectionRunId: importResultFixture.collectionRunId,
      neighborhoodId: neighborhoodFixture.id,
      sourceListingId: "listing-1",
      sourceRow: 2,
      layout: "三房",
      areaSqm: 89,
      listingPrice: 520,
      daysOnMarket: 12,
      status: "active",
      capturedAt: "2026-07-14T06:00:00Z",
      attributes: { floor: "8" },
    },
  ],
  transactions: [
    {
      id: "transaction-observation-1",
      collectionRunId: importResultFixture.collectionRunId,
      neighborhoodId: neighborhoodFixture.id,
      sourceRecordId: "transaction-1",
      sourceRow: 3,
      layout: "三房",
      areaSqm: 89,
      transactionPrice: 505,
      transactionDate: "2026-07-01",
      originalListingRef: "listing-1",
      capturedAt: "2026-07-14T06:00:00Z",
    },
  ],
  rawPayloadBase64: btoa("[]"),
};

describe("DataManagementPage", () => {
  beforeEach(() => {
    vi.mocked(listDataSources).mockResolvedValue([sourceFixture]);
    vi.mocked(searchNeighborhoods).mockResolvedValue({
      items: [neighborhoodFixture],
      total: 1,
      page: 1,
      pageSize: 50,
      filters: { cities: ["杭州"], areas: [{ city: "杭州", area: "滨江" }] },
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    window.sessionStorage.clear();
    window.history.replaceState({}, "", "/data");
  });

  it("renders a locked state without requesting protected data", async () => {
    render(<DataManagementPage />);

    expect(await screen.findByText("数据管理已锁定")).toBeInTheDocument();
    expect(listDataSources).not.toHaveBeenCalled();
  });

  it("renders a loading state while the real catalog request is pending", async () => {
    setAccessToken("secret-token");
    vi.mocked(listDataSources).mockReturnValue(new Promise(() => undefined));
    render(<DataManagementPage />);

    expect(await screen.findByText("正在读取数据目录")).toBeInTheDocument();
  });

  it("renders an actionable empty catalog state", async () => {
    setAccessToken("secret-token");
    vi.mocked(listDataSources).mockResolvedValue([]);
    vi.mocked(searchNeighborhoods).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      pageSize: 50,
      filters: { cities: [], areas: [] },
    });
    render(<DataManagementPage />);

    expect(await screen.findByTestId("catalog-empty-state")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "新建" })).toHaveLength(2);
    expect(screen.getByRole("button", { name: "创建批次" })).toBeInTheDocument();
  });

  it("creates and selects a real data source", async () => {
    setAccessToken("secret-token");
    const created = { ...sourceFixture, id: "44444444-4444-4444-4444-444444444444", name: "新增来源" };
    vi.mocked(createDataSource).mockResolvedValue(created);
    render(<DataManagementPage />);
    await screen.findByRole("heading", { name: "新建采集批次" });

    fireEvent.click(screen.getAllByRole("button", { name: "新建" })[0]);
    fireEvent.change(screen.getByLabelText("名称"), { target: { value: "新增来源" } });
    fireEvent.change(screen.getByLabelText("城市"), { target: { value: "杭州" } });
    fireEvent.click(screen.getByRole("button", { name: "创建数据源" }));

    await waitFor(() => expect(createDataSource).toHaveBeenCalled());
    expect(screen.getByLabelText("数据源")).toHaveValue(created.id);
  });

  it("creates and selects a real neighborhood", async () => {
    setAccessToken("secret-token");
    const created = { ...neighborhoodFixture, id: "55555555-5555-5555-5555-555555555555", name: "新增小区" };
    vi.mocked(createNeighborhood).mockResolvedValue(created);
    render(<DataManagementPage />);
    await screen.findByRole("heading", { name: "新建采集批次" });

    fireEvent.click(screen.getAllByRole("button", { name: "新建" })[1]);
    fireEvent.change(screen.getByLabelText("小区名称"), { target: { value: "新增小区" } });
    fireEvent.change(screen.getByLabelText("城市"), { target: { value: "杭州" } });
    fireEvent.change(screen.getByLabelText("区域"), { target: { value: "滨江" } });
    fireEvent.change(screen.getByLabelText("可选户型（逗号分隔）"), { target: { value: "三房，四房" } });
    fireEvent.click(screen.getByRole("button", { name: "创建小区" }));

    await waitFor(() => expect(createNeighborhood).toHaveBeenCalledWith({
      name: "新增小区",
      city: "杭州",
      area: "滨江",
      availableLayouts: ["三房", "四房"],
    }));
    expect(screen.getByLabelText("小区")).toHaveValue(created.id);
  });

  it("shows local validation without sending a batch", async () => {
    await renderReadyImportForm();
    fireEvent.change(screen.getByLabelText("来源引用"), { target: { value: "registry-1" } });
    fireEvent.change(screen.getByLabelText("采集时间"), { target: { value: "2026-07-14T14:00" } });
    fireEvent.change(screen.getByLabelText("记录数组"), { target: { value: "{}" } });
    fireEvent.click(screen.getByRole("button", { name: "创建批次" }));

    expect(await screen.findByText("导入校验未通过")).toBeInTheDocument();
    expect(screen.getByText("JSON 顶层必须是记录数组。")).toBeInTheDocument();
    expect(importJSONCollectionRun).not.toHaveBeenCalled();
  });

  it("renders backend row errors and rejected counts", async () => {
    vi.mocked(importJSONCollectionRun).mockRejectedValue(
      new ApiError(
        "validation_failed",
        "invalid",
        422,
        [{ row: 2, field: "listingPrice", code: "required", message: "listingPrice is required" }],
        0,
        1,
      ),
    );
    await fillValidJSONImport();
    fireEvent.click(screen.getByRole("button", { name: "创建批次" }));

    expect(await screen.findByText("导入校验未通过")).toBeInTheDocument();
    expect(screen.getByText("接受 0 条，拒绝 1 条。")).toBeInTheDocument();
    expect(screen.getByText("第 2 行")).toBeInTheDocument();
    expect(screen.queryByText("批次导入成功")).not.toBeInTheDocument();
  });

  it("shows only the real successful batch result and detail link", async () => {
    vi.mocked(importJSONCollectionRun).mockResolvedValue(importResultFixture);
    await fillValidJSONImport();
    fireEvent.click(screen.getByRole("button", { name: "创建批次" }));

    expect(await screen.findByText("批次导入成功")).toBeInTheDocument();
    expect(screen.getByText(importResultFixture.collectionRunId)).toBeInTheDocument();
    expect(screen.getByText("已刷新")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看批次" })).toHaveAttribute(
      "href",
      `/data/imports/${importResultFixture.collectionRunId}`,
    );
  });

  it("uploads CSV files with the same selected metadata", async () => {
    vi.mocked(importCSVCollectionRun).mockResolvedValue({
      ...importResultFixture,
      collectionRun: { ...importResultFixture.collectionRun, format: "csv" },
    });
    await renderReadyImportForm();
    fireEvent.change(screen.getByLabelText("来源引用"), { target: { value: "registry-csv" } });
    fireEvent.change(screen.getByLabelText("采集时间"), { target: { value: "2026-07-14T14:00" } });
    fireEvent.click(screen.getByRole("button", { name: "CSV" }));
    const file = new File(["recordType,sourceRecordId\r\n"], "records.csv", { type: "text/csv" });
    fireEvent.change(screen.getByLabelText("选择 CSV"), { target: { files: [file] } });
    fireEvent.click(screen.getByRole("button", { name: "创建批次" }));

    await waitFor(() => {
      expect(importCSVCollectionRun).toHaveBeenCalledWith(
        expect.objectContaining({ sourceRef: "registry-csv", coverage: "full" }),
        file,
        expect.any(AbortSignal),
      );
    });
    expect(await screen.findByText("批次导入成功")).toBeInTheDocument();
  });

  it("preserves input and removes prior conclusions on request failure", async () => {
    vi.mocked(importJSONCollectionRun).mockRejectedValue(new Error("offline"));
    await fillValidJSONImport();
    fireEvent.click(screen.getByRole("button", { name: "创建批次" }));

    expect(await screen.findByText("导入请求失败")).toBeInTheDocument();
    expect(screen.getByLabelText("来源引用")).toHaveValue("registry-1");
    expect(screen.getByLabelText("记录数组")).toHaveValue(validRecordsJSON);
    expect(screen.queryByText("批次导入成功")).not.toBeInTheDocument();
    expect(screen.queryByText(importResultFixture.collectionRunId)).not.toBeInTheDocument();
  });
});

describe("ImportDetailPage", () => {
  afterEach(() => {
    vi.clearAllMocks();
    window.sessionStorage.clear();
    window.history.replaceState({}, "", "/data");
  });

  it("stays locked without a token", async () => {
    window.history.replaceState({}, "", `/data/imports/${importResultFixture.collectionRunId}`);
    render(<ImportDetailPage />);

    expect(await screen.findByText("批次详情已锁定")).toBeInTheDocument();
    expect(getCollectionRunDetail).not.toHaveBeenCalled();
  });

  it("loads after the session is unlocked without requiring a page reload", async () => {
    window.history.replaceState({}, "", `/data/imports/${importResultFixture.collectionRunId}`);
    vi.mocked(getCollectionRunDetail).mockResolvedValue(detailFixture);
    render(<ImportDetailPage />);
    expect(await screen.findByText("批次详情已锁定")).toBeInTheDocument();

    await act(async () => {
      setAccessToken("secret-token");
    });

    expect(await screen.findByText("采集批次详情")).toBeInTheDocument();
    expect(getCollectionRunDetail).toHaveBeenCalled();
  });

  it("loads source, metadata, normalized observations, and validation summary", async () => {
    setAccessToken("secret-token");
    window.history.replaceState({}, "", `/data/imports/${importResultFixture.collectionRunId}`);
    vi.mocked(getCollectionRunDetail).mockResolvedValue(detailFixture);
    render(<ImportDetailPage />);

    expect(await screen.findByText("采集批次详情")).toBeInTheDocument();
    expect(getCollectionRunDetail).toHaveBeenCalledWith(
      importResultFixture.collectionRunId,
      expect.any(AbortSignal),
    );
    expect(screen.getByText(sourceFixture.name)).toBeInTheDocument();
    expect(screen.getAllByText("listing-1")).toHaveLength(2);
    expect(screen.getByText("transaction-1")).toBeInTheDocument();
    expect(screen.getByText("无校验错误")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "下载原始载荷" })).toBeInTheDocument();
  });

  it("shows a failed state without retaining batch content", async () => {
    setAccessToken("secret-token");
    window.history.replaceState({}, "", `/data/imports/${importResultFixture.collectionRunId}`);
    vi.mocked(getCollectionRunDetail).mockRejectedValue(new Error("offline"));
    render(<ImportDetailPage />);

    expect(await screen.findByText("批次详情读取失败")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
    expect(screen.queryByText(sourceFixture.name)).not.toBeInTheDocument();
  });
});

async function renderReadyImportForm() {
  setAccessToken("secret-token");
  render(<DataManagementPage />);
  await screen.findByRole("heading", { name: "新建采集批次" });
}

async function fillValidJSONImport() {
  await renderReadyImportForm();
  fireEvent.change(screen.getByLabelText("来源引用"), { target: { value: "registry-1" } });
  fireEvent.change(screen.getByLabelText("采集时间"), { target: { value: "2026-07-14T14:00" } });
  fireEvent.change(screen.getByLabelText("记录数组"), { target: { value: validRecordsJSON } });
}

const validRecordsJSON = JSON.stringify([
  {
    recordType: "listing",
    sourceRecordId: "listing-1",
    layout: "三房",
    areaSqm: 89,
    listingPrice: 520,
    daysOnMarket: 12,
    status: "active",
  },
]);
