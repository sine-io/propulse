import { afterEach, describe, expect, it, vi } from "vitest";

import type { WatchlistItem } from "./api-client";
import {
  buildWatchlistCSV,
  downloadWatchlistCSV,
  escapeCSVCell,
  getWatchlistReportFilename,
  watchlistReportHeaders,
} from "./watchlist-report";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("watchlist report", () => {
  it("exports a BOM-prefixed weekly CSV with complete evidence", () => {
    const csv = buildWatchlistCSV(
      [watchlistFixture()],
      new Date("2026-07-12T16:30:00Z"),
    );

    expect(csv.charCodeAt(0)).toBe(0xfeff);
    for (const header of watchlistReportHeaders) {
      expect(csv).toContain(header);
    }
    expect(csv).toContain("2026-07-13");
    expect(csv).toContain("run-current");
    expect(csv).toContain("market-metrics/test.1");
    expect(csv).toContain("source-current");
    expect(csv).toContain("'+weekly.csv");
    expect(csv).toContain("full,current,sufficient");
    expect(csv).toContain("stale_data | partial_coverage");
    expect(csv).toContain("available,,run-current,run-baseline");
    expect(csv).toContain("当前 18；基准 15；变化 +3；+20.0%");
    expect(csv).toContain("当前 6；基准 4；变化 +2；+50.0%");
    expect(csv).toContain("当前 5；基准 5；变化 0；0.0%");
  });

  it("escapes delimiters, quotes, line breaks, and spreadsheet formulas", () => {
    expect(escapeCSVCell('报价,"低位"\n下周')).toBe('"报价,""低位""\n下周"');
    expect(escapeCSVCell("=1+1")).toBe("'=1+1");
    expect(escapeCSVCell("  @command")).toBe("'  @command");
    expect(escapeCSVCell(-3)).toBe("-3");

    const csv = buildWatchlistCSV([watchlistFixture({
      name: '=HYPERLINK("https://example.invalid","小区")',
      advice: "先核实,再报价\n不要追高",
    })]);
    expect(csv).toContain('"\'=HYPERLINK(""https://example.invalid"",""小区"")"');
    expect(csv).toContain('"先核实,再报价\n不要追高"');
  });

  it("uses the Shanghai week start in the filename", () => {
    expect(getWatchlistReportFilename(new Date("2026-07-12T16:30:00Z"))).toBe(
      "propulse-watchlist-2026-07-13.csv",
    );
  });

  it("creates and revokes a downloadable CSV blob", () => {
    const createObjectURL = vi.fn((blob: Blob) => {
      void blob;
      return "blob:watchlist-report";
    });
    const revokeObjectURL = vi.fn();
    const createDescriptor = Object.getOwnPropertyDescriptor(URL, "createObjectURL");
    const revokeDescriptor = Object.getOwnPropertyDescriptor(URL, "revokeObjectURL");
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: revokeObjectURL });
    let clickedDownload = "";
    let clickedHref = "";
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      clickedDownload = this.download;
      clickedHref = this.href;
    });

    try {
      const filename = downloadWatchlistCSV(
        [watchlistFixture()],
        new Date("2026-07-15T01:00:00Z"),
      );
      const blob = createObjectURL.mock.calls[0]?.[0];
      expect(blob).toBeInstanceOf(Blob);
      expect(blob).toMatchObject({ type: "text/csv;charset=utf-8" });
      expect(filename).toBe("propulse-watchlist-2026-07-13.csv");
      expect(clickedDownload).toBe(filename);
      expect(clickedHref).toBe("blob:watchlist-report");
      expect(revokeObjectURL).toHaveBeenCalledWith("blob:watchlist-report");
    } finally {
      restoreProperty(URL, "createObjectURL", createDescriptor);
      restoreProperty(URL, "revokeObjectURL", revokeDescriptor);
    }
  });
});

function watchlistFixture(overrides: Partial<WatchlistItem> = {}): WatchlistItem {
  return {
    id: "watchlist-1",
    neighborhoodId: "neighborhood-1",
    name: "接口花园",
    city: "杭州",
    area: "南城",
    targetLayout: "三房",
    status: "重点看",
    listedHomes: 18,
    priceCutHomes: 6,
    transactionMomentum: "weak",
    targetLayoutSupply: 7,
    targetLayoutScarcity: "medium",
    advice: "按证据继续约看。",
    hasMetric: true,
    collectionRunId: "run-current",
    algorithmVersion: "market-metrics/test.1",
    sourceIds: ["source-current"],
    collectedAt: "2026-07-14T08:00:00Z",
    transactionSampleCount: 5,
    coverage: "full",
    freshness: "current",
    qualityState: "sufficient",
    qualityWarnings: ["stale_data", "partial_coverage"],
    weeklyComparison: {
      status: "available",
      currentBatch: {
        collectionRunId: "run-current",
        dataSourceId: "source-current",
        sourceRef: "+weekly.csv",
        collectedAt: "2026-07-14T08:00:00Z",
        coverage: "full",
      },
      baselineBatch: {
        collectionRunId: "run-baseline",
        dataSourceId: "source-current",
        sourceRef: "baseline.csv",
        collectedAt: "2026-07-07T08:00:00Z",
        coverage: "full",
      },
      listedHomes: metricChange(18, 15),
      priceCutHomes: metricChange(6, 4),
      recent30DayTransactions: metricChange(5, 5),
    },
    ...overrides,
  };
}

function metricChange(current: number, baseline: number) {
  return {
    current,
    baseline,
    absoluteChange: current - baseline,
    percentageChange: ((current - baseline) / baseline) * 100,
    percentageStatus: "available" as const,
  };
}

function restoreProperty(
  target: typeof URL,
  key: "createObjectURL" | "revokeObjectURL",
  descriptor: PropertyDescriptor | undefined,
) {
  if (descriptor) {
    Object.defineProperty(target, key, descriptor);
  } else {
    delete (target as unknown as Record<string, unknown>)[key];
  }
}
