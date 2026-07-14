"use client";

import Link from "next/link";
import {
  AlertTriangle,
  CheckCircle,
  Database,
  ExternalLink,
  LockKeyhole,
  MapPin,
  RefreshCw,
} from "lucide-react";
import { useEffect, useState } from "react";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import { ApiError, getActionWindow, type ActionWindowResponse } from "@/lib/api-client";
import { StatusBadge } from "./status-badge";

type DecisionFactor = ActionWindowResponse["factors"][number];
type DecisionFactorEvidence = DecisionFactor["evidence"][number];

type AccessState = "checking" | "locked" | "unlocked";
type PageState = "checking" | "locked" | "loading" | "ready" | "blocked";

type BlockedState = {
  code:
    | "capacity_required"
    | "watchlist_required"
    | "metric_required"
    | "metric_stale"
    | "metric_insufficient"
    | "request_failed";
  detail: string;
  href?: string;
  linkLabel?: string;
  title: string;
};

export function ActionWindowPage() {
  const [accessState, setAccessState] = useState<AccessState>("checking");
  const [pageState, setPageState] = useState<PageState>("checking");
  const [recommendation, setRecommendation] = useState<ActionWindowResponse>();
  const [blocked, setBlocked] = useState<BlockedState>();
  const [checkedItems, setCheckedItems] = useState<string[]>([]);
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
      setRecommendation(undefined);
      setBlocked(undefined);
      setPageState("locked");
      return;
    }

    const controller = new AbortController();
    setRecommendation(undefined);
    setBlocked(undefined);
    setPageState("loading");
    getActionWindow(controller.signal)
      .then((response) => {
        setRecommendation(response);
        setCheckedItems([]);
        setPageState("ready");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setRecommendation(undefined);
        if (error instanceof ApiError && error.status === 401) {
          setPageState("locked");
          return;
        }
        setBlocked(actionWindowBlockedState(error));
        setPageState("blocked");
      });

    return () => controller.abort();
  }, [accessState, requestVersion]);

  const toggleChecklistItem = (item: string) => {
    setCheckedItems((current) =>
      current.includes(item)
        ? current.filter((value) => value !== item)
        : [...current, item],
    );
  };

  return (
    <main className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6 lg:px-8">
      <section className="mb-8">
        <h1 className="text-3xl font-bold text-slate-900">现在适合看、等、砍价，还是出手？</h1>
        <p className="mt-2 text-slate-500">结合资金预算与目标小区的可信市场指标生成决策。</p>
      </section>

      {pageState === "checking" || pageState === "loading" ? (
        <DecisionState icon={Database} title="正在检查出手窗口" tone="slate" />
      ) : null}
      {pageState === "locked" ? (
        <DecisionState
          icon={LockKeyhole}
          title="出手窗口已锁定"
          detail="解锁个人空间后才能读取测算、观察池和市场指标。"
          tone="amber"
        />
      ) : null}
      {pageState === "blocked" && blocked ? (
        <DecisionState
          icon={AlertTriangle}
          title={blocked.title}
          detail={blocked.detail}
          tone={blocked.code === "request_failed" ? "rose" : "amber"}
          action={
            <div className="flex flex-wrap items-center gap-2">
              {blocked.href && blocked.linkLabel ? (
                <Link
                  href={blocked.href}
                  className="inline-flex h-9 items-center rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800"
                >
                  {blocked.linkLabel}
                </Link>
              ) : null}
              <button
                type="button"
                onClick={() => setRequestVersion((version) => version + 1)}
                className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                <RefreshCw aria-hidden="true" className="h-4 w-4" />
                重试
              </button>
            </div>
          }
        />
      ) : null}

      {pageState === "ready" && recommendation ? (
        <>
          <section className="border-l-4 border-l-blue-600 bg-white p-6 shadow-sm sm:p-8">
            <div className="mb-4 flex flex-col justify-between gap-4 md:flex-row md:items-center">
              <div>
                <p className="mb-1 text-sm font-semibold text-slate-500">当前核心策略</p>
                <h2 className="text-3xl font-bold text-slate-900">建议{recommendation.action}</h2>
              </div>
              <div className="md:text-right">
                <p className="mb-1 text-sm font-semibold text-slate-500">策略信心</p>
                <StatusBadge tone={recommendation.confidence === "低" ? "amber" : "emerald"}>
                  {recommendation.confidence}
                </StatusBadge>
              </div>
            </div>
            <p className="border border-slate-100 bg-slate-50 p-4 text-base leading-relaxed text-slate-700">
              {recommendation.summary}
            </p>
            <ul className="mt-4 space-y-2 text-sm text-slate-600">
              {recommendation.confidenceReasons.map((reason) => (
                <li key={reason} className="flex items-start gap-2">
                  <span aria-hidden="true" className="mt-2 h-1.5 w-1.5 flex-none rounded-full bg-blue-500" />
                  <span>{reason}</span>
                </li>
              ))}
            </ul>
          </section>

          <DecisionContext recommendation={recommendation} />

          <section>
            <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
              <div>
                <h2 className="text-xl font-bold text-slate-900">决策因子与证据</h2>
                <p className="mt-1 text-sm text-slate-500">六项证据来自本次策略使用的同一测算与指标。</p>
              </div>
              <StatusBadge
                tone={recommendation.metric.qualityState === "sufficient" ? "emerald" : "amber"}
              >
                {qualityStateCopy[recommendation.metric.qualityState]}
              </StatusBadge>
            </div>
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              {recommendation.factors.map((factor) => (
                <DecisionFactorItem
                  key={factor.key}
                  factor={factor}
                  recommendation={recommendation}
                />
              ))}
            </div>
          </section>

          <section className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <div className="bg-slate-900 p-6 text-white shadow-sm">
              <h3 className="mb-4 flex items-center text-lg font-bold">
                <CheckCircle aria-hidden="true" className="h-5 w-5" />
                <span className="ml-2">行动清单</span>
              </h3>
              <ul className="space-y-3 text-sm text-slate-200">
                {recommendation.checklist.map((item) => {
                  const checked = checkedItems.includes(item);
                  return (
                    <li key={item}>
                      <label className="flex cursor-pointer items-start gap-3">
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleChecklistItem(item)}
                          className="mt-0.5 h-4 w-4 flex-none rounded border-slate-500 bg-slate-800 text-emerald-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400"
                        />
                        <span className={checked ? "text-slate-400 line-through" : undefined}>{item}</span>
                      </label>
                    </li>
                  );
                })}
              </ul>
            </div>

            <div className="border border-rose-200 bg-rose-50 p-6">
              <h3 className="mb-4 flex items-center text-lg font-bold text-rose-900">
                <AlertTriangle aria-hidden="true" className="h-5 w-5" />
                <span className="ml-2">风险警示</span>
              </h3>
              <ul className="space-y-3 text-sm text-rose-800">
                {recommendation.risks.map((risk) => (
                  <li key={risk} className="flex items-start gap-3">
                    <span aria-hidden="true" className="mt-2 h-1.5 w-1.5 flex-none rounded-full bg-rose-500" />
                    <span>{risk}</span>
                  </li>
                ))}
              </ul>
            </div>
          </section>
        </>
      ) : null}
    </main>
  );
}

