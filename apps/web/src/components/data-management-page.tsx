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
  History,
  Landmark,
  LoaderCircle,
  LockKeyhole,
  Plus,
  RefreshCw,
  Search,
  Table2,
  Trash2,
  Upload,
} from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createCapacityPolicy,
  createDataSource,
  createNeighborhood,
  getCSVImportTemplate,
  importCSVCollectionRun,
  importJSONCollectionRun,
  listCapacityPolicies,
  listCollectionRuns,
  listDataSources,
  searchNeighborhoods,
  type CollectionRunsPage,
  type CreateHousingPolicyVersionRequest,
  type CreateDataSourceRequest,
  type DataSource,
  type HousingPolicyVersion,
  type ImportCollectionRunResponse,
  type ImportJSONRecord,
  type ImportMetadata,
  type Neighborhood,
  type ValidationIssue,
} from "@/lib/api-client";
import {
  buildManualRecords,
  createManualRow,
  type ManualRow,
  type ManualRowType,
} from "@/lib/manual-import";
import { CenteredLoadingState } from "./centered-loading-state";

type LoadState = "locked" | "loading" | "ready" | "failed";
type ImportState = "idle" | "submitting" | "success" | "validation_failed" | "failed";
type ImportMode = "manual" | "json" | "csv";

const emptySourceDraft: CreateDataSourceRequest = {
  name: "",
  sourceType: "manual_import",
  city: "",
  notes: "",
};

const emptyNeighborhoodDraft = {
  name: "",
  city: "",
  area: "",
  availableLayouts: "",
};

