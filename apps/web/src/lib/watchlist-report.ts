import type { MetricChangeValue, WatchlistItem } from "./api-client";
import { getShanghaiWeekStart } from "./shanghai-date";

export const watchlistReportHeaders = [
  "报表周次",
  "观察池条目ID",
  "小区ID",
  "小区名称",
  "城市",
  "区域",
  "目标户型",
  "状态",
  "建议",
  "挂牌套数",
  "降价套数",
  "成交动量",
  "目标户型供应套数",
  "目标户型稀缺度",
  "成交样本数",
  "采集时间",
  "当前批次ID",
  "算法版本",
  "数据源ID",
  "sourceRef",
  "覆盖范围",
  "新鲜度",
  "质量状态",
  "质量警告",
  "周对比状态",
  "周对比不可用原因",
  "周对比当前批次ID",
  "周对比基准批次ID",
  "挂牌变化",
  "降价变化",
  "近30天成交变化",
] as const;

export function buildWatchlistCSV(items: WatchlistItem[], now = new Date()): string {
  const weekStart = getShanghaiWeekStart(now);
  const rows = items.map((item) => {
    const comparison = item.weeklyComparison;
    const currentBatch = comparison?.currentBatch;
    return [
      weekStart,
      item.id,
      item.neighborhoodId,
      item.name,
      item.city,
      item.area,
      item.targetLayout,
      item.status,
      item.advice,
      item.listedHomes,
      item.priceCutHomes,
      item.transactionMomentum,
      item.targetLayoutSupply,
      item.targetLayoutScarcity,
      item.transactionSampleCount,
      item.collectedAt,
      item.collectionRunId,
      item.algorithmVersion,
      item.sourceIds.length > 0 ? item.sourceIds.join(" | ") : currentBatch?.dataSourceId,
      currentBatch?.sourceRef,
      item.coverage,
      item.freshness,
      item.qualityState,
      item.qualityWarnings.join(" | "),
      comparison?.status,
      comparison?.reason,
      currentBatch?.collectionRunId,
      comparison?.baselineBatch?.collectionRunId,
      formatMetricChange(comparison?.listedHomes),
      formatMetricChange(comparison?.priceCutHomes),
      formatMetricChange(comparison?.recent30DayTransactions),
    ];
  });

  return `\uFEFF${[watchlistReportHeaders, ...rows]
    .map((row) => row.map(escapeCSVCell).join(","))
    .join("\r\n")}\r\n`;
}

export function getWatchlistReportFilename(now = new Date()): string {
  return `propulse-watchlist-${getShanghaiWeekStart(now)}.csv`;
}

export function downloadWatchlistCSV(items: WatchlistItem[], now = new Date()): string {
  const filename = getWatchlistReportFilename(now);
  const blob = new Blob([buildWatchlistCSV(items, now)], {
    type: "text/csv;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  let link: HTMLAnchorElement | undefined;
  try {
    link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.hidden = true;
    document.body.appendChild(link);
    link.click();
  } finally {
    link?.remove();
    URL.revokeObjectURL(url);
  }
  return filename;
}

export function escapeCSVCell(value: unknown): string {
  if (value === null || value === undefined) return "";
  let text = String(value);
  if (typeof value === "string" && /^\s*[=+\-@]/u.test(text)) {
    text = `'${text}`;
  }
  if (/[,"\r\n]/u.test(text)) {
    return `"${text.replaceAll('"', '""')}"`;
  }
  return text;
}

function formatMetricChange(change?: MetricChangeValue): string {
  if (!change) return "";
  const absolute = `${change.absoluteChange > 0 ? "+" : ""}${change.absoluteChange}`;
  const percentage = change.percentageStatus === "zero_baseline"
    ? "基准为0"
    : change.percentageChange == null
      ? "比例不可用"
      : `${change.percentageChange > 0 ? "+" : ""}${change.percentageChange.toFixed(1)}%`;
  return `当前 ${change.current}；基准 ${change.baseline}；变化 ${absolute}；${percentage}`;
}