function DecisionContext({ recommendation }: { recommendation: ActionWindowResponse }) {
  return (
    <section className="grid gap-4 border-y border-slate-200 py-5 text-sm md:grid-cols-3">
      <div className="min-w-0">
        <p className="text-xs font-medium text-slate-500">评估目标</p>
        <Link
          href={`/neighborhoods?id=${encodeURIComponent(recommendation.target.neighborhoodId)}`}
          className="mt-1 inline-flex max-w-full items-center gap-2 font-semibold text-blue-700 hover:underline"
        >
          <MapPin aria-hidden="true" className="h-4 w-4 flex-none" />
          <span className="truncate">{recommendation.target.name}</span>
        </Link>
        <p className="mt-1 text-xs text-slate-500">
          {recommendation.target.area} · {recommendation.target.targetLayout}
        </p>
      </div>
      <div className="min-w-0">
        <p className="text-xs font-medium text-slate-500">资金测算</p>
        <Link
          href={`/calculator?calculationId=${encodeURIComponent(recommendation.capacityCalculation.id)}`}
          className="mt-1 inline-flex max-w-full items-center gap-2 font-semibold text-blue-700 hover:underline"
        >
          <span className="truncate">测算 {shortReference(recommendation.capacityCalculation.id)}</span>
          <ExternalLink aria-hidden="true" className="h-3.5 w-3.5 flex-none" />
        </Link>
        <p className="mt-1 text-xs text-slate-500">
          {formatDecisionTimestamp(recommendation.capacityCalculation.createdAt)} ·{" "}
          {recommendation.capacityCalculation.ruleVersion} ·{" "}
          {traceabilityCopy[recommendation.capacityCalculation.traceabilityStatus]}
        </p>
      </div>
      <div className="min-w-0">
        <p className="text-xs font-medium text-slate-500">市场指标</p>
        <Link
          href={`/data/imports/${encodeURIComponent(recommendation.metric.collectionRunId)}`}
          className="mt-1 inline-flex max-w-full items-center gap-2 font-semibold text-blue-700 hover:underline"
        >
          <span className="truncate">批次 {shortReference(recommendation.metric.collectionRunId)}</span>
          <ExternalLink aria-hidden="true" className="h-3.5 w-3.5 flex-none" />
        </Link>
        <p className="mt-1 text-xs text-slate-500">
          {formatDecisionTimestamp(recommendation.metric.collectedAt)} ·{" "}
          {coverageCopy[recommendation.metric.coverage]} ·{" "}
          {freshnessCopy[recommendation.metric.freshness]} · 挂牌{" "}
          {recommendation.metric.listingSampleCount} / 成交{" "}
          {recommendation.metric.transactionSampleCount}
        </p>
      </div>
    </section>
  );
}

