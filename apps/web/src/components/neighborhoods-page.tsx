"use client";

import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  ArrowLeft,
  CalendarClock,
  CheckCircle,
  Database,
  History,
  LockKeyhole,
  MapPin,
  Plus,
  RefreshCw,
  Search,
} from "lucide-react";
import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  type TooltipContentProps,
} from "recharts";

import {
  addWatchlistItem,
  ApiError,
  getMetricHistory,
  getNeighborhood,
  getNeighborhoodMetrics,
  searchNeighborhoods,
  type MetricHistoryResponse,
  type Neighborhood,
  type NeighborhoodMetricResponse,
  type NeighborhoodSearchResponse,
} from "@/lib/api-client";
import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";

import { StatusBadge } from "./status-badge";

type DetailPageState = "loading" | "not_found" | "select_layout" | "metric_loading" | "no_metric" | "ready" | "failed";
type CatalogState = "loading" | "ready" | "failed";
type SubmitState = "idle" | "locked" | "submitting" | "duplicate" | "failed";

type AddSelection = {
  area: string;
  city: string;
  neighborhoodId: string;
  q: string;
  targetLayout: string;
};

type NeighborhoodView = {
  history?: MetricHistoryResponse;
  historyFailed: boolean;
  metric?: NeighborhoodMetricResponse;
  neighborhood?: Neighborhood;
};

type TrendPoint = {
  collectedAt: string;
  coverage: "full" | "partial";
  label: string;
  listedHomes: number;
  priceCutHomes: number;
  sourceRef: string;
  transactionCount: number;
};

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const emptySelection: AddSelection = {
  area: "",
  city: "",
  neighborhoodId: "",
  q: "",
  targetLayout: "",
};

interface NeighborhoodsPageProps {
  initialNeighborhoodId?: string;
  navigate?: (href: string) => void;
}

export function NeighborhoodsPage({
  initialNeighborhoodId,
  navigate = (href) => window.location.assign(href),
}: NeighborhoodsPageProps) {
  const [routeReady, setRouteReady] = useState(initialNeighborhoodId !== undefined);
  const [neighborhoodId, setNeighborhoodId] = useState(initialNeighborhoodId?.trim() ?? "");

  useEffect(() => {
    if (initialNeighborhoodId !== undefined) {
      setNeighborhoodId(initialNeighborhoodId.trim());
      setRouteReady(true);
      return;
    }

    const syncRoute = () => {
      setNeighborhoodId(new URLSearchParams(window.location.search).get("id")?.trim() ?? "");
      setRouteReady(true);
    };
    syncRoute();
    window.addEventListener("popstate", syncRoute);
    return () => window.removeEventListener("popstate", syncRoute);
  }, [initialNeighborhoodId]);

  if (!routeReady) {
    return <PageStateBand icon={Database} title="正在读取目标小区" tone="slate" />;
  }
  if (!neighborhoodId) {
    return <NeighborhoodSelector navigate={navigate} />;
  }

  return <NeighborhoodDetail key={neighborhoodId} neighborhoodId={neighborhoodId} navigate={navigate} />;
}

