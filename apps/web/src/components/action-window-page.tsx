"use client";

import { AlertTriangle, CheckCircle } from "lucide-react";
import { useEffect, useState } from "react";

import {
  ApiError,
  getActionWindow,
  type ActionWindowResponse,
} from "@/lib/api-client";
import { StatusBadge } from "./status-badge";

export function ActionWindowPage() {
  const [recommendation, setRecommendation] =
    useState<ActionWindowResponse>();
  const [errorMessage, setErrorMessage] = useState<string>();

  useEffect(() => {
    const controller = new AbortController();

    getActionWindow(controller.signal)
      .then((response) => {
        setRecommendation(response);
        setErrorMessage(undefined);
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setRecommendation(undefined);
        setErrorMessage(actionWindowErrorMessage(error));
      });

    return () => controller.abort();
  }, []);

  return (
    <main className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6 lg:px-8">
      <section className="mb-8">
        <h1 className="text-3xl font-bold text-slate-900">
          现在适合看、等、砍价，还是出手？
        </h1>
        <p className="mt-2 text-slate-500">
          结合你的资金预算与目标小区的市场行情，生成动态决策建议。
        </p>
      </section>

      {errorMessage ? (
        <section className="border-l-4 border-amber-500 bg-amber-50 px-5 py-4 text-amber-950">
          <h2 className="font-semibold">暂时无法生成出手窗口</h2>
          <p className="mt-1 text-sm text-amber-800">{errorMessage}</p>
        </section>
      ) : null}

      {recommendation ? (
        <>
          <section className="border-l-4 border-l-blue-600 bg-white p-6 shadow-sm sm:p-8">
            <div className="mb-4 flex flex-col justify-between gap-4 md:flex-row md:items-center">
              <div>
                <p className="mb-1 text-sm font-semibold uppercase text-slate-500">
                  当前核心策略
                </p>
                <h2 className="text-3xl font-bold text-slate-900">
                  建议{recommendation.action}
                </h2>
              </div>
              <div className="md:text-right">
                <p className="mb-1 text-sm font-semibold uppercase text-slate-500">
                  策略信心
                </p>
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
                {recommendation.checklist.map((item) => (
                  <li key={item} className="flex items-start gap-3">
                    <CheckCircle aria-hidden="true" className="mt-0.5 h-4 w-4 flex-none text-emerald-400" />
                    <span>{item}</span>
                  </li>
                ))}
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

function actionWindowErrorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) {
    return "决策服务暂时不可用。";
  }
  switch (error.code) {
    case "access_required":
      return "个人空间尚未解锁。";
    case "capacity_required":
      return "请先完成并保存一次换房测算。";
    case "watchlist_required":
      return "请先向观察池添加目标小区。";
    case "metric_required":
      return "目标小区还没有可用的市场数据。";
    default:
      return "决策服务暂时不可用。";
  }
}
