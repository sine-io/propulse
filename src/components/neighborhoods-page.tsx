import {
  Activity,
  AlertCircle,
  Bell,
  BookOpen,
  Plus,
  Target,
  TrendingUp,
  type LucideIcon,
} from "lucide-react";
import type { CSSProperties } from "react";

import {
  type NeighborhoodSignalInput,
} from "@/lib/decision";
import { defaultNeighborhoodInput } from "@/lib/sample-data";

import { StatusBadge } from "./status-badge";

const trend = [20, 22, 25, 24, 30, 35, 38, 42];
const metrics: Array<[string, string, string, string, boolean]> = [
  ["当前挂牌价区间", "520-620", "万", "较上月 -2%", false],
  ["近90天成交区间", "495-545", "万", "价差约 8.5%", false],
  ["当前在售套数", "42", "套", "较上月 +18%", true],
  ["本周降价房源", "11", "套", "占比 26%", true],
  ["平均成交周期", "78", "天", "逐渐拉长", false],
  ["带看转定率", "2.4", "%", "成交偏弱", false],
];
const reasons: Array<[string, string, LucideIcon]> = [
  ["1. 供应显著增加", "在售套数连续 4 周攀升，突破历史均值。", AlertCircle],
  [
    "2. 降价意愿增强",
    "本周 11 套降价，降价房源占比提升，说明部分房东急售。",
    AlertCircle,
  ],
  ["3. 成交尚未放量", "买方仍在观望，价格支撑力度弱。", Activity],
];

export function NeighborhoodsPage({
  input = defaultNeighborhoodInput,
}: {
  input?: NeighborhoodSignalInput;
}) {
  return (
    <main className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6 lg:px-8">
      <section className="flex flex-col items-start justify-between rounded-2xl border border-slate-200 bg-white p-6 shadow-sm md:flex-row md:items-center">
        <div>
          <div className="mb-2 flex items-center space-x-3">
            <h1 className="text-2xl font-bold text-slate-900">{input.name}</h1>
            <StatusBadge tone="blue">滨江核心区</StatusBadge>
            <span className="text-sm text-slate-500">关注户型: 三房 90-110㎡</span>
          </div>
          <p className="text-sm text-slate-500">更新时间: 今天 10:30</p>
        </div>
        <div className="mt-4 flex space-x-3 md:mt-0">
          <button className="flex items-center space-x-2 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
            <Bell aria-hidden="true" className="h-4 w-4" />
            <span>降价提醒</span>
          </button>
          <button className="flex items-center space-x-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
            <Plus aria-hidden="true" className="h-4 w-4" />
            <span>加入观察池</span>
          </button>
        </div>
      </section>

      <section className="relative overflow-hidden rounded-2xl border border-blue-100 bg-gradient-to-r from-blue-50 to-white p-6">
        <Target
          aria-hidden="true"
          className="absolute right-6 top-6 h-32 w-32 text-blue-500 opacity-10"
        />
        <h2 className="mb-2 text-sm font-bold uppercase tracking-wider text-blue-600">
          综合研判结论
        </h2>
        <div className="mb-4 flex items-center space-x-3">
          <span className="text-2xl font-bold text-slate-900">适合试探性砍价</span>
          <StatusBadge tone="emerald">买方窗口开启</StatusBadge>
        </div>
        <p className="mb-4 max-w-3xl text-lg text-slate-700">
          挂牌量连续 4 周增加，降价房源变多，但成交偏弱未见放量，房东预期开始松动。
        </p>
        <div className="max-w-3xl rounded-xl border border-white bg-white/60 p-4 backdrop-blur-sm">
          <p className="mb-1 font-semibold text-slate-900">下一步行动建议：</p>
          <p className="text-slate-700">
            可以开始密集看房，但不急下定。重点关注总价{" "}
            <span className="font-bold text-blue-700">500-530万</span>{" "}
            以内的三房，针对挂牌超过 60 天的房源尝试 5%-8% 的砍价空间。
          </p>
        </div>
      </section>

      <section className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-6">
        {metrics.map(([label, value, unit, sub, alert]) => (
          <div key={label} className="rounded-xl border border-slate-200 bg-white p-4">
            <p className="mb-2 text-xs text-slate-500">{label}</p>
            <p className="flex items-baseline text-xl font-bold text-slate-900">
              {value}
              <span className="ml-1 text-xs font-normal text-slate-500">{unit}</span>
            </p>
            <p
              className={`mt-2 text-xs font-medium ${
                alert ? "flex items-center text-amber-600" : "text-slate-400"
              }`}
            >
              {alert ? <TrendingUp aria-hidden="true" className="mr-1 h-3 w-3" /> : null}
              {sub}
            </p>
          </div>
        ))}
      </section>

      <section className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="rounded-2xl border border-slate-200 bg-white p-6 lg:col-span-2">
          <h2 className="mb-6 font-bold text-slate-800">
            挂牌量与降价趋势 (近8周)
          </h2>
          <div className="relative flex h-48 items-end justify-between space-x-2 border-b border-slate-100 pb-2">
            <div className="absolute bottom-2 left-0 top-0 flex flex-col justify-between text-[10px] text-slate-400">
              <span>50套</span>
              <span>25套</span>
              <span>0</span>
            </div>
            <div className="flex h-full w-full items-end justify-between pl-8">
              {trend.map((value, index) => (
                <div key={index} className="group flex h-full w-8 flex-col items-center">
                  <div className="relative flex h-full w-full items-end justify-center">
                    <div
                      className="bar-grow relative w-full rounded-t-sm bg-blue-100 group-hover:bg-blue-200"
                      style={{ "--h": `${(value / 50) * 100}%` } as CSSProperties}
                    >
                      <div
                        className="absolute -top-1 left-1/2 z-10 h-2 w-2 -translate-x-1/2 rounded-full bg-amber-500"
                        style={{ bottom: `${index * 2 + 5}px` }}
                      />
                    </div>
                  </div>
                  <span className="mt-2 text-[10px] text-slate-400">W{index + 1}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="mt-4 flex justify-center space-x-6 text-xs text-slate-500">
            <span className="flex items-center">
              <span className="mr-2 h-3 w-3 rounded-sm bg-blue-100" />
              在售套数
            </span>
            <span className="flex items-center">
              <span className="mr-2 h-2 w-2 rounded-full bg-amber-500" />
              降价频次趋势
            </span>
          </div>
        </div>

        <aside className="rounded-2xl border border-slate-200 bg-slate-50 p-6">
          <h2 className="mb-4 flex items-center font-bold text-slate-800">
            <BookOpen aria-hidden="true" className="h-5 w-5" />
            <span className="ml-2">判断依据拆解</span>
          </h2>
          <ul className="space-y-4">
            {reasons.map(([title, body, Icon]) => (
              <li key={title} className="flex items-start">
                <Icon aria-hidden="true" className="mr-3 mt-0.5 h-5 w-5 text-amber-500" />
                <div>
                  <p className="text-sm font-semibold text-slate-800">{title}</p>
                  <p className="mt-1 text-xs text-slate-600">{body}</p>
                </div>
              </li>
            ))}
          </ul>
        </aside>
      </section>
    </main>
  );
}