function NeighborhoodSelector({ navigate }: { navigate: (href: string) => void }) {
  const [selection, setSelection] = useState<AddSelection>(() => readAddSelection());
  const [queryDraft, setQueryDraft] = useState(() => readAddSelection().q);
  const [results, setResults] = useState<Neighborhood[]>([]);
  const [filters, setFilters] = useState<{ cities: string[]; areas: { city: string; area: string }[] }>({ cities: [], areas: [] });
  const [catalogState, setCatalogState] = useState<CatalogState>("loading");
  const [invalidSelection, setInvalidSelection] = useState<string>();
  const [requestVersion, setRequestVersion] = useState(0);

  const commitSelection = useCallback((next: AddSelection, method: "push" | "replace" = "push") => {
    setSelection(next);
    writeAddSelection(next, method);
  }, []);

  useEffect(() => {
    const syncSelection = () => {
      const next = readAddSelection();
      setSelection(next);
      setQueryDraft(next.q);
      setInvalidSelection(undefined);
    };
    window.addEventListener("popstate", syncSelection);
    return () => window.removeEventListener("popstate", syncSelection);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    setCatalogState("loading");

    const selectedRequest = selection.neighborhoodId
      ? getNeighborhood(selection.neighborhoodId, controller.signal)
      : Promise.resolve<Neighborhood | undefined>(undefined);

    Promise.allSettled([
      searchNeighborhoods({
        area: selection.area,
        city: selection.city,
        page: 1,
        pageSize: 100,
        q: selection.q,
      }, controller.signal),
      selectedRequest,
    ]).then(([searchResult, selectedResult]) => {
      if (controller.signal.aborted) return;
      if (searchResult.status === "rejected") {
        if (!isAbortError(searchResult.reason)) {
          setResults([]);
          setCatalogState("failed");
        }
        return;
      }

      let selectedNeighborhood: Neighborhood | undefined;
      let selectedMissing = false;
      if (selectedResult.status === "fulfilled") {
        selectedNeighborhood = selectedResult.value;
      } else if (isNotFound(selectedResult.reason)) {
        selectedMissing = true;
      } else if (!isAbortError(selectedResult.reason)) {
        setResults([]);
        setCatalogState("failed");
        return;
      }

      const reconciled = reconcileCatalogSelection(
        selection,
        searchResult.value,
        selectedNeighborhood,
        selectedMissing,
      );
      setFilters(searchResult.value.filters);
      setResults(reconciled.results);
      if (reconciled.message) setInvalidSelection(reconciled.message);
      setCatalogState("ready");
      if (!sameSelection(reconciled.selection, selection)) {
        commitSelection(reconciled.selection, "replace");
      }
    });

    return () => controller.abort();
  }, [
    commitSelection,
    requestVersion,
    selection,
  ]);

  const selectedNeighborhood = results.find((item) => item.id === selection.neighborhoodId);
  const areaOptions = filters.areas.filter((item) => item.city === selection.city);
  const submitSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setInvalidSelection(undefined);
    commitSelection({
      ...selection,
      neighborhoodId: "",
      q: queryDraft.trim(),
      targetLayout: "",
    });
  };

  return (
    <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
      <header className="border-b border-slate-200 pb-6">
        <p className="text-sm font-semibold text-blue-700">目标小区</p>
        <h1 className="mt-1 text-3xl font-bold text-slate-900">添加目标小区</h1>
      </header>

      <section aria-label="目标小区筛选" className="grid gap-5 border-b border-slate-200 py-6 sm:grid-cols-2">
        <form onSubmit={submitSearch} className="sm:col-span-2">
          <label className="mb-2 block text-sm font-medium text-slate-700" htmlFor="neighborhood-search">小区名称</label>
          <div className="flex max-w-2xl gap-2">
            <input
              id="neighborhood-search"
              value={queryDraft}
              onChange={(event) => setQueryDraft(event.target.value)}
              placeholder="输入小区名称"
              className="h-11 min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            />
            <button type="submit" className="inline-flex h-11 flex-none items-center gap-2 rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800">
              <Search aria-hidden="true" className="h-4 w-4" />
              搜索
            </button>
          </div>
        </form>

        <NativeSelect
          id="target-city"
          label="城市"
          value={selection.city}
          disabled={catalogState !== "ready"}
          placeholder="选择城市"
          options={filters.cities}
          onChange={(city) => {
            setInvalidSelection(undefined);
            commitSelection({ ...selection, area: "", city, neighborhoodId: "", targetLayout: "" });
          }}
        />
        <NativeSelect
          id="target-area"
          label="板块"
          value={selection.area}
          disabled={!selection.city || catalogState !== "ready"}
          placeholder="选择板块"
          options={areaOptions.map((item) => item.area)}
          onChange={(area) => {
            setInvalidSelection(undefined);
            commitSelection({ ...selection, area, neighborhoodId: "", targetLayout: "" });
          }}
        />
        <div className="sm:col-span-2">
          <label className="mb-2 block text-sm font-medium text-slate-700" htmlFor="target-neighborhood">小区</label>
          <select
            id="target-neighborhood"
            value={selection.neighborhoodId}
            disabled={!selection.city || !selection.area || catalogState !== "ready"}
            onChange={(event) => {
              setInvalidSelection(undefined);
              commitSelection({ ...selection, neighborhoodId: event.target.value, targetLayout: "" });
            }}
            className="h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500"
          >
            <option value="">选择小区</option>
            {results.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </div>
      </section>

      {catalogState === "loading" ? <PageStateBand icon={Database} title="正在加载小区目录" tone="slate" /> : null}
      {catalogState === "failed" ? (
        <PageStateBand
          icon={AlertTriangle}
          title="小区搜索失败"
          detail="当前筛选条件没有返回可用目录。"
          tone="rose"
          action={<RetryButton onClick={() => setRequestVersion((version) => version + 1)} />}
        />
      ) : null}
      {catalogState === "ready" && selection.city && selection.area && results.length === 0 ? (
        <PageStateBand icon={Search} title="没有匹配的小区" detail="可以调整名称、城市或板块后重新搜索。" tone="slate" />
      ) : null}
      {invalidSelection ? <PageStateBand icon={AlertTriangle} title="原选择已失效" detail={invalidSelection} tone="amber" /> : null}

      <WatchlistTargetAction
        canSubmit={Boolean(selection.city && selection.area && selectedNeighborhood)}
        neighborhood={selectedNeighborhood}
        targetLayout={selection.targetLayout}
        navigate={navigate}
        onCancel={() => navigate("/")}
        onTargetLayoutChange={(targetLayout) => {
          setInvalidSelection(undefined);
          commitSelection({ ...selection, targetLayout });
        }}
      />
    </main>
  );
}

