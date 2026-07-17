"use client";

import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  Building2,
  CalendarClock,
  CarFront,
  CheckCircle,
  Database,
  History,
  LockKeyhole,
  MapPinned,
  MapPin,
  Plus,
  RefreshCw,
  Search,
  Zap,
} from "lucide-react";
import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  type TooltipContentProps,
} from "recharts";

import {
  addWatchlistItem,
  ApiError,
  compareCommunityMarkets,
  getListingAdjustments,
  getMarketListings,
  getMarketTransactions,
  getMetricHistory,
  getCommunityMarketSnapshot,
  getNeighborhood,
  getNeighborhoodMetrics,
  searchNeighborhoods,
  type CommunityMarketComparison,
  type MetricHistoryResponse,
  type CommunityMarketSnapshot,
  type ListingAdjustment,
  type MarketListingsPage,
  type MarketListQuery,
  type MarketTransaction,
  type MarketTransactionsPage,
  type Neighborhood,
  type NeighborhoodMetricResponse,
  type NeighborhoodSearchResponse,
} from "@/lib/api-client";
import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";

import { StatusBadge } from "./status-badge";
import { CenteredLoadingState } from "./centered-loading-state";

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
  communityMarket?: CommunityMarketSnapshot;
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
    return <CenteredLoadingState className="min-h-[55vh]" title="正在读取目标小区" />;
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
          {catalogState === "ready" && selection.q && results.length > 0 ? (
            <ul aria-label="小区名称搜索结果" className="mt-2 max-w-2xl divide-y divide-slate-200 border border-slate-200 bg-white">
              {results.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    aria-pressed={selection.neighborhoodId === item.id}
                    onClick={() => {
                      setInvalidSelection(undefined);
                      commitSelection({
                        area: item.area,
                        city: item.city ?? "",
                        neighborhoodId: item.id,
                        q: selection.q,
                        targetLayout: "",
                      });
                    }}
                    className={`flex min-h-14 w-full items-center justify-between gap-4 px-3 py-2 text-left text-sm hover:bg-slate-50 ${selection.neighborhoodId === item.id ? "bg-blue-50" : ""}`}
                  >
                    <span className="font-medium text-slate-900">{item.name}</span>
                    <span className="text-xs text-slate-500">{item.city ?? "城市未标注"} · {item.area}</span>
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
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

      {catalogState === "loading" ? <CenteredLoadingState title="正在加载小区目录" /> : null}
      {catalogState === "failed" ? (
        <PageStateBand
          icon={AlertTriangle}
          title="小区搜索失败"
          detail="当前筛选条件没有返回可用目录。"
          tone="rose"
          action={<RetryButton onClick={() => setRequestVersion((version) => version + 1)} />}
        />
      ) : null}
      {catalogState === "ready" && results.length === 0 && (Boolean(selection.q) || Boolean(selection.city && selection.area)) ? (
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
      .then(async (neighborhood) => {
        if (controller.signal.aborted) return;
        let communityMarket: CommunityMarketSnapshot | undefined;
        try {
          communityMarket = await getCommunityMarketSnapshot(neighborhoodId, controller.signal);
        } catch (error: unknown) {
          if (!isNotFound(error) && !isAbortError(error)) throw error;
        }
        if (controller.signal.aborted) return;
        setView({ communityMarket, historyFailed: false, neighborhood });
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
        <CenteredLoadingState title="正在加载小区身份" />
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
          {view.communityMarket ? <CommunityMarketWorkspace neighborhoodId={neighborhoodId} snapshot={view.communityMarket} /> : null}
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
        <CenteredLoadingState title="正在加载该户型的指标与历史" />
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

type MarketTab = "overview" | "listings" | "transactions" | "trends" | "profile" | "surroundings";

const marketTabs: { id: MarketTab; label: string }[] = [
  { id: "overview", label: "概览" },
  { id: "listings", label: "在售" },
  { id: "transactions", label: "成交" },
  { id: "trends", label: "趋势与调价" },
  { id: "profile", label: "小区档案" },
  { id: "surroundings", label: "周边" },
];

function CommunityMarketWorkspace({ neighborhoodId, snapshot }: { neighborhoodId: string; snapshot: CommunityMarketSnapshot }) {
  const complete = snapshot.qualityStatus === "complete" && Boolean(snapshot.collectionRunId);
  const [activeTab, setActiveTab] = useState<MarketTab>("overview");
  const [peerOptions, setPeerOptions] = useState<Neighborhood[]>([]);
  const [peerId, setPeerId] = useState("");
  const [comparison, setComparison] = useState<CommunityMarketComparison>();
  const [comparisonState, setComparisonState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [listingQuery, setListingQuery] = useState<MarketListQuery>({ page: 1, pageSize: 10, sortBy: "date", sortOrder: "desc" });
  const [transactionQuery, setTransactionQuery] = useState<MarketListQuery>({ page: 1, pageSize: 10, sortBy: "date", sortOrder: "desc" });
  const [listings, setListings] = useState<MarketListingsPage>();
  const [transactions, setTransactions] = useState<MarketTransactionsPage>();
  const [listingsState, setListingsState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [transactionsState, setTransactionsState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [expandedRoom, setExpandedRoom] = useState<string>();
  const [adjustments, setAdjustments] = useState<Record<string, ListingAdjustment[]>>({});
  const [adjustmentError, setAdjustmentError] = useState<string>();

  useEffect(() => {
    if (!complete) return;
    const controller = new AbortController();
    searchNeighborhoods({ page: 1, pageSize: 100 }, controller.signal)
      .then((result) => {
        const peers = result.items.filter((item) => item.id !== neighborhoodId);
        setPeerOptions(peers);
        setPeerId((current) => current || peers.find((item) => item.name.includes("亲和") || item.name.includes("鸣泉"))?.id || peers[0]?.id || "");
      })
      .catch((error: unknown) => {
        if (!isAbortError(error)) setPeerOptions([]);
      });
    return () => controller.abort();
  }, [complete, neighborhoodId]);

  useEffect(() => {
    if (!complete || !peerId) {
      setComparison(undefined);
      setComparisonState("idle");
      return;
    }
    const controller = new AbortController();
    setComparisonState("loading");
    compareCommunityMarkets(neighborhoodId, peerId, controller.signal)
      .then((result) => {
        setComparison(result);
        setComparisonState("ready");
      })
      .catch((error: unknown) => {
        if (!isAbortError(error)) setComparisonState("failed");
      });
    return () => controller.abort();
  }, [complete, neighborhoodId, peerId]);

  useEffect(() => {
    if (!complete) return;
    const controller = new AbortController();
    setListingsState("loading");
    getMarketListings(neighborhoodId, listingQuery, controller.signal)
      .then((result) => {
        setListings(result);
        setListingsState("ready");
      })
      .catch((error: unknown) => {
        if (!isAbortError(error)) setListingsState("failed");
      });
    return () => controller.abort();
  }, [complete, listingQuery, neighborhoodId]);

  useEffect(() => {
    if (!complete) return;
    const controller = new AbortController();
    setTransactionsState("loading");
    getMarketTransactions(neighborhoodId, transactionQuery, controller.signal)
      .then((result) => {
        setTransactions(result);
        setTransactionsState("ready");
      })
      .catch((error: unknown) => {
        if (!isAbortError(error)) setTransactionsState("failed");
      });
    return () => controller.abort();
  }, [complete, neighborhoodId, transactionQuery]);

  const toggleAdjustments = (roomId: string) => {
    if (expandedRoom === roomId) {
      setExpandedRoom(undefined);
      return;
    }
    setExpandedRoom(roomId);
    setAdjustmentError(undefined);
    if (adjustments[roomId]) return;
    getListingAdjustments(neighborhoodId, roomId)
      .then((result) => setAdjustments((current) => ({ ...current, [roomId]: result.items })))
      .catch(() => setAdjustmentError("调价记录读取失败"));
  };

  return (
    <section aria-label="房见小区市场" className="border-b border-slate-200 pb-6">
      <div className="flex flex-wrap items-end justify-between gap-4 border-b border-slate-200 pb-4">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-blue-700">小区聚合行情</p>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <h2 className="text-xl font-bold text-slate-900">{snapshot.communityName}</h2>
            <span className={`rounded px-2 py-1 text-xs font-medium ${complete ? "bg-emerald-50 text-emerald-800" : "bg-amber-50 text-amber-800"}`}>
              {complete ? "完整数据包" : "仅聚合快照"}
            </span>
          </div>
          <p className="mt-2 break-words text-xs text-slate-500">采集于 {formatDateTime(snapshot.collectedAt)} · {snapshot.sourceRef}</p>
        </div>
        <label className="min-w-56 text-sm font-medium text-slate-700">
          对比小区
          <select
            aria-label="对比小区"
            className="mt-1 block h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
            disabled={!complete || peerOptions.length === 0}
            onChange={(event) => setPeerId(event.target.value)}
            value={peerId}
          >
            <option value="">{complete ? "选择对比小区" : "完整采集后可比较"}</option>
            {peerOptions.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
        </label>
      </div>

      <div aria-label="市场数据视图" className="flex gap-1 overflow-x-auto border-b border-slate-200 py-3" role="tablist">
        {marketTabs.map((tab) => (
          <button
            key={tab.id}
            aria-selected={activeTab === tab.id}
            className={`h-9 flex-none rounded-md px-3 text-sm font-medium ${activeTab === tab.id ? "bg-slate-900 text-white" : "text-slate-600 hover:bg-slate-100"}`}
            onClick={() => setActiveTab(tab.id)}
            role="tab"
            type="button"
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === "overview" ? (
        <div role="tabpanel">
          {comparison ? <ComparisonOverview comparison={comparison} /> : null}
          {comparisonState === "loading" ? <p className="py-4 text-sm text-slate-500">正在读取对比行情...</p> : null}
          {comparisonState === "failed" ? <p className="py-4 text-sm text-rose-700">对比行情读取失败，当前小区数据仍可查看。</p> : null}
          <CommunityMarketProfilePanel snapshot={snapshot} />
        </div>
      ) : null}
      {activeTab === "listings" ? (
        <MarketListingsView
          adjustments={adjustments}
          adjustmentError={adjustmentError}
          complete={complete}
          expandedRoom={expandedRoom}
          page={listings}
          query={listingQuery}
          setQuery={setListingQuery}
          state={listingsState}
          toggleAdjustments={toggleAdjustments}
        />
      ) : null}
      {activeTab === "transactions" ? (
        <MarketTransactionsView complete={complete} page={transactions} query={transactionQuery} setQuery={setTransactionQuery} state={transactionsState} />
      ) : null}
      {activeTab === "trends" ? <MarketTrendsView snapshot={snapshot} /> : null}
      {activeTab === "profile" ? <CommunityMarketProfilePanel snapshot={snapshot} /> : null}
      {activeTab === "surroundings" ? <MarketSurroundingsView snapshot={snapshot} /> : null}
    </section>
  );
}

function ComparisonOverview({ comparison }: { comparison: CommunityMarketComparison }) {
  const metrics = [
    { label: "挂牌均价", metric: comparison.listingUnitPrice, unit: "元/㎡" },
    { label: "供应量", metric: comparison.supply, unit: "套" },
    { label: "近三月成交", metric: comparison.recentTrades, unit: "套" },
    { label: "挂牌成交价差", metric: comparison.listingTradeGap, unit: "元/㎡" },
    { label: "成交周期", metric: comparison.averageTradeCycle, unit: "天" },
  ];
  return (
    <section aria-label="双小区行情比较" className="py-5">
      <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-end gap-3 border-b border-slate-200 pb-3 text-sm">
        <h3 className="font-semibold text-slate-900">{comparison.primary.communityName}</h3>
        <span className="text-xs text-slate-400">对比</span>
        <h3 className="text-right font-semibold text-slate-900">{comparison.peer.communityName}</h3>
      </div>
      <div className="grid gap-px bg-slate-200 sm:grid-cols-2 lg:grid-cols-5">
        {metrics.map(({ label, metric, unit }) => (
          <div className="bg-white px-3 py-4" key={label}>
            <p className="text-xs text-slate-500">{label}</p>
            <div className="mt-2 flex items-baseline justify-between gap-2 text-sm font-semibold text-slate-900">
              <span>{formatComparisonValue(metric.primary, unit)}</span>
              <span>{formatComparisonValue(metric.peer, unit)}</span>
            </div>
            <p className={`mt-2 text-xs ${(metric.delta ?? 0) > 0 ? "text-rose-700" : "text-emerald-700"}`}>差值 {formatSigned(metric.delta, unit)}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function MarketFilterBar({ query, setQuery }: { query: MarketListQuery; setQuery: (query: MarketListQuery) => void }) {
  const update = (patch: Partial<MarketListQuery>) => setQuery({ ...query, ...patch, page: 1 });
  return (
    <div className="grid gap-3 border-b border-slate-200 py-4 sm:grid-cols-2 lg:grid-cols-5">
      <label className="text-xs font-medium text-slate-600">户型
        <select aria-label="市场户型筛选" className="mt-1 block h-9 w-full rounded-md border border-slate-300 bg-white px-2 text-sm" onChange={(event) => update({ layout: event.target.value || undefined })} value={query.layout ?? ""}>
          <option value="">全部户型</option>{["一室", "二室", "三室", "四室", "五室"].map((item) => <option key={item}>{item}</option>)}
        </select>
      </label>
      <label className="text-xs font-medium text-slate-600">楼层
        <select aria-label="市场楼层筛选" className="mt-1 block h-9 w-full rounded-md border border-slate-300 bg-white px-2 text-sm" onChange={(event) => update({ floor: (event.target.value || undefined) as MarketListQuery["floor"] })} value={query.floor ?? ""}>
          <option value="">全部楼层</option>{["高楼层", "中楼层", "低楼层"].map((item) => <option key={item}>{item}</option>)}
        </select>
      </label>
      <label className="text-xs font-medium text-slate-600">最低总价（万）
        <input aria-label="最低总价" className="mt-1 block h-9 w-full rounded-md border border-slate-300 px-2 text-sm" min="0" onChange={(event) => update({ minPriceWan: event.target.value ? Number(event.target.value) : undefined })} type="number" value={query.minPriceWan ?? ""} />
      </label>
      <label className="text-xs font-medium text-slate-600">最高总价（万）
        <input aria-label="最高总价" className="mt-1 block h-9 w-full rounded-md border border-slate-300 px-2 text-sm" min="0" onChange={(event) => update({ maxPriceWan: event.target.value ? Number(event.target.value) : undefined })} type="number" value={query.maxPriceWan ?? ""} />
      </label>
      <label className="text-xs font-medium text-slate-600">排序
        <select aria-label="市场排序" className="mt-1 block h-9 w-full rounded-md border border-slate-300 bg-white px-2 text-sm" onChange={(event) => update({ sortBy: event.target.value as MarketListQuery["sortBy"] })} value={query.sortBy ?? "date"}>
          <option value="date">日期</option><option value="price">总价</option><option value="unitPrice">单价</option><option value="area">面积</option><option value="adjustments">调价次数</option>
        </select>
      </label>
    </div>
  );
}

function MarketListingsView({ adjustments, adjustmentError, complete, expandedRoom, page, query, setQuery, state, toggleAdjustments }: {
  adjustments: Record<string, ListingAdjustment[]>;
  adjustmentError?: string;
  complete: boolean;
  expandedRoom?: string;
  page?: MarketListingsPage;
  query: MarketListQuery;
  setQuery: (query: MarketListQuery) => void;
  state: "idle" | "loading" | "ready" | "failed";
  toggleAdjustments: (roomId: string) => void;
}) {
  if (!complete) return <MarketUnavailableState />;
  return (
    <div role="tabpanel">
      <MarketFilterBar query={query} setQuery={setQuery} />
      {state === "loading" ? <p className="py-8 text-center text-sm text-slate-500">正在读取在售房源...</p> : null}
      {state === "failed" ? <p className="py-8 text-center text-sm text-rose-700">在售房源读取失败</p> : null}
      {state === "ready" && page?.items.length === 0 ? <p className="py-8 text-center text-sm text-slate-500">没有符合筛选条件的在售房源</p> : null}
      <div className="divide-y divide-slate-200">
        {page?.items.map((item) => (
          <article className="py-4" key={item.roomId}>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-[1.1fr_.8fr_.8fr_1fr_auto] lg:items-center">
              <div><p className="font-semibold text-slate-900">{item.layout} · {formatProfileNumber(item.areaSqm, "㎡")}</p><p className="mt-1 text-xs text-slate-500">{item.floorDescription || item.floorBand} · {item.orientation || "朝向未知"}</p></div>
              <div><p className="text-xs text-slate-500">挂牌总价</p><p className="font-semibold text-slate-900">{formatWan(item.listingTotalPriceWan)}</p></div>
              <div><p className="text-xs text-slate-500">挂牌单价</p><p className="font-semibold text-slate-900">{formatUnitPrice(item.listingUnitPrice)}</p></div>
              <div><p className="text-xs text-slate-500">挂牌时间</p><p className="font-semibold text-slate-900">{formatDateOnly(item.listedAt)} · {item.daysOnMarket} 天</p></div>
              <button aria-expanded={expandedRoom === item.roomId} className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-medium" disabled={item.adjustmentCount === 0} onClick={() => toggleAdjustments(item.roomId)} type="button">
                <History aria-hidden="true" className="h-4 w-4" />{item.adjustmentCount} 次
              </button>
            </div>
            {expandedRoom === item.roomId ? <AdjustmentTimeline error={adjustmentError} items={adjustments[item.roomId]} /> : null}
          </article>
        ))}
      </div>
      {page ? <MarketPagination page={page.page} pageSize={page.pageSize} setPage={(next) => setQuery({ ...query, page: next })} total={page.total} /> : null}
    </div>
  );
}

function AdjustmentTimeline({ error, items }: { error?: string; items?: ListingAdjustment[] }) {
  if (error) return <p className="mt-3 border-l-2 border-rose-400 pl-3 text-sm text-rose-700">{error}</p>;
  if (!items) return <p className="mt-3 text-sm text-slate-500">正在读取调价记录...</p>;
  if (items.length === 0) return <p className="mt-3 text-sm text-slate-500">该房源没有可用调价明细</p>;
  return (
    <ol aria-label="调价时间线" className="mt-4 space-y-2 border-l-2 border-slate-200 pl-4">
      {items.map((item) => <li key={`${item.adjustedAt}-${item.priceBeforeWan}-${item.priceAfterWan}`} className="text-sm"><span className="font-medium text-slate-900">{formatDateOnly(item.adjustedAt)}</span><span className="ml-3 text-slate-600">{formatWan(item.priceBeforeWan)} → {formatWan(item.priceAfterWan)}</span><span className={`ml-3 ${item.amountWan < 0 ? "text-emerald-700" : "text-rose-700"}`}>{formatSigned(item.amountWan, "万")}</span></li>)}
    </ol>
  );
}

function MarketTransactionsView({ complete, page, query, setQuery, state }: { complete: boolean; page?: MarketTransactionsPage; query: MarketListQuery; setQuery: (query: MarketListQuery) => void; state: "idle" | "loading" | "ready" | "failed" }) {
  if (!complete) return <MarketUnavailableState />;
  return (
    <div role="tabpanel">
      <MarketFilterBar query={query} setQuery={setQuery} />
      {state === "loading" ? <p className="py-8 text-center text-sm text-slate-500">正在读取历史成交...</p> : null}
      {state === "failed" ? <p className="py-8 text-center text-sm text-rose-700">成交记录读取失败</p> : null}
      {state === "ready" && page?.items.length === 0 ? <p className="py-8 text-center text-sm text-slate-500">没有符合筛选条件的成交记录</p> : null}
      <div className="divide-y divide-slate-200">
        {page?.items.map((item) => <TransactionRow item={item} key={item.roomId} />)}
      </div>
      {page ? <MarketPagination page={page.page} pageSize={page.pageSize} setPage={(next) => setQuery({ ...query, page: next })} total={page.total} /> : null}
    </div>
  );
}

function TransactionRow({ item }: { item: MarketTransaction }) {
  return (
    <article className="grid gap-3 py-4 sm:grid-cols-2 lg:grid-cols-[1.1fr_.8fr_.8fr_.8fr_1fr] lg:items-center">
      <div><p className="font-semibold text-slate-900">{item.layout} · {formatProfileNumber(item.areaSqm, "㎡")}</p><p className="mt-1 text-xs text-slate-500">{item.floorDescription || item.floorBand} · {item.orientation || "朝向未知"}</p></div>
      <div><p className="text-xs text-slate-500">挂牌 / 成交</p><p className="font-semibold text-slate-900">{formatWan(item.listingTotalPriceWan)} / {formatWan(item.tradeTotalPriceWan)}</p></div>
      <div><p className="text-xs text-slate-500">成交单价</p><p className="font-semibold text-slate-900">{formatUnitPrice(item.tradeUnitPrice)}</p></div>
      <div><p className="text-xs text-slate-500">议价空间</p><p className="font-semibold text-emerald-700">{formatWan(item.negotiationWan)} · {formatProfileNumber(item.negotiationPercent, "%")}</p></div>
      <div><p className="text-xs text-slate-500">成交日期</p><p className="font-semibold text-slate-900">{formatDateOnly(item.tradeDate)}</p></div>
    </article>
  );
}

function MarketPagination({ page, pageSize, setPage, total }: { page: number; pageSize: number; setPage: (page: number) => void; total: number }) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="flex items-center justify-between border-t border-slate-200 pt-4 text-sm text-slate-600">
      <span>共 {total} 条 · 第 {page}/{pages} 页</span>
      <div className="flex gap-2">
        <button aria-label="上一页" className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 disabled:opacity-40" disabled={page <= 1} onClick={() => setPage(page - 1)} type="button"><ArrowLeft className="h-4 w-4" /></button>
        <button aria-label="下一页" className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 disabled:opacity-40" disabled={page >= pages} onClick={() => setPage(page + 1)} type="button"><ArrowRight className="h-4 w-4" /></button>
      </div>
    </div>
  );
}

function MarketTrendsView({ snapshot }: { snapshot: CommunityMarketSnapshot }) {
  const trade = analysisRows(snapshot.analysis, "tradeTrends", "tradeTrends");
  const supply = analysisRows(snapshot.analysis, "supplyTrend", "supplyTrend");
  const cycle = analysisRows(snapshot.analysis, "tradeCycle", "tradeCycle6");
  const hot = analysisRows(snapshot.analysis, "hotIndex", "hotIndex");
  const confidence = analysisRows(snapshot.analysis, "confidenceIndex", "confidenceIndex");
  const roomTypes = analysisRows(snapshot.analysis, "roomType", "roomTypes");
  if ([trade, supply, cycle, hot, confidence, roomTypes].every((rows) => rows.length === 0)) return <MarketUnavailableState />;
  return (
    <div className="grid gap-8 py-6 lg:grid-cols-2" role="tabpanel">
      <MarketLineChart lines={[{ key: "avgTradePriceCommunity", label: "小区成交均价", color: "#2563eb" }, { key: "avgTradePriceDistrict", label: "区域成交均价", color: "#0f766e" }]} rows={trade} title="成交价格趋势" xKey="tradeDate" />
      <MarketLineChart lines={[{ key: "num", label: "供应套数", color: "#2563eb" }, { key: "takeLook", label: "带看", color: "#d97706" }, { key: "supplyDemandRatio", label: "供需比", color: "#0f766e" }]} rows={supply} title="供需趋势" xKey="listingDate" />
      <MarketLineChart lines={[{ key: "avgDealCycle", label: "平均成交周期", color: "#7c3aed" }]} rows={cycle} title="成交周期" xKey="tradeDate" />
      <MarketLineChart lines={[{ key: "hot", label: "热度", color: "#dc2626" }]} rows={hot} title="小区热度" xKey="listingDate" />
      <MarketLineChart lines={[{ key: "confidenceIndex", label: "信心指数", color: "#0891b2" }]} rows={confidence} title="市场信心" xKey="tradeDate" />
      <section>
        <h3 className="text-sm font-semibold text-slate-900">户型成交结构</h3>
        <div className="mt-4 divide-y divide-slate-200 border-y border-slate-200">
          {roomTypes.slice(-8).map((row, index) => <div className="grid grid-cols-3 gap-2 py-2 text-sm" key={`${String(row.tradeDate)}-${String(row.roomTypeFilter)}-${index}`}><span>{String(row.tradeDate ?? "-")}</span><span>{String(row.roomTypeFilter ?? "-")}</span><span className="text-right">{String(row.tradeNum ?? 0)} 套 · {formatUnitPrice(numberValue(row.avgTradePrice))}</span></div>)}
        </div>
      </section>
    </div>
  );
}

function MarketLineChart({ lines, rows, title, xKey }: { lines: { key: string; label: string; color: string }[]; rows: Record<string, unknown>[]; title: string; xKey: string }) {
  return (
    <section className="min-w-0">
      <h3 className="text-sm font-semibold text-slate-900">{title}</h3>
      <div className="mt-3 h-64 w-full">
        <ResponsiveContainer height="100%" width="100%">
          <LineChart data={rows} margin={{ top: 8, right: 12, left: -8, bottom: 4 }}>
            <CartesianGrid stroke="#e2e8f0" strokeDasharray="3 3" vertical={false} />
            <XAxis dataKey={xKey} tick={{ fill: "#64748b", fontSize: 10 }} />
            <YAxis tick={{ fill: "#64748b", fontSize: 10 }} />
            <Tooltip /><Legend wrapperStyle={{ fontSize: 11 }} />
            {lines.map((line) => <Line dataKey={line.key} dot={false} key={line.key} name={line.label} stroke={line.color} strokeWidth={2} type="monotone" />)}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}

function MarketSurroundingsView({ snapshot }: { snapshot: CommunityMarketSnapshot }) {
  const groups = surroundingGroups(snapshot.surroundings);
  if (groups.length === 0) return <MarketUnavailableState />;
  return (
    <div className="grid gap-x-8 gap-y-6 py-6 sm:grid-cols-2 lg:grid-cols-3" role="tabpanel">
      {groups.map((group) => (
        <section className="border-t border-slate-200 pt-4" key={group.name}>
          <h3 className="flex items-center gap-2 text-sm font-semibold text-slate-900"><MapPinned className="h-4 w-4 text-blue-600" />{group.name}<span className="font-normal text-slate-500">{group.total} 处</span></h3>
          <ul className="mt-3 space-y-2 text-sm text-slate-700">{group.items.map((item) => <li className="flex justify-between gap-3" key={`${group.name}-${item.name}`}><span className="min-w-0 break-words">{item.name}</span><span className="flex-none text-xs text-slate-500">{item.distance}</span></li>)}</ul>
        </section>
      ))}
    </div>
  );
}

function MarketUnavailableState() {
  return <div className="my-6 border-l-4 border-amber-400 bg-amber-50 px-5 py-4 text-sm text-amber-950">当前快照没有这一视图所需的完整来源数据。</div>;
}

function analysisRows(analysis: Record<string, unknown>, section: string, field: string): Record<string, unknown>[] {
  const sectionValue = asRecord(analysis?.[section]);
  const rows = sectionValue?.[field];
  return Array.isArray(rows) ? rows.filter((item): item is Record<string, unknown> => Boolean(asRecord(item))) : [];
}

function surroundingGroups(surroundings: Record<string, unknown>): { name: string; total: number; items: { name: string; distance: string }[] }[] {
  const poi = surroundings?.poi;
  if (!Array.isArray(poi)) return [];
  return poi.flatMap((rawGroup) => {
    const group = asRecord(rawGroup);
    const page = asRecord(group?.itemPageDate);
    const rows = Array.isArray(page?.rows) ? page.rows : [];
    const items = rows.slice(0, 5).flatMap((rawItem) => {
      const item = asRecord(rawItem);
      if (!item) return [];
      return [{ name: String(item.poiName ?? "未命名设施"), distance: item.distance == null ? "" : `${item.distance} m` }];
    });
    return group ? [{ name: String(group.bizType ?? "周边"), total: numberValue(page?.total), items }] : [];
  });
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value != null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

function numberValue(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : Number(value) || 0;
}

function formatComparisonValue(value: number | null, unit: string): string {
  return value == null ? "暂无" : `${new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 1 }).format(value)} ${unit}`;
}

function formatSigned(value: number | null, unit: string): string {
  if (value == null) return "暂无";
  const sign = value > 0 ? "+" : "";
  return `${sign}${new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 1 }).format(value)} ${unit}`;
}

function formatWan(value: number): string {
  return `${new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value)} 万`;
}

function formatDateOnly(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeZone: "Asia/Shanghai" }).format(new Date(value));
}

function CommunityMarketProfilePanel({ snapshot }: { snapshot: CommunityMarketSnapshot }) {
  const cards = [
    { label: "挂牌均价", value: formatUnitPrice(snapshot.listingAvgUnitPrice), detail: snapshot.latestListingDate ? `截至 ${snapshot.latestListingDate}` : "日期未知" },
    { label: "在售套数", value: formatCount(snapshot.listingCount), detail: snapshot.onSaleAreaRangeSqm ? `面积 ${snapshot.onSaleAreaRangeSqm} ㎡` : "面积段未知" },
    { label: "近三月成交", value: formatCount(snapshot.tradeCount3Months), detail: formatUnitPrice(snapshot.tradeUnitPrice3Months) },
    { label: "近六月成交均价", value: formatUnitPrice(snapshot.tradeAvgUnitPrice6Months), detail: snapshot.latestTradeDate ? `最新成交 ${snapshot.latestTradeDate}` : "成交日期未知" },
    { label: "近三月新增挂牌", value: formatCount(snapshot.newListingCount3Months), detail: formatUnitPrice(snapshot.newListingUnitPrice3Months) },
    { label: "带看与转化", value: formatCount(snapshot.takeLookCount), detail: snapshot.takeLookConversionRatePercent == null ? "转化率未知" : `转化率 ${snapshot.takeLookConversionRatePercent.toFixed(2)}%` },
  ];
  const profileGroups = [
    {
      icon: Building2,
      title: "建筑与规模",
      items: [
        { label: "省份", value: formatProfileLocation(snapshot.provinceName, snapshot.provinceCode) },
        { label: "住宅类型", value: formatProfileText(snapshot.propertyType) },
        { label: "标签", value: snapshot.propertyTags?.join("、") || "暂无" },
        { label: "楼栋数", value: formatProfileNumber(snapshot.buildingCount, "栋") },
        { label: "建筑类型", value: formatProfileText(snapshot.buildingType) },
        { label: "建成年份", value: formatProfileNumber(snapshot.buildingYear, "年") },
        { label: "开发商", value: formatProfileText(snapshot.developer) },
        { label: "户数", value: formatProfileNumber(snapshot.householdCount, "户") },
        { label: "容积率", value: formatProfileNumber(snapshot.plotRatio) },
        { label: "绿地面积", value: formatProfileNumber(snapshot.greenAreaSqm, "㎡") },
        { label: "绿化率", value: formatProfileNumber(snapshot.greeningRatePercent, "%") },
      ],
    },
    {
      icon: CarFront,
      title: "物业与停车",
      items: [
        { label: "物业公司", value: formatProfileText(snapshot.propertyManagementCompany) },
        { label: "物业费", value: formatProfileText(snapshot.propertyFee) },
        { label: "固定车位数", value: formatProfileNumber(snapshot.fixedParkingSpaces, "个") },
        { label: "车位比", value: formatProfileText(snapshot.parkingRatio) },
        { label: "停车费", value: formatProfileText(snapshot.parkingFee) },
      ],
    },
    {
      icon: Zap,
      title: "能源与管理",
      items: [
        { label: "供暖", value: formatProfileText(snapshot.heatingType) },
        { label: "用水", value: formatProfileText(snapshot.waterType) },
        { label: "用电", value: formatProfileText(snapshot.electricityType) },
        { label: "燃气费", value: formatProfileText(snapshot.gasCost) },
        { label: "封闭管理", value: formatProfileText(snapshot.closedManagement) },
        { label: "人车分流", value: formatProfileText(snapshot.manCarSeparation) },
      ],
    },
  ];

  return (
    <section aria-label="小区聚合行情与档案" className="border-b border-slate-200 pb-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-blue-700">小区聚合行情</p>
          <h2 className="mt-1 text-xl font-bold text-slate-900">{snapshot.communityName}</h2>
        </div>
        <p className="max-w-full break-all text-right text-xs text-slate-500">采集于 {formatDateTime(snapshot.collectedAt)} · {snapshot.sourceRef}</p>
      </div>
      <div className="mt-4 grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-6">
        {cards.map((card) => (
          <article key={card.label} className="rounded-md border border-slate-200 bg-white p-4">
            <h3 className="text-xs font-medium text-slate-500">{card.label}</h3>
            <p className="mt-2 break-words text-lg font-bold text-slate-900">{card.value}</p>
            <p className="mt-2 break-words text-xs text-slate-500">{card.detail}</p>
          </article>
        ))}
      </div>
      <div aria-label="小区档案" className="mt-6 grid gap-x-8 gap-y-6 lg:grid-cols-3">
        {profileGroups.map((group) => (
          <CommunityProfileGroup key={group.title} icon={group.icon} items={group.items} title={group.title} />
        ))}
      </div>
      <p className="mt-4 text-sm text-slate-600">
        房见公开接口提供的小区级聚合值。它不会替代单套挂牌、成交明细，也不参与房源粒度指标推算。
      </p>
    </section>
  );
}

function CommunityProfileGroup({
  icon: Icon,
  items,
  title,
}: {
  icon: typeof Building2;
  items: { label: string; value: string }[];
  title: string;
}) {
  return (
    <section className="min-w-0 border-t border-slate-200 pt-4">
      <h3 className="flex items-center gap-2 text-sm font-semibold text-slate-800">
        <Icon aria-hidden="true" className="h-4 w-4 flex-none text-blue-600" />
        {title}
      </h3>
      <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
        {items.map((item) => (
          <div key={item.label} className="min-w-0">
            <dt className="text-xs text-slate-500">{item.label}</dt>
            <dd className="mt-1 break-words font-medium text-slate-900">{item.value}</dd>
          </div>
        ))}
      </dl>
    </section>
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

function formatUnitPrice(value?: number | null): string {
  return value == null ? "暂无" : `${new Intl.NumberFormat("zh-CN").format(value)} 元/㎡`;
}

function formatCount(value?: number | null): string {
	return value == null ? "暂无" : `${value} 套`;
}

function formatProfileText(value?: string | null): string {
  return value?.trim() || "暂无";
}

function formatProfileNumber(value?: number | null, unit = ""): string {
  if (value == null) return "暂无";
  const formatted = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 4 }).format(value);
  return unit ? `${formatted} ${unit}` : formatted;
}

function formatProfileLocation(name?: string | null, code?: string | null): string {
  if (name && code) return `${name}（${code}）`;
  return name || code || "暂无";
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
