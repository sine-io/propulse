"use client";

import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  CalendarClock,
  Database,
  ExternalLink,
  History,
  MapPin,
  RefreshCw,
  Search,
} from "lucide-react";
import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  type TooltipContentProps,
} from "recharts";

import {
  ApiError,
  getMetricHistory,
  getNeighborhood,
  getNeighborhoodMetrics,
  searchNeighborhoods,
  type MetricHistoryResponse,
  type Neighborhood,
  type NeighborhoodMetricResponse,
} from "@/lib/api-client";

import { StatusBadge } from "./status-badge";

type PageState = "checking" | "loading" | "not_found" | "no_metric" | "ready" | "failed";

type NeighborhoodView = {
  history?: MetricHistoryResponse;
  historyFailed: boolean;
  metric?: NeighborhoodMetricResponse;
  neighborhood?: Neighborhood;
};

type TrendPoint = {
  collectedAt: string;
  coverage: "full" | "partial";
  label: string;
  listedHomes: number;
  priceCutHomes: number;
  sourceRef: string;
  transactionCount: number;
};

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function NeighborhoodsPage({ initialNeighborhoodId }: { initialNeighborhoodId?: string }) {
  const [routeReady, setRouteReady] = useState(Boolean(initialNeighborhoodId));
  const [neighborhoodId, setNeighborhoodId] = useState(initialNeighborhoodId?.trim() ?? "");
  const [pageState, setPageState] = useState<PageState>("checking");
  const [view, setView] = useState<NeighborhoodView>({ historyFailed: false });
  const [requestVersion, setRequestVersion] = useState(0);

  useEffect(() => {
    if (initialNeighborhoodId !== undefined) {
      setNeighborhoodId(initialNeighborhoodId.trim());
      setRouteReady(true);
      return;
    }

    const syncRoute = () => {
      setNeighborhoodId(new URLSearchParams(window.location.search).get("id")?.trim() ?? "");
      setRouteReady(true);
    };
    syncRoute();
    window.addEventListener("popstate", syncRoute);
    return () => window.removeEventListener("popstate", syncRoute);
  }, [initialNeighborhoodId]);

  useEffect(() => {
    if (!routeReady) {
      setPageState("checking");
      return;
    }
    if (!neighborhoodId) {
      setView({ historyFailed: false });
      setPageState("checking");
      return;
    }
    if (!uuidPattern.test(neighborhoodId)) {
      setView({ historyFailed: false });
      setPageState("not_found");
      return;
    }

    const controller = new AbortController();
    setView({ historyFailed: false });
    setPageState("loading");

    Promise.allSettled([
      getNeighborhood(neighborhoodId, controller.signal),
      getNeighborhoodMetrics(neighborhoodId, controller.signal),
      getMetricHistory(neighborhoodId, {}, controller.signal),
    ]).then(([neighborhoodResult, metricResult, historyResult]) => {
      if (controller.signal.aborted) return;

      if (neighborhoodResult.status === "rejected") {
        setPageState(isNotFound(neighborhoodResult.reason) ? "not_found" : "failed");
        return;
      }
      if (metricResult.status === "rejected") {
        if (isNotFound(metricResult.reason)) {
          setView({ historyFailed: historyResult.status === "rejected", neighborhood: neighborhoodResult.value });
          setPageState("no_metric");
          return;
        }
        setPageState("failed");
        return;
      }

      setView({
        history: historyResult.status === "fulfilled" ? historyResult.value : undefined,
        historyFailed: historyResult.status === "rejected",
        metric: metricResult.value,
        neighborhood: neighborhoodResult.value,
      });
      setPageState("ready");
    });

    return () => controller.abort();
  }, [neighborhoodId, requestVersion, routeReady]);

  if (!routeReady) {
    return <PageStateBand icon={Database} title="正在读取目标小区" tone="slate" />;
  }
  if (!neighborhoodId) {
    return <NeighborhoodSelector />;
  }

  return (
    <main className="mx-auto max-w-7xl space-y-8 px-4 py-8 sm:px-6 lg:px-8">
      {pageState === "loading" ? (
        <PageStateBand icon={Database} title="正在加载小区身份、指标与历史" tone="slate" />
      ) : null}
      {pageState === "not_found" ? (
        <PageStateBand
          icon={AlertTriangle}
          title="找不到该小区"
          detail="小区 ID 无效或记录已不存在。"
          tone="amber"
          action={<StateLink href="/neighborhoods" label="重新选择" />}
        />
      ) : null}
      {pageState === "failed" ? (
        <PageStateBand
          icon={AlertTriangle}
          title="小区数据读取失败"
          detail="请求没有返回可用的小区身份和当前指标。"
          tone="rose"
          action={<RetryButton onClick={() => setRequestVersion((version) => version + 1)} />}
        />
      ) : null}
      {pageState === "no_metric" && view.neighborhood ? (
        <>
          <NeighborhoodHeader neighborhood={view.neighborhood} />
          <PageStateBand
            icon={Database}
            title="该小区暂无市场指标"
            detail="当前没有可展示的挂牌或成交批次，不会用 0 或样例结论代替。"
            tone="amber"
            action={<StateLink href="/data" label="前往数据管理" />}
          />
        </>
      ) : null}
      {pageState === "ready" && view.neighborhood && view.metric ? (
        <NeighborhoodReadyView
          history={view.history}
          historyFailed={view.historyFailed}
          metric={view.metric}
          neighborhood={view.neighborhood}
          retry={() => setRequestVersion((version) => version + 1)}
        />
      ) : null}
    </main>
  );
}