function NeighborhoodDetail({ neighborhoodId, navigate }: { neighborhoodId: string; navigate: (href: string) => void }) {
  const [pageState, setPageState] = useState<DetailPageState>("loading");
  const [view, setView] = useState<NeighborhoodView>({ historyFailed: false });
  const [targetLayout, setTargetLayout] = useState(() => readDetailTargetLayout());
  const [invalidSelection, setInvalidSelection] = useState<string>();
  const [requestVersion, setRequestVersion] = useState(0);

  useEffect(() => {
    const syncTarget = () => {
      setTargetLayout(readDetailTargetLayout());
      setInvalidSelection(undefined);
    };
    window.addEventListener("popstate", syncTarget);
    return () => window.removeEventListener("popstate", syncTarget);
  }, []);

  useEffect(() => {
    if (!uuidPattern.test(neighborhoodId)) {
      setView({ historyFailed: false });
      setPageState("not_found");
      return;
    }

    const controller = new AbortController();
    setView({ historyFailed: false });
    setPageState("loading");
    getNeighborhood(neighborhoodId, controller.signal)
      .then((neighborhood) => {
        if (controller.signal.aborted) return;
        setView({ historyFailed: false, neighborhood });
        setPageState("select_layout");
      })
      .catch((error: unknown) => {
        if (isAbortError(error)) return;
        setPageState(isNotFound(error) ? "not_found" : "failed");
    });
    return () => controller.abort();
  }, [neighborhoodId, requestVersion]);

  useEffect(() => {
    if (!view.neighborhood) return;
    if (targetLayout && !view.neighborhood.availableLayouts.includes(targetLayout)) {
      setTargetLayout("");
      writeDetailTargetLayout(neighborhoodId, "", "replace");
      setInvalidSelection("该户型已不在当前小区目录中，请重新选择。");
      setPageState("select_layout");
      return;
    }
    setPageState(targetLayout ? "metric_loading" : "select_layout");
  }, [neighborhoodId, targetLayout, view.neighborhood]);

  useEffect(() => {
    if (!view.neighborhood || !targetLayout || pageState !== "metric_loading") return;
    const controller = new AbortController();
    Promise.allSettled([
      getNeighborhoodMetrics(neighborhoodId, targetLayout, controller.signal),
      getMetricHistory(neighborhoodId, targetLayout, {}, controller.signal),
    ]).then(([metricResult, historyResult]) => {
      if (controller.signal.aborted) return;
      if (metricResult.status === "rejected") {
        if (isNotFound(metricResult.reason)) {
          setView((current) => ({ ...current, historyFailed: historyResult.status === "rejected", metric: undefined }));
          setPageState("no_metric");
          return;
        }
        setPageState("failed");
        return;
      }
      if (
        metricResult.value.neighborhoodId !== neighborhoodId ||
        metricResult.value.targetLayout !== targetLayout ||
        (historyResult.status === "fulfilled" && (
          historyResult.value.neighborhoodId !== neighborhoodId ||
          historyResult.value.targetLayout !== targetLayout
        ))
      ) {
        setPageState("failed");
        return;
      }
      setView((current) => ({
        ...current,
        history: historyResult.status === "fulfilled" ? historyResult.value : undefined,
        historyFailed: historyResult.status === "rejected",
        metric: metricResult.value,
      }));
      setPageState("ready");
    });
    return () => controller.abort();
  }, [neighborhoodId, pageState, requestVersion, targetLayout, view.neighborhood]);

  return (
    <main className="mx-auto w-full max-w-7xl space-y-8 px-4 py-8 sm:px-6 lg:px-8">
      {pageState === "loading" ? (
        <PageStateBand icon={Database} title="正在加载小区身份" tone="slate" />
      ) : null}
      {pageState === "not_found" ? (
        <PageStateBand
          icon={AlertTriangle}
          title="找不到该小区"
          detail="小区 ID 无效或记录已不存在。"
          tone="amber"
          action={<StateLink href="/neighborhoods" label="重新选择" />}
        />
      ) : null}
      {pageState === "failed" ? (
        <PageStateBand
          icon={AlertTriangle}
          title="小区数据读取失败"
          detail="请求没有返回可用的小区身份和当前指标。"
          tone="rose"
          action={<RetryButton onClick={() => setRequestVersion((version) => version + 1)} />}
        />
      ) : null}
      {view.neighborhood ? (
        <>
          <NeighborhoodHeader neighborhood={view.neighborhood} metric={view.metric} targetLayout={targetLayout} />
          {invalidSelection ? <PageStateBand icon={AlertTriangle} title="原选择已失效" detail={invalidSelection} tone="amber" /> : null}
          <WatchlistTargetAction
            canSubmit
            neighborhood={view.neighborhood}
            targetLayout={targetLayout}
            navigate={navigate}
            onTargetLayoutChange={(layout) => {
              setInvalidSelection(undefined);
              setTargetLayout(layout);
              setView((current) => ({ ...current, history: undefined, historyFailed: false, metric: undefined }));
              writeDetailTargetLayout(neighborhoodId, layout, "push");
              setPageState(layout ? "metric_loading" : "select_layout");
            }}
          />
        </>
      ) : null}
      {pageState === "select_layout" && view.neighborhood ? (
        <PageStateBand icon={MapPin} title="请选择目标户型" tone="slate" />
      ) : null}
      {pageState === "metric_loading" ? (
        <PageStateBand icon={Database} title="正在加载该户型的指标与历史" tone="slate" />
      ) : null}
      {pageState === "no_metric" && view.neighborhood ? (
        <>
          <PageStateBand
            icon={Database}
            title="该小区暂无市场指标"
            detail="当前没有可展示的挂牌或成交批次，不会用 0 或样例结论代替。"
            tone="amber"
            action={<StateLink href="/data" label="前往数据管理" />}
          />
        </>
      ) : null}
      {pageState === "ready" && view.neighborhood && view.metric ? (
        <NeighborhoodReadyView
          history={view.history}
          historyFailed={view.historyFailed}
          metric={view.metric}
          neighborhood={view.neighborhood}
          renderHeader={false}
          retry={() => setRequestVersion((version) => version + 1)}
        />
      ) : null}
    </main>
  );
}

