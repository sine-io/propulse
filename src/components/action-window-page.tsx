import {
  recommendActionWindow,
  type ActionWindowInput,
} from "@/lib/decision";
import { actionWindowInput } from "@/lib/sample-data";

import { StatusBadge } from "./status-badge";

const matrix = [
  ["自身预算是否安全", "偏安全", "首付充裕但目标总价略高于安全线"],
  ["旧房置换回款风险", "需观察", "旧房看房量一般，回款节奏仍需锁定"],
  ["目标小区价格预期", "明显松动", "降价房源连续三周增加"],
  ["近期成交量支撑", "偏弱", "成交未放量，买家话语权高"],
  ["目标户型稀缺度", "中等", "同户型在售约 12 套，有挑选余地"],
  ["替代小区是否更优", "有备选", "已添加 2 个同板块可替代小区"],
];

export function ActionWindowPage({
  input = actionWindowInput,
}: {
  input?: ActionWindowInput;
}) {
  const recommendation = recommendActionWindow(input);

  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      <section className="rounded-[2rem] border border-slate-200 bg-white p-7 shadow-sm">
        <p className="text-sm font-bold text-slate-500">综合判断</p>
        <div className="mt-3 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-4xl font-black text-slate-950">
              现在适合看、等、砍价，还是出手？
            </h1>
            <p className="mt-4 max-w-4xl text-lg leading-8 text-slate-600">
              {recommendation.summary}
            </p>
          </div>
          <div className="rounded-[1.5rem] bg-slate-950 p-5 text-white">
            <p className="text-sm font-semibold text-slate-300">行动建议</p>
            <p className="mt-1 text-4xl font-black">{recommendation.action}</p>
            <p className="mt-2 text-sm text-slate-300">
              信心等级：{recommendation.confidence}
            </p>
          </div>
        </div>
      </section>

      <section className="mt-6 grid gap-6 lg:grid-cols-[1.12fr_0.88fr]">
        <div className="overflow-hidden rounded-[2rem] border border-slate-200 bg-white">
          <div className="border-b border-slate-100 bg-slate-50 px-6 py-4">
            <h2 className="text-xl font-black text-slate-950">
              决策影响因子矩阵
            </h2>
          </div>
          <div className="divide-y divide-slate-100">
            {matrix.map(([factor, status, desc]) => (
              <div
                key={factor}
                className="grid gap-3 p-5 sm:grid-cols-[1fr_auto]"
              >
                <div>
                  <p className="font-bold text-slate-900">{factor}</p>
                  <p className="mt-1 text-sm leading-6 text-slate-500">
                    {desc}
                  </p>
                </div>
                <StatusBadge
                  tone={
                    status === "明显松动" || status === "偏弱"
                      ? "emerald"
                      : status === "需观察" || status === "中等"
                        ? "amber"
                        : "blue"
                  }
                >
                  {status}
                </StatusBadge>
              </div>
            ))}
          </div>
        </div>

        <div className="space-y-5">
          <div className="rounded-[2rem] bg-slate-950 p-6 text-white">
            <h2 className="text-xl font-black">本周行动清单</h2>
            <ul className="mt-5 space-y-4">
              {recommendation.checklist.map((item) => (
                <li key={item} className="flex gap-3 text-sm leading-6">
                  <span className="mt-1 size-4 rounded border border-slate-600" />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </div>

          <div className="rounded-[2rem] border border-rose-100 bg-rose-50 p-6">
            <h2 className="text-xl font-black text-rose-900">风险提示</h2>
            <ul className="mt-4 space-y-3 text-sm leading-6 text-rose-800">
              {recommendation.risks.map((risk) => (
                <li key={risk}>· {risk}</li>
              ))}
              <li>· 如果旧房最终低于底线成交价，应立即重新测算安全总价。</li>
              <li>· 如果目标小区成交突然放量，砍价窗口可能收窄。</li>
            </ul>
          </div>
        </div>
      </section>
    </main>
  );
}