function NeighborhoodSelector() {
  const [query, setQuery] = useState("");
  const [submittedQuery, setSubmittedQuery] = useState("");
  const [results, setResults] = useState<Neighborhood[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "failed">("loading");
  const [requestVersion, setRequestVersion] = useState(0);

  const runSearch = useCallback((search: string, signal?: AbortSignal) => {
    setState("loading");
    searchNeighborhoods(search, signal)
      .then((response) => {
        setResults(response.items);
        setState("ready");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setResults([]);
        setState("failed");
      });
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    runSearch(submittedQuery, controller.signal);
    return () => controller.abort();
  }, [requestVersion, runSearch, submittedQuery]);

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmittedQuery(query.trim());
  };

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
      <section className="border-b border-slate-200 pb-6">
        <p className="text-sm font-semibold text-blue-700">目标小区</p>
        <h1 className="mt-1 text-3xl font-bold text-slate-900">选择要查看的小区</h1>
        <form onSubmit={submit} className="mt-6 flex max-w-2xl gap-2">
          <label className="sr-only" htmlFor="neighborhood-search">搜索小区名称或区域</label>
          <input
            id="neighborhood-search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索小区名称或区域"
            className="h-11 min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
          />
          <button type="submit" className="inline-flex h-11 items-center gap-2 rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800">
            <Search aria-hidden="true" className="h-4 w-4" />
            搜索
          </button>
        </form>
      </section>

      {state === "loading" ? <PageStateBand icon={Database} title="正在搜索小区" tone="slate" /> : null}
      {state === "failed" ? (
        <PageStateBand
          icon={AlertTriangle}
          title="小区搜索失败"
          tone="rose"
          action={<RetryButton onClick={() => setRequestVersion((version) => version + 1)} />}
        />
      ) : null}
      {state === "ready" && results.length === 0 ? (
        <PageStateBand icon={Search} title="没有匹配的小区" detail="可以调整名称或区域后重新搜索。" tone="slate" />
      ) : null}
      {state === "ready" && results.length > 0 ? (
        <section aria-label="小区搜索结果" className="divide-y divide-slate-200 border-y border-slate-200">
          {results.map((item) => (
            <Link
              key={item.id}
              href={`/neighborhoods?id=${encodeURIComponent(item.id)}`}
              className="flex items-center justify-between gap-4 px-1 py-4 hover:bg-slate-50"
            >
              <div>
                <h2 className="font-semibold text-slate-900">{item.name}</h2>
                <p className="mt-1 text-sm text-slate-500">{item.area} · {item.targetLayout}</p>
              </div>
              <ExternalLink aria-hidden="true" className="h-4 w-4 flex-none text-slate-400" />
            </Link>
          ))}
        </section>
      ) : null}
    </main>
  );
}