function DecisionFactorItem({
  factor,
  recommendation,
}: {
  factor: DecisionFactor;
  recommendation: ActionWindowResponse;
}) {
  const sourceHref = factorSourceHref(factor, recommendation);
  return (
    <article className="rounded-lg border border-slate-200 bg-white p-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold text-slate-900">{factorLabel[factor.key]}</h3>
        <StatusBadge tone={factorTone[factor.status]}>{factorStatusCopy[factor.status]}</StatusBadge>
      </div>
      <p className="mt-3 min-h-10 text-sm leading-relaxed text-slate-700">{factor.summary}</p>
      {factor.evidence.length > 0 ? (
        <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 border-t border-slate-100 pt-4">
          {factor.evidence.map((evidence) => (
            <div key={evidence.key} className="min-w-0">
              <dt className="text-xs text-slate-500">{evidence.label}</dt>
              <dd className="mt-1 break-words text-sm font-medium text-slate-900">
                {formatFactorEvidence(evidence)}
              </dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className="mt-4 border-t border-slate-100 pt-4 text-xs text-slate-500">暂无可核验值</p>
      )}
      <div className="mt-4 border-t border-slate-100 pt-3 text-xs text-slate-500">
        {factor.source && sourceHref ? (
          <Link
            href={sourceHref}
            className="inline-flex items-center gap-1.5 text-blue-700 hover:underline"
          >
            <span>
              {factorSourceCopy[factor.source.type]} {shortReference(factor.source.id)} ·{" "}
              {formatDecisionTimestamp(factor.source.observedAt)}
            </span>
            <ExternalLink aria-hidden="true" className="h-3.5 w-3.5" />
          </Link>
        ) : (
          <span>无可用来源</span>
        )}
      </div>
    </article>
  );
}

const factorLabel: Record<DecisionFactor["key"], string> = {
  budget_pressure: "资金压力",
  down_payment_gap: "首付缺口",
  market_signal: "目标小区信号",
  transaction_momentum: "成交动量",
  target_layout_supply: "目标户型供给",
  alternatives: "可比备选",
};

const factorStatusCopy: Record<DecisionFactor["status"], string> = {
  positive: "支持",
  neutral: "中性",
  caution: "谨慎",
  negative: "阻断",
  unknown: "未知",
};

const factorTone: Record<
  DecisionFactor["status"],
  "emerald" | "blue" | "amber" | "rose" | "slate"
> = {
  positive: "emerald",
  neutral: "blue",
  caution: "amber",
  negative: "rose",
  unknown: "slate",
};

const factorSourceCopy: Record<NonNullable<DecisionFactor["source"]>["type"], string> = {
  capacity_calculation: "资金测算记录",
  neighborhood_metric: "小区指标批次",
  alternative_comparison: "备选比较",
};

const qualityStateCopy: Record<ActionWindowResponse["metric"]["qualityState"], string> = {
  sufficient: "证据充分",
  low_confidence: "证据可信度较低",
  insufficient_data: "证据不足",
};

const traceabilityCopy: Record<
  ActionWindowResponse["capacityCalculation"]["traceabilityStatus"],
  string
> = {
  complete: "规则可追溯",
  legacy_unversioned: "旧版未版本化",
};

const coverageCopy: Record<ActionWindowResponse["metric"]["coverage"], string> = {
  full: "完整覆盖",
  partial: "部分覆盖",
};

const freshnessCopy: Record<ActionWindowResponse["metric"]["freshness"], string> = {
  current: "数据当前",
  stale: "数据陈旧",
  expired: "数据过期",
};

const evidenceTextCopy: Record<string, string> = {
  bargain: "适合砍价",
  complete: "可追溯",
  current: "当前",
  danger: "危险",
  full: "完整覆盖",
  high: "高",
  low: "低",
  medium: "中",
  safe: "安全",
  stable: "平稳",
  strained: "承压",
  strong: "活跃",
  sufficient: "充分",
  weak: "偏弱",
};

function formatFactorEvidence(evidence: DecisionFactorEvidence): string {
  if (evidence.valueType === "number") {
    if (evidence.numberValue == null) return "暂无";
    const value = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(
      evidence.numberValue,
    );
    return `${value}${evidence.unit ? ` ${evidence.unit}` : ""}`;
  }
  if (evidence.valueType === "boolean") {
    if (evidence.booleanValue == null) return "暂无";
    return evidence.booleanValue ? "是" : "否";
  }
  if (!evidence.textValue) return "暂无";
  if (evidence.key === "window_start" || evidence.key === "window_end") {
    return formatDecisionTimestamp(evidence.textValue);
  }
  return evidenceTextCopy[evidence.textValue] ?? evidence.textValue;
}

function factorSourceHref(
  factor: DecisionFactor,
  recommendation: ActionWindowResponse,
): string | undefined {
  switch (factor.source?.type) {
    case "capacity_calculation":
      return `/calculator?calculationId=${encodeURIComponent(factor.source.id)}`;
    case "neighborhood_metric":
      return `/data/imports/${encodeURIComponent(recommendation.metric.collectionRunId)}`;
    case "alternative_comparison":
      return "/watchlist";
    default:
      return undefined;
  }
}

function shortReference(value: string): string {
  return value.length > 12 ? `${value.slice(0, 8)}…` : value;
}

function formatDecisionTimestamp(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "Asia/Shanghai",
  }).format(new Date(value));
}

function actionWindowBlockedState(error: unknown): BlockedState {
  if (!(error instanceof ApiError)) {
    return { code: "request_failed", title: "决策服务不可用", detail: "请求没有返回可用结论。" };
  }
  switch (error.code) {
    case "capacity_required":
      return {
        code: "capacity_required",
        title: "需要换房测算",
        detail: "当前没有已保存的资金承压结果。",
        href: "/calculator",
        linkLabel: "前往换房测算",
      };
    case "watchlist_required":
      return {
        code: "watchlist_required",
        title: "需要目标小区",
        detail: "观察池中还没有可评估的小区。",
        href: "/neighborhoods",
        linkLabel: "添加目标小区",
      };
    case "metric_required":
      return {
        code: "metric_required",
        title: "需要市场数据",
        detail: "目标小区还没有当前算法版本的指标。",
        href: "/data",
        linkLabel: "前往数据管理",
      };
    case "metric_stale":
      return {
        code: "metric_stale",
        title: "指标已经过期",
        detail: "过期数据不能生成行动、信心或风险结论。",
        href: "/data",
        linkLabel: "补充最新数据",
      };
    case "metric_insufficient":
      return {
        code: "metric_insufficient",
        title: "市场数据不足",
        detail: "覆盖范围或成交样本不足，当前不能生成出手建议。",
        href: "/data",
        linkLabel: "补充完整数据",
      };
    default:
      return { code: "request_failed", title: "决策服务不可用", detail: "请求没有返回可用结论。" };
  }
}

function DecisionState({
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
    <section role="status" className={`flex min-h-28 flex-wrap items-center justify-between gap-4 border-l-4 px-5 py-5 ${toneClass}`}>
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
