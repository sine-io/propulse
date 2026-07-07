import Link from "next/link";

import { methodTopics } from "@/lib/sample-data";

export function MethodsPage() {
  const [primary, ...rest] = methodTopics;

  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      <section className="mx-auto max-w-3xl text-center">
        <h1 className="text-4xl font-black tracking-tight text-slate-950">
          不是记术语，而是学会判断
        </h1>
        <p className="mt-4 text-lg leading-8 text-slate-600">
          每个方法都从真实问题出发：先指出常见误判，再给出正确看法，最后落到下一步行动。
        </p>
      </section>

      <section className="mt-10 grid gap-6 lg:grid-cols-[0.35fr_0.65fr]">
        <aside className="rounded-[2rem] border border-slate-200 bg-white p-5 shadow-sm">
          <h2 className="font-black text-slate-950">问题式目录</h2>
          <nav className="mt-4 space-y-2" aria-label="判断方法目录">
            {methodTopics.map((topic, index) => (
              <a
                key={topic.title}
                href={`#method-${index}`}
                className={`block rounded-2xl px-4 py-3 text-sm font-bold ${
                  index === 0
                    ? "bg-blue-50 text-blue-700"
                    : "bg-slate-50 text-slate-600 hover:bg-slate-100"
                }`}
              >
                {topic.title}
              </a>
            ))}
          </nav>
        </aside>

        <article
          id="method-0"
          className="rounded-[2rem] border border-slate-200 bg-white p-6 shadow-sm"
        >
          <h2 className="text-3xl font-black text-slate-950">
            {primary.title}
          </h2>
          <div className="mt-6 space-y-6">
            <MethodSection tone="rose" title="常见误判" body={primary.wrong} />
            <MethodSection tone="emerald" title="正确看法" body={primary.right} />
            <div className="rounded-3xl border border-blue-100 bg-blue-50 p-5">
              <h3 className="font-black text-blue-950">你应该怎么用</h3>
              <p className="mt-2 leading-7 text-blue-900">{primary.action}</p>
            </div>
          </div>
        </article>
      </section>

      <section className="mt-6 grid gap-5 md:grid-cols-2">
        {rest.map((topic, index) => (
          <article
            id={`method-${index + 1}`}
            key={topic.title}
            className="rounded-[1.75rem] border border-slate-200 bg-white p-6"
          >
            <h2 className="text-xl font-black text-slate-950">{topic.title}</h2>
            <p className="mt-3 text-sm font-semibold text-rose-600">
              常见误判：{topic.wrong}
            </p>
            <p className="mt-3 leading-7 text-slate-700">{topic.right}</p>
            <p className="mt-4 rounded-2xl bg-slate-50 p-4 text-sm font-semibold text-slate-700">
              行动：{topic.action}
            </p>
          </article>
        ))}
      </section>

      <section className="mt-6 rounded-[2rem] bg-slate-950 p-7 text-center text-white">
        <h2 className="text-2xl font-black">学以致用：本周复盘练习</h2>
        <p className="mx-auto mt-3 max-w-2xl leading-7 text-slate-300">
          去目标小区页记录本周挂牌、降价和成交变化，写下你的行动判断：看 / 等 / 砍价 / 出手。
        </p>
        <Link
          href="/neighborhoods"
          className="mt-5 inline-flex rounded-2xl bg-white px-5 py-3 text-sm font-bold text-slate-950"
        >
          前往目标小区实践
        </Link>
      </section>
    </main>
  );
}

function MethodSection({
  title,
  body,
  tone,
}: {
  title: string;
  body: string;
  tone: "rose" | "emerald";
}) {
  const toneClass =
    tone === "rose"
      ? "border-rose-100 bg-rose-50 text-rose-900"
      : "border-emerald-100 bg-emerald-50 text-emerald-900";

  return (
    <section className={`rounded-3xl border p-5 ${toneClass}`}>
      <h3 className="font-black">{title}</h3>
      <p className="mt-2 leading-7">{body}</p>
    </section>
  );
}
