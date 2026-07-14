"use client";

import Link from "next/link";
import {
  AlertTriangle,
  CheckCircle,
  Database,
  LockKeyhole,
  RefreshCw,
} from "lucide-react";
import { useEffect, useState } from "react";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import { ApiError, getActionWindow, type ActionWindowResponse } from "@/lib/api-client";
import { StatusBadge } from "./status-badge";

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
