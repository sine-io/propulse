"use client";

import { CheckCircle, XCircle } from "lucide-react";
import { useEffect, useState } from "react";

import {
  ApiError,
  getActionWindow,
  type ActionWindowResponse,
} from "@/lib/api-client";
import { StatusBadge } from "./status-badge";

const matrix = [
  ["自身预算是否安全", "偏安全", "green", "首付充裕，月供压力可控"],
  ["旧房置换回款风险", "需观察", "yellow", "旧房挂牌中，看房量一般"],
  ["目标小区价格预期", "明显松动", "green", "降价房源连续三周增加"],
  ["近期成交量支撑", "偏弱", "green", "成交未放量，买家话语权高"],
  ["目标户型稀缺度", "中等", "yellow", "同户型在售约 12 套，有挑选余地"],
  ["是否存在备选小区", "有备选", "green", "已添加 2 个同板块可替代小区"],
] as const;

const fallbackActionWindow: ActionWindowResponse = {
  action: "砍价",
  confidence: "高",
  summary:
    "你的预算目前处于偏安全区间。目标小区（青枫花园）供应增加且降价房源变多，属于典型的买方窗口初期。当前更适合广泛看房、试探底价，但不建议追高或不还价直接成交。",
  checklist: [
    "约看 3 套总价在 500-530 万以内的三房。",
    "对挂牌时间超过 60 天的 2 套房源，尝试让中介报价砍 5%-8%。",
    "检视自己旧房的挂牌价，如果看房量少于每周 2 组，考虑下调 2%。",
  ],
  risks: [
    "如果旧房最终低于 310 万成交，建议立即返回首页重新测算安全总价。",
    "如果目标小区成交量突然连续两周放大，砍价窗口可能随时关闭。",
    "单套极低价房源可能是“钓鱼房”或有硬伤，不代表小区整体见底。",
  ],
};

export function ActionWindowPage() {
  const [recommendation, setRecommendation] =
    useState<ActionWindowResponse>(fallbackActionWindow);
  const [isFallback, setIsFallback] = useState(true);

  useEffect(() => {
    const controller = new AbortController();

    getActionWindow(controller.signal)
      .then((response) => {
        setRecommendation(response);
        setIsFallback(false);
      })
      .catch((error: unknown) => {
        if (
          error instanceof DOMException &&
          error.name === "AbortError"
        ) {
          return;
        }

        if (error instanceof ApiError && error.code === "capacity_required") {
          setRecommendation(fallbackActionWindow);
          setIsFallback(true);
          return;
        }

        setRecommendation(fallbackActionWindow);
        setIsFallback(true);
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
          结合你的资金预算与目标小区的市场行情，生成的动态决策建议。
        </p>
      </section>

      <section className="rounded-2xl border-l-4 border-l-blue-600 bg-white p-8 shadow-md">
        <div className="mb-4 flex flex-col justify-between md:flex-row md:items-center">
          <div>
            <p className="mb-1 text-sm font-semibold uppercase text-slate-500">
              当前核心策略
            </p>
            <h2 className="text-3xl font-bold text-slate-900">
              {isFallback ? "积极看房，大胆砍价" : `建议${recommendation.action}`}
            </h2>
          </div>
          <div className="mt-4 md:mt-0 md:text-right">
            <p className="mb-1 text-sm font-semibold uppercase text-slate-500">
              策略执行信心
            </p>
            <StatusBadge tone="emerald">
              {isFallback ? "中高 (75%)" : recommendation.confidence}
            </StatusBadge>
          </div>
        </div>
        <p className="rounded-xl border border-slate-100 bg-slate-50 p-4 text-lg leading-relaxed text-slate-700">
          {isFallback ? (
            <>
              你的预算目前处于偏安全区间。目标小区（青枫花园）供应增加且降价房源变多，属于典型的
              <strong className="text-blue-700">买方窗口初期</strong>
              。当前更适合广泛看房、试探底价，
              <strong>但不建议追高或不还价直接成交</strong>。
            </>
          ) : (
            recommendation.summary
          )}
        </p>
      </section>

      <section className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white lg:col-span-2">
          <div className="border-b border-slate-100 bg-slate-50 p-5">
            <h3 className="font-bold text-slate-800">决策影响因子矩阵</h3>
          </div>
          <div className="divide-y divide-slate-100">
            {matrix.map(([factor, status, color, desc]) => (
              <div
                key={factor}
                className="flex items-center justify-between p-4 transition-colors hover:bg-slate-50"
              >
                <div className="flex-1">
                  <p className="font-medium text-slate-800">{factor}</p>
                  <p className="mt-1 text-xs text-slate-500">{desc}</p>
                </div>
                <div className="ml-4 w-24 flex-shrink-0 text-right">
                  <span
                    className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      color === "green"
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-amber-100 text-amber-800"
                    }`}
                  >
                    <span
                      className={`mr-1.5 h-1.5 w-1.5 rounded-full ${
                        color === "green" ? "bg-emerald-500" : "bg-amber-500"
                      }`}
                    />
                    {status}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <aside className="space-y-6">
          <section className="rounded-2xl bg-slate-900 p-6 text-white shadow-lg">
            <h3 className="mb-4 flex items-center text-lg font-bold">
              <CheckCircle aria-hidden="true" className="h-5 w-5" />
              <span className="ml-2">本周行动清单</span>
            </h3>
            <ul className="space-y-4 text-sm text-slate-300">
              {recommendation.checklist.map((item) => (
                <li key={item} className="flex items-start">
                  <span className="mr-3 mt-0.5 h-5 w-5 flex-shrink-0 rounded border border-slate-600" />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </section>

          <section className="rounded-2xl border border-rose-100 bg-rose-50 p-6">
            <h3 className="mb-3 flex items-center text-sm font-bold uppercase tracking-wide text-rose-800">
              <XCircle aria-hidden="true" className="h-5 w-5" />
              <span className="ml-2">风险警示</span>
            </h3>
            <ul className="list-inside list-disc space-y-2 text-sm text-rose-700/80">
              {recommendation.risks.map((risk) => (
                <li key={risk}>{risk}</li>
              ))}
            </ul>
          </section>
        </aside>
      </section>
    </main>
  );
}
