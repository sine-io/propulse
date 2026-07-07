import Link from "next/link";

import { StatusBadge } from "./status-badge";

const modules = [
  {
    title: "换房能力",
    description: "把现金、旧房净回款、月供和过渡成本放在同一张表里。",
    metrics: ["安全总价", "月供压力", "资金缺口"],
  },
  {
    title: "目标小区观察",
    description: "追踪挂牌、成交、降价和供应压力，看懂房东预期。",
    metrics: ["挂牌变化", "降价房源", "成交区间"],
  },
  {
    title: "出手窗口",
    description: "合并预算状态和小区信号，输出看、等、砍价或出手。",
    metrics: ["预算安全", "买方窗口", "行动清单"],
  },
];

export function HomePage() {
  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8 lg:py-16">
      <section className="grid gap-10 lg:grid-cols-[1fr_0.92fr] lg:items-center">
        <div>
          <h1 className="max-w-3xl text-4xl font-black tracking-tight text-slate-950 sm:text-5xl lg:text-6xl">
            想买房或换房，先算清压力，再判断时机
          </h1>
          <p className="mt-6 max-w-2xl text-lg leading-8 text-slate-600">
            输入你的预算和目标小区，判断现在能不能买、压力有多大、是否适合看房、等待、砍价或出手。
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <Link
              href="/calculator"
              className="rounded-2xl bg-blue-600 px-6 py-4 text-center text-base font-bold text-white shadow-xl shadow-blue-600/20 transition hover:-translate-y-0.5 hover:bg-blue-700"
            >
              开始换房测算
            </Link>
            <Link
              href="/neighborhoods"
              className="rounded-2xl border border-slate-200 bg-white px-6 py-4 text-center text-base font-bold text-slate-800 shadow-sm transition hover:-translate-y-0.5 hover:border-slate-300"
            >
              添加目标小区
            </Link>
          </div>
        </div>

        <div className="relative rounded-[2rem] border border-slate-200 bg-white p-5 shadow-2xl shadow-slate-950/10">
          <div className="absolute -right-6 -top-6 size-28 rounded-full bg-teal-200/40 blur-2xl" />
          <div className="relative space-y-4">
            <div className="rounded-3xl border border-slate-100 bg-slate-50 p-5">
              <div className="mb-4 flex items-center justify-between">
                <h2 className="text-sm font-bold text-slate-500">
                  决策仪表盘预览
                </h2>
                <StatusBadge tone="emerald">月供偏安全</StatusBadge>
              </div>
              <div className="grid grid-cols-3 gap-3">
                {[
                  ["安全总价", "520 万", "text-emerald-600"],
                  ["勉强总价", "580 万", "text-amber-600"],
                  ["危险总价", "620万+", "text-rose-600"],
                ].map(([label, value, color]) => (
                  <div key={label} className="rounded-2xl bg-white p-4">
                    <p className="text-xs text-slate-500">{label}</p>
                    <p className={`mt-1 text-xl font-black ${color}`}>
                      {value}
                    </p>
                  </div>
                ))}
              </div>
            </div>

            <div className="rounded-3xl border border-blue-100 bg-blue-50 p-5">
              <div className="mb-3 flex items-center justify-between">
                <h2 className="font-bold text-blue-950">青枫花园信号</h2>
                <StatusBadge tone="amber">房东预期松动</StatusBadge>
              </div>
              <div className="grid gap-2 text-sm text-blue-900 sm:grid-cols-2">
                <p>挂牌连续增加</p>
                <p>降价房源变多</p>
                <p>成交量偏弱</p>
                <p>议价空间扩大</p>
              </div>
            </div>

            <div className="rounded-3xl bg-slate-950 p-5 text-white">
              <p className="text-sm font-semibold text-slate-300">
                当前综合建议
              </p>
              <p className="mt-2 text-xl font-black">
                可以开始看房，但不急下定
              </p>
              <p className="mt-2 text-sm leading-6 text-slate-300">
                下一步：重点关注 500-530 万三房，针对挂牌超过 60 天房源尝试砍价。
              </p>
            </div>
          </div>
        </div>
      </section>

      <section className="mt-16 grid gap-5 md:grid-cols-3">
        {modules.map((module) => (
          <article
            key={module.title}
            className="rounded-[1.75rem] border border-slate-200 bg-white p-6 shadow-sm"
          >
            <h2 className="text-xl font-black text-slate-950">
              {module.title}
            </h2>
            <p className="mt-3 leading-7 text-slate-600">
              {module.description}
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              {module.metrics.map((metric) => (
                <StatusBadge key={metric} tone="slate">
                  {metric}
                </StatusBadge>
              ))}
            </div>
          </article>
        ))}
      </section>

      <section className="mt-8 grid gap-5 lg:grid-cols-[1.2fr_0.8fr]">
        <div className="rounded-[1.75rem] border border-slate-200 bg-white p-6">
          <h2 className="text-2xl font-black text-slate-950">边用边学判断方法</h2>
          <div className="mt-5 grid gap-3 sm:grid-cols-3">
            {[
              "为什么不能只看挂牌价？",
              "什么是买方窗口？",
              "改善为什么要看新旧房价差？",
            ].map((question) => (
              <Link
                key={question}
                href="/methods"
                className="rounded-2xl border border-slate-100 bg-slate-50 p-4 text-sm font-semibold text-slate-700 transition hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700"
              >
                {question}
              </Link>
            ))}
          </div>
        </div>
        <div className="rounded-[1.75rem] border border-slate-200 bg-white p-6">
          <h2 className="text-2xl font-black text-slate-950">工具模板</h2>
          <p className="mt-3 leading-7 text-slate-600">
            用预算表、小区观察表、看房记录和谈价清单，把线上判断带到线下复盘。
          </p>
          <Link
            href="/templates"
            className="mt-5 inline-flex rounded-2xl bg-slate-950 px-5 py-3 text-sm font-bold text-white"
          >
            查看模板
          </Link>
        </div>
      </section>
    </main>
  );
}
