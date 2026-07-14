"use client";

import Link from "next/link";
import {
  AlertCircle,
  CheckCircle2,
  Database,
  Download,
  ExternalLink,
  FileJson,
  FileSpreadsheet,
  LoaderCircle,
  LockKeyhole,
  Plus,
  RefreshCw,
  Search,
  Upload,
} from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createDataSource,
  createNeighborhood,
  getCSVImportTemplate,
  importCSVCollectionRun,
  importJSONCollectionRun,
  listDataSources,
  searchNeighborhoods,
  type CreateDataSourceRequest,
  type CreateNeighborhoodRequest,
  type DataSource,
  type ImportCollectionRunResponse,
  type ImportJSONRecord,
  type ImportMetadata,
  type Neighborhood,
  type ValidationIssue,
} from "@/lib/api-client";

type LoadState = "locked" | "loading" | "ready" | "failed";
type ImportState = "idle" | "submitting" | "success" | "validation_failed" | "failed";
type ImportMode = "json" | "csv";

const emptySourceDraft: CreateDataSourceRequest = {
  name: "",
  sourceType: "manual_import",
  city: "",
  notes: "",
};

const emptyNeighborhoodDraft: CreateNeighborhoodRequest = {
  name: "",
  area: "",
  targetLayout: "",
};

