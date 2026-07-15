import {
  ArrowRight,
  BarChart3,
  CheckCircle2,
  CircleAlert,
  ClipboardCheck,
  Lightbulb,
  ListChecks,
  Target,
} from "lucide-react";
import Link from "next/link";

import {
  buildMethodPath,
  defaultMethodArticle,
  methodArticles,
  type MethodArticle,
} from "@/lib/method-articles";
import { methodRuleMeta } from "@/lib/method-rules";

interface MethodsPageProps {
  article?: MethodArticle;
}

export function MethodsPage({ article = defaultMethodArticle }: MethodsPageProps) {
  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <header className="mx-auto mb-8 max-w-3xl text-center">
        <h1 className="text-3xl font-bold text-slate-900">
          不是死记术语，而是学会判断
        </h1>
        <p className="mt-3 text-slate-500">
          从真实购房问题出发，核对家庭边界、市场证据和数据质量，再决定下一步行动。
        </p>
      </header>

      <div className="flex flex-col gap-8 md:flex-row md:items-start">
        <aside className="w-full shrink-0 md:sticky md:top-6 md:w-1/3 lg:w-1/4">
          <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
            <h2 className="border-b border-slate-100 bg-slate-50 p-4 font-semibold text-slate-800">
              问题场景目录
            </h2>
            <nav className="text-sm font-medium" aria-label="问题场景目录">
              {methodArticles.map((topic) => {
                const isCurrent = topic.slug === article.slug;
                return (
                  <Link
                    key={topic.slug}
                    href={buildMethodPath(topic.slug)}
                    aria-current={isCurrent ? "page" : undefined}
                    className={`block border-l-4 px-4 py-3 leading-5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 ${
                      isCurrent
                        ? "border-blue-600 bg-blue-50 text-blue-800"
                        : "border-transparent text-slate-600 hover:bg-slate-50 hover:text-slate-900"
                    }`}
                  >
                    {topic.title}
                  </Link>
                );
              })}
            </nav>
          </div>
        </aside>

        <div className="min-w-0 flex-1">
          <article className="space-y-8 rounded-lg border border-slate-200 bg-white p-5 shadow-sm sm:p-8">
            <header className="border-b border-slate-200 pb-6">
              <p className="mb-2 text-sm font-semibold text-blue-700">购房判断方法</p>
              <h2 className="text-2xl font-bold leading-9 text-slate-900">
                {article.title}
              </h2>
            </header>

            <section aria-labelledby="real-question-heading">
              <h3
                id="real-question-heading"
                className="mb-3 flex items-center gap-2 font-bold text-slate-900"
              >
                <Target aria-hidden="true" className="h-5 w-5 text-blue-600" />
                真实问题
              </h3>
              <p className="leading-7 text-slate-700">{article.realQuestion}</p>
            </section>

            <div className="grid gap-6 lg:grid-cols-2">
              <section aria-labelledby="common-mistake-heading">
                <h3
                  id="common-mistake-heading"
                  className="mb-3 flex items-center gap-2 font-bold text-rose-700"
                >
                  <CircleAlert aria-hidden="true" className="h-5 w-5" />
                  常见误判
                </h3>
                <p className="h-full border-l-4 border-rose-200 bg-rose-50 p-4 leading-7 text-slate-700">
                  {article.commonMistake}
                </p>
              </section>

              <section aria-labelledby="correct-judgment-heading">
                <h3
                  id="correct-judgment-heading"
                  className="mb-3 flex items-center gap-2 font-bold text-emerald-700"
                >
                  <CheckCircle2 aria-hidden="true" className="h-5 w-5" />
                  正确判断
                </h3>
                <p className="h-full border-l-4 border-emerald-200 bg-emerald-50 p-4 leading-7 text-slate-700">
                  {article.correctJudgment}
                </p>
              </section>
            </div>

            <section aria-labelledby="key-metrics-heading">
              <h3
                id="key-metrics-heading"
                className="mb-3 flex items-center gap-2 font-bold text-slate-900"
              >
                <BarChart3 aria-hidden="true" className="h-5 w-5 text-blue-600" />
                你需要盯住的关键指标
              </h3>
              <ul className="divide-y divide-slate-100 border-y border-slate-200">
                {article.keyMetrics.map((metric) => (
                  <li
                    key={metric.name}
                    className="grid gap-1 py-3 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-4"
                  >
                    <span className="font-semibold text-slate-800">{metric.name}</span>
                    <span className="text-sm leading-6 text-slate-600">{metric.usage}</span>
                  </li>
                ))}
              </ul>
            </section>

            <section
              aria-labelledby="method-example-heading"
              className="rounded-lg border border-blue-100 bg-blue-50 p-5"
            >
              <h3
                id="method-example-heading"
                className="mb-3 flex items-center gap-2 font-bold text-blue-900"
              >
                <Lightbulb aria-hidden="true" className="h-5 w-5" />
                示例（仅用于说明判断过程）
              </h3>
              <dl className="space-y-3 text-sm leading-6 text-blue-950">
                <div>
                  <dt className="font-semibold">情境</dt>
                  <dd>{article.example.situation}</dd>
                </div>
                <div>
                  <dt className="font-semibold">判断</dt>
                  <dd>{article.example.interpretation}</dd>
                </div>
              </dl>
            </section>

            <section aria-labelledby="method-actions-heading">
              <h3
                id="method-actions-heading"
                className="mb-3 flex items-center gap-2 font-bold text-slate-900"
              >
                <ListChecks aria-hidden="true" className="h-5 w-5 text-blue-600" />
                行动建议
              </h3>
              <ol className="space-y-3">
                {article.actions.map((action, index) => (
                  <li key={action} className="flex gap-3 text-sm leading-6 text-slate-700">
                    <span
                      aria-hidden="true"
                      className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-slate-900 text-xs font-semibold text-white"
                    >
                      {index + 1}
                    </span>
                    <span>{action}</span>
                  </li>
                ))}
              </ol>
            </section>

            <section
              aria-labelledby="method-provenance-heading"
              className="rounded-lg border border-slate-200 bg-slate-50 p-5"
            >
              <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <h3
                  id="method-provenance-heading"
                  className="flex items-center gap-2 font-bold text-slate-800"
                >
                  <ClipboardCheck aria-hidden="true" className="h-5 w-5" />
                  方法适用范围与来源
                </h3>
                <span className="rounded-full bg-slate-200 px-2 py-1 text-xs font-medium text-slate-700">
                  规则版本 {methodRuleMeta.version}
                </span>
              </div>
              <dl className="grid gap-4 text-sm text-slate-600 sm:grid-cols-2">
                <div>
                  <dt className="font-semibold text-slate-800">本文适用范围</dt>
                  <dd className="mt-1 leading-6">{article.applicableScope}</dd>
                </div>
                <div>
                  <dt className="font-semibold text-slate-800">规则适用范围</dt>
                  <dd className="mt-1 leading-6">{methodRuleMeta.applicableScope}</dd>
                </div>
                <div>
                  <dt className="font-semibold text-slate-800">样本要求</dt>
                  <dd className="mt-1 leading-6">{methodRuleMeta.sampleRequirement}</dd>
                </div>
                <div>
                  <dt className="font-semibold text-slate-800">来源</dt>
                  <dd className="mt-1 leading-6">{methodRuleMeta.source}</dd>
                </div>
                <div className="sm:col-span-2">
                  <dt className="font-semibold text-slate-800">局限</dt>
                  <dd className="mt-1 leading-6">{methodRuleMeta.limitation}</dd>
                </div>
              </dl>
              <p className="mt-4 text-xs leading-5 text-slate-500">
                更新时间 {methodRuleMeta.updatedAt}，生效日期 {methodRuleMeta.effectiveDate}；方法规则与测算规则共用同一版本方案。
              </p>
            </section>
          </article>

          <section className="mt-6 rounded-lg bg-slate-900 p-6 text-center text-white">
            <h3 className="font-bold">用目标小区的真实数据练习判断</h3>
            <p className="mx-auto mt-2 max-w-xl text-sm leading-6 text-slate-300">
              查看最新批次、来源和质量状态，再把本文方法转化为自己的观察与行动依据。
            </p>
            <Link
              href="/neighborhoods"
              className="mt-4 inline-flex items-center gap-2 rounded-lg bg-white px-5 py-2 font-medium text-slate-900 transition-colors hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
            >
              前往目标小区实践
              <ArrowRight aria-hidden="true" className="h-4 w-4" />
            </Link>
          </section>
        </div>
      </div>
    </main>
  );
}
