"use client";

import Link from "next/link";
import {
  AlertCircle,
  ArrowLeft,
  Database,
  Download,
  FileJson,
  LoaderCircle,
  LockKeyhole,
  RefreshCw,
} from "lucide-react";
import { useEffect, useState } from "react";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import {
  ApiError,
  getCollectionRunDetail,
  type CollectionRunDetail,
} from "@/lib/api-client";

type DetailState = "loading" | "locked" | "ready" | "failed";

export function ImportDetailPage() {
  const [accessChecked, setAccessChecked] = useState(false);
  const [unlocked, setUnlocked] = useState(false);
  const [state, setState] = useState<DetailState>("loading");
  const [id, setID] = useState("");
  const [detail, setDetail] = useState<CollectionRunDetail>();
  const [error, setError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const sync = () => {
      const hasToken = Boolean(getAccessToken());
      setAccessChecked(true);
      setUnlocked(hasToken);
      if (!hasToken) {
        setState("locked");
        setDetail(undefined);
      }
    };
    sync();
    return subscribeToAccessToken(sync);
  }, []);

  useEffect(() => {
    if (!accessChecked) {
      return;
    }
    const runID = collectionRunIDFromPath(window.location.pathname);
    setID(runID);
    if (!unlocked) {
      setState("locked");
      return;
    }
    if (!runID) {
      setError("批次地址无效。");
      setState("failed");
      return;
    }
    const controller = new AbortController();
    setState("loading");
    setError("");
    setDetail(undefined);
    getCollectionRunDetail(runID, controller.signal)
      .then((response) => {
        setDetail(response);
        setState("ready");
      })
      .catch((caught: unknown) => {
        if (caught instanceof DOMException && caught.name === "AbortError") {
          return;
        }
        if (caught instanceof ApiError && caught.status === 401) {
          setState("locked");
          return;
        }
        setError(
          caught instanceof ApiError && caught.code === "import_not_found"
            ? "批次不存在。"
            : "批次详情暂时无法读取。",
        );
        setState("failed");
      });
    return () => controller.abort();
  }, [accessChecked, reloadKey, unlocked]);

  if (state === "loading") {
    return <DetailPageState icon={LoaderCircle} title="正在读取批次详情" spinning />;
  }
  if (state === "locked") {
    return <DetailPageState icon={LockKeyhole} title="批次详情已锁定" detail="请先解锁个人空间。" />;
  }
  if (state === "failed" || !detail) {
    return (
      <DetailPageState
        icon={AlertCircle}
        title="批次详情读取失败"
        detail={error}
        action={
          <button type="button" onClick={() => setReloadKey((value) => value + 1)} className={secondaryButtonClass}>
            <RefreshCw aria-hidden="true" className="h-4 w-4" />
            重试
          </button>
        }
      />
    );
  }

  const run = detail.collectionRun;
  const summary = run.validationSummary;
  return (
    <main className="mx-auto w-full max-w-7xl px-4 py-7 sm:px-6 lg:px-8">
      <Link href="/data" className="inline-flex items-center gap-2 text-sm font-medium text-slate-600 hover:text-slate-950">
        <ArrowLeft aria-hidden="true" className="h-4 w-4" />
        返回数据管理
      </Link>

      <div className="mt-5 flex flex-col justify-between gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-start">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <Database aria-hidden="true" className="h-5 w-5 text-emerald-700" />
            <h1 className="text-xl font-bold text-slate-950">采集批次详情</h1>
          </div>
          <p className="mt-2 break-all font-mono text-xs text-slate-500">{id}</p>
        </div>
        <button
          type="button"
          onClick={() => downloadRawPayload(detail)}
          className={secondaryButtonClass}
        >
          <Download aria-hidden="true" className="h-4 w-4" />
          下载原始载荷
        </button>
      </div>

      <section aria-labelledby="trace-title" className="grid gap-6 border-b border-slate-200 py-6 lg:grid-cols-[1fr_2fr]">
        <div>
          <h2 id="trace-title" className="text-sm font-semibold text-slate-900">数据来源</h2>
          <dl className="mt-3 space-y-3 text-sm">
            <Definition label="名称" value={detail.source.name} />
            <Definition label="城市" value={detail.source.city} />
            <Definition label="来源类型" value={detail.source.sourceType} mono />
            <Definition label="备注" value={detail.source.notes || "-"} />
          </dl>
        </div>
        <div>
          <h2 className="text-sm font-semibold text-slate-900">采集元数据</h2>
          <dl className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Definition label="来源引用" value={run.sourceRef} />
            <Definition label="采集时间" value={formatDateTime(run.collectedAt)} />
            <Definition label="覆盖范围" value={run.coverage === "full" ? "完整覆盖" : "部分覆盖"} />
            <Definition label="导入格式" value={run.format.toUpperCase()} mono />
            <Definition label="指标刷新" value={metricStatusLabel(run.metricStatus)} />
            <Definition label="创建时间" value={formatDateTime(run.createdAt)} />
            <div className="sm:col-span-2 lg:col-span-3">
              <Definition label="内容校验和" value={run.contentChecksum} mono />
            </div>
          </dl>
        </div>
      </section>

      <section aria-labelledby="validation-title" className="border-b border-slate-200 py-6">
        <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
          <div>
            <h2 id="validation-title" className="text-sm font-semibold text-slate-900">校验摘要</h2>
            <p className="mt-1 text-sm text-slate-500">批次状态：{run.status === "completed" ? "已完成" : run.status}</p>
          </div>
          <dl className="grid grid-cols-3 gap-px overflow-hidden border border-slate-200 bg-slate-200">
            <SummaryStat label="记录" value={summary.recordCount} />
            <SummaryStat label="挂牌" value={summary.listingCount} />
            <SummaryStat label="成交" value={summary.transactionCount} />
          </dl>
        </div>
        {summary.issues.length > 0 ? (
          <ul className="mt-4 divide-y divide-slate-200 border-y border-slate-200 text-sm">
            {summary.issues.map((issue, index) => (
              <li key={`${issue.row ?? "request"}-${issue.field}-${index}`} className="grid gap-1 py-2 sm:grid-cols-[8rem_10rem_1fr]">
                <span>{issue.row ? `第 ${issue.row} 行` : "请求字段"}</span>
                <span className="font-mono text-xs">{issue.field}</span>
                <span>{issue.message}</span>
              </li>
            ))}
          </ul>
        ) : (
          <div className="mt-4 flex items-center gap-2 text-sm text-emerald-700">
            <FileJson aria-hidden="true" className="h-4 w-4" />
            无校验错误
          </div>
        )}
      </section>

      <section aria-labelledby="listing-title" className="py-6">
        <div className="mb-3 flex items-center justify-between">
          <h2 id="listing-title" className="text-sm font-semibold text-slate-900">挂牌观察</h2>
          <span className="text-xs text-slate-500">{detail.listings.length} 条</span>
        </div>
        <div className="overflow-x-auto border border-slate-200">
          <table className="min-w-[920px] w-full border-collapse text-left text-sm">
            <thead className="bg-slate-50 text-xs text-slate-500">
              <tr>
                <TableHead>源记录 ID</TableHead><TableHead>物理行</TableHead><TableHead>户型</TableHead><TableHead>面积</TableHead><TableHead>挂牌价</TableHead><TableHead>在市天数</TableHead><TableHead>状态</TableHead><TableHead>属性</TableHead>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {detail.listings.map((item) => (
                <tr key={item.id} className="bg-white text-slate-700">
                  <TableCell mono>{item.sourceListingId}</TableCell><TableCell>{item.sourceRow}</TableCell><TableCell>{item.layout}</TableCell><TableCell>{item.areaSqm} ㎡</TableCell><TableCell>{item.listingPrice} 万</TableCell><TableCell>{item.daysOnMarket}</TableCell><TableCell>{listingStatusLabel(item.status)}</TableCell><TableCell mono>{JSON.stringify(item.attributes)}</TableCell>
                </tr>
              ))}
              {detail.listings.length === 0 ? <EmptyTableRow columns={8} /> : null}
            </tbody>
          </table>
        </div>
      </section>

      <section aria-labelledby="transaction-title" className="border-t border-slate-200 py-6">
        <div className="mb-3 flex items-center justify-between">
          <h2 id="transaction-title" className="text-sm font-semibold text-slate-900">成交观察</h2>
          <span className="text-xs text-slate-500">{detail.transactions.length} 条</span>
        </div>
        <div className="overflow-x-auto border border-slate-200">
          <table className="min-w-[760px] w-full border-collapse text-left text-sm">
            <thead className="bg-slate-50 text-xs text-slate-500">
              <tr>
                <TableHead>源记录 ID</TableHead><TableHead>物理行</TableHead><TableHead>户型</TableHead><TableHead>面积</TableHead><TableHead>成交价</TableHead><TableHead>成交日期</TableHead><TableHead>原挂牌引用</TableHead>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {detail.transactions.map((item) => (
                <tr key={item.id} className="bg-white text-slate-700">
                  <TableCell mono>{item.sourceRecordId}</TableCell><TableCell>{item.sourceRow}</TableCell><TableCell>{item.layout}</TableCell><TableCell>{item.areaSqm} ㎡</TableCell><TableCell>{item.transactionPrice} 万</TableCell><TableCell>{item.transactionDate}</TableCell><TableCell mono>{item.originalListingRef ?? "-"}</TableCell>
                </tr>
              ))}
              {detail.transactions.length === 0 ? <EmptyTableRow columns={7} /> : null}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}