export function DataManagementPage() {
  const [accessChecked, setAccessChecked] = useState(false);
  const [unlocked, setUnlocked] = useState(false);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [loadError, setLoadError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [dataSources, setDataSources] = useState<DataSource[]>([]);
  const [neighborhoods, setNeighborhoods] = useState<Neighborhood[]>([]);
  const [policies, setPolicies] = useState<HousingPolicyVersion[]>([]);
  const [collectionRuns, setCollectionRuns] = useState<CollectionRunsPage>({ items: [], page: 1, pageSize: 20, total: 0 });
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
  const [mode, setMode] = useState<ImportMode>("manual");
  const [manualRows, setManualRows] = useState<ManualRow[]>(() => [createManualRow("listing")]);
  const [jsonText, setJSONText] = useState("");
  const [csvFile, setCSVFile] = useState<File>();
  const [importState, setImportState] = useState<ImportState>("idle");
  const [importResult, setImportResult] = useState<ImportCollectionRunResponse>();
  const [validationIssues, setValidationIssues] = useState<ValidationIssue[]>([]);
  const [rejectedRecordCount, setRejectedRecordCount] = useState(0);
  const [importError, setImportError] = useState("");
  const [templateError, setTemplateError] = useState("");
  const [historyDataSourceID, setHistoryDataSourceID] = useState("");
  const [historyNeighborhoodID, setHistoryNeighborhoodID] = useState("");
  const [historyMetricStatus, setHistoryMetricStatus] = useState("");
  const [historyFrom, setHistoryFrom] = useState("");
  const [historyTo, setHistoryTo] = useState("");
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState("");
  const [showPolicyForm, setShowPolicyForm] = useState(false);
  const [policyJSON, setPolicyJSON] = useState("");
  const [policySubmitting, setPolicySubmitting] = useState(false);
  const [policyError, setPolicyError] = useState("");
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
      listCapacityPolicies("天津", controller.signal),
      listCollectionRuns({ page: 1, pageSize: 20 }, controller.signal),
    ])
      .then(([sources, neighborhoodPage, policyVersions, runPage]) => {
        setDataSources(sources);
        setNeighborhoods(neighborhoodPage.items);
        setPolicies(policyVersions);
        setCollectionRuns(runPage);
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

  const addManualRow = (recordType: ManualRowType) => {
    setManualRows((rows) => [...rows, createManualRow(recordType)]);
    invalidateImport();
  };

  const updateManualRow = (localId: string, patch: Partial<ManualRow>) => {
    setManualRows((rows) => rows.map((row) => (row.localId === localId ? { ...row, ...patch } : row)));
    invalidateImport();
  };

  const removeManualRow = (localId: string) => {
    setManualRows((rows) => (rows.length <= 1 ? rows : rows.filter((row) => row.localId !== localId)));
    invalidateImport();
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
        city: neighborhoodDraft.city.trim(),
        area: neighborhoodDraft.area.trim(),
        availableLayouts: [...new Set(
          neighborhoodDraft.availableLayouts
            .split(/[,，\n]/)
            .map((layout) => layout.trim())
            .filter(Boolean),
        )],
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
    if (mode === "manual") {
      const built = buildManualRecords(manualRows);
      if (built.records.length === 0 && built.issues.length === 0) {
        localIssues.push(localIssue("records", "请至少录入一条房源。"));
      }
      localIssues.push(...built.issues);
      records = built.records;
    } else if (mode === "json") {
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
        mode === "csv"
          ? await importCSVCollectionRun(
              metadataResult.metadata,
              csvFile as File,
              controller.signal,
            )
          : await importJSONCollectionRun(
              { ...metadataResult.metadata, records },
              controller.signal,
            );
      if (importController.current !== controller) {
        return;
      }
      setImportResult(result);
      setImportState("success");
      void refreshCollectionRuns(1);
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

  const refreshCollectionRuns = async (page = collectionRuns.page) => {
    setHistoryLoading(true);
    setHistoryError("");
    try {
      const next = await listCollectionRuns({
        dataSourceId: historyDataSourceID || undefined,
        neighborhoodId: historyNeighborhoodID || undefined,
        metricStatus: historyMetricStatus
          ? historyMetricStatus as "pending" | "completed" | "failed"
          : undefined,
        from: historyFrom ? new Date(`${historyFrom}T00:00:00+08:00`).toISOString() : undefined,
        to: historyTo ? new Date(`${historyTo}T23:59:59+08:00`).toISOString() : undefined,
        page,
        pageSize: collectionRuns.pageSize,
      });
      setCollectionRuns(next);
    } catch (error) {
      setHistoryError(apiFailureMessage(error, "批次历史读取失败。"));
    } finally {
      setHistoryLoading(false);
    }
  };

  const openPolicyForm = () => {
    const latest = policies[0];
    const draft: CreateHousingPolicyVersionRequest = latest
      ? {
          city: latest.city,
          effectiveFrom: latest.effectiveTo ?? "",
          effectiveTo: null,
          enabled: true,
          name: "",
          rules: latest.rules,
          sources: latest.sources,
          version: "",
        }
      : emptyPolicyDraft();
    setPolicyJSON(JSON.stringify(draft, null, 2));
    setPolicyError("");
    setShowPolicyForm(true);
  };

  const submitPolicy = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setPolicyError("");
    let draft: CreateHousingPolicyVersionRequest;
    try {
      draft = JSON.parse(policyJSON) as CreateHousingPolicyVersionRequest;
    } catch {
      setPolicyError("政策版本 JSON 无法解析。");
      return;
    }
    setPolicySubmitting(true);
    try {
      const created = await createCapacityPolicy(draft);
      setPolicies((current) => [created, ...current.filter((item) => item.id !== created.id)]);
      try {
        setPolicies(await listCapacityPolicies("天津"));
      } catch {
        // The version was created successfully; keep the optimistic list if refresh fails.
      }
      setShowPolicyForm(false);
      setPolicyJSON("");
    } catch (error) {
      setPolicyError(apiFailureMessage(error, "政策版本创建失败。"));
    } finally {
      setPolicySubmitting(false);
    }
  };

  if (!accessChecked || loadState === "loading") {
    return <CenteredLoadingState className="min-h-[55vh]" title="正在读取数据目录" />;
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

      <section aria-labelledby="workflow-title" className="mb-7 border-b border-slate-200 pb-7">
        <div className="flex items-start gap-3">
          <Database aria-hidden="true" className="mt-0.5 h-5 w-5 flex-none text-blue-700" />
          <div>
            <h2 id="workflow-title" className="text-base font-semibold text-slate-900">数据管理流程</h2>
            <p className="mt-1 max-w-4xl text-sm leading-6 text-slate-600">
              数据源表示数据出处，小区表示数据归属，两者没有固定绑定。每个采集批次把一个数据源和一个小区关联起来，再生成可审计的挂牌或成交观测。
            </p>
          </div>
        </div>
      </section>

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
                <option key={item.id} value={item.id}>{item.name} · {item.city ?? "城市未标注"} · {item.area} · {item.availableLayouts.join("/")}</option>
              ))}
            </select>
            {neighborhoods.length === 0 && searchQuery.trim() ? (
              <p className="mt-2 text-sm text-slate-500">没有匹配小区。</p>
            ) : null}
            {catalogError ? <p role="alert" className="mt-2 text-sm text-rose-700">{catalogError}</p> : null}
            {showNeighborhoodForm ? (
              <form onSubmit={submitNeighborhood} className="mt-3 grid gap-3 border-l-2 border-blue-200 pl-4 sm:grid-cols-2">
                <Field label="小区名称" value={neighborhoodDraft.name} onChange={(value) => setNeighborhoodDraft((current) => ({ ...current, name: value }))} required />
                <Field label="城市" value={neighborhoodDraft.city} onChange={(value) => setNeighborhoodDraft((current) => ({ ...current, city: value }))} required />
                <Field label="区域" value={neighborhoodDraft.area} onChange={(value) => setNeighborhoodDraft((current) => ({ ...current, area: value }))} required />
                <Field label="可选户型（逗号分隔）" value={neighborhoodDraft.availableLayouts} onChange={(value) => setNeighborhoodDraft((current) => ({ ...current, availableLayouts: value }))} required />
                {neighborhoodCreateError ? <p role="alert" className="text-sm text-rose-700 sm:col-span-2">{neighborhoodCreateError}</p> : null}
                <div className="flex gap-2 sm:col-span-2">
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
            <ModeButton active={mode === "manual"} onClick={() => { setMode("manual"); invalidateImport(); }} icon={Table2}>表单</ModeButton>
            <ModeButton active={mode === "json"} onClick={() => { setMode("json"); invalidateImport(); }} icon={FileJson}>JSON</ModeButton>
            <ModeButton active={mode === "csv"} onClick={() => { setMode("csv"); invalidateImport(); }} icon={FileSpreadsheet}>CSV</ModeButton>
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <label className={labelClass}>
            来源引用
            <input aria-label="来源引用" value={sourceRef} onChange={(event) => { setSourceRef(event.target.value); invalidateImport(); }} className={`${inputClass} mt-1.5`} />
            <span className="mt-1 block text-xs font-normal text-slate-500">来源页面、文件或查询的稳定引用。</span>
          </label>
          <label className={labelClass}>
            采集时间
            <input aria-label="采集时间" type="datetime-local" value={collectedAt} onChange={(event) => { setCollectedAt(event.target.value); invalidateImport(); }} className={`${inputClass} mt-1.5`} />
            <span className="mt-1 block text-xs font-normal text-slate-500">源数据实际采集时间，不是导入时间。</span>
          </label>
          <label className={labelClass}>
            覆盖范围
            <select value={coverage} onChange={(event) => { setCoverage(event.target.value as "full" | "partial"); invalidateImport(); }} className={`${inputClass} mt-1.5`}>
              <option value="full">完整覆盖</option>
              <option value="partial">部分覆盖</option>
            </select>
            <span className="mt-1 block text-xs font-normal text-slate-500">完整覆盖会更新当前库存；部分覆盖只增加观测。</span>
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
          {mode === "manual" ? (
            <ManualEntry
              rows={manualRows}
              onAdd={addManualRow}
              onUpdate={updateManualRow}
              onRemove={removeManualRow}
            />
          ) : mode === "json" ? (
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
            {importState === "submitting" ? "正在导入" : mode === "manual" ? "导入录入房源" : "创建批次"}
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

      <section aria-labelledby="history-title" className="mt-10 border-t border-slate-200 pt-7">
        <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-end">
          <div>
            <div className="flex items-center gap-2">
              <History aria-hidden="true" className="h-5 w-5 text-blue-700" />
              <h2 id="history-title" className="text-base font-semibold text-slate-900">采集批次历史</h2>
            </div>
            <p className="mt-1 text-sm text-slate-500">房见小区聚合快照单独保存，不计入下表的挂牌/成交采集批次。</p>
          </div>
          <span className="text-sm text-slate-500">共 {collectionRuns.total} 个批次</span>
        </div>

        <form
          className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-6"
          onSubmit={(event) => { event.preventDefault(); void refreshCollectionRuns(1); }}
        >
          <HistorySelect label="筛选数据源" value={historyDataSourceID} onChange={setHistoryDataSourceID}>
            <option value="">全部数据源</option>
            {dataSources.map((source) => <option key={source.id} value={source.id}>{source.name}</option>)}
          </HistorySelect>
          <HistorySelect label="筛选小区" value={historyNeighborhoodID} onChange={setHistoryNeighborhoodID}>
            <option value="">全部小区</option>
            {neighborhoods.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </HistorySelect>
          <HistorySelect label="指标状态" value={historyMetricStatus} onChange={setHistoryMetricStatus}>
            <option value="">全部状态</option>
            <option value="pending">等待刷新</option>
            <option value="completed">已刷新</option>
            <option value="failed">刷新失败</option>
          </HistorySelect>
          <HistoryDate label="开始日期" value={historyFrom} onChange={setHistoryFrom} />
          <HistoryDate label="结束日期" value={historyTo} onChange={setHistoryTo} />
          <button type="submit" disabled={historyLoading} className={`${secondaryButtonClass} self-end`}>
            {historyLoading ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Search aria-hidden="true" className="h-4 w-4" />}
            筛选
          </button>
        </form>

        {historyError ? <p role="alert" className="mt-4 text-sm text-rose-700">{historyError}</p> : null}
        <CollectionRunTable page={collectionRuns} />
        {collectionRuns.total > collectionRuns.pageSize ? (
          <div className="mt-4 flex items-center justify-end gap-2">
            <button type="button" disabled={historyLoading || collectionRuns.page <= 1} onClick={() => void refreshCollectionRuns(collectionRuns.page - 1)} className={secondaryButtonClass}>上一页</button>
            <span className="text-sm text-slate-500">第 {collectionRuns.page} 页</span>
            <button type="button" disabled={historyLoading || collectionRuns.page * collectionRuns.pageSize >= collectionRuns.total} onClick={() => void refreshCollectionRuns(collectionRuns.page + 1)} className={secondaryButtonClass}>下一页</button>
          </div>
        ) : null}
      </section>

      <section aria-labelledby="policy-title" className="mt-10 border-t border-slate-200 pt-7">
        <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
          <div>
            <div className="flex items-center gap-2">
              <Landmark aria-hidden="true" className="h-5 w-5 text-blue-700" />
              <h2 id="policy-title" className="text-base font-semibold text-slate-900">测算政策</h2>
            </div>
            <p className="mt-1 text-sm text-slate-500">版本只追加、不覆盖；未来生效版本可提前启用。</p>
          </div>
          <button type="button" onClick={openPolicyForm} className={primaryButtonClass}>
            <Plus aria-hidden="true" className="h-4 w-4" />
            录入新版本
          </button>
        </div>

        {showPolicyForm ? (
          <form onSubmit={submitPolicy} className="mt-5 border-l-2 border-blue-300 pl-4">
            <label htmlFor="policy-json" className={labelClass}>政策版本 JSON</label>
            <textarea
              id="policy-json"
              value={policyJSON}
              onChange={(event) => setPolicyJSON(event.target.value)}
              spellCheck={false}
              className="mt-2 h-80 w-full resize-y border border-slate-300 bg-slate-50 p-3 font-mono text-xs leading-5 text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            />
            {policyError ? <p role="alert" className="mt-2 text-sm text-rose-700">{policyError}</p> : null}
            <div className="mt-3 flex gap-2">
              <button type="submit" disabled={policySubmitting} className={primaryButtonClass}>
                {policySubmitting ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Plus aria-hidden="true" className="h-4 w-4" />}
                创建政策版本
              </button>
              <button type="button" onClick={() => setShowPolicyForm(false)} className={secondaryButtonClass}>取消</button>
            </div>
          </form>
        ) : null}

        <PolicyVersionTable policies={policies} />
      </section>
    </main>
  );
}

function CollectionRunTable({ page }: { page: CollectionRunsPage }) {
  if (page.items.length === 0) {
    return (
      <div className="mt-5 border border-dashed border-slate-300 px-4 py-10 text-center text-sm text-slate-500">
        尚未创建挂牌/成交采集批次
      </div>
    );
  }
  return (
    <div className="mt-5 overflow-x-auto border border-slate-200">
      <table className="min-w-full divide-y divide-slate-200 text-left text-sm">
        <thead className="bg-slate-50 text-xs text-slate-500">
          <tr>
            <th className="px-3 py-2.5 font-medium">批次</th>
            <th className="px-3 py-2.5 font-medium">来源与归属</th>
            <th className="px-3 py-2.5 font-medium">采集时间</th>
            <th className="px-3 py-2.5 font-medium">覆盖</th>
            <th className="px-3 py-2.5 font-medium">记录</th>
            <th className="px-3 py-2.5 font-medium">指标</th>
            <th className="px-3 py-2.5 font-medium"><span className="sr-only">操作</span></th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 bg-white text-slate-700">
          {page.items.map((item) => (
            <tr key={item.collectionRun.id}>
              <td className="whitespace-nowrap px-3 py-3 font-mono text-xs text-slate-600">{item.collectionRun.id.slice(0, 8)}</td>
              <td className="px-3 py-3">
                <p className="font-medium text-slate-900">{item.source.name}</p>
                <p className="mt-0.5 text-xs text-slate-500">{item.neighborhoodName}</p>
              </td>
              <td className="whitespace-nowrap px-3 py-3 text-xs">{formatDateTime(item.collectionRun.collectedAt)}</td>
              <td className="whitespace-nowrap px-3 py-3">{item.collectionRun.coverage === "full" ? "完整覆盖" : "部分覆盖"}</td>
              <td className="whitespace-nowrap px-3 py-3 text-xs">共 {item.recordCount} · 挂牌 {item.listingCount} · 成交 {item.transactionCount}</td>
              <td className="whitespace-nowrap px-3 py-3">{collectionMetricStatusLabel(item.collectionRun.metricStatus)}</td>
              <td className="whitespace-nowrap px-3 py-3 text-right">
                <Link href={item.detailHref} className="font-medium text-blue-700 hover:text-blue-900">详情</Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PolicyVersionTable({ policies }: { policies: HousingPolicyVersion[] }) {
  if (policies.length === 0) {
    return <p className="mt-5 border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500">尚未录入测算政策版本。</p>;
  }
  const today = new Date().toISOString().slice(0, 10);
  return (
    <div className="mt-5 overflow-x-auto border border-slate-200">
      <table className="min-w-full divide-y divide-slate-200 text-left text-sm">
        <thead className="bg-slate-50 text-xs text-slate-500">
          <tr>
            <th className="px-3 py-2.5 font-medium">版本</th>
            <th className="px-3 py-2.5 font-medium">生效区间</th>
            <th className="px-3 py-2.5 font-medium">状态</th>
            <th className="px-3 py-2.5 font-medium">官方来源</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 bg-white text-slate-700">
          {policies.map((policy) => (
            <tr key={policy.id}>
              <td className="px-3 py-3">
                <p className="font-medium text-slate-900">{policy.name}</p>
                <p className="mt-0.5 font-mono text-xs text-slate-500">{policy.version}</p>
              </td>
              <td className="whitespace-nowrap px-3 py-3 text-xs">{policy.effectiveFrom} 至 {policy.effectiveTo ?? "持续有效"}</td>
              <td className="whitespace-nowrap px-3 py-3">{!policy.enabled ? "未启用" : policy.effectiveFrom > today ? "未来生效" : policy.effectiveTo && policy.effectiveTo <= today ? "已结束" : "当前有效"}</td>
              <td className="px-3 py-3">
                <ul className="space-y-1">
                  {policy.sources.map((source) => (
                    <li key={`${policy.id}-${source.code}`}>
                      <a href={source.url} target="_blank" rel="noreferrer" className="text-blue-700 hover:text-blue-900">{source.issuer} · {source.title}</a>
                    </li>
                  ))}
                </ul>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function HistorySelect({
  children,
  label,
  onChange,
  value,
}: {
  children: React.ReactNode;
  label: string;
  onChange: (value: string) => void;
  value: string;
}) {
  return (
    <label className={labelClass}>
      {label}
      <select value={value} onChange={(event) => onChange(event.target.value)} className={`${inputClass} mt-1`}>{children}</select>
    </label>
  );
}

function HistoryDate({ label, onChange, value }: { label: string; onChange: (value: string) => void; value: string }) {
  return (
    <label className={labelClass}>
      {label}
      <input type="date" value={value} onChange={(event) => onChange(event.target.value)} className={`${inputClass} mt-1`} />
    </label>
  );
}

function ManualEntry({
  rows,
  onAdd,
  onUpdate,
  onRemove,
}: {
  rows: ManualRow[];
  onAdd: (recordType: ManualRowType) => void;
  onUpdate: (localId: string, patch: Partial<ManualRow>) => void;
  onRemove: (localId: string) => void;
}) {
  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <label className={labelClass}>逐条录入房源</label>
        <div className="flex flex-wrap gap-2">
          <button type="button" onClick={() => onAdd("listing")} className={iconTextButtonClass}>
            <Plus aria-hidden="true" className="h-4 w-4" />
            加一条挂牌
          </button>
          <button type="button" onClick={() => onAdd("transaction")} className={iconTextButtonClass}>
            <Plus aria-hidden="true" className="h-4 w-4" />
            加一条成交
          </button>
        </div>
      </div>
      <p className="mb-3 text-xs text-slate-500">价格单位为万；挂牌填挂牌天数与状态，成交填成交日期。房源编号留空会自动生成。</p>
      <ul className="grid gap-3">
        {rows.map((row, index) => (
          <ManualRowCard
            key={row.localId}
            row={row}
            index={index + 1}
            canRemove={rows.length > 1}
            onUpdate={onUpdate}
            onRemove={onRemove}
          />
        ))}
      </ul>
    </div>
  );
}

function ManualRowCard({
  row,
  index,
  canRemove,
  onUpdate,
  onRemove,
}: {
  row: ManualRow;
  index: number;
  canRemove: boolean;
  onUpdate: (localId: string, patch: Partial<ManualRow>) => void;
  onRemove: (localId: string) => void;
}) {
  const isListing = row.recordType === "listing";
  const set = (patch: Partial<ManualRow>) => onUpdate(row.localId, patch);
  return (
    <li className="grid gap-3 border border-slate-200 bg-slate-50 p-3 sm:grid-cols-2 lg:grid-cols-4">
      <div className="flex items-end justify-between gap-2 sm:col-span-2 lg:col-span-4">
        <div className="inline-flex rounded-md border border-slate-300 bg-white p-0.5 text-xs" role="group" aria-label={`第 ${index} 条类型`}>
          <TypeToggle active={isListing} onClick={() => set({ recordType: "listing", status: row.status || "active", daysOnMarket: row.daysOnMarket || "0" })}>挂牌</TypeToggle>
          <TypeToggle active={!isListing} onClick={() => set({ recordType: "transaction" })}>成交</TypeToggle>
        </div>
        {canRemove ? (
          <button type="button" onClick={() => onRemove(row.localId)} aria-label={`删除第 ${index} 条`} className="inline-flex h-8 items-center gap-1 rounded-md px-2 text-xs font-medium text-rose-700 hover:bg-rose-50">
            <Trash2 aria-hidden="true" className="h-4 w-4" />
            删除
          </button>
        ) : null}
      </div>
      <ManualField label="户型" value={row.layout} onChange={(value) => set({ layout: value })} placeholder="三房" />
      <ManualField label="面积 (㎡)" value={row.areaSqm} onChange={(value) => set({ areaSqm: value })} placeholder="138.6" inputMode="decimal" />
      <ManualField label={isListing ? "挂牌总价 (万)" : "成交总价 (万)"} value={row.price} onChange={(value) => set({ price: value })} placeholder="178" inputMode="decimal" />
      {isListing ? (
        <>
          <ManualField label="挂牌天数" value={row.daysOnMarket} onChange={(value) => set({ daysOnMarket: value })} placeholder="45" inputMode="numeric" />
          <label className={labelClass}>
            状态
            <select value={row.status} onChange={(event) => set({ status: event.target.value as ManualRow["status"] })} className={`${inputClass} mt-1`}>
              <option value="active">在售</option>
              <option value="pending">已锁定</option>
              <option value="withdrawn">已下架</option>
              <option value="sold">已成交</option>
            </select>
          </label>
        </>
      ) : (
        <>
          <ManualField label="成交日期" value={row.transactionDate} onChange={(value) => set({ transactionDate: value })} type="date" />
          <ManualField label="原挂牌编号 (选填)" value={row.originalListingRef} onChange={(value) => set({ originalListingRef: value })} placeholder="留空可不填" />
        </>
      )}
      <ManualField label="房源编号 (选填)" value={row.sourceRecordId} onChange={(value) => set({ sourceRecordId: value })} placeholder="留空自动生成" />
    </li>
  );
}

function ManualField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
  inputMode,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
  inputMode?: "decimal" | "numeric";
}) {
  return (
    <label className={labelClass}>
      {label}
      <input
        type={type}
        inputMode={inputMode}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className={`${inputClass} mt-1`}
      />
    </label>
  );
}

function TypeToggle({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`h-7 rounded px-3 font-medium ${active ? "bg-slate-900 text-white" : "text-slate-600 hover:text-slate-900"}`}
    >
      {children}
    </button>
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

function collectionMetricStatusLabel(status: "pending" | "completed" | "failed"): string {
  return status === "completed" ? "已刷新" : status === "failed" ? "刷新失败" : "等待刷新";
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "Asia/Shanghai",
  }).format(new Date(value));
}

function emptyPolicyDraft(): CreateHousingPolicyVersionRequest {
  return {
    city: "天津",
    effectiveFrom: "",
    effectiveTo: null,
    enabled: true,
    name: "",
    rules: {
      downPayment: {
        combinedFirst: 0,
        combinedSecond: 0,
        commercialFirst: 0,
        commercialSecond: 0,
        providentFirst: 0,
        providentSecond: 0,
      },
      interest: {
        commercialFirst: 0,
        commercialSecond: 0,
        providentFirstOverFiveYears: 0,
        providentFirstUpToFiveYears: 0,
        providentSecondOverFiveYears: 0,
        providentSecondUpToFiveYears: 0,
      },
      tax: {
        deedAreaThresholdSqm: 140,
        deedFirstOverAreaRate: 0,
        deedFirstUpToAreaRate: 0,
        deedSecondOverAreaRate: 0,
        deedSecondUpToAreaRate: 0,
        incomeTaxAssessedRate: 0,
        incomeTaxExemptHoldingYears: 5,
        incomeTaxGainRate: 0,
        vatExemptHoldingYears: 2,
        vatRate: 0,
        vatSurchargeRate: 0,
      },
    },
    sources: [{ code: "", effectiveDate: "", issuer: "", title: "", url: "" }],
    version: "",
  };
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