function WatchlistTargetAction({
  canSubmit,
  navigate,
  neighborhood,
  onCancel,
  onTargetLayoutChange,
  targetLayout,
}: {
  canSubmit: boolean;
  navigate: (href: string) => void;
  neighborhood?: Neighborhood;
  onCancel?: () => void;
  onTargetLayoutChange: (value: string) => void;
  targetLayout: string;
}) {
  const [accessState, setAccessState] = useState<"checking" | "locked" | "unlocked">("checking");
  const [submitState, setSubmitState] = useState<SubmitState>("idle");

  useEffect(() => {
    const syncAccess = () => {
      const next = getAccessToken() ? "unlocked" : "locked";
      setAccessState(next);
      if (next === "unlocked") {
        setSubmitState((current) => current === "locked" ? "idle" : current);
      }
    };
    syncAccess();
    return subscribeToAccessToken(syncAccess);
  }, []);

  useEffect(() => {
    setSubmitState("idle");
  }, [neighborhood?.id, targetLayout]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit || !neighborhood || !targetLayout || submitState === "submitting") return;
    if (accessState !== "unlocked") {
      setSubmitState("locked");
      return;
    }

    setSubmitState("submitting");
    try {
      const item = await addWatchlistItem({ neighborhoodId: neighborhood.id, targetLayout });
      if (item.neighborhoodId !== neighborhood.id || item.targetLayout !== targetLayout) {
        throw new Error("watchlist target mismatch");
      }
      navigate("/watchlist");
    } catch (error: unknown) {
      if (error instanceof ApiError && error.status === 401) {
        setAccessState("locked");
        setSubmitState("locked");
      } else if (error instanceof ApiError && (error.status === 409 || error.code === "watchlist_item_exists")) {
        setSubmitState("duplicate");
      } else {
        setSubmitState("failed");
      }
    }
  };

  const complete = canSubmit && Boolean(neighborhood && targetLayout);
  const submitLabel = submitState === "submitting"
    ? "正在加入"
    : submitState === "failed"
      ? "重试加入"
      : "加入观察池";

  return (
    <section aria-label="加入观察池" className="border-b border-slate-200 py-6">
      <form onSubmit={submit} className="grid items-end gap-5 sm:grid-cols-[minmax(0,1fr)_auto]">
        <NativeSelect
          id={`target-layout-${neighborhood?.id ?? "none"}`}
          label="目标户型"
          value={targetLayout}
          disabled={!neighborhood}
          placeholder="选择目标户型"
          options={neighborhood?.availableLayouts ?? []}
          onChange={onTargetLayoutChange}
        />
        <div className="flex flex-wrap gap-2">
          {onCancel ? (
            <button
              type="button"
              onClick={onCancel}
              className="inline-flex h-11 items-center gap-2 rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              <ArrowLeft aria-hidden="true" className="h-4 w-4" />
              取消
            </button>
          ) : null}
          <button
            type="submit"
            disabled={!complete || submitState === "submitting" || submitState === "duplicate"}
            className="inline-flex h-11 items-center gap-2 rounded-md bg-blue-600 px-4 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Plus aria-hidden="true" className="h-4 w-4" />
            {submitLabel}
          </button>
        </div>
      </form>

      {submitState === "locked" ? (
        <div role="status" className="mt-4 flex items-start gap-2 border-l-4 border-amber-400 bg-amber-50 px-4 py-3 text-sm text-amber-950">
          <LockKeyhole aria-hidden="true" className="mt-0.5 h-4 w-4 flex-none" />
          <span>个人空间尚未解锁，当前选择已保留。</span>
        </div>
      ) : null}
      {submitState === "duplicate" ? (
        <div role="status" className="mt-4 flex flex-wrap items-center justify-between gap-3 border-l-4 border-blue-500 bg-blue-50 px-4 py-3 text-sm text-blue-950">
          <span className="flex items-center gap-2"><CheckCircle aria-hidden="true" className="h-4 w-4" />该小区已在观察池中。</span>
          <Link href="/watchlist" className="font-medium text-blue-700 hover:underline">查看观察池</Link>
        </div>
      ) : null}
      {submitState === "failed" ? (
        <div role="alert" className="mt-4 border-l-4 border-rose-400 bg-rose-50 px-4 py-3 text-sm text-rose-950">
          加入观察池失败，当前选择已保留。
        </div>
      ) : null}
    </section>
  );
}