function DetailPageState({
  icon: Icon,
  title,
  detail,
  action,
  spinning = false,
}: {
  icon: typeof Database;
  title: string;
  detail?: string;
  action?: React.ReactNode;
  spinning?: boolean;
}) {
  return (
    <main className="mx-auto flex min-h-[55vh] w-full max-w-3xl items-center justify-center px-4 py-12">
      <section className="w-full border border-slate-200 bg-white p-8 text-center">
        <Icon aria-hidden="true" className={`mx-auto h-8 w-8 text-slate-500 ${spinning ? "animate-spin" : ""}`} />
        <h1 className="mt-4 text-lg font-semibold text-slate-900">{title}</h1>
        {detail ? <p className="mt-2 text-sm text-slate-600">{detail}</p> : null}
        {action ? <div className="mt-5 flex justify-center">{action}</div> : null}
      </section>
    </main>
  );
}

function Definition({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className={`mt-1 break-words text-sm text-slate-900 ${mono ? "font-mono text-xs" : ""}`}>{value}</dd>
    </div>
  );
}

function SummaryStat({ label, value }: { label: string; value: number }) {
  return (
    <div className="min-w-20 bg-white px-3 py-2 text-center">
      <dt className="text-[11px] text-slate-500">{label}</dt>
      <dd className="mt-0.5 font-semibold text-slate-900">{value}</dd>
    </div>
  );
}

