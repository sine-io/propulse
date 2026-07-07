import Link from "next/link";
import {
  Activity,
  BookOpen,
  Calculator,
  ChevronRight,
  Eye,
  type LucideIcon,
  Plus,
  Target,
  TrendingDown,
  TrendingUp,
} from "lucide-react";

import { StatusBadge } from "./status-badge";

export function HomePage() {
  return (
    <main className="mx-auto max-w-7xl space-y-12 px-4 py-8 pb-12 sm:px-6 lg:px-8">
      <section className="mt-8 grid grid-cols-1 items-center gap-12 lg:grid-cols-2">
        <div className="space-y-8">
          <h1 className="text-5xl font-bold leading-tight tracking-tight text-slate-900">
            想买房或换房，
            <br />
            先算清压力，
            <span className="text-blue-600">再判断时机</span>
          </h1>
          <p className="max-w-lg text-xl leading-relaxed text-slate-600">
            输入你的预算和目标小区，判断现在能不能买、压力有多大、是否适合看房、等待、砍价或出手。
          </p>
          <div className="flex flex-col gap-4 sm:flex-row">
            <Link
              href="/calculator"
              className="rounded-xl bg-blue-600 px-8 py-4 text-center text-lg font-medium text-white shadow-lg shadow-blue-600/20 transition-colors hover:bg-blue-700"
            >
              开始换房测算
            </Link>
            <Link
              href="/neighborhoods"
              className="flex items-center justify-center space-x-2 rounded-xl border-2 border-slate-200 bg-white px-8 py-4 text-lg font-medium text-slate-700 transition-colors hover:border-slate-300"
            >
              <Plus aria-hidden="true" className="h-4 w-4" />
              <span>添加目标小区</span>
            </Link>
          </div>
        </div>

        <div className="relative overflow-hidden rounded-2xl border border-slate-100 bg-white p-8 shadow-xl">
          <h2 className="relative z-10 mb-6 text-sm font-semibold uppercase tracking-wider text-slate-400">
            决策仪表盘 预览
          </h2>

          <div className="relative z-10 space-y-6">
            <section className="rounded-xl border border-slate-100 bg-slate-50/50 p-5 transition-colors hover:bg-white">
              <div className="mb-3 flex items-center justify-between">
                <h3 className="flex items-center font-semibold text-slate-800">
                  <Calculator aria-hidden="true" className="h-5 w-5" />
                  <span className="ml-2">换房能力</span>
                </h3>
                <StatusBadge tone="emerald">月供偏安全</StatusBadge>
              </div>
              <div className="grid grid-cols-3 gap-4 text-sm">
                <Metric label="安全总价" value="520 万" className="text-slate-900" />
                <Metric label="勉强总价" value="580 万" className="text-amber-600" />
                <Metric label="危险总价" value="620万+" className="text-rose-600" />
              </div>
            </section>

            <section className="rounded-xl border border-slate-100 bg-slate-50/50 p-5 transition-colors hover:bg-white">
              <div className="mb-3 flex items-center justify-between">
                <h3 className="flex items-center font-semibold text-slate-800">
                  <Target aria-hidden="true" className="h-5 w-5" />
                  <span className="ml-2">青枫花园 信号</span>
                </h3>
                <StatusBadge tone="amber">房东预期松动</StatusBadge>
              </div>
              <div className="grid grid-cols-1 gap-x-4 gap-y-2 text-sm sm:grid-cols-2">
                <Signal icon={TrendingUp} iconClassName="text-rose-500">
                  挂牌连续增加
                </Signal>
                <Signal icon={TrendingUp} iconClassName="text-rose-500">
                  降价房源变多
                </Signal>
                <Signal icon={TrendingDown} iconClassName="text-emerald-500">
                  成交量偏弱
                </Signal>
                <Signal icon={Activity} iconClassName="text-slate-500">
                  议价空间扩大
                </Signal>
              </div>
            </section>

            <section className="rounded-xl border border-blue-100 bg-blue-50 p-5">
              <h3 className="mb-2 font-semibold text-blue-900">当前综合建议</h3>
              <p className="mb-1 text-lg font-medium text-blue-800">
                可以开始看房，但不急下定
              </p>
              <p className="text-sm text-blue-600/80">
                下一步：重点关注 500-530 万三房，针对挂牌超 60 天房源尝试砍价。
              </p>
            </section>
          </div>
        </div>
      </section>

      <section className="grid grid-cols-1 gap-8 border-t border-slate-200 pt-8 md:grid-cols-3">
        <Feature
          icon={Calculator}
          iconClassName="bg-blue-100 text-blue-600"
          title="测算换房能力"
        >
          告别模糊估算。精准计算安全总价、月供压力和资金缺口，知道自己“能买多大的房”。
        </Feature>
        <Feature
          icon={Eye}
          iconClassName="bg-emerald-100 text-emerald-600"
          title="追踪小区情绪"
        >
          不看营销软文，只看真实数据。追踪挂牌、成交、降价频率，判断房东预期和供应压力。
        </Feature>
        <Link href="/methods" className="group space-y-4">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-purple-100 text-purple-600 transition-colors group-hover:bg-purple-200">
            <BookOpen aria-hidden="true" className="h-5 w-5" />
          </div>
          <h3 className="flex items-center text-xl font-bold text-slate-900">
            边用边学方法
            <ChevronRight
              aria-hidden="true"
              className="ml-1 h-4 w-4 opacity-0 transition-opacity group-hover:opacity-100"
            />
          </h3>
          <ul className="space-y-2 text-sm text-slate-600">
            {[
              "为什么不能只看挂牌价？",
              "什么是买方窗口？",
              "改善为什么要看新旧差？",
            ].map((item) => (
              <li key={item} className="flex items-center">
                <span className="mr-2 h-1.5 w-1.5 rounded-full bg-slate-400" />
                {item}
              </li>
            ))}
          </ul>
        </Link>
      </section>
    </main>
  );
}

function Metric({
  label,
  value,
  className,
}: {
  label: string;
  value: string;
  className: string;
}) {
  return (
    <div>
      <p className="text-slate-500">{label}</p>
      <p className={`text-lg font-bold ${className}`}>{value}</p>
    </div>
  );
}

function Signal({
  children,
  icon: Icon,
  iconClassName,
}: {
  children: React.ReactNode;
  icon: LucideIcon;
  iconClassName: string;
}) {
  return (
    <div className="flex items-center text-slate-600">
      <Icon aria-hidden="true" className={`h-4 w-4 ${iconClassName}`} />
      <span className="ml-2">{children}</span>
    </div>
  );
}

function Feature({
  children,
  icon: Icon,
  iconClassName,
  title,
}: {
  children: React.ReactNode;
  icon: LucideIcon;
  iconClassName: string;
  title: string;
}) {
  return (
    <article className="space-y-4">
      <div
        className={`mb-4 flex h-12 w-12 items-center justify-center rounded-xl ${iconClassName}`}
      >
        <Icon aria-hidden="true" className="h-5 w-5" />
      </div>
      <h3 className="text-xl font-bold text-slate-900">{title}</h3>
      <p className="text-sm text-slate-600">{children}</p>
    </article>
  );
}
