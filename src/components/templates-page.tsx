import { templates } from "@/lib/sample-data";

import { StatusBadge } from "./status-badge";

export function TemplatesPage() {
  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      <section className="rounded-[2rem] border border-slate-200 bg-white p-7 shadow-sm">
        <h1 className="text-4xl font-black text-slate-950">工具模板</h1>
        <p className="mt-4 max-w-3xl text-lg leading-8 text-slate-600">
          把房脉的方法带到站外：预算、观察、看房、谈价、复盘都用统一结构记录，避免被单套房源或短期情绪带偏。
        </p>
      </section>

      <section className="mt-6 grid gap-5 md:grid-cols-2 lg:grid-cols-3">
        {templates.map((template) => (
          <article
            key={template.title}
            className="flex min-h-72 flex-col rounded-[1.75rem] border border-slate-200 bg-white p-6 shadow-sm"
          >
            <div className="flex items-start justify-between gap-3">
              <h2 className="text-2xl font-black text-slate-950">
                {template.title}
              </h2>
              <StatusBadge tone="blue">可复制</StatusBadge>
            </div>
            <p className="mt-4 flex-1 leading-7 text-slate-600">
              {template.description}
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              {template.fields.map((field) => (
                <StatusBadge key={field} tone="slate">
                  {field}
                </StatusBadge>
              ))}
            </div>
            <button className="mt-6 rounded-2xl bg-slate-950 px-4 py-3 text-sm font-bold text-white">
              复制模板结构
            </button>
          </article>
        ))}
      </section>
    </main>
  );
}
