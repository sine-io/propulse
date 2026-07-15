"use client";

import {
  AlertTriangle,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Edit3,
  History,
  LoaderCircle,
  LockKeyhole,
  Save,
  X,
} from "lucide-react";
import { FormEvent, useEffect, useMemo, useRef, useState } from "react";

import {
  ApiError,
  createReviewNote,
  listReviewNotes,
  updateReviewNote,
  type ReviewNote,
  type WatchlistItem,
} from "@/lib/api-client";
import { getShanghaiWeekStart } from "@/lib/shanghai-date";

const reviewPageSize = 10;

type AccessState = "checking" | "locked" | "unlocked";
type HistoryState = "checking" | "locked" | "loading" | "empty" | "ready" | "failed";

interface ReviewNotesSectionProps {
  accessState: AccessState;
  watchlistItems: WatchlistItem[];
}

export function ReviewNotesSection({
  accessState,
  watchlistItems,
}: ReviewNotesSectionProps) {
  const [historyState, setHistoryState] = useState<HistoryState>("checking");
  const [notes, setNotes] = useState<ReviewNote[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [reloadVersion, setReloadVersion] = useState(0);
  const [weekStartDate, setWeekStartDate] = useState(() => getShanghaiWeekStart());
  const [neighborhoodId, setNeighborhoodId] = useState("");
  const [content, setContent] = useState("");
  const [editingNote, setEditingNote] = useState<ReviewNote>();
  const [isSaving, setIsSaving] = useState(false);
  const [formError, setFormError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();
  const saveController = useRef<AbortController | undefined>(undefined);
  const editor = useRef<HTMLTextAreaElement>(null);

  const neighborhoodNames = useMemo(() => {
    const names = new Map<string, string>();
    for (const item of watchlistItems) {
      names.set(item.neighborhoodId, `${item.name} · ${item.area}`);
    }
    return names;
  }, [watchlistItems]);
  const neighborhoodOptions = useMemo(
    () => Array.from(neighborhoodNames, ([id, label]) => ({ id, label })),
    [neighborhoodNames],
  );

  useEffect(() => {
    if (accessState === "checking") {
      setHistoryState("checking");
      setNotes([]);
      setTotal(0);
      return;
    }
    if (accessState === "locked") {
      setHistoryState("locked");
      setNotes([]);
      setTotal(0);
      return;
    }

    const controller = new AbortController();
    setHistoryState("loading");
    listReviewNotes(page, reviewPageSize, controller.signal)
      .then((response) => {
        setNotes(response.items);
        setTotal(response.total);
        setHistoryState(response.items.length === 0 ? "empty" : "ready");
      })
      .catch((error: unknown) => {
        if (isAbortError(error)) return;
        setNotes([]);
        setTotal(0);
        setHistoryState(
          error instanceof ApiError && error.status === 401 ? "locked" : "failed",
        );
      });

    return () => controller.abort();
  }, [accessState, page, reloadVersion]);

  useEffect(() => {
    if (accessState !== "unlocked") {
      saveController.current?.abort();
    }
  }, [accessState]);

  useEffect(
    () => () => {
      saveController.current?.abort();
    },
    [],
  );

  const trimmedContent = content.trim();
  const contentLength = Array.from(trimmedContent).length;
  const pageCount = Math.max(1, Math.ceil(total / reviewPageSize));

  const resetEditor = () => {
    setEditingNote(undefined);
    setContent("");
    setNeighborhoodId("");
    setWeekStartDate(getShanghaiWeekStart());
    setFormError(undefined);
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (accessState !== "unlocked" || isSaving) return;
    if (contentLength < 1) {
      setFormError("请输入 1–8000 个 Unicode 字符的复盘正文。");
      return;
    }
    if (contentLength > 8000) {
      setFormError("复盘正文不能超过 8000 个 Unicode 字符。");
      return;
    }
    if (!editingNote && !weekStartDate) {
      setFormError("请选择复盘周次。");
      return;
    }

    const controller = new AbortController();
    saveController.current = controller;
    setIsSaving(true);
    setFormError(undefined);
    setFeedback(undefined);
    try {
      if (editingNote) {
        await updateReviewNote(
          editingNote.id,
          { content: trimmedContent },
          controller.signal,
        );
        setFeedback("复盘已更新，历史记录已重新读取。");
      } else {
        await createReviewNote(
          {
            content: trimmedContent,
            kind: "review",
            neighborhoodId: neighborhoodId || null,
            weekStartDate,
          },
          controller.signal,
        );
        setFeedback("复盘已保存，历史记录已重新读取。");
      }
      resetEditor();
      setPage(1);
      setReloadVersion((version) => version + 1);
    } catch (error) {
      if (isAbortError(error)) return;
      setFormError(editingNote ? "复盘更新失败，正文已保留。" : "复盘保存失败，输入已保留。");
    } finally {
      if (saveController.current === controller) {
        saveController.current = undefined;
        setIsSaving(false);
      }
    }
  };

  const beginEditing = (note: ReviewNote) => {
    setEditingNote(note);
    setContent(note.content);
    setFormError(undefined);
    setFeedback(undefined);
    editor.current?.focus();
  };

  return (
    <section aria-labelledby="review-notes-title" className="mt-12 border-t border-slate-200 pt-8">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 id="review-notes-title" className="text-xl font-bold text-slate-900">
            每周复盘
          </h2>
          <p className="mt-1 text-sm text-slate-500">记录看房感受和下周计划。</p>
        </div>
        {isSaving ? (
          <p role="status" className="inline-flex items-center gap-2 text-sm text-blue-700">
            <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" />
            正在保存复盘
          </p>
        ) : null}
      </div>

      {historyState === "checking" ? (
        <ReviewState icon={LoaderCircle} title="正在确认复盘访问状态" />
      ) : null}
      {historyState === "locked" ? (
        <ReviewState
          icon={LockKeyhole}
          title="复盘记录已锁定"
          detail="解锁个人空间后才能创建和读取复盘。"
          tone="amber"
        />
      ) : null}

      {accessState === "unlocked" ? (
        <div className="grid gap-8 lg:grid-cols-[minmax(0,5fr)_minmax(320px,4fr)]">
          <form onSubmit={submit} className="self-start rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <h3 className="font-semibold text-slate-900">
                {editingNote ? "重新编辑复盘" : "新建复盘"}
              </h3>
              {editingNote ? (
                <button
                  type="button"
                  onClick={resetEditor}
                  aria-label="取消重新编辑"
                  title="取消重新编辑"
                  className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100 hover:text-slate-900"
                >
                  <X aria-hidden="true" className="h-4 w-4" />
                </button>
              ) : null}
            </div>

            {editingNote ? (
              <dl className="mt-4 grid gap-3 bg-slate-50 p-3 text-sm sm:grid-cols-3">
                <div>
                  <dt className="text-xs text-slate-500">类型</dt>
                  <dd className="mt-1 font-medium text-slate-800">{reviewKindLabel(editingNote.kind)}</dd>
                </div>
                <div>
                  <dt className="text-xs text-slate-500">周次</dt>
                  <dd className="mt-1 font-medium text-slate-800">{editingNote.weekStartDate ?? "未指定"}</dd>
                </div>
                <div className="min-w-0">
                  <dt className="text-xs text-slate-500">关联小区</dt>
                  <dd className="mt-1 break-words font-medium text-slate-800">
                    {reviewNeighborhoodLabel(editingNote.neighborhoodId, neighborhoodNames)}
                  </dd>
                </div>
              </dl>
            ) : (
              <div className="mt-4 grid gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor="review-week" className="mb-1.5 block text-sm font-medium text-slate-700">
                    复盘周次
                  </label>
                  <div className="relative">
                    <CalendarDays aria-hidden="true" className="pointer-events-none absolute left-3 top-3 h-4 w-4 text-slate-400" />
                    <input
                      id="review-week"
                      type="date"
                      required
                      value={weekStartDate}
                      onChange={(event) => setWeekStartDate(event.target.value)}
                      className="h-10 w-full rounded-md border border-slate-300 bg-white pl-9 pr-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                    />
                  </div>
                </div>
                <div>
                  <label htmlFor="review-neighborhood" className="mb-1.5 block text-sm font-medium text-slate-700">
                    关联观察池小区（可选）
                  </label>
                  <select
                    id="review-neighborhood"
                    value={neighborhoodId}
                    onChange={(event) => setNeighborhoodId(event.target.value)}
                    className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  >
                    <option value="">不关联小区</option>
                    {neighborhoodOptions.map((option) => (
                      <option key={option.id} value={option.id}>{option.label}</option>
                    ))}
                  </select>
                </div>
              </div>
            )}

            <label htmlFor="review-content" className="mb-1.5 mt-4 block text-sm font-medium text-slate-700">
              复盘正文
            </label>
            <textarea
              ref={editor}
              id="review-content"
              value={content}
              onChange={(event) => {
                setContent(event.target.value);
                setFormError(undefined);
              }}
              rows={8}
              aria-describedby="review-content-count"
              placeholder="记录本周看房感受、判断变化和下周计划。"
              className="w-full resize-y rounded-md border border-slate-300 px-3 py-2 text-sm leading-6 text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            />
            <div className="mt-1 flex items-start justify-between gap-3">
              <div>
                {formError ? <p role="alert" className="text-sm text-rose-700">{formError}</p> : null}
                {feedback ? <p role="status" className="text-sm text-emerald-700">{feedback}</p> : null}
              </div>
              <p id="review-content-count" className={`flex-none text-xs ${contentLength > 8000 ? "text-rose-700" : "text-slate-500"}`}>
                {contentLength}/8000
              </p>
            </div>
            <button
              type="submit"
              disabled={isSaving}
              className="mt-4 inline-flex h-10 items-center gap-2 rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isSaving ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Save aria-hidden="true" className="h-4 w-4" />}
              {isSaving ? "正在保存" : editingNote ? "更新复盘" : "保存复盘"}
            </button>
          </form>

          <div aria-live="polite" aria-busy={historyState === "loading"}>
            <div className="flex items-center justify-between gap-3">
              <h3 className="flex items-center gap-2 font-semibold text-slate-900">
                <History aria-hidden="true" className="h-4 w-4" />
                复盘历史
              </h3>
              {total > 0 ? <p className="text-xs text-slate-500">共 {total} 条</p> : null}
            </div>

            {historyState === "loading" ? (
              <ReviewState icon={LoaderCircle} title="正在加载复盘历史" compact />
            ) : null}
            {historyState === "failed" ? (
              <ReviewState
                icon={AlertTriangle}
                title="复盘历史读取失败"
                detail="创建表单仍可使用；可单独重试历史记录。"
                tone="rose"
                compact
                action={
                  <button
                    type="button"
                    onClick={() => setReloadVersion((version) => version + 1)}
                    className="mt-3 inline-flex h-9 items-center gap-2 rounded-md border border-rose-300 bg-white px-3 text-sm font-medium text-rose-700 hover:bg-rose-50"
                  >
                    <History aria-hidden="true" className="h-4 w-4" />
                    重试历史
                  </button>
                }
              />
            ) : null}
            {historyState === "empty" ? (
              <p className="mt-4 border-y border-slate-200 py-8 text-center text-sm text-slate-500">暂无复盘历史</p>
            ) : null}
            {historyState === "ready" ? (
              <ol className="mt-4 space-y-3">
                {notes.map((note) => (
                  <li key={note.id} className="rounded-lg border border-slate-200 bg-white p-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0 text-xs text-slate-500">
                        <p>{reviewKindLabel(note.kind)} · {note.weekStartDate ?? "未指定周次"}</p>
                        <p className="mt-1 break-words">{reviewNeighborhoodLabel(note.neighborhoodId, neighborhoodNames)}</p>
                      </div>
                      <button
                        type="button"
                        onClick={() => beginEditing(note)}
                        className="inline-flex h-8 flex-none items-center gap-1.5 rounded-md border border-slate-300 px-2.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
                      >
                        <Edit3 aria-hidden="true" className="h-3.5 w-3.5" />
                        重新编辑
                      </button>
                    </div>
                    <p className="mt-3 whitespace-pre-wrap break-words text-sm leading-6 text-slate-800">{note.content}</p>
                    <p className="mt-3 text-xs text-slate-500">更新于 {formatShanghaiTimestamp(note.updatedAt)}</p>
                  </li>
                ))}
              </ol>
            ) : null}

            {historyState === "ready" && pageCount > 1 ? (
              <nav aria-label="复盘历史分页" className="mt-4 flex items-center justify-between gap-3">
                <button
                  type="button"
                  disabled={page <= 1}
                  onClick={() => setPage((value) => Math.max(1, value - 1))}
                  className="inline-flex h-9 items-center gap-1 rounded-md border border-slate-300 px-3 text-sm text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ChevronLeft aria-hidden="true" className="h-4 w-4" />
                  上一页
                </button>
                <span className="text-sm text-slate-500">第 {page} / {pageCount} 页</span>
                <button
                  type="button"
                  disabled={page >= pageCount}
                  onClick={() => setPage((value) => Math.min(pageCount, value + 1))}
                  className="inline-flex h-9 items-center gap-1 rounded-md border border-slate-300 px-3 text-sm text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  下一页
                  <ChevronRight aria-hidden="true" className="h-4 w-4" />
                </button>
              </nav>
            ) : null}
          </div>
        </div>
      ) : null}
    </section>
  );
}

function ReviewState({
  action,
  compact = false,
  detail,
  icon: Icon,
  title,
  tone = "slate",
}: {
  action?: React.ReactNode;
  compact?: boolean;
  detail?: string;
  icon: typeof LoaderCircle;
  title: string;
  tone?: "amber" | "rose" | "slate";
}) {
  const toneClass = {
    amber: "border-amber-300 bg-amber-50 text-amber-950",
    rose: "border-rose-300 bg-rose-50 text-rose-950",
    slate: "border-slate-200 bg-slate-50 text-slate-800",
  }[tone];
  return (
    <div role="status" className={`${compact ? "mt-4" : ""} border-l-4 px-4 py-4 ${toneClass}`}>
      <div className="flex items-start gap-3">
        <Icon aria-hidden="true" className={`mt-0.5 h-4 w-4 flex-none ${Icon === LoaderCircle ? "animate-spin" : ""}`} />
        <div>
          <p className="text-sm font-semibold">{title}</p>
          {detail ? <p className="mt-1 text-sm opacity-80">{detail}</p> : null}
          {action}
        </div>
      </div>
    </div>
  );
}

function reviewKindLabel(kind: ReviewNote["kind"]): string {
  return kind === "review" ? "决策复盘" : "看房记录";
}

function reviewNeighborhoodLabel(
  neighborhoodId: string | null,
  names: Map<string, string>,
): string {
  if (!neighborhoodId) return "未关联小区";
  return names.get(neighborhoodId) ?? `关联小区 ${neighborhoodId}`;
}

function formatShanghaiTimestamp(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "Asia/Shanghai",
  }).format(new Date(value));
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}