function NeighborhoodReadyView({
  history,
  historyFailed,
  metric,
  neighborhood,
  retry,
}: {
  history?: MetricHistoryResponse;
  historyFailed: boolean;
  metric: NeighborhoodMetricResponse;
  neighborhood: Neighborhood;
  retry: () => void;
}) {
  const stale = metric.freshness === "stale" || metric.freshness === "expired";
  const insufficient = metric.qualityState !== "sufficient" || metric.transactionMomentum === "unknown";
  const currentHistoryPoint = history?.items.find((point) => point.batch.collectionRunId === metric.collectionRunId);
  const trend = useMemo<TrendPoint[]>(
    () => (history?.items ?? []).map((point) => ({
      collectedAt: point.collectedAt,
      coverage: point.coverage,
      label: formatShortDate(point.collectedAt),
      listedHomes: point.listedHomes,
      priceCutHomes: point.priceCutHomes,
      sourceRef: point.batch.sourceRef,
      transactionCount: point.transactionSampleCount,
    })),
    [history],
  );

  return (
    <>
      <NeighborhoodHeader neighborhood={neighborhood} metric={metric} />

      {stale ? (
        <PageStateBand
          icon={CalendarClock}
          title={metric.freshness === "expired" ? "市场数据已过期" : "市场数据已陈旧"}
          detail="当前信息仅用于核对历史，不生成新的买入或议价窗口。"
          tone="amber"
        />
      ) : insufficient ? (
        <PageStateBand
          icon={AlertTriangle}
          title="市场数据不足"
          detail="覆盖范围或挂牌、成交样本不足，当前结论已降级。"
          tone="amber"
        />
      ) : null}

      <section className="border-l-4 border-blue-600 bg-white px-5 py-5 sm:px-6">
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="text-xl font-bold text-slate-900">{metric.status}</h2>
          <StatusBadge tone={stale || insufficient ? "amber" : signalTone(metric.status)}>
            {freshnessCopy[metric.freshness]}
          </StatusBadge>
        </div>
        <p className="mt-3 max-w-4xl text-sm leading-6 text-slate-700">{metric.advice}</p>
        <ul className="mt-4 space-y-2 text-sm text-slate-700">
          {(metric.reasons ?? []).map((reason) => (
            <li key={reason} className="flex items-start gap-2">
              <Activity aria-hidden="true" className="mt-0.5 h-4 w-4 flex-none text-blue-600" />
              <span>{reason} <a href="#market-evidence" className="font-medium text-blue-700 hover:underline">查看证据</a></span>
            </li>
          ))}
        </ul>
      </section>

      <MetricGrid metric={metric} />

      <section className="border-t border-slate-200 pt-6">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className="text-sm font-semibold text-blue-700">真实批次</p>
            <h2 className="mt-1 text-xl font-bold text-slate-900">近 8 周挂牌与降价趋势</h2>
          </div>
          {history ? <p className="text-xs text-slate-500">{formatDateTime(history.window.from)} 至 {formatDateTime(history.window.to)}</p> : null}
        </div>

        {historyFailed ? (
          <PageStateBand
            icon={AlertTriangle}
            title="历史趋势读取失败"
            detail="当前指标仍可查看，趋势区域没有回退样例。"
            tone="rose"
            action={<RetryButton onClick={retry} />}
          />
        ) : trend.length < 2 ? (
          <PageStateBand icon={History} title="暂无趋势" detail="至少需要两个真实批次才能比较变化。" tone="slate" />
        ) : (
          <div className="mt-5 h-72 w-full" aria-label="真实挂牌与降价批次趋势图">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart3View data={trend} />
            </ResponsiveContainer>
          </div>
        )}
      </section>

      <section id="market-evidence" className="border-t border-slate-200 pt-6">
        <h2 className="text-xl font-bold text-slate-900">来源与质量证据</h2>
        <dl className="mt-4 grid gap-x-8 gap-y-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <EvidenceItem label="采集时间" value={formatDateTime(metric.collectedAt)} />
          <EvidenceItem label="计算时间" value={formatDateTime(metric.calculatedAt)} />
          <EvidenceItem label="算法版本" value={metric.algorithmVersion} />
          <EvidenceItem label="覆盖与新鲜度" value={`${coverageCopy[metric.coverage]} · ${freshnessCopy[metric.freshness]}`} />
          <EvidenceItem label="挂牌样本" value={`${metric.listingSampleCount} 条`} />
          <EvidenceItem label="成交样本" value={`${metric.transactionSampleCount} 条`} />
          <EvidenceItem label="来源 ID" value={metric.sourceIds.join(", ") || "暂无"} />
          <div>
            <dt className="text-xs text-slate-500">采集批次</dt>
            <dd className="mt-1 break-all font-medium text-slate-900">
              <Link href={`/data/imports/${metric.collectionRunId}`} className="text-blue-700 hover:underline">
                {currentHistoryPoint?.batch.sourceRef || metric.collectionRunId}
              </Link>
            </dd>
          </div>
        </dl>
        {metric.transactionEvidence ? (
          <p className="mt-5 text-sm text-slate-600">
            成交窗口 {metric.transactionEvidence.windowStart} 至 {metric.transactionEvidence.windowEnd}：最近 30 天 {metric.transactionEvidence.recent30DayTransactionCount} 笔，此前 60 天 {metric.transactionEvidence.preceding60DayTransactionCount} 笔。
          </p>
        ) : null}
        {metric.qualityWarnings.length > 0 ? (
          <ul className="mt-4 flex flex-wrap gap-2">
            {metric.qualityWarnings.map((warning) => (
              <li key={warning} className="rounded-md border border-amber-300 bg-amber-50 px-2 py-1 text-xs text-amber-900">
                {warningCopy[warning] ?? warning}
              </li>
            ))}
          </ul>
        ) : null}
      </section>
    </>
  );
}