function NativeSelect({
  disabled,
  id,
  label,
  onChange,
  options,
  placeholder,
  value,
}: {
  disabled?: boolean;
  id: string;
  label: string;
  onChange: (value: string) => void;
  options: string[];
  placeholder: string;
  value: string;
}) {
  return (
    <div className="min-w-0">
      <label className="mb-2 block text-sm font-medium text-slate-700" htmlFor={id}>{label}</label>
      <select
        id={id}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        className="h-11 w-full min-w-0 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500"
      >
        <option value="">{placeholder}</option>
        {options.map((option) => <option key={option} value={option}>{option}</option>)}
      </select>
    </div>
  );
}

function reconcileCatalogSelection(
  current: AddSelection,
  response: NeighborhoodSearchResponse,
  selectedNeighborhood: Neighborhood | undefined,
  selectedMissing: boolean,
): { message?: string; results: Neighborhood[]; selection: AddSelection } {
  let selection = { ...current };
  let message: string | undefined;

  if (selection.city && !response.filters.cities.includes(selection.city)) {
    selection = { ...selection, area: "", city: "", neighborhoodId: "", targetLayout: "" };
    message = "原城市已不在可用目录中。";
  } else if (selection.area && (!selection.city || !response.filters.areas.some((item) => item.city === selection.city && item.area === selection.area))) {
    selection = { ...selection, area: "", neighborhoodId: "", targetLayout: "" };
    message = "原板块已不在所选城市的可用目录中。";
  } else if (selection.neighborhoodId && selectedMissing) {
    selection = { ...selection, neighborhoodId: "", targetLayout: "" };
    message = "原小区已不存在。";
  } else if (selection.neighborhoodId && selectedNeighborhood && !neighborhoodMatchesSelection(selectedNeighborhood, selection)) {
    selection = { ...selection, neighborhoodId: "", targetLayout: "" };
    message = "原小区不再匹配当前城市、板块或名称条件。";
  } else if (selection.targetLayout && selectedNeighborhood && !selectedNeighborhood.availableLayouts.includes(selection.targetLayout)) {
    selection = { ...selection, targetLayout: "" };
    message = "原目标户型已不在该小区目录中。";
  }

  const results = [...response.items];
  if (
    selection.neighborhoodId &&
    selectedNeighborhood &&
    neighborhoodMatchesSelection(selectedNeighborhood, selection) &&
    !results.some((item) => item.id === selectedNeighborhood.id)
  ) {
    results.push(selectedNeighborhood);
    results.sort(compareNeighborhoods);
  }
  return { message, results, selection };
}

function neighborhoodMatchesSelection(neighborhood: Neighborhood, selection: AddSelection): boolean {
  return Boolean(
    neighborhood.city &&
    neighborhood.city === selection.city &&
    neighborhood.area === selection.area &&
    (!selection.q || neighborhood.name.toLocaleLowerCase("zh-CN").includes(selection.q.toLocaleLowerCase("zh-CN"))) &&
    neighborhood.availableLayouts.length > 0,
  );
}

function compareNeighborhoods(left: Neighborhood, right: Neighborhood): number {
  for (const [a, b] of [
    [left.city ?? "", right.city ?? ""],
    [left.area, right.area],
    [left.name, right.name],
    [left.id, right.id],
  ]) {
    const compared = a.localeCompare(b, "zh-CN");
    if (compared !== 0) return compared;
  }
  return 0;
}

function readAddSelection(): AddSelection {
  if (typeof window === "undefined") return emptySelection;
  const params = new URLSearchParams(window.location.search);
  return {
    area: params.get("area")?.trim() ?? "",
    city: params.get("city")?.trim() ?? "",
    neighborhoodId: params.get("neighborhoodId")?.trim() ?? "",
    q: params.get("q")?.trim() ?? "",
    targetLayout: params.get("targetLayout")?.trim() ?? "",
  };
}

function writeAddSelection(selection: AddSelection, method: "push" | "replace"): void {
  const params = new URLSearchParams();
  for (const [key, value] of [
    ["city", selection.city],
    ["area", selection.area],
    ["q", selection.q],
    ["neighborhoodId", selection.neighborhoodId],
    ["targetLayout", selection.targetLayout],
  ] as const) {
    if (value) params.set(key, value);
  }
  writeURL(params, method);
}

