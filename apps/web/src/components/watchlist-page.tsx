"use client";

import Link from "next/link";
import {
  AlertTriangle,
  Bell,
  CheckCircle,
  Database,
  Eye,
  LockKeyhole,
  RefreshCw,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import { ApiError, getWatchlist, type WatchlistItem } from "@/lib/api-client";
import { StatusBadge } from "./status-badge";

type PageState = "checking" | "locked" | "loading" | "ready" | "failed";

type CommunityView = {
  advice: string;
  collectedAt?: string;
  cuts: string;
  icon: "check" | "eye" | "warning";
  listed: string;
  meta: string;
  name: string;
  qualityLabel?: string;
  status: string;
  statusTone: "emerald" | "amber" | "slate";
  transaction: string;
  transactionSamples: number;
  warnings: string[];
};

export function WatchlistPage() {
  const [accessState, setAccessState] = useState<"checking" | "locked" | "unlocked">(
    "checking",
  );
  const [pageState, setPageState] = useState<PageState>("checking");
  const [items, setItems] = useState<WatchlistItem[]>([]);
  const [requestVersion, setRequestVersion] = useState(0);

  useEffect(() => {
    const syncAccess = () => setAccessState(getAccessToken() ? "unlocked" : "locked");
    syncAccess();
    return subscribeToAccessToken(syncAccess);
  }, []);

  useEffect(() => {
    if (accessState === "checking") {
      setPageState("checking");
      return;
    }
    if (accessState === "locked") {
      setItems([]);
      setPageState("locked");
      return;
    }

    const controller = new AbortController();
    setItems([]);
    setPageState("loading");
    getWatchlist(controller.signal)
      .then((response) => {
        setItems(response.items);
        setPageState("ready");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setItems([]);
        setPageState(error instanceof ApiError && error.status === 401 ? "locked" : "failed");
      });

    return () => controller.abort();
  }, [accessState, requestVersion]);

  const communities = useMemo(() => items.map(toCommunityView), [items]);
  const stats = useMemo(
    () => ({
      bargain: items.filter((item) => ["重点看", "适合砍价"].includes(item.status)).length,
      hard: items.filter((item) => item.status === "价格偏硬").length,
      insufficient: items.filter(
        (item) =>
          !item.hasMetric ||
          item.qualityState !== "sufficient" ||
          item.transactionMomentum === "unknown",
      ).length,
      total: items.length,
    }),
    [items],
  );
  const qualityAlerts = useMemo(
    () =>
      communities.flatMap((community) =>
        community.warnings.map((warning) => `${community.name}：${warning}`),
      ),
    [communities],
  );

  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <section className="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold text-slate-900">我的观察池</h1>
          <p className="mt-2 text-slate-500">每周跟踪，不错过买方窗口期。</p>
        </div>
        <Link href="/templates" className="text-sm font-medium text-blue-600 hover:underline">
          导出本周报表
        </Link>
      </section>

      {pageState === "checking" || pageState === "loading" ? (
        <StateBand icon={Database} title="正在加载观察池" tone="slate" />
      ) : null}
      {pageState === "locked" ? (
        <StateBand
          icon={LockKeyhole}
          title="观察池已锁定"
          detail="解锁个人空间后才能读取你的目标小区和市场数据。"
          tone="amber"
        />
      ) : null}
      {pageState === "failed" ? (
        <StateBand
          icon={AlertTriangle}
          title="观察池读取失败"
          detail="请求没有返回可用数据。"
          tone="rose"
          action={
            <button
              type="button"
              onClick={() => setRequestVersion((version) => version + 1)}
              className="inline-flex h-9 items-center gap-2 rounded-md border border-rose-300 bg-white px-3 text-sm font-medium text-rose-700 hover:bg-rose-50"
            >
              <RefreshCw aria-hidden="true" className="h-4 w-4" />
              重试
            </button>
          }
        />
      ) : null}

      {pageState === "ready" ? (
        <>
          <section className="mb-8 grid grid-cols-2 gap-3 md:grid-cols-4">
            {[
              ["观察小区", stats.total, "text-slate-900"],
              ["可关注窗口", stats.bargain, "text-emerald-700"],
              ["价格偏硬", stats.hard, "text-slate-700"],
              ["数据不足", stats.insufficient, "text-amber-700"],
            ].map(([label, value, color]) => (
              <div key={label} className="rounded-lg border border-slate-200 bg-white p-4">
                <p className="text-xs text-slate-500">{label}</p>
                <p className={`mt-1 text-2xl font-bold ${color}`}>{value}</p>
              </div>
            ))}
          </section>

          {communities.length === 0 ? (
            <section className="border-y border-slate-200 py-12 text-center">
              <h2 className="font-semibold text-slate-900">观察池暂无小区</h2>
              <Link href="/neighborhoods" className="mt-3 inline-block text-sm font-medium text-blue-600 hover:underline">
                添加目标小区
              </Link>
            </section>
          ) : (
            <section className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,2fr)_minmax(260px,1fr)]">
              <div className="space-y-4">
                <h2 className="font-bold text-slate-800">小区状态</h2>
                {communities.map((community) => (
                  <CommunityCard key={community.name + community.meta} {...community} />
                ))}
              </div>

              <aside className="self-start border-l border-slate-200 pl-0 lg:pl-6">
                <h2 className="flex items-center font-bold text-slate-800">
                  <Bell aria-hidden="true" className="mr-2 h-5 w-5 text-amber-600" />
                  数据质量提醒
                </h2>
                {qualityAlerts.length > 0 ? (
                  <ul className="mt-4 space-y-3 text-sm text-slate-700">
                    {qualityAlerts.map((alert) => (
                      <li key={alert} className="border-l-2 border-amber-400 pl-3">
                        {alert}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="mt-4 text-sm text-slate-500">当前没有质量告警。</p>
                )}
              </aside>
            </section>
          )}
        </>
      ) : null}
    </main>
  );
}

function toCommunityView(item: WatchlistItem): CommunityView {
  const canBargain = item.status === "重点看" || item.status === "适合砍价";
  const stale = item.freshness === "stale" || item.freshness === "expired";
  const insufficient =
    !item.hasMetric ||
    item.qualityState !== "sufficient" ||
    item.transactionMomentum === "unknown";

  return {
    advice: item.advice,
    collectedAt: item.collectedAt ? formatCollectedAt(item.collectedAt) : undefined,
    cuts: item.hasMetric ? `${item.priceCutHomes} 套` : "暂无",
    icon: insufficient || stale ? "warning" : canBargain ? "check" : "eye",
    listed: item.hasMetric ? `${item.listedHomes} 套` : "暂无",
    meta: `${item.area} · ${item.targetLayout}`,
    name: item.name,
    qualityLabel: stale ? "数据已过期" : insufficient ? "数据不足" : undefined,
    status: item.status,
    statusTone: insufficient || stale ? "amber" : canBargain ? "emerald" : "slate",
    transaction: momentumCopy[item.transactionMomentum],
    transactionSamples: item.transactionSampleCount,
    warnings: item.qualityWarnings.map((warning) => warningCopy[warning] ?? warning),
  };
}

const momentumCopy: Record<WatchlistItem["transactionMomentum"], string> = {
  unknown: "成交数据不足",
  stable: "平稳",
  strong: "活跃",
  weak: "偏弱",
};

const warningCopy: Record<string, string> = {
  expired_data: "市场数据已经过期",
  insufficient_listing_samples: "挂牌样本不足",
  insufficient_transaction_samples: "成交样本不足",
  metric_refresh_pending: "新批次指标仍在刷新",
  metric_unavailable: "尚无可用指标",
  no_full_inventory: "缺少完整挂牌批次",
  partial_coverage: "当前批次为部分覆盖",
  stale_data: "市场数据已陈旧",
};

function formatCollectedAt(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "Asia/Shanghai",
  }).format(new Date(value));
}

