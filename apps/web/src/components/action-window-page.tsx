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
import {
  ApiError,
  getActionWindow,
  getWatchlist,
  type ActionWindowResponse,
  type WatchlistItem,
} from "@/lib/api-client";
import { StatusBadge } from "./status-badge";

type DecisionFactor = ActionWindowResponse["factors"][number];
type DecisionFactorEvidence = DecisionFactor["evidence"][number];
type AlternativeComparison = ActionWindowResponse["alternativeComparison"];
type AlternativeCandidate = AlternativeComparison["candidates"][number];

type AccessState = "checking" | "locked" | "unlocked";
type WatchlistState = "idle" | "loading" | "ready" | "failed";
type RecommendationState = "idle" | "loading" | "ready" | "blocked";

type BlockedState = {
  code:
    | "capacity_required"
    | "watchlist_required"
    | "neighborhood_not_watched"
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
  const [watchlistState, setWatchlistState] = useState<WatchlistState>("idle");
  const [watchlist, setWatchlist] = useState<WatchlistItem[]>([]);
  const [selectedNeighborhoodID, setSelectedNeighborhoodID] = useState("");
  const [rejectedNeighborhoodID, setRejectedNeighborhoodID] = useState<string>();
  const [recommendationState, setRecommendationState] = useState<RecommendationState>("idle");
  const [recommendation, setRecommendation] = useState<ActionWindowResponse>();
  const [blocked, setBlocked] = useState<BlockedState>();
  const [checkedItems, setCheckedItems] = useState<string[]>([]);
  const [watchlistRequestVersion, setWatchlistRequestVersion] = useState(0);
  const [recommendationRequestVersion, setRecommendationRequestVersion] = useState(0);

  const selectedWatchlistItem = watchlist.find(
    (item) => item.neighborhoodId === selectedNeighborhoodID,
  );
  const selectionWasRejected = rejectedNeighborhoodID === selectedNeighborhoodID;
  const hasValidSelection = Boolean(selectedWatchlistItem) && !selectionWasRejected;

  useEffect(() => {
    const syncAccess = () => setAccessState(getAccessToken() ? "unlocked" : "locked");
    syncAccess();
    return subscribeToAccessToken(syncAccess);
  }, []);

  useEffect(() => {
    const syncSelection = () => {
      setSelectedNeighborhoodID(readNeighborhoodIDFromURL());
      setRecommendation(undefined);
      setBlocked(undefined);
      setCheckedItems([]);
      setRejectedNeighborhoodID(undefined);
      setRecommendationState("idle");
      setRecommendationRequestVersion((version) => version + 1);
    };
    syncSelection();
    window.addEventListener("popstate", syncSelection);
    return () => window.removeEventListener("popstate", syncSelection);
  }, []);

  useEffect(() => {
    if (accessState !== "unlocked") {
      setWatchlist([]);
      setWatchlistState("idle");
      setRecommendation(undefined);
      setBlocked(undefined);
      setCheckedItems([]);
      setRecommendationState("idle");
      return;
    }

    const controller = new AbortController();
    let active = true;
    setWatchlist([]);
    setWatchlistState("loading");
    setRecommendation(undefined);
    setBlocked(undefined);
    setCheckedItems([]);
    setRecommendationState("idle");
    getWatchlist(controller.signal)
      .then((response) => {
        if (!active) return;
        setWatchlist(response.items);
        setWatchlistState("ready");
      })
      .catch((error: unknown) => {
        if (!active || isAbortError(error)) return;
        if (error instanceof ApiError && error.status === 401) {
          setAccessState("locked");
          return;
        }
        setWatchlistState("failed");
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [accessState, watchlistRequestVersion]);

  useEffect(() => {
    if (
      accessState !== "unlocked" ||
      watchlistState !== "ready" ||
      !selectedWatchlistItem ||
      selectionWasRejected
    ) {
      return;
    }

    const controller = new AbortController();
    let active = true;
    setRecommendation(undefined);
    setBlocked(undefined);
    setCheckedItems([]);
    setRecommendationState("loading");
    getActionWindow(selectedNeighborhoodID, controller.signal)
      .then((response) => {
        if (!active) return;
        if (response.target.neighborhoodId !== selectedNeighborhoodID) {
          setBlocked({
            code: "request_failed",
            title: "决策目标不一致",
            detail: "服务返回的目标与当前选择不一致。",
          });
          setRecommendationState("blocked");
          return;
        }
        setRecommendation(response);
        setCheckedItems([]);
        setRecommendationState("ready");
      })
      .catch((error: unknown) => {
        if (!active || isAbortError(error)) return;
        setRecommendation(undefined);
        if (error instanceof ApiError && error.status === 401) {
          setAccessState("locked");
          return;
        }
        if (error instanceof ApiError && error.code === "neighborhood_not_watched") {
          setRejectedNeighborhoodID(selectedNeighborhoodID);
          setRecommendationState("idle");
          return;
        }
        setBlocked(actionWindowBlockedState(error));
        setRecommendationState("blocked");
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [
    accessState,
    recommendationRequestVersion,
    selectedNeighborhoodID,
    selectedWatchlistItem,
    selectionWasRejected,
    watchlistState,
  ]);

  const selectNeighborhood = (neighborhoodID: string) => {
    const url = new URL(window.location.href);
    if (neighborhoodID) {
      url.searchParams.set("neighborhoodId", neighborhoodID);
    } else {
      url.searchParams.delete("neighborhoodId");
    }
    window.history.pushState({}, "", `${url.pathname}${url.search}${url.hash}`);
    setRecommendation(undefined);
    setBlocked(undefined);
    setCheckedItems([]);
    setRejectedNeighborhoodID(undefined);
    setRecommendationState("idle");
    setSelectedNeighborhoodID(neighborhoodID);
  };

  const toggleChecklistItem = (item: string) => {
    setCheckedItems((current) =>
      current.includes(item)
        ? current.filter((value) => value !== item)
        : [...current, item],
    );
  };

  const displayedTarget = hasValidSelection
    ? recommendationState === "ready" && recommendation
      ? recommendation.target
      : selectedWatchlistItem
    : undefined;
  const displayedCollectedAt = recommendationState === "ready" && recommendation
    ? recommendation.metric.collectedAt
    : selectedWatchlistItem?.collectedAt;
  const selectionIsMissing = selectedNeighborhoodID === "";
  const selectionIsUnavailable = !selectionIsMissing && !hasValidSelection;

  return (
    <main className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6 lg:px-8">
      <section className="mb-8">
        <h1 className="text-3xl font-bold text-slate-900">
          {displayedTarget ? `${displayedTarget.name}出手窗口` : "现在适合看、等、砍价，还是出手？"}
        </h1>
        <p className="mt-2 text-slate-500">
          {displayedTarget ? (
            <>
              {displayedTarget.area} · {displayedTarget.targetLayout}
              {displayedCollectedAt ? ` · 指标采集 ${formatDecisionTimestamp(displayedCollectedAt)}` : ""}
            </>
          ) : (
            "结合资金预算与目标小区的可信市场指标生成决策。"
          )}
        </p>
      </section>

      {accessState === "unlocked" && watchlistState === "ready" && watchlist.length > 0 ? (
        <section className="border-y border-slate-200 py-5">
          <label htmlFor="action-window-neighborhood" className="block text-sm font-semibold text-slate-900">
            目标小区
          </label>
          <select
            id="action-window-neighborhood"
            value={hasValidSelection ? selectedNeighborhoodID : ""}
            onChange={(event) => selectNeighborhood(event.target.value)}
            className="mt-2 h-11 w-full max-w-xl rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
          >
            <option value="">请选择已关注小区</option>
            {watchlist.map((item) => {
              const responseTarget = recommendation?.target.neighborhoodId === item.neighborhoodId
                ? recommendation.target
                : undefined;
              return (
                <option key={item.neighborhoodId} value={item.neighborhoodId}>
                  {responseTarget?.name ?? item.name} · {responseTarget?.targetLayout ?? item.targetLayout}
                </option>
              );
            })}
          </select>
          {displayedTarget ? (
            <p className="mt-2 text-xs text-slate-500">
              {displayedTarget.name} · {displayedTarget.targetLayout}
              {displayedCollectedAt ? ` · 指标采集 ${formatDecisionTimestamp(displayedCollectedAt)}` : ""}
            </p>
          ) : null}
        </section>
      ) : null}

      {accessState === "checking" || (accessState === "unlocked" && watchlistState === "loading") ? (
        <DecisionState icon={Database} title="正在读取观察池" tone="slate" />
      ) : null}
      {accessState === "locked" ? (
        <DecisionState
          icon={LockKeyhole}
          title="出手窗口已锁定"
          detail="解锁个人空间后才能读取测算、观察池和市场指标。"
          tone="amber"
        />
      ) : null}
      {accessState === "unlocked" && watchlistState === "failed" ? (
        <DecisionState
          icon={AlertTriangle}
          title="观察池读取失败"
          detail="请求没有返回可用的观察池。"
          tone="rose"
          action={(
            <button
              type="button"
              onClick={() => setWatchlistRequestVersion((version) => version + 1)}
              className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              <RefreshCw aria-hidden="true" className="h-4 w-4" />
              重试
            </button>
          )}
        />
      ) : null}
      {accessState === "unlocked" && watchlistState === "ready" && watchlist.length === 0 ? (
        <DecisionState
          icon={MapPin}
          title="观察池暂无小区"
          detail="添加目标小区后才能生成出手建议。"
          tone="amber"
          action={(
            <Link
              href="/neighborhoods"
              className="inline-flex h-9 items-center rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800"
            >
              添加目标小区
            </Link>
          )}
        />
      ) : null}
      {accessState === "unlocked" && watchlistState === "ready" && watchlist.length > 0 && selectionIsMissing ? (
        <DecisionState
          icon={MapPin}
          title="选择目标小区"
          detail="当前尚未指定本次评估目标。"
          tone="slate"
        />
      ) : null}
      {accessState === "unlocked" && watchlistState === "ready" && watchlist.length > 0 && selectionIsUnavailable ? (
        <DecisionState
          icon={AlertTriangle}
          title="目标已不在观察池"
          detail="原目标不可用于本次评估，请重新选择。"
          tone="amber"
        />
      ) : null}
      {accessState === "unlocked" && watchlistState === "ready" && hasValidSelection && recommendationState === "loading" ? (
        <DecisionState icon={Database} title="正在检查出手窗口" tone="slate" />
      ) : null}
      {accessState === "unlocked" && recommendationState === "blocked" && blocked ? (
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
                onClick={() => setRecommendationRequestVersion((version) => version + 1)}
                className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                <RefreshCw aria-hidden="true" className="h-4 w-4" />
                重试
              </button>
            </div>
          }
        />
      ) : null}

      {recommendationState === "ready" && recommendation ? (
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

          <AlternativeComparisonSection comparison={recommendation.alternativeComparison} />

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

function AlternativeComparisonSection({ comparison }: { comparison: AlternativeComparison }) {
  return (
    <section>
      <div className="flex flex-wrap items-end justify-between gap-3 border-b border-slate-200 pb-4">
        <div>
          <h2 className="text-xl font-bold text-slate-900">可比备选明细</h2>
          <p className="mt-1 text-sm text-slate-500">
            规则 {comparison.ruleVersion} · 参考采集 {formatDecisionTimestamp(comparison.referenceCollectedAt)} · 安全总价 {formatNumber(comparison.safeTotalPrice)} 万元
          </p>
        </div>
        <StatusBadge tone={comparisonTone[comparison.status]}>
          {comparisonStatusCopy[comparison.status]}
        </StatusBadge>
      </div>
      {comparison.candidates.length === 0 ? (
        <p className="py-6 text-sm text-slate-500">观察池中没有其他候选小区。</p>
      ) : (
        <div>
          {comparison.candidates.map((candidate) => (
            <AlternativeCandidateRow key={candidate.neighborhoodId} candidate={candidate} />
          ))}
        </div>
      )}
    </section>
  );
}

function AlternativeCandidateRow({ candidate }: { candidate: AlternativeCandidate }) {
  return (
    <article className="border-b border-slate-200 py-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <Link
            href={`/neighborhoods?id=${encodeURIComponent(candidate.neighborhoodId)}`}
            className="font-semibold text-blue-700 hover:underline"
          >
            {candidate.name}
          </Link>
          <p className="mt-1 text-xs text-slate-500">
            {candidate.area} · {candidate.targetLayout}
          </p>
        </div>
        <StatusBadge tone={candidateTone[candidate.status]}>
          {candidateStatusCopy[candidate.status]}
        </StatusBadge>
      </div>

      <dl className="mt-4 grid grid-cols-2 gap-x-5 gap-y-4 text-sm lg:grid-cols-4">
        <ComparisonValue label="成交中位价" value={formatPriceDifference(candidate)} />
        <ComparisonValue label="市场信号" value={formatSignalDifference(candidate)} />
        <ComparisonValue label="目标户型供给" value={formatSupplyDifference(candidate)} />
        <ComparisonValue label="预算约束" value={formatBudgetStatus(candidate.withinBudget)} />
      </dl>

      <div className="mt-4 flex flex-wrap gap-x-5 gap-y-2 text-xs text-slate-600">
        <span>改善：{formatDimensions(candidate.improvements)}</span>
        <span>劣化：{formatDimensions(candidate.deteriorations)}</span>
      </div>
      <ul className="mt-3 flex flex-wrap gap-2 text-xs text-slate-600">
        {candidate.reasons.map((reason) => (
          <li key={reason} className="border-l-2 border-slate-300 pl-2">
            {alternativeReasonCopy[reason]}
          </li>
        ))}
      </ul>

      <div className="mt-4 text-xs text-slate-500">
        {candidate.metric ? (
          <Link
            href={`/data/imports/${encodeURIComponent(candidate.metric.collectionRunId)}`}
            className="inline-flex items-center gap-1.5 text-blue-700 hover:underline"
          >
            <span>
              候选批次 {shortReference(candidate.metric.collectionRunId)} ·{" "}
              {formatDecisionTimestamp(candidate.metric.collectedAt)}
            </span>
            <ExternalLink aria-hidden="true" className="h-3.5 w-3.5" />
          </Link>
        ) : (
          <span>无合格指标来源</span>
        )}
      </div>
    </article>
  );
}

function ComparisonValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className="mt-1 break-words font-medium text-slate-900">{value}</dd>
    </div>
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

const comparisonStatusCopy: Record<AlternativeComparison["status"], string> = {
  better_found: "发现更优备选",
  none: "未发现更优备选",
  unknown: "备选证据不足",
};

const comparisonTone: Record<
  AlternativeComparison["status"],
  "emerald" | "slate" | "amber"
> = {
  better_found: "emerald",
  none: "slate",
  unknown: "amber",
};

const candidateStatusCopy: Record<AlternativeCandidate["status"], string> = {
  better: "更优",
  not_better: "未达更优门槛",
  unknown: "数据不足",
};

const candidateTone: Record<
  AlternativeCandidate["status"],
  "emerald" | "slate" | "amber"
> = {
  better: "emerald",
  not_better: "slate",
  unknown: "amber",
};

const dimensionCopy: Record<AlternativeCandidate["improvements"][number], string> = {
  transaction_price: "成交价",
  market_signal: "市场信号",
  target_layout_supply: "户型供给",
};

const alternativeReasonCopy: Record<AlternativeCandidate["reasons"][number], string> = {
  layout_mismatch: "目标户型不一致",
  metric_missing: "缺少当前算法指标",
  algorithm_version_mismatch: "指标算法版本不一致",
  coverage_not_full: "不是完整覆盖批次",
  metric_not_current: "指标不在当前新鲜度",
  metric_quality_insufficient: "指标质量不足",
  transaction_evidence_insufficient: "成交样本少于 3 笔",
  comparison_window_mismatch: "采集时间相差超过 7 天",
  transaction_price_missing: "缺少成交价格区间",
  target_evidence_insufficient: "目标小区证据不足",
  signal_not_comparable: "市场信号不可比较",
  over_budget: "候选成交中位价超出安全总价",
  insufficient_improvements: "实质改善少于两项",
  deterioration_present: "至少一项指标明显劣化",
  better_threshold_met: "预算内、至少两项改善且无劣化",
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
  better_found: "发现更优备选",
  complete: "可追溯",
  current: "当前",
  danger: "危险",
  full: "完整覆盖",
  high: "高",
  low: "低",
  medium: "中",
  none: "未发现更优备选",
  safe: "安全",
  stable: "平稳",
  strained: "承压",
  strong: "活跃",
  sufficient: "充分",
  unknown: "未知",
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

function formatPriceDifference(candidate: AlternativeCandidate): string {
  if (
    candidate.targetTransactionPriceMidpoint == null ||
    candidate.candidateTransactionPriceMidpoint == null ||
    candidate.priceDifference == null ||
    candidate.priceDifferencePct == null
  ) {
    return "不可比";
  }
  return `${formatNumber(candidate.targetTransactionPriceMidpoint)} → ${formatNumber(candidate.candidateTransactionPriceMidpoint)} 万元（${formatSigned(candidate.priceDifference)} / ${formatSigned(candidate.priceDifferencePct)}%）`;
}

function formatSignalDifference(candidate: AlternativeCandidate): string {
  if (!candidate.targetSignal || !candidate.candidateSignal || candidate.signalRankDifference == null) {
    return "不可比";
  }
  return `${candidate.targetSignal} → ${candidate.candidateSignal}（${formatSigned(candidate.signalRankDifference)} 级）`;
}

function formatSupplyDifference(candidate: AlternativeCandidate): string {
  if (candidate.candidateTargetLayoutSupply == null || candidate.supplyDifference == null) {
    return "不可比";
  }
  const percentage = candidate.supplyDifferencePct == null
    ? "基线为 0"
    : `${formatSigned(candidate.supplyDifferencePct)}%`;
  return `${candidate.targetLayoutSupply} → ${candidate.candidateTargetLayoutSupply} 套（${formatSigned(candidate.supplyDifference)} / ${percentage}）`;
}

function formatBudgetStatus(withinBudget: boolean | null): string {
  if (withinBudget == null) return "不可判断";
  return withinBudget ? "在安全总价内" : "超出安全总价";
}

function formatDimensions(dimensions: AlternativeCandidate["improvements"]): string {
  if (dimensions.length === 0) return "无";
  return dimensions.map((dimension) => dimensionCopy[dimension]).join("、");
}

function formatSigned(value: number): string {
  const formatted = formatNumber(value);
  return value > 0 ? `+${formatted}` : formatted;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value);
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

function readNeighborhoodIDFromURL(): string {
  return new URLSearchParams(window.location.search).get("neighborhoodId")?.trim() ?? "";
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
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
        detail: "请从观察池中明确选择本次评估目标。",
        href: "/watchlist",
        linkLabel: "查看观察池",
      };
    case "neighborhood_not_watched":
      return {
        code: "neighborhood_not_watched",
        title: "目标已不在观察池",
        detail: "原目标不可用于本次评估，请重新选择。",
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
