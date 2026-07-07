import {
  evaluateNeighborhoodSignal,
  type NeighborhoodSignalInput,
} from "@/lib/decision";
import {
  alternateNeighborhoods,
  defaultNeighborhoodInput,
} from "@/lib/sample-data";

import { StatusBadge } from "./status-badge";

const trend = [20, 22, 25, 24, 30, 35, 38, 42];

export function NeighborhoodsPage({
  input = defaultNeighborhoodInput,
}: {
  input?: NeighborhoodSignalInput;
}) {
  const signal = evaluateNeighborhoodSignal(input);

  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      <section className="rounded-[2rem] border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-3xl font-black text-slate-950">
                {input.name}
              </h1>
              <StatusBadge tone="blue">滨江核心区</StatusBadge>
              <StatusBadge tone="slate">三房 90-110m2</StatusBadge>
            </div>
            <p className="mt-3 leading-7 text-slate-600">
              顶部先给结论，再给数据：当前重点看挂牌、成交、降价和供应压力是否同时支持行动。
            </p>
          </div>
          <div className="flex gap-3">
            <button className="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm font-bold text-slate-700">
              设置提醒
            </button>
            <button className="rounded-2xl bg-blue-600 px-4 py-3 text-sm font-bold text-white">
              加入观察池
            </button>
          </div>
        </div>
      </section>

      <section className="mt-6 rounded-[2rem] border border-blue-100 bg-gradient-to-br from-blue-50 to-white p-6">
        <p className="text-sm font-bold text-blue-600">当前判断</p>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h2 className="text-3xl font-black text-slate-950">
            {signal.status}
          </h2>
          <StatusBadge tone="emerald">买方窗口开启</StatusBadge>
        </div>
        <p className="mt-4 max-w-4xl text-lg leading-8 text-slate-700">
          挂牌量连续增加，降价房源变多，但成交偏弱未见放量，房东预期开始松动。
        </p>
        <div className="mt-5 rounded-3xl border border-white bg-white/70 p-5">
          <p className="font-black text-slate-950">下一步行动建议</p>
          <p className="mt-2 leading-7 text-slate-700">{signal.nextAction}</p>
        </div>
      </section>

      <section className="mt-6 grid gap-4 md:grid-cols-3 lg:grid-cols-6">
        {[
          ["当前挂牌价区间", input.listingPriceRange.join("-"), "万"],
          ["近90天成交区间", input.transactionPriceRange.join("-"), "万"],
          ["当前在售套数", String(input.listedHomes), "套"],
          ["本周降价房源", String(input.priceCutHomes), "套"],
          ["挂牌成交差", `${Math.round(signal.priceGapPct * 100)}`, "%"],
          ["平均成交周期", String(input.avgDaysOnMarket), "天"],
        ].map(([label, value, unit]) => (
          <div
            key={label}
            className="rounded-[1.5rem] border border-slate-200 bg-white p-4"
          >
            <p className="text-xs font-semibold text-slate-500">{label}</p>
            <p className="mt-2 text-2xl font-black text-slate-950">
              {value}
              <span className="ml-1 text-sm font-semibold text-slate-400">
                {unit}
              </span>
            </p>
          </div>
        ))}
      </section>

      <section className="mt-6 grid gap-6 lg:grid-cols-[1.25fr_0.75fr]">
        <div className="rounded-[2rem] border border-slate-200 bg-white p-6">
          <h2 className="text-xl font-black text-slate-950">
            挂牌量与降价趋势（近 8 周）
          </h2>
          <div className="mt-8 flex h-56 items-end gap-3 border-b border-slate-200 pb-3">
            {trend.map((value, index) => (
              <div key={index} className="flex flex-1 flex-col items-center">
                <div
                  className="w-full rounded-t-xl bg-blue-100"
                  style={{ height: `${(value / 50) * 100}%` }}
                />
                <span className="mt-2 text-xs font-semibold text-slate-400">
                  W{index + 1}
                </span>
              </div>
            ))}
          </div>
          <p className="mt-4 text-sm text-slate-500">
            蓝色柱表示在售套数，连续抬升时要关注降价占比是否同步上升。
          </p>
        </div>

        <div className="rounded-[2rem] border border-slate-200 bg-slate-50 p-6">
          <h2 className="text-xl font-black text-slate-950">判断依据拆解</h2>
          <ul className="mt-5 space-y-3">
            {signal.reasons.map((reason) => (
              <li
                key={reason}
                className="rounded-2xl border border-slate-200 bg-white p-4 text-sm leading-6 text-slate-700"
              >
                {reason}
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="mt-6 rounded-[2rem] border border-slate-200 bg-white p-6">
        <h2 className="text-xl font-black text-slate-950">替代小区对比</h2>
        <div className="mt-5 grid gap-4 lg:grid-cols-2">
          {alternateNeighborhoods.map((item) => (
            <article
              key={item.name}
              className="rounded-[1.5rem] border border-slate-200 bg-slate-50 p-5"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h3 className="text-lg font-black text-slate-950">
                    {item.name}
                  </h3>
                  <p className="mt-1 text-sm text-slate-500">
                    {item.area} · {item.layout}
                  </p>
                </div>
                <StatusBadge tone={item.status === "重点看" ? "blue" : "amber"}>
                  {item.status}
                </StatusBadge>
              </div>
              <div className="mt-4 grid grid-cols-3 gap-3 text-sm">
                <Metric label="在售" value={`${item.listedHomes}套`} />
                <Metric label="降价" value={`${item.priceCutHomes}套`} />
                <Metric label="成交" value={item.transaction} />
              </div>
              <p className="mt-4 rounded-2xl bg-white p-3 text-sm leading-6 text-slate-700">
                {item.advice}
              </p>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs font-semibold text-slate-400">{label}</p>
      <p className="mt-1 font-black text-slate-900">{value}</p>
    </div>
  );
}