export function DataManagementPage() {
  const [accessChecked, setAccessChecked] = useState(false);
  const [unlocked, setUnlocked] = useState(false);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [loadError, setLoadError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [dataSources, setDataSources] = useState<DataSource[]>([]);
  const [neighborhoods, setNeighborhoods] = useState<Neighborhood[]>([]);
  const [selectedDataSourceID, setSelectedDataSourceID] = useState("");
  const [selectedNeighborhoodID, setSelectedNeighborhoodID] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [isSearching, setIsSearching] = useState(false);
  const [catalogError, setCatalogError] = useState("");
  const [showSourceForm, setShowSourceForm] = useState(false);
  const [sourceDraft, setSourceDraft] = useState(emptySourceDraft);
  const [sourceCreateError, setSourceCreateError] = useState("");
  const [isCreatingSource, setIsCreatingSource] = useState(false);
  const [showNeighborhoodForm, setShowNeighborhoodForm] = useState(false);
  const [neighborhoodDraft, setNeighborhoodDraft] = useState(emptyNeighborhoodDraft);
  const [neighborhoodCreateError, setNeighborhoodCreateError] = useState("");
  const [isCreatingNeighborhood, setIsCreatingNeighborhood] = useState(false);

  const [sourceRef, setSourceRef] = useState("");
  const [collectedAt, setCollectedAt] = useState("");
  const [coverage, setCoverage] = useState<"full" | "partial">("full");
  const [mode, setMode] = useState<ImportMode>("json");
  const [jsonText, setJSONText] = useState("");
  const [csvFile, setCSVFile] = useState<File>();
  const [importState, setImportState] = useState<ImportState>("idle");
  const [importResult, setImportResult] = useState<ImportCollectionRunResponse>();
  const [validationIssues, setValidationIssues] = useState<ValidationIssue[]>([]);
  const [rejectedRecordCount, setRejectedRecordCount] = useState(0);
  const [importError, setImportError] = useState("");
  const [templateError, setTemplateError] = useState("");
  const importController = useRef<AbortController | undefined>(undefined);

  useEffect(() => {
    const syncAccess = () => {
      const hasToken = Boolean(getAccessToken());
      setUnlocked(hasToken);
      setAccessChecked(true);
      if (!hasToken) {
        setLoadState("locked");
        setImportState("idle");
        setImportResult(undefined);
        setValidationIssues([]);
        setImportError("");
      }
    };
    syncAccess();
    return subscribeToAccessToken(syncAccess);
  }, []);

  useEffect(() => {
    if (!accessChecked || !unlocked) {
      return;
    }
    const controller = new AbortController();
    setLoadState("loading");
    setLoadError("");
    Promise.all([
      listDataSources(controller.signal),
      searchNeighborhoods("", controller.signal),
    ])
      .then(([sources, neighborhoodPage]) => {
        setDataSources(sources);
        setNeighborhoods(neighborhoodPage.items);
        setSelectedDataSourceID((current) =>
          current && sources.some((source) => source.id === current)
            ? current
            : sources[0]?.id ?? "",
        );
        setSelectedNeighborhoodID((current) =>
          current && neighborhoodPage.items.some((item) => item.id === current)
            ? current
            : neighborhoodPage.items[0]?.id ?? "",
        );
        setLoadState("ready");
      })
      .catch((error: unknown) => {
        if (isAbortError(error)) {
          return;
        }
        if (error instanceof ApiError && error.status === 401) {
          setLoadState("locked");
          return;
        }
        setLoadError("数据目录暂时无法读取。");
        setLoadState("failed");
      });
    return () => controller.abort();
  }, [accessChecked, unlocked, reloadKey]);

  const invalidateImport = () => {
    importController.current?.abort();
    importController.current = undefined;
    setImportState("idle");
    setImportResult(undefined);
    setValidationIssues([]);
    setRejectedRecordCount(0);
    setImportError("");
  };

  const selectDataSource = (id: string) => {
    setSelectedDataSourceID(id);
    invalidateImport();
  };

  const selectNeighborhood = (id: string) => {
    setSelectedNeighborhoodID(id);
    invalidateImport();
  };

  const submitSource = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsCreatingSource(true);
    setSourceCreateError("");
    try {
      const created = await createDataSource({
        ...sourceDraft,
        name: sourceDraft.name.trim(),
        sourceType: sourceDraft.sourceType.trim(),
        city: sourceDraft.city.trim(),
        notes: sourceDraft.notes?.trim(),
      });
      setDataSources((current) => [
        created,
        ...current.filter((source) => source.id !== created.id),
      ]);
      selectDataSource(created.id);
      setSourceDraft(emptySourceDraft);
      setShowSourceForm(false);
    } catch (error) {
      if (!(error instanceof ApiError && error.status === 401)) {
        setSourceCreateError(apiFailureMessage(error, "数据源创建失败。"));
      }
    } finally {
      setIsCreatingSource(false);
    }
  };

  const runNeighborhoodSearch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSearching(true);
    setCatalogError("");
    try {
      const page = await searchNeighborhoods(searchQuery);
      setNeighborhoods(page.items);
      if (!page.items.some((item) => item.id === selectedNeighborhoodID)) {
        selectNeighborhood(page.items[0]?.id ?? "");
      }
    } catch (error) {
      setCatalogError(apiFailureMessage(error, "小区搜索失败。"));
    } finally {
      setIsSearching(false);
    }
  };

  const submitNeighborhood = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsCreatingNeighborhood(true);
    setNeighborhoodCreateError("");
    try {
      const created = await createNeighborhood({
        name: neighborhoodDraft.name.trim(),
        area: neighborhoodDraft.area.trim(),
        targetLayout: neighborhoodDraft.targetLayout.trim(),
      });
      setNeighborhoods((current) => [
        created,
        ...current.filter((item) => item.id !== created.id),
      ]);
      selectNeighborhood(created.id);
      setNeighborhoodDraft(emptyNeighborhoodDraft);
      setShowNeighborhoodForm(false);
    } catch (error) {
      if (!(error instanceof ApiError && error.status === 401)) {
        setNeighborhoodCreateError(apiFailureMessage(error, "小区创建失败。"));
      }
    } finally {
      setIsCreatingNeighborhood(false);
    }
  };

  const readJSONFile = async (file?: File) => {
    if (!file) {
      return;
    }
    invalidateImport();
    try {
      setJSONText(await file.text());
    } catch {
      setImportState("failed");
      setImportError("JSON 文件读取失败。");
    }
  };

  const downloadTemplate = async () => {
    setTemplateError("");
    try {
      const blob = await getCSVImportTemplate();
      downloadBlob(blob, "propulse-import-template.csv");
    } catch (error) {
      if (!(error instanceof ApiError && error.status === 401)) {
        setTemplateError(apiFailureMessage(error, "CSV 模板下载失败。"));
      }
    }
  };

  const submitImport = async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    importController.current?.abort();
    setImportResult(undefined);
    setValidationIssues([]);
    setRejectedRecordCount(0);
    setImportError("");

    const metadataResult = importMetadata({
      selectedDataSourceID,
      selectedNeighborhoodID,
      sourceRef,
      collectedAt,
      coverage,
    });
    let records: ImportJSONRecord[] = [];
    const localIssues = [...metadataResult.issues];
    if (mode === "json") {
      if (!jsonText.trim()) {
        localIssues.push(localIssue("records", "请粘贴记录数组或选择 JSON 文件。"));
      } else {
        try {
          const parsed: unknown = JSON.parse(jsonText);
          if (!Array.isArray(parsed)) {
            localIssues.push(localIssue("records", "JSON 顶层必须是记录数组。"));
          } else {
            records = parsed as ImportJSONRecord[];
          }
        } catch {
          localIssues.push(localIssue("records", "JSON 内容无法解析。"));
        }
      }
    } else if (!csvFile) {
      localIssues.push(localIssue("file", "请选择 CSV 文件。"));
    }
    if (localIssues.length > 0 || !metadataResult.metadata) {
      setValidationIssues(localIssues);
      setRejectedRecordCount(records.length);
      setImportState("validation_failed");
      return;
    }

    const controller = new AbortController();
    importController.current = controller;
    setImportState("submitting");
    try {
      const result =
        mode === "json"
          ? await importJSONCollectionRun(
              { ...metadataResult.metadata, records },
              controller.signal,
            )
          : await importCSVCollectionRun(
              metadataResult.metadata,
              csvFile as File,
              controller.signal,
            );
      if (importController.current !== controller) {
        return;
      }
      setImportResult(result);
      setImportState("success");
    } catch (error) {
      if (isAbortError(error) || importController.current !== controller) {
        return;
      }
      if (error instanceof ApiError && error.status === 401) {
        setImportState("idle");
        return;
      }
      if (error instanceof ApiError && error.code === "validation_failed") {
        setValidationIssues(error.details);
        setRejectedRecordCount(error.rejectedRecordCount ?? records.length);
        setImportState("validation_failed");
        return;
      }
      setImportError(apiFailureMessage(error, "导入请求失败，批次未创建。"));
      setImportState("failed");
    } finally {
      if (importController.current === controller) {
        importController.current = undefined;
      }
    }
  };

  if (!accessChecked || loadState === "loading") {
    return <PageState icon={LoaderCircle} title="正在读取数据目录" spinning />;
  }
  if (!unlocked || loadState === "locked") {
    return <PageState icon={LockKeyhole} title="数据管理已锁定" detail="请先解锁个人空间。" />;
  }
  if (loadState === "failed") {
    return (
      <PageState
        icon={AlertCircle}
        title="数据目录读取失败"
        detail={loadError}
        action={
          <button type="button" onClick={() => setReloadKey((value) => value + 1)} className={secondaryButtonClass}>
            <RefreshCw aria-hidden="true" className="h-4 w-4" />
            重试
          </button>
        }
      />
    );
  }

  const catalogEmpty = dataSources.length === 0 || neighborhoods.length === 0;

  return (
    <main className="mx-auto w-full max-w-7xl px-4 py-7 sm:px-6 lg:px-8">
      <div className="mb-6 flex flex-col justify-between gap-3 border-b border-slate-200 pb-5 sm:flex-row sm:items-end">
        <div>
          <h1 className="text-2xl font-bold text-slate-950">数据管理</h1>
          <p className="mt-1 text-sm text-slate-500">可信采集批次</p>
        </div>
        <div className="flex items-center gap-2 text-xs text-slate-500">
          <Database aria-hidden="true" className="h-4 w-4 text-emerald-600" />
          已连接受保护数据目录
        </div>
      </div>

      {catalogEmpty ? (
        <div className="mb-5 border-l-4 border-amber-400 bg-amber-50 px-4 py-3 text-sm text-amber-900" data-testid="catalog-empty-state">
          数据目录尚未齐备，请创建缺少的数据源或小区。
        </div>
      ) : null}

      <section aria-labelledby="catalog-title" className="border-b border-slate-200 pb-7">
        <div className="mb-4 flex items-center justify-between">
          <h2 id="catalog-title" className="text-base font-semibold text-slate-900">来源与小区</h2>
        </div>
        <div className="grid gap-6 lg:grid-cols-2">
          <div>
            <div className="mb-2 flex items-center justify-between gap-3">
              <label htmlFor="data-source" className={labelClass}>数据源</label>
              <button type="button" onClick={() => setShowSourceForm((value) => !value)} className={iconTextButtonClass}>
                <Plus aria-hidden="true" className="h-4 w-4" />
                新建
              </button>
            </div>
            <select
              id="data-source"
              value={selectedDataSourceID}
              onChange={(event) => selectDataSource(event.target.value)}
              className={inputClass}
            >
              <option value="">选择数据源</option>
              {dataSources.map((source) => (
                <option key={source.id} value={source.id}>{source.name} · {source.city}</option>
              ))}
            </select>
            {showSourceForm ? (
              <form onSubmit={submitSource} className="mt-3 grid gap-3 border-l-2 border-blue-200 pl-4 sm:grid-cols-2">
                <Field label="名称" value={sourceDraft.name} onChange={(value) => setSourceDraft((current) => ({ ...current, name: value }))} required />
                <Field label="城市" value={sourceDraft.city} onChange={(value) => setSourceDraft((current) => ({ ...current, city: value }))} required />
                <Field label="来源类型" value={sourceDraft.sourceType} onChange={(value) => setSourceDraft((current) => ({ ...current, sourceType: value }))} required />
                <Field label="备注" value={sourceDraft.notes ?? ""} onChange={(value) => setSourceDraft((current) => ({ ...current, notes: value }))} />
                {sourceCreateError ? <p role="alert" className="text-sm text-rose-700 sm:col-span-2">{sourceCreateError}</p> : null}
                <div className="flex gap-2 sm:col-span-2">
                  <button type="submit" disabled={isCreatingSource} className={primaryButtonClass}>
                    {isCreatingSource ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Plus aria-hidden="true" className="h-4 w-4" />}
                    创建数据源
                  </button>
                  <button type="button" onClick={() => setShowSourceForm(false)} className={secondaryButtonClass}>取消</button>
                </div>
              </form>
            ) : null}
          </div>

          <div>
            <div className="mb-2 flex items-center justify-between gap-3">
              <label htmlFor="neighborhood" className={labelClass}>小区</label>
              <button type="button" onClick={() => setShowNeighborhoodForm((value) => !value)} className={iconTextButtonClass}>
                <Plus aria-hidden="true" className="h-4 w-4" />
                新建
              </button>
            </div>
            <form onSubmit={runNeighborhoodSearch} className="mb-2 flex gap-2">
              <div className="relative min-w-0 flex-1">
                <Search aria-hidden="true" className="absolute left-3 top-3 h-4 w-4 text-slate-400" />
                <input aria-label="搜索小区" value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} className={`${inputClass} pl-9`} />
              </div>
              <button type="submit" disabled={isSearching} aria-label="执行小区搜索" className={squareButtonClass}>
                {isSearching ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Search aria-hidden="true" className="h-4 w-4" />}
              </button>
            </form>
            <select
              id="neighborhood"
              value={selectedNeighborhoodID}
              onChange={(event) => selectNeighborhood(event.target.value)}
              className={inputClass}
            >
              <option value="">选择小区</option>
              {neighborhoods.map((item) => (
                <option key={item.id} value={item.id}>{item.name} · {item.area} · {item.targetLayout}</option>
              ))}
            </select>
            {neighborhoods.length === 0 && searchQuery.trim() ? (
              <p className="mt-2 text-sm text-slate-500">没有匹配小区。</p>
            ) : null}
            {catalogError ? <p role="alert" className="mt-2 text-sm text-rose-700">{catalogError}</p> : null}
            {showNeighborhoodForm ? (
              <form onSubmit={submitNeighborhood} className="mt-3 grid gap-3 border-l-2 border-blue-200 pl-4 sm:grid-cols-3">
                <Field label="小区名称" value={neighborhoodDraft.name} onChange={(value) => setNeighborhoodDraft((current) => ({ ...current, name: value }))} required />
                <Field label="区域" value={neighborhoodDraft.area} onChange={(value) => setNeighborhoodDraft((current) => ({ ...current, area: value }))} required />
                <Field label="关注户型" value={neighborhoodDraft.targetLayout} onChange={(value) => setNeighborhoodDraft((current) => ({ ...current, targetLayout: value }))} required />
                {neighborhoodCreateError ? <p role="alert" className="text-sm text-rose-700 sm:col-span-3">{neighborhoodCreateError}</p> : null}
                <div className="flex gap-2 sm:col-span-3">
                  <button type="submit" disabled={isCreatingNeighborhood} className={primaryButtonClass}>
                    {isCreatingNeighborhood ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Plus aria-hidden="true" className="h-4 w-4" />}
                    创建小区
                  </button>
                  <button type="button" onClick={() => setShowNeighborhoodForm(false)} className={secondaryButtonClass}>取消</button>
                </div>
              </form>
            ) : null}
          </div>
        </div>
      </section>

      <form onSubmit={submitImport} className="pt-7">
        <div className="mb-5 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <h2 className="text-base font-semibold text-slate-900">新建采集批次</h2>
          <div className="inline-flex w-fit rounded-md border border-slate-300 bg-slate-100 p-1" role="group" aria-label="导入格式">
            <ModeButton active={mode === "json"} onClick={() => { setMode("json"); invalidateImport(); }} icon={FileJson}>JSON</ModeButton>
            <ModeButton active={mode === "csv"} onClick={() => { setMode("csv"); invalidateImport(); }} icon={FileSpreadsheet}>CSV</ModeButton>
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <label className={labelClass}>
            来源引用
            <input value={sourceRef} onChange={(event) => { setSourceRef(event.target.value); invalidateImport(); }} className={`${inputClass} mt-1.5`} />
          </label>
          <label className={labelClass}>
            采集时间
            <input type="datetime-local" value={collectedAt} onChange={(event) => { setCollectedAt(event.target.value); invalidateImport(); }} className={`${inputClass} mt-1.5`} />
          </label>
          <label className={labelClass}>
            覆盖范围
            <select value={coverage} onChange={(event) => { setCoverage(event.target.value as "full" | "partial"); invalidateImport(); }} className={`${inputClass} mt-1.5`}>
              <option value="full">完整覆盖</option>
              <option value="partial">部分覆盖</option>
            </select>
          </label>
          <div className={labelClass}>
            当前格式
            <div className="mt-1.5 flex h-10 items-center gap-2 border-b border-slate-300 px-1 text-sm text-slate-700">
              {mode === "json" ? <FileJson aria-hidden="true" className="h-4 w-4" /> : <FileSpreadsheet aria-hidden="true" className="h-4 w-4" />}
              {mode.toUpperCase()}
            </div>
          </div>
        </div>

        <div className="mt-6 border border-slate-200 bg-white p-4 sm:p-5">
          {mode === "json" ? (
            <div>
              <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                <label htmlFor="json-records" className={labelClass}>记录数组</label>
                <label className={secondaryButtonClass}>
                  <Upload aria-hidden="true" className="h-4 w-4" />
                  选择 JSON
                  <input type="file" accept="application/json,.json" className="sr-only" onChange={(event) => void readJSONFile(event.target.files?.[0])} />
                </label>
              </div>
              <textarea
                id="json-records"
                value={jsonText}
                onChange={(event) => { setJSONText(event.target.value); invalidateImport(); }}
                spellCheck={false}
                className="h-64 w-full resize-y border border-slate-300 bg-slate-50 p-3 font-mono text-xs leading-5 text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              />
            </div>
          ) : (
            <div className="flex min-h-52 flex-col items-center justify-center border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-center">
              <FileSpreadsheet aria-hidden="true" className="h-9 w-9 text-slate-400" />
              <p className="mt-3 text-sm font-medium text-slate-800">{csvFile?.name ?? "未选择 CSV 文件"}</p>
              <div className="mt-4 flex flex-wrap justify-center gap-2">
                <label className={primaryButtonClass}>
                  <Upload aria-hidden="true" className="h-4 w-4" />
                  选择 CSV
                  <input type="file" accept="text/csv,.csv" className="sr-only" onChange={(event) => { setCSVFile(event.target.files?.[0]); invalidateImport(); }} />
                </label>
                <button type="button" onClick={() => void downloadTemplate()} className={secondaryButtonClass}>
                  <Download aria-hidden="true" className="h-4 w-4" />
                  下载模板
                </button>
              </div>
              {templateError ? <p role="alert" className="mt-3 text-sm text-rose-700">{templateError}</p> : null}
            </div>
          )}
        </div>

        <div className="mt-4 flex justify-end">
          <button type="submit" disabled={importState === "submitting"} className={primaryButtonClass}>
            {importState === "submitting" ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Upload aria-hidden="true" className="h-4 w-4" />}
            {importState === "submitting" ? "正在导入" : "创建批次"}
          </button>
        </div>
      </form>

      <ImportFeedback
        state={importState}
        result={importResult}
        issues={validationIssues}
        rejectedRecordCount={rejectedRecordCount}
        error={importError}
        onRetry={() => void submitImport()}
      />
    </main>
  );
}

