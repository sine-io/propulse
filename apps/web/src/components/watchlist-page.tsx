"use client";

import Link from "next/link";
import { Bell, CheckCircle, Eye } from "lucide-react";
import { useEffect, useState } from "react";

import { ApiError, getWatchlist, type WatchlistItem } from "@/lib/api-client";
import { StatusBadge } from "./status-badge";

type CommunityView = {
  advice: string;
  cuts: string;
  emphasized: boolean;
  icon: "check" | "eye";
  listed: string;
  listedDelta: string;
  meta: string;
  name: string;
  status: string;
  statusTone: "emerald" | "amber";
  transaction: string;
};

const emptyStats = {
	bargain: 0,
	hard: 0,
	priceCuts: 0,
	total: 0,
};

export function WatchlistPage() {
	const [communities, setCommunities] = useState<CommunityView[]>([]);
	const [stats, setStats] = useState(emptyStats);
	const [errorMessage, setErrorMessage] = useState<string>();

  useEffect(() => {
    const controller = new AbortController();

    getWatchlist(controller.signal)
		.then((response) => {
			setErrorMessage(undefined);
        const nextCommunities = response.items.map(toCommunityView);
        setCommunities(nextCommunities);
        setStats({
          bargain: response.items.filter((item) =>
            ["重点看", "适合砍价"].includes(item.status),
          ).length,
          hard: response.items.filter((item) => item.status === "价格偏硬").length,
          priceCuts: response.items.filter((item) => item.priceCutHomes > 0).length,
          total: response.items.length,
        });
      })
      .catch((error: unknown) => {
        if (!(error instanceof DOMException && error.name === "AbortError")) {
				setCommunities([]);
				setStats(emptyStats);
				setErrorMessage(
					error instanceof ApiError && error.status === 401
						? "个人空间尚未解锁。"
						: "观察池暂时无法读取。",
				);
        }
      });

    return () => controller.abort();
  }, []);

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

		{errorMessage ? (
			<p role="status" className="mb-6 border-l-4 border-amber-500 bg-amber-50 px-4 py-3 text-sm text-amber-900">
				{errorMessage}
			</p>
		) : null}

      <section className="mb-8 grid grid-cols-2 gap-4 md:grid-cols-4">
        {[
          [
            "本周关注小区",
            String(stats.total),
            "个",
            "text-slate-800",
            "border-slate-200 bg-white",
          ],
          [
            "出现降价信号",
            String(stats.priceCuts),
            "个",
            "text-amber-600",
            "border-slate-200 bg-white",
          ],
          [
            "进入可砍价窗口",
            String(stats.bargain),
            "个",
            "text-blue-700",
            "border-blue-200 bg-blue-50/50",
          ],
          [
            "价格仍偏硬",
            String(stats.hard),
            "个",
            "text-slate-600",
            "border-slate-200 bg-white",
          ],
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
          {communities.length > 0 ? (
            communities.map((community) => (
              <CommunityCard key={`${community.name}-${community.meta}`} {...community} />
            ))
          ) : (
            <section className="rounded-xl border border-slate-200 bg-white p-8 text-center shadow-sm">
              <p className="font-medium text-slate-800">观察池暂无小区</p>
              <p className="mt-2 text-sm text-slate-500">
                添加目标小区后，本周变化和砍价信号会显示在这里。
              </p>
            </section>
          )}
        </div>

        <aside className="space-y-6">
          <section className="rounded-xl border border-amber-100 bg-amber-50 p-5">
            <h2 className="mb-3 flex items-center font-bold text-amber-900">
              <Bell aria-hidden="true" className="mr-2 h-5 w-5" />
              异动提醒
            </h2>
            <ul className="space-y-3 text-sm">
              {communities.length > 0 ? (
                [
                  "青枫花园 新增 2 套目标户型，挂牌价处于低位。",
                  "星河湾 有 1 套房源单次降价 30 万。",
                ].map((item) => (
                  <li key={item} className="flex items-start">
                    <span className="mr-2 mt-1.5 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-amber-500" />
                    <span className="text-amber-800">{item}</span>
                  </li>
                ))
              ) : (
                <li className="text-amber-800">暂无异动提醒。</li>
              )}
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
                  placeholder="例如：约看目标小区3套底价房"
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

function toCommunityView(item: WatchlistItem, index: number): CommunityView {
  const canBargain = item.status === "重点看" || item.status === "适合砍价";

  return {
    advice: item.advice,
    cuts: `${item.priceCutHomes}套`,
    emphasized: index === 0,
    icon: canBargain ? "check" : "eye",
    listed: `${item.listedHomes}套`,
    listedDelta: "",
    meta: `${item.area} · ${item.targetLayout}`,
    name: item.name,
    status: item.status,
    statusTone: canBargain ? "emerald" : "amber",
    transaction: momentumCopy[item.transactionMomentum],
  };
}

const momentumCopy: Record<WatchlistItem["transactionMomentum"], string> = {
  stable: "平稳",
  strong: "活跃",
  weak: "偏弱",
};

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
          {name}{" "}
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