function TableHead({ children }: { children: React.ReactNode }) {
  return <th scope="col" className="whitespace-nowrap px-3 py-2 font-medium">{children}</th>;
}

function TableCell({ children, mono = false }: { children: React.ReactNode; mono?: boolean }) {
  return <td className={`max-w-64 break-words px-3 py-2.5 align-top ${mono ? "font-mono text-xs" : ""}`}>{children}</td>;
}

function EmptyTableRow({ columns }: { columns: number }) {
  return <tr><td colSpan={columns} className="px-3 py-8 text-center text-sm text-slate-500">无记录</td></tr>;
}

function collectionRunIDFromPath(pathname: string): string {
  const parts = pathname.split("/").filter(Boolean);
  const importsIndex = parts.lastIndexOf("imports");
  if (importsIndex < 0 || !parts[importsIndex + 1] || parts[importsIndex + 1] === "_") {
    return "";
  }
  try {
    return decodeURIComponent(parts[importsIndex + 1]);
  } catch {
    return "";
  }
}

function downloadRawPayload(detail: CollectionRunDetail) {
  const binary = atob(detail.rawPayloadBase64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  const blob = new Blob([bytes], { type: detail.collectionRun.rawContentType });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `collection-run-${detail.collectionRun.id}.${detail.collectionRun.format}`;
  anchor.click();
  URL.revokeObjectURL(url);
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function metricStatusLabel(status: CollectionRunDetail["collectionRun"]["metricStatus"]): string {
  switch (status) {
    case "completed": return "已刷新";
    case "failed": return "刷新失败";
    default: return "等待刷新";
  }
}

function listingStatusLabel(status: CollectionRunDetail["listings"][number]["status"]): string {
  switch (status) {
    case "active": return "在售";
    case "pending": return "待定";
    case "withdrawn": return "撤牌";
    case "sold": return "已售";
  }
}

const secondaryButtonClass = "inline-flex h-10 cursor-pointer items-center justify-center gap-2 rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50";