function readDetailTargetLayout(): string {
  if (typeof window === "undefined") return "";
  return new URLSearchParams(window.location.search).get("targetLayout")?.trim() ?? "";
}

function writeDetailTargetLayout(neighborhoodId: string, targetLayout: string, method: "push" | "replace"): void {
  const params = new URLSearchParams({ id: neighborhoodId });
  if (targetLayout) params.set("targetLayout", targetLayout);
  writeURL(params, method);
}

function writeURL(params: URLSearchParams, method: "push" | "replace"): void {
  const query = params.size > 0 ? `?${params.toString()}` : "";
  const href = `${window.location.pathname}${query}`;
  if (method === "push") window.history.pushState({}, "", href);
  else window.history.replaceState({}, "", href);
}

function sameSelection(left: AddSelection, right: AddSelection): boolean {
  return left.area === right.area &&
    left.city === right.city &&
    left.neighborhoodId === right.neighborhoodId &&
    left.q === right.q &&
    left.targetLayout === right.targetLayout;
}

function NeighborhoodReadyView({
  history,
  historyFailed,
  metric,
  neighborhood,
  renderHeader = true,
  retry,
}: {
  history?: MetricHistoryResponse;
  historyFailed: boolean;
  metric: NeighborhoodMetricResponse;
  neighborhood: Neighborhood;
  renderHeader?: boolean;
  retry: () => void;
}) {
  const stale = metric.freshness === "stale" || metric.freshness === "expired";
  const insufficient = metric.qualityState !== "sufficient" || metric.transactionMomentum === "unknown";
  const currentHistoryPoint = history?.items.find((point) => point.batch.collectionRunId === metric.collectionRunId);
  const trend = useMemo<TrendPoint[]>(
    () => (history?.items ?? []).map((point) => ({
      collectedAt: point.collectedAt,
      coverage: point.coverage,
      label: formatShortDate(point.collectedAt),
      listedHomes: point.listedHomes,
      priceCutHomes: point.priceCutHomes,
      sourceRef: point.batch.sourceRef,
      transactionCount: point.transactionSampleCount,
    })),
    [history],
  );

  return (
    <>
      {renderHeader ? <NeighborhoodHeader neighborhood={neighborhood} metric={metric} targetLayout={metric.targetLayout} /> : null}

      {stale ? (
        <PageStateBand
          icon={CalendarClock}
          title={metric.freshness === "expired" ? "市场数据已过期" : "市场数据已陈旧"}
          detail="当前信息仅用于核对历史，不生成新的买入或议价窗口。"
          tone="amber"
        />
      ) : insufficient ? (
        <PageStateBand
          icon={AlertTriangle}
          title="市场数据不足"
          detail="覆盖范围或挂牌、成交样本不足，当前结论已降级。"
          tone="amber"
        />
      ) : null}

      <section className="border-l-4 border-blue-600 bg-white px-5 py-5 sm:px-6">
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="text-xl font-bold text-slate-900">{metric.status}</h2>
          <StatusBadge tone={stale || insufficient ? "amber" : signalTone(metric.status)}>
            {freshnessCopy[metric.freshness]}
          </StatusBadge>
        </div>
        <p className="mt-3 max-w-4xl text-sm leading-6 text-slate-700">{metric.advice}</p>
        <ul className="mt-4 space-y-2 text-sm text-slate-700">
          {(metric.reasons ?? []).map((reason) => (
            <li key={reason} className="flex items-start gap-2">
              <Activity aria-hidden="true" className="mt-0.5 h-4 w-4 flex-none text-blue-600" />
              <span>{reason} <a href="#market-evidence" className="font-medium text-blue-700 hover:underline">查看证据</a></span>
            </li>
          ))}
        </ul>
      </section>

      <MetricGrid metric={metric} />

      <section className="border-t border-slate-200 pt-6">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className="text-sm font-semibold text-blue-700">真实批次</p>
            <h2 className="mt-1 text-xl font-bold text-slate-900">近 8 周挂牌与降价趋势</h2>
          </div>
          {history ? <p className="text-xs text-slate-500">{formatDateTime(history.window.from)} 至 {formatDateTime(history.window.to)}</p> : null}
        </div>

        {historyFailed ? (
          <PageStateBand
            icon={AlertTriangle}
            title="历史趋势读取失败"
            detail="当前指标仍可查看，趋势区域没有回退样例。"
            tone="rose"
            action={<RetryButton onClick={retry} />}
          />
        ) : trend.length < 2 ? (
          <PageStateBand icon={History} title="暂无趋势" detail="至少需要两个真实批次才能比较变化。" tone="slate" />
        ) : (
          <div className="mt-5 h-72 w-full" aria-label="真实挂牌与降价批次趋势图">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart3View data={trend} />
            </ResponsiveContainer>
          </div>
        )}
      </section>

      <section id="market-evidence" className="border-t border-slate-200 pt-6">
        <h2 className="text-xl font-bold text-slate-900">来源与质量证据</h2>
        <dl className="mt-4 grid gap-x-8 gap-y-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <EvidenceItem label="采集时间" value={formatDateTime(metric.collectedAt)} />
          <EvidenceItem label="计算时间" value={formatDateTime(metric.calculatedAt)} />
          <EvidenceItem label="算法版本" value={metric.algorithmVersion} />
          <EvidenceItem label="覆盖与新鲜度" value={`${coverageCopy[metric.coverage]} · ${freshnessCopy[metric.freshness]}`} />
          <EvidenceItem label="挂牌样本" value={`${metric.listingSampleCount} 条`} />
          <EvidenceItem label="成交样本" value={`${metric.transactionSampleCount} 条`} />
          <EvidenceItem label="来源 ID" value={metric.sourceIds.join(", ") || "暂无"} />
          <div>
            <dt className="text-xs text-slate-500">采集批次</dt>
            <dd className="mt-1 break-all font-medium text-slate-900">
              <Link href={`/data/imports/${metric.collectionRunId}`} className="text-blue-700 hover:underline">
                {currentHistoryPoint?.batch.sourceRef || metric.collectionRunId}
              </Link>
            </dd>
          </div>
        </dl>
        {metric.transactionEvidence ? (
          <p className="mt-5 text-sm text-slate-600">
            成交窗口 {metric.transactionEvidence.windowStart} 至 {metric.transactionEvidence.windowEnd}：最近 30 天 {metric.transactionEvidence.recent30DayTransactionCount} 笔，此前 60 天 {metric.transactionEvidence.preceding60DayTransactionCount} 笔。
          </p>
        ) : null}
        {metric.qualityWarnings.length > 0 ? (
          <ul className="mt-4 flex flex-wrap gap-2">
            {metric.qualityWarnings.map((warning) => (
              <li key={warning} className="rounded-md border border-amber-300 bg-amber-50 px-2 py-1 text-xs text-amber-900">
                {warningCopy[warning] ?? warning}
              </li>
            ))}
          </ul>
        ) : null}
      </section>
    </>
  );
}

