import Link from "next/link";

import { alternateNeighborhoods } from "@/lib/sample-data";

import { StatusBadge } from "./status-badge";

const primary = {
  name: "青枫花园",
  area: "滨江核心",
  layout: "三房",
  status: "适合砍价",
  listedHomes: 42,
  priceCutHomes: 11,
  transaction: "偏弱",
  advice: "约看 500-530 万三房，尝试砍价，窗口期已打开。",
};

export function WatchlistPage() {
  const items = [primary, ...alternateNeighborhoods];

  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      <section className="flex flex-col gap-5 rounded-[2rem] border border-slate-200 bg-white p-7 shadow-sm lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-4xl font-black text-slate-950">我的观察池</h1>
          <p className="mt-4 max-w-3xl text-lg leading-8 text-slate-600">
            每周跟踪关注小区、提醒规则和决策记录，形成稳定复盘习惯。
          </p>
        </div>
        <Link
          href="/templates"
          className="rounded-2xl bg-slate-950 px-5 py-3 text-sm font-bold text-white"
        >
          导出周监测表
        </Link>
      </section>

      <section className="mt-6 grid gap-4 md:grid-cols-4">
        {[
          ["本周关注小区", "3 个"],
          ["出现降价信号", "2 个"],
          ["进入可砍价窗口", "1 个"],
          ["价格仍偏硬", "1 个"],
        ].map(([label, value]) => (
          <div
            key={label}
            className="rounded-[1.5rem] border border-slate-200 bg-white p-5"
          >
            <p className="text-sm font-semibold text-slate-500">{label}</p>
            <p className="mt-2 text-3xl font-black text-slate-950">{value}</p>
          </div>
        ))}
      </section>

      <section className="mt-6 grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
        <div className="space-y-4">
          <h2 className="text-xl font-black text-slate-950">小区动态</h2>
          {items.map((item) => (
            <article
              key={item.name}
              className="rounded-[1.5rem] border border-slate-200 bg-white p-5 shadow-sm"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h3 className="text-xl font-black text-slate-950">
                    {item.name}
                  </h3>
                  <p className="mt-1 text-sm text-slate-500">
                    {item.area} · {item.layout}
                  </p>
                </div>
                <StatusBadge
                  tone={item.status === "适合砍价" ? "emerald" : "amber"}
                >
                  {item.status}
                </StatusBadge>
              </div>
              <div className="mt-4 flex flex-wrap gap-5 text-sm">
                <span>在售：{item.listedHomes} 套</span>
                <span>降价：{item.priceCutHomes} 套</span>
                <span>成交：{item.transaction}</span>
              </div>
              <p className="mt-4 rounded-2xl bg-slate-50 p-3 text-sm leading-6 text-slate-700">
                建议：{item.advice}
              </p>
            </article>
          ))}
        </div>

        <aside className="space-y-5">
          <div className="rounded-[2rem] border border-amber-100 bg-amber-50 p-6">
            <h2 className="text-xl font-black text-amber-950">异动提醒</h2>
            <ul className="mt-4 space-y-3 text-sm leading-6 text-amber-900">
              <li>· 青枫花园新增 2 套目标户型，挂牌价处于低位。</li>
              <li>· 星河湾有 1 套房源单次降价 30 万。</li>
            </ul>
          </div>

          <div className="rounded-[2rem] border border-slate-200 bg-white p-6">
            <h2 className="text-xl font-black text-slate-950">本周行动复盘</h2>
            <label className="mt-4 block">
              <span className="text-sm font-semibold text-slate-600">
                我看了哪些房？感觉如何？
              </span>
              <textarea
                rows={3}
                className="mt-2 w-full rounded-2xl border border-slate-200 bg-slate-50 p-3 outline-none focus:border-blue-400 focus:ring-4 focus:ring-blue-100"
                placeholder="记录实地看房感受..."
              />
            </label>
            <label className="mt-4 block">
              <span className="text-sm font-semibold text-slate-600">
                下周行动计划
              </span>
              <input
                className="mt-2 w-full rounded-2xl border border-slate-200 bg-slate-50 p-3 outline-none focus:border-blue-400 focus:ring-4 focus:ring-blue-100"
                placeholder="例如：约看青枫花园 3 套底价房"
              />
            </label>
            <button className="mt-5 w-full rounded-2xl bg-blue-600 px-4 py-3 text-sm font-bold text-white">
              保存复盘记录
            </button>
          </div>
        </aside>
      </section>
    </main>
  );
}