function NeighborhoodHeader({ neighborhood, metric }: { neighborhood: Neighborhood; metric?: NeighborhoodMetricResponse }) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4 border-b border-slate-200 pb-5">
      <div>
        <div className="flex items-center gap-2 text-sm text-slate-500">
          <MapPin aria-hidden="true" className="h-4 w-4" />
          <span>{neighborhood.area}</span>
        </div>
        <h1 className="mt-2 text-3xl font-bold text-slate-900">{neighborhood.name}</h1>
        <p className="mt-2 text-sm text-slate-600">目标户型：{neighborhood.targetLayout}</p>
      </div>
      {metric ? (
        <div className="text-right text-xs text-slate-500">
          <p>采集于 {formatDateTime(metric.collectedAt)}</p>
          <p className="mt-1">计算于 {formatDateTime(metric.calculatedAt)}</p>
        </div>
      ) : null}
    </header>
  );
}

function MetricGrid({ metric }: { metric: NeighborhoodMetricResponse }) {
  const cards = [
    { label: "挂牌价区间", value: formatPriceRange(metric.listingPriceMin, metric.listingPriceMax), detail: `${metric.listingSampleCount} 条挂牌样本` },
    { label: "近 90 天成交区间", value: formatPriceRange(metric.transactionPriceMin, metric.transactionPriceMax), detail: `${metric.transactionSampleCount} 条成交样本` },
    { label: "当前在售", value: `${metric.listedHomes} 套`, detail: coverageCopy[metric.coverage] },
    { label: "当前降价", value: `${metric.priceCutHomes} 套`, detail: `占在售 ${((metric.priceCutShare ?? 0) * 100).toFixed(1)}%` },
    { label: "平均挂牌时长", value: metric.avgDaysOnMarket == null ? "暂无" : `${metric.avgDaysOnMarket.toFixed(1)} 天`, detail: "非成交周期" },
    { label: "目标户型供给", value: `${metric.targetLayoutSupply} 套`, detail: `稀缺度 ${scarcityCopy[metric.targetLayoutScarcity ?? "unknown"]}` },
  ];

  return (
    <section aria-label="当前市场指标" className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-6">
      {cards.map((card) => (
        <article key={card.label} className="rounded-md border border-slate-200 bg-white p-4">
          <h2 className="text-xs font-medium text-slate-500">{card.label}</h2>
          <p className="mt-2 text-lg font-bold text-slate-900">{card.value}</p>
          <p className="mt-2 text-xs text-slate-500">{card.detail}</p>
        </article>
      ))}
    </section>
  );
}

function BarChart3View({ data }: { data: TrendPoint[] }) {
  return (
    <BarChart data={data} margin={{ top: 8, right: 8, left: -12, bottom: 4 }}>
      <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
      <XAxis dataKey="label" tick={{ fill: "#64748b", fontSize: 11 }} />
      <YAxis allowDecimals={false} tick={{ fill: "#64748b", fontSize: 11 }} />
      <Tooltip content={TrendTooltip} />
      <Legend wrapperStyle={{ fontSize: 12 }} />
      <Bar dataKey="listedHomes" name="在售套数" radius={[3, 3, 0, 0]}>
        {data.map((point) => <Cell key={`listed-${point.collectedAt}`} fill={point.coverage === "full" ? "#2563eb" : "#94a3b8"} />)}
      </Bar>
      <Bar dataKey="priceCutHomes" name="降价套数" radius={[3, 3, 0, 0]}>
        {data.map((point) => <Cell key={`cut-${point.collectedAt}`} fill={point.coverage === "full" ? "#d97706" : "#cbd5e1"} />)}
      </Bar>
    </BarChart>
  );
}