function NeighborhoodHeader({
  metric,
  neighborhood,
  targetLayout,
}: {
  metric?: NeighborhoodMetricResponse;
  neighborhood: Neighborhood;
  targetLayout: string;
}) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4 border-b border-slate-200 pb-5">
      <div>
        <div className="flex items-center gap-2 text-sm text-slate-500">
          <MapPin aria-hidden="true" className="h-4 w-4" />
          <span>{[neighborhood.city, neighborhood.area].filter(Boolean).join(" · ")}</span>
        </div>
        <h1 className="mt-2 text-3xl font-bold text-slate-900">{neighborhood.name}</h1>
        {targetLayout ? <p className="mt-2 text-sm text-slate-600">目标户型：{targetLayout}</p> : null}
      </div>
      {metric ? (
        <div className="text-right text-xs text-slate-500">
          <p>采集于 {formatDateTime(metric.collectedAt)}</p>
          <p className="mt-1">计算于 {formatDateTime(metric.calculatedAt)}</p>
        </div>
      ) : null}
    </header>
  );
}

function MetricGrid({ metric }: { metric: NeighborhoodMetricResponse }) {
  const cards = [
    { label: "挂牌价区间", value: formatPriceRange(metric.listingPriceMin, metric.listingPriceMax), detail: `${metric.listingSampleCount} 条挂牌样本` },
    { label: "近 90 天成交区间", value: formatPriceRange(metric.transactionPriceMin, metric.transactionPriceMax), detail: `${metric.transactionSampleCount} 条成交样本` },
    { label: "当前在售", value: `${metric.listedHomes} 套`, detail: coverageCopy[metric.coverage] },
    { label: "当前降价", value: `${metric.priceCutHomes} 套`, detail: `占在售 ${((metric.priceCutShare ?? 0) * 100).toFixed(1)}%` },
    { label: "平均挂牌时长", value: metric.avgDaysOnMarket == null ? "暂无" : `${metric.avgDaysOnMarket.toFixed(1)} 天`, detail: "非成交周期" },
    { label: "目标户型供给", value: `${metric.targetLayoutSupply} 套`, detail: `稀缺度 ${scarcityCopy[metric.targetLayoutScarcity ?? "unknown"]}` },
  ];

  return (
    <section aria-label="当前市场指标" className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-6">
      {cards.map((card) => (
        <article key={card.label} className="rounded-md border border-slate-200 bg-white p-4">
          <h2 className="text-xs font-medium text-slate-500">{card.label}</h2>
          <p className="mt-2 text-lg font-bold text-slate-900">{card.value}</p>
          <p className="mt-2 text-xs text-slate-500">{card.detail}</p>
        </article>
      ))}
    </section>
  );
}