function ImportFeedback({
  state,
  result,
  issues,
  rejectedRecordCount,
  error,
  onRetry,
}: {
  state: ImportState;
  result?: ImportCollectionRunResponse;
  issues: ValidationIssue[];
  rejectedRecordCount: number;
  error: string;
  onRetry: () => void;
}) {
  if (state === "validation_failed") {
    return (
      <section aria-labelledby="validation-title" className="mt-6 border-l-4 border-rose-500 bg-rose-50 p-4" role="alert">
        <div className="flex items-start gap-3">
          <AlertCircle aria-hidden="true" className="mt-0.5 h-5 w-5 flex-none text-rose-700" />
          <div className="min-w-0 flex-1">
            <h2 id="validation-title" className="font-semibold text-rose-950">导入校验未通过</h2>
            <p className="mt-1 text-sm text-rose-800">接受 0 条，拒绝 {rejectedRecordCount} 条。</p>
            <ul className="mt-3 divide-y divide-rose-200 border-t border-rose-200 text-sm text-rose-900">
              {issues.map((issue, index) => (
                <li key={`${issue.row ?? "request"}-${issue.field}-${issue.code}-${index}`} className="grid gap-1 py-2 sm:grid-cols-[8rem_10rem_1fr]">
                  <span>{issue.row ? `第 ${issue.row} 行` : "请求字段"}</span>
                  <span className="font-mono text-xs">{issue.field}</span>
                  <span>{issue.message}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </section>
    );
  }
  if (state === "failed") {
    return (
      <section className="mt-6 flex flex-col justify-between gap-3 border-l-4 border-amber-500 bg-amber-50 p-4 sm:flex-row sm:items-center" role="alert">
        <div>
          <h2 className="font-semibold text-amber-950">导入请求失败</h2>
          <p className="mt-1 text-sm text-amber-800">{error}</p>
        </div>
        <button type="button" onClick={onRetry} className={secondaryButtonClass}>
          <RefreshCw aria-hidden="true" className="h-4 w-4" />
          重试
        </button>
      </section>
    );
  }
  if (state !== "success" || !result) {
    return null;
  }
  return (
    <section aria-labelledby="success-title" className="mt-6 border border-emerald-200 bg-emerald-50 p-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
        <div className="flex gap-3">
          <CheckCircle2 aria-hidden="true" className="mt-0.5 h-5 w-5 flex-none text-emerald-700" />
          <div>
            <h2 id="success-title" className="font-semibold text-emerald-950">批次导入成功</h2>
            <p className="mt-1 break-all font-mono text-xs text-emerald-800">{result.collectionRunId}</p>
          </div>
        </div>
        <Link href={`/data/imports/${result.collectionRunId}`} className={secondaryButtonClass}>
          查看批次
          <ExternalLink aria-hidden="true" className="h-4 w-4" />
        </Link>
      </div>
      <dl className="mt-5 grid grid-cols-2 gap-px overflow-hidden border border-emerald-200 bg-emerald-200 sm:grid-cols-5">
        <ResultStat label="接受记录" value={String(result.acceptedRecordCount)} />
        <ResultStat label="挂牌观察" value={String(result.listingObservationCount)} />
        <ResultStat label="成交观察" value={String(result.transactionObservationCount)} />
        <ResultStat label="导入类型" value={result.idempotentReplay ? "幂等重放" : "新批次"} />
        <ResultStat label="指标刷新" value={metricStatusLabel(result.metricRefreshStatus)} />
      </dl>
    </section>
  );
}

function ResultStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-white px-3 py-3">
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className="mt-1 text-sm font-semibold text-slate-900">{value}</dd>
    </div>
  );
}

function PageState({
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

function Field({
  label,
  value,
  onChange,
  required = false,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
}) {
  return (
    <label className={labelClass}>
      {label}
      <input value={value} onChange={(event) => onChange(event.target.value)} required={required} className={`${inputClass} mt-1`} />
    </label>
  );
}

function ModeButton({
  active,
  onClick,
  icon: Icon,
  children,
}: {
  active: boolean;
  onClick: () => void;
  icon: typeof FileJson;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`inline-flex h-8 items-center gap-2 rounded px-3 text-sm font-medium ${active ? "bg-white text-slate-950 shadow-sm" : "text-slate-600 hover:text-slate-900"}`}
    >
      <Icon aria-hidden="true" className="h-4 w-4" />
      {children}
    </button>
  );
}

function importMetadata(input: {
  selectedDataSourceID: string;
  selectedNeighborhoodID: string;
  sourceRef: string;
  collectedAt: string;
  coverage: "full" | "partial";
}): { metadata?: ImportMetadata; issues: ValidationIssue[] } {
  const issues: ValidationIssue[] = [];
  if (!input.selectedDataSourceID) {
    issues.push(localIssue("dataSourceId", "请选择数据源。"));
  }
  if (!input.selectedNeighborhoodID) {
    issues.push(localIssue("neighborhoodId", "请选择小区。"));
  }
  if (!input.sourceRef.trim()) {
    issues.push(localIssue("sourceRef", "请填写来源引用。"));
  }
  const parsedDate = input.collectedAt ? new Date(input.collectedAt) : undefined;
  if (!parsedDate || Number.isNaN(parsedDate.getTime())) {
    issues.push(localIssue("collectedAt", "请填写有效采集时间。"));
  }
  if (issues.length > 0 || !parsedDate) {
    return { issues };
  }
  return {
    issues,
    metadata: {
      dataSourceId: input.selectedDataSourceID,
      neighborhoodId: input.selectedNeighborhoodID,
      sourceRef: input.sourceRef.trim(),
      collectedAt: parsedDate.toISOString(),
      coverage: input.coverage,
    },
  };
}

function localIssue(field: string, message: string): ValidationIssue {
  return { field, code: "required", message };
}

function metricStatusLabel(status: ImportCollectionRunResponse["metricRefreshStatus"]): string {
  switch (status) {
    case "completed":
      return "已刷新";
    case "failed":
      return "刷新失败";
    default:
      return "等待刷新";
  }
}

function apiFailureMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.message) {
    return error.message;
  }
  return fallback;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

const labelClass = "block text-sm font-medium text-slate-700";
const inputClass = "h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100";
const primaryButtonClass = "inline-flex h-10 cursor-pointer items-center justify-center gap-2 rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60";
const secondaryButtonClass = "inline-flex h-10 cursor-pointer items-center justify-center gap-2 rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50";
const iconTextButtonClass = "inline-flex h-8 items-center gap-1.5 rounded-md px-2 text-sm font-medium text-blue-700 hover:bg-blue-50";
const squareButtonClass = "inline-flex h-10 w-10 flex-none items-center justify-center rounded-md border border-slate-300 bg-white text-slate-700 hover:bg-slate-50 disabled:opacity-60";