function StateBand({
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
    amber: "border-amber-300 bg-amber-50 text-amber-950",
    rose: "border-rose-300 bg-rose-50 text-rose-950",
    slate: "border-slate-200 bg-slate-50 text-slate-800",
  }[tone];
  return (
    <section role="status" className={`flex min-h-24 items-center justify-between gap-4 border-l-4 px-5 py-4 ${toneClass}`}>
      <div className="flex items-start gap-3">
        <Icon aria-hidden="true" className="mt-0.5 h-5 w-5 flex-none" />
        <div>
          <h2 className="font-semibold">{title}</h2>
          {detail ? <p className="mt-1 text-sm opacity-80">{detail}</p> : null}
        </div>
      </div>
      {action}
    </section>
  );
}

function CommunityCard(community: CommunityView) {
  const Icon = community.icon === "check" ? CheckCircle : community.icon === "warning" ? AlertTriangle : Eye;
  return (
    <article className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-lg font-bold text-slate-900">{community.name}</h3>
          <p className="mt-1 text-xs text-slate-500">{community.meta}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {community.qualityLabel ? <StatusBadge tone="amber">{community.qualityLabel}</StatusBadge> : null}
          <StatusBadge tone={community.statusTone}>{community.status}</StatusBadge>
        </div>
      </div>

      <dl className="mt-5 grid grid-cols-3 gap-3 border-y border-slate-100 py-4 text-sm">
        <div>
          <dt className="text-xs text-slate-500">在售</dt>
          <dd className="mt-1 font-medium text-slate-900">{community.listed}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-500">降价</dt>
          <dd className="mt-1 font-medium text-slate-900">{community.cuts}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-500">成交</dt>
          <dd className="mt-1 font-medium text-slate-900">{community.transaction}</dd>
        </div>
      </dl>

      <div className="mt-4 flex items-start gap-2 text-sm text-slate-700">
        <Icon aria-hidden="true" className="mt-0.5 h-4 w-4 flex-none text-slate-500" />
        <p>{community.advice}</p>
      </div>
      <p className="mt-3 text-xs text-slate-500">
        {community.collectedAt
          ? `采集于 ${community.collectedAt} · ${community.transactionSamples} 笔成交样本`
          : "尚无采集批次"}
      </p>
    </article>
  );
}