function BarChart3View({ data }: { data: TrendPoint[] }) {
  return (
    <BarChart data={data} margin={{ top: 8, right: 8, left: -12, bottom: 4 }}>
      <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
      <XAxis dataKey="label" tick={{ fill: "#64748b", fontSize: 11 }} />
      <YAxis allowDecimals={false} tick={{ fill: "#64748b", fontSize: 11 }} />
      <Tooltip content={TrendTooltip} />
      <Legend wrapperStyle={{ fontSize: 12 }} />
      <Bar dataKey="listedHomes" name="在售套数" radius={[3, 3, 0, 0]}>
        {data.map((point) => <Cell key={`listed-${point.collectedAt}`} fill={point.coverage === "full" ? "#2563eb" : "#94a3b8"} />)}
      </Bar>
      <Bar dataKey="priceCutHomes" name="降价套数" radius={[3, 3, 0, 0]}>
        {data.map((point) => <Cell key={`cut-${point.collectedAt}`} fill={point.coverage === "full" ? "#d97706" : "#cbd5e1"} />)}
      </Bar>
    </BarChart>
  );
}

function TrendTooltip({ active, payload }: TooltipContentProps) {
  const point = payload?.[0]?.payload as TrendPoint | undefined;
  if (!active || !point) return null;
  return (
    <div className="rounded-md border border-slate-200 bg-white p-3 text-xs shadow-lg">
      <p className="font-semibold text-slate-900">{formatDateTime(point.collectedAt)}</p>
      <p className="mt-2 text-slate-700">在售 {point.listedHomes} 套 · 降价 {point.priceCutHomes} 套</p>
      <p className="mt-1 text-slate-700">成交样本 {point.transactionCount} 条</p>
      <p className="mt-1 text-slate-500">{coverageCopy[point.coverage]} · {point.sourceRef}</p>
    </div>
  );
}

function EvidenceItem({ label, value }: { label: string; value: string }) {
  return <div><dt className="text-xs text-slate-500">{label}</dt><dd className="mt-1 break-words font-medium text-slate-900">{value}</dd></div>;
}

function PageStateBand({
  action,
  detail,
  icon: Icon,
  title,
  tone,
}: {
  action?: React.ReactNode;
  detail?: string;
  icon: typeof Database;
  title: string;
  tone: "slate" | "amber" | "rose";
}) {
  const toneClass = {
    amber: "border-amber-400 bg-amber-50 text-amber-950",
    rose: "border-rose-400 bg-rose-50 text-rose-950",
    slate: "border-slate-300 bg-slate-50 text-slate-800",
  }[tone];
  return (
    <section role="status" className={`my-6 flex min-h-24 flex-wrap items-center justify-between gap-4 border-l-4 px-5 py-4 ${toneClass}`}>
      <div className="flex items-start gap-3">
        <Icon aria-hidden="true" className="mt-0.5 h-5 w-5 flex-none" />
        <div><h2 className="font-semibold">{title}</h2>{detail ? <p className="mt-1 text-sm opacity-80">{detail}</p> : null}</div>
      </div>
      {action}
    </section>
  );
}

function RetryButton({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="inline-flex h-9 items-center gap-2 rounded-md border border-current bg-white px-3 text-sm font-medium">
      <RefreshCw aria-hidden="true" className="h-4 w-4" />重试
    </button>
  );
}

function StateLink({ href, label }: { href: string; label: string }) {
  return <Link href={href} className="inline-flex h-9 items-center rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800">{label}</Link>;
}

function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

function formatPriceRange(min?: number | null, max?: number | null): string {
  if (min == null || max == null) return "暂无";
  return `${formatNumber(min)}-${formatNumber(max)} 万`;
}

function formatNumber(value: number): string {
  return Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1);
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Shanghai" }).format(new Date(value));
}

function formatShortDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { month: "numeric", day: "numeric", timeZone: "Asia/Shanghai" }).format(new Date(value));
}

function signalTone(status: string): "emerald" | "blue" | "slate" {
  if (status === "重点看" || status === "适合砍价") return "emerald";
  return status === "继续观察" ? "blue" : "slate";
}

const freshnessCopy: Record<string, string> = { unknown: "新鲜度未知", current: "当前", stale: "已陈旧", expired: "已过期" };
const coverageCopy: Record<string, string> = { full: "完整覆盖", partial: "部分覆盖" };
const scarcityCopy: Record<string, string> = { unknown: "未知", low: "低", medium: "中", high: "高" };
const warningCopy: Record<string, string> = {
  expired_data: "市场数据已经过期",
  insufficient_listing_samples: "挂牌样本不足",
  insufficient_transaction_samples: "成交样本不足",
  metric_refresh_pending: "新批次指标仍在刷新",
  no_full_inventory: "缺少完整挂牌批次",
  partial_coverage: "当前批次为部分覆盖",
  stale_data: "市场数据已陈旧",
};
