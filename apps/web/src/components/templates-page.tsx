"use client";

import { Check, Copy, FileText, LoaderCircle } from "lucide-react";
import { useState } from "react";

import {
  buildTemplateMarkdown,
  decisionTemplates,
  type DecisionTemplate,
} from "@/lib/template-catalog";

import { StatusBadge } from "./status-badge";

export function TemplatesPage() {
  const [copiedId, setCopiedId] = useState<string>();
  const [copyErrorId, setCopyErrorId] = useState<string>();
  const [copyingId, setCopyingId] = useState<string>();

  const copyStructure = async (template: DecisionTemplate) => {
    if (copyingId) return;
    setCopyingId(template.id);
    setCopiedId(undefined);
    setCopyErrorId(undefined);
    try {
      if (!navigator.clipboard?.writeText) {
        throw new Error("clipboard unavailable");
      }
      await navigator.clipboard.writeText(buildTemplateMarkdown(template));
      setCopiedId(template.id);
    } catch {
      setCopyErrorId(template.id);
    } finally {
      setCopyingId(undefined);
    }
  };

  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      <section className="border-b border-slate-200 pb-7">
        <h1 className="text-3xl font-bold text-slate-950">工具模板</h1>
        <p className="mt-3 max-w-3xl leading-7 text-slate-600">
          把房脉的方法带到站外：预算、观察、看房、谈价、复盘都用统一结构记录，避免被单套房源或短期情绪带偏。
        </p>
      </section>

      <section className="mt-6 grid gap-5 md:grid-cols-2 lg:grid-cols-3">
        {decisionTemplates.map((template) => (
          <article
            key={template.id}
            className="flex min-h-80 flex-col rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
          >
            <div className="flex items-start justify-between gap-3">
              <h2 className="text-lg font-bold text-slate-950">
                {template.title}
              </h2>
              <StatusBadge tone="blue">v{template.version}</StatusBadge>
            </div>
            <p className="mt-3 text-sm leading-6 text-slate-600">
              {template.description}
            </p>
            <ul className="mt-4 flex-1 space-y-2 border-t border-slate-100 pt-4 text-sm text-slate-700">
              {template.sections.map((section) => (
                <li key={section.title} className="flex items-center gap-2">
                  <FileText aria-hidden="true" className="h-4 w-4 flex-none text-slate-400" />
                  <span>{section.title}</span>
                  <span className="ml-auto text-xs text-slate-400">{section.fields.length} 项</span>
                </li>
              ))}
            </ul>
            <button
              type="button"
              onClick={() => copyStructure(template)}
              disabled={copyingId !== undefined}
              className="mt-5 inline-flex h-10 items-center justify-center gap-2 rounded-md bg-slate-950 px-4 text-sm font-medium text-white transition-colors hover:bg-slate-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {copyingId === template.id ? (
                <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" />
              ) : copiedId === template.id ? (
                <Check aria-hidden="true" className="h-4 w-4" />
              ) : (
                <Copy aria-hidden="true" className="h-4 w-4" />
              )}
              {copyingId === template.id ? "正在复制" : copiedId === template.id ? "已复制" : "复制模板结构"}
            </button>
            {copiedId === template.id ? (
              <p role="status" className="mt-2 text-xs font-medium text-emerald-700">
                {template.title} {template.version} 已复制到剪贴板。
              </p>
            ) : null}
            {copyErrorId === template.id ? (
              <p role="alert" className="mt-2 text-xs font-medium text-rose-700">
                {template.title} 复制失败，请检查剪贴板权限后重试。
              </p>
            ) : null}
          </article>
        ))}
      </section>
    </main>
  );
}
