import Link from "next/link";
import { Bell, CheckCircle, Eye } from "lucide-react";

import { StatusBadge } from "./status-badge";

export function WatchlistPage() {
  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <section className="mb-8 flex items-end justify-between">
        <div>
          <h1 className="text-3xl font-bold text-slate-900">我的观察池</h1>
          <p className="mt-2 text-slate-500">每周跟踪，不错过买方窗口期。</p>
        </div>
        <Link
          href="/templates"
          className="text-sm font-medium text-blue-600 hover:underline"
        >
          导出本周报表
        </Link>
      </section>

      <section className="mb-8 grid grid-cols-2 gap-4 md:grid-cols-4">
        {[
          ["本周关注小区", "5", "个", "text-slate-800", "border-slate-200 bg-white"],
          ["出现降价信号", "2", "个", "text-amber-600", "border-slate-200 bg-white"],
          ["进入可砍价窗口", "1", "个", "text-blue-700", "border-blue-200 bg-blue-50/50"],
          ["价格仍偏硬", "2", "个", "text-slate-600", "border-slate-200 bg-white"],
        ].map(([label, value, unit, color, cardClass]) => (
          <div key={label} className={`rounded-xl border p-4 ${cardClass}`}>
            <p
              className={`mb-1 text-xs ${
                label === "进入可砍价窗口" ? "text-blue-600" : "text-slate-500"
              }`}
            >
              {label}
            </p>
            <p className={`text-2xl font-bold ${color}`}>
              {value} <span className="text-sm font-normal text-slate-400">{unit}</span>
            </p>
          </div>
        ))}
      </section>

      <section className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        <div className="space-y-4 lg:col-span-2">
          <h2 className="mb-4 font-bold text-slate-800">小区动态 (本周变化)</h2>
          <CommunityCard
            emphasized
            name="青枫花园"
            meta="滨江核心 · 三房"
            status="适合砍价"
            statusTone="emerald"
            listed="42套"
            listedDelta="(+8)"
            cuts="11套"
            transaction="偏弱"
            icon="check"
            advice="约看 500-530 万三房，尝试砍价，窗口期已打开。"
          />
          <CommunityCard
            name="云澜府"
            meta="城东新区 · 四房"
            status="继续观察"
            statusTone="amber"
            listed="28套"
            listedDelta="(-)"
            cuts="2套"
            transaction="平稳"
            icon="eye"
            advice="挂牌价无明显松动，且超出安全预算，建议暂缓约看。"
          />
        </div>

        <aside className="space-y-6">
          <section className="rounded-xl border border-amber-100 bg-amber-50 p-5">
            <h2 className="mb-3 flex items-center font-bold text-amber-900">
              <Bell aria-hidden="true" className="mr-2 h-5 w-5" />
              异动提醒
            </h2>
            <ul className="space-y-3 text-sm">
              {[
                "青枫花园 新增 2 套目标户型，挂牌价处于低位。",
                "星河湾 有 1 套房源单次降价 30 万。",
              ].map((item) => (
                <li key={item} className="flex items-start">
                  <span className="mr-2 mt-1.5 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-amber-500" />
                  <span className="text-amber-800">{item}</span>
                </li>
              ))}
            </ul>
          </section>

          <section className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
            <h2 className="mb-4 font-bold text-slate-800">本周行动复盘</h2>
            <div className="space-y-3">
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-slate-500">
                  我看了哪些房？感觉如何？
                </span>
                <textarea
                  className="w-full rounded-lg border border-slate-200 bg-slate-50 p-2 text-sm outline-none focus:border-blue-500"
                  rows={2}
                  placeholder="记录实地看房感受..."
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-slate-500">
                  下周行动计划
                </span>
                <input
                  className="w-full rounded-lg border border-slate-200 bg-slate-50 p-2 text-sm outline-none focus:border-blue-500"
                  placeholder="例如：约看青枫花园3套底价房"
                />
              </label>
              <button className="mt-2 w-full rounded-lg bg-slate-900 py-2 text-sm font-medium text-white transition-colors hover:bg-slate-800">
                保存复盘记录
              </button>
            </div>
          </section>
        </aside>
      </section>
    </main>
  );
}

function CommunityCard({
  advice,
  cuts,
  emphasized = false,
  icon,
  listed,
  listedDelta,
  meta,
  name,
  status,
  statusTone,
  transaction,
}: {
  advice: string;
  cuts: string;
  emphasized?: boolean;
  icon: "check" | "eye";
  listed: string;
  listedDelta: string;
  meta: string;
  name: string;
  status: string;
  statusTone: "emerald" | "amber";
  transaction: string;
}) {
  const Icon = icon === "check" ? CheckCircle : Eye;

  return (
    <article
      className={`relative overflow-hidden rounded-xl border bg-white p-5 shadow-sm transition-shadow hover:shadow-md ${
        emphasized ? "border-blue-200" : "border-slate-200"
      }`}
    >
      {emphasized ? <div className="absolute bottom-0 left-0 top-0 w-1 bg-blue-500" /> : null}
      <div className="mb-3 flex items-start justify-between">
        <h3 className="text-lg font-bold text-slate-900">
          {name}
          <span className="ml-2 text-xs font-normal text-slate-500">{meta}</span>
        </h3>
        <StatusBadge tone={statusTone}>{status}</StatusBadge>
      </div>
      <div className="mb-4 flex space-x-6 text-sm">
        <div>
          <span className="text-slate-500">在售：</span>
          <span className="font-medium text-slate-800">{listed}</span>{" "}
          <span className="text-xs text-amber-500">{listedDelta}</span>
        </div>
        <div>
          <span className="text-slate-500">降价：</span>
          <span className="font-medium text-slate-800">{cuts}</span>
        </div>
        <div>
          <span className="text-slate-500">成交：</span>
          <span className="font-medium text-slate-800">{transaction}</span>
        </div>
      </div>
      <div className="flex items-start rounded-lg border border-slate-100 bg-slate-50 p-3 text-sm text-slate-700">
        <Icon
          aria-hidden="true"
          className={`mr-2 mt-0.5 h-4 w-4 ${emphasized ? "text-blue-500" : "text-slate-400"}`}
        />
        <span>建议：{advice}</span>
      </div>
    </article>
  );
}