function TrendTooltip({ active, payload }: TooltipContentProps) {
  const point = payload?.[0]?.payload as TrendPoint | undefined;
  if (!active || !point) return null;
  return (
    <div className="rounded-md border border-slate-200 bg-white p-3 text-xs shadow-lg">
      <p className="font-semibold text-slate-900">{formatDateTime(point.collectedAt)}</p>
      <p className="mt-2 text-slate-700">在售 {point.listedHomes} 套 · 降价 {point.priceCutHomes} 套</p>
      <p className="mt-1 text-slate-700">成交样本 {point.transactionCount} 条</p>
      <p className="mt-1 text-slate-500">{coverageCopy[point.coverage]} · {point.sourceRef}</p>
    </div>
  );
}

function EvidenceItem({ label, value }: { label: string; value: string }) {
  return <div><dt className="text-xs text-slate-500">{label}</dt><dd className="mt-1 break-words font-medium text-slate-900">{value}</dd></div>;
}

function PageStateBand({
  action,
  detail,
  icon: Icon,
  title,
  tone,
}: {
  action?: React.ReactNode;
  detail?: string;
  icon: typeof Database;
  title: string;
  tone: "slate" | "amber" | "rose";
}) {
  const toneClass = {
    amber: "border-amber-400 bg-amber-50 text-amber-950",
    rose: "border-rose-400 bg-rose-50 text-rose-950",
    slate: "border-slate-300 bg-slate-50 text-slate-800",
  }[tone];
  return (
    <section role="status" className={`my-6 flex min-h-24 flex-wrap items-center justify-between gap-4 border-l-4 px-5 py-4 ${toneClass}`}>
      <div className="flex items-start gap-3">
        <Icon aria-hidden="true" className="mt-0.5 h-5 w-5 flex-none" />
        <div><h2 className="font-semibold">{title}</h2>{detail ? <p className="mt-1 text-sm opacity-80">{detail}</p> : null}</div>
      </div>
      {action}
    </section>
  );
}

function RetryButton({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="inline-flex h-9 items-center gap-2 rounded-md border border-current bg-white px-3 text-sm font-medium">
      <RefreshCw aria-hidden="true" className="h-4 w-4" />重试
    </button>
  );
}

function StateLink({ href, label }: { href: string; label: string }) {
  return <Link href={href} className="inline-flex h-9 items-center rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800">{label}</Link>;
}

function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

function formatPriceRange(min?: number | null, max?: number | null): string {
  if (min == null || max == null) return "暂无";
  return `${formatNumber(min)}-${formatNumber(max)} 万`;
}

function formatNumber(value: number): string {
  return Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1);
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Shanghai" }).format(new Date(value));
}

function formatShortDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { month: "numeric", day: "numeric", timeZone: "Asia/Shanghai" }).format(new Date(value));
}

function signalTone(status: string): "emerald" | "blue" | "slate" {
  if (status === "重点看" || status === "适合砍价") return "emerald";
  return status === "继续观察" ? "blue" : "slate";
}

const freshnessCopy: Record<string, string> = { unknown: "新鲜度未知", current: "当前", stale: "已陈旧", expired: "已过期" };
const coverageCopy: Record<string, string> = { full: "完整覆盖", partial: "部分覆盖" };
const scarcityCopy: Record<string, string> = { unknown: "未知", low: "低", medium: "中", high: "高" };
const warningCopy: Record<string, string> = {
  expired_data: "市场数据已经过期",
  insufficient_listing_samples: "挂牌样本不足",
  insufficient_transaction_samples: "成交样本不足",
  metric_refresh_pending: "新批次指标仍在刷新",
  no_full_inventory: "缺少完整挂牌批次",
  partial_coverage: "当前批次为部分覆盖",
  stale_data: "市场数据已陈旧",
};
