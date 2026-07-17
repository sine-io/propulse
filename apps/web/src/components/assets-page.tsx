"use client";

import {
  AlertTriangle,
  Building2,
  LoaderCircle,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createAsset,
  deleteAsset,
  getMarketListingDetail,
  getMarketListings,
  listAssets,
  searchNeighborhoods,
  updateAsset,
  type Asset,
  type CreateAssetInput,
  type MarketListing,
  type MarketListingDetail,
  type Neighborhood,
  type UpdateAssetInput,
} from "@/lib/api-client";

type PageState = "locked" | "loading" | "ready" | "failed";
type EditorMode = { kind: "create" } | { kind: "edit"; asset: Asset };
type SourceMode = "market_listing" | "manual";

type AssetForm = {
  name: string;
  originalPurchasePriceWan: string;
  purchasedOn: string;
  currentLoanBalanceWan: string;
  layout: string;
  areaSqm: string;
  floorBand: string;
  floorDescription: string;
  orientation: string;
  currentListingPriceWan: string;
};

const emptyForm: AssetForm = {
  name: "",
  originalPurchasePriceWan: "",
  purchasedOn: "",
  currentLoanBalanceWan: "",
  layout: "",
  areaSqm: "",
  floorBand: "",
  floorDescription: "",
  orientation: "",
  currentListingPriceWan: "",
};

export function AssetsPage() {
  const [state, setState] = useState<PageState>("loading");
  const [items, setItems] = useState<Asset[]>([]);
  const [selectedID, setSelectedID] = useState<string>();
  const [editor, setEditor] = useState<EditorMode>();
  const [deleteTarget, setDeleteTarget] = useState<Asset>();
  const [deleteState, setDeleteState] = useState<"idle" | "deleting" | "failed">("idle");
  const [loadVersion, setLoadVersion] = useState(0);

  const load = useCallback(() => setLoadVersion((value) => value + 1), []);

  useEffect(() => {
    const syncAccess = () => {
      if (!getAccessToken()) {
        setState("locked");
        setItems([]);
        setSelectedID(undefined);
        return;
      }
      load();
    };
    syncAccess();
    return subscribeToAccessToken(syncAccess);
  }, [load]);

  useEffect(() => {
    if (!getAccessToken() || loadVersion === 0) return;
    const controller = new AbortController();
    setState("loading");
    listAssets(1, 100, controller.signal)
      .then((response) => {
        setItems(response.items);
        setSelectedID((current) => current && response.items.some((item) => item.id === current) ? current : response.items[0]?.id);
        setState("ready");
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        setState(error instanceof ApiError && error.status === 401 ? "locked" : "failed");
      });
    return () => controller.abort();
  }, [loadVersion]);

  const selected = items.find((item) => item.id === selectedID);

  const confirmDelete = async () => {
    if (!deleteTarget || deleteState === "deleting") return;
    setDeleteState("deleting");
    try {
      await deleteAsset(deleteTarget.id);
      setDeleteTarget(undefined);
      setDeleteState("idle");
      load();
    } catch {
      setDeleteState("failed");
    }
  };

  return (
    <main className="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <header className="flex flex-wrap items-end justify-between gap-4 border-b border-slate-200 pb-6">
        <div>
          <p className="text-sm font-semibold text-blue-700">个人空间</p>
          <h1 className="mt-1 text-3xl font-bold text-slate-900">我的资产</h1>
        </div>
        <button
          type="button"
          onClick={() => setEditor({ kind: "create" })}
          disabled={state !== "ready"}
          className="inline-flex h-10 items-center gap-2 rounded-md bg-blue-600 px-4 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Plus aria-hidden="true" className="h-4 w-4" />
          新增资产
        </button>
      </header>

      {state === "locked" ? (
        <StateBand icon={Building2} title="个人资产已锁定" detail="解锁个人空间后可读取和维护资产档案。" />
      ) : null}
      {state === "loading" ? <Loading label="正在读取资产档案" /> : null}
      {state === "failed" ? (
        <StateBand
          icon={AlertTriangle}
          title="资产档案读取失败"
          detail="现有资产没有被修改。"
          action={<RetryButton onClick={load} />}
          tone="rose"
        />
      ) : null}
      {state === "ready" && items.length === 0 ? (
        <StateBand
          icon={Building2}
          title="还没有资产档案"
          detail=""
          action={<button type="button" onClick={() => setEditor({ kind: "create" })} className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-900 px-3 text-sm font-medium text-white"><Plus aria-hidden="true" className="h-4 w-4" />新增资产</button>}
        />
      ) : null}

      {state === "ready" && items.length > 0 ? (
        <div className="grid gap-8 py-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(18rem,0.65fr)]">
          <section aria-label="资产列表" className="min-w-0">
            <div className="overflow-x-auto border-y border-slate-200 bg-white">
              <table className="w-full min-w-[680px] text-left text-sm">
                <thead className="border-b border-slate-200 bg-slate-50 text-xs text-slate-500">
                  <tr>
                    <th className="px-4 py-3 font-medium">资产</th>
                    <th className="px-4 py-3 font-medium">房屋资料</th>
                    <th className="px-4 py-3 text-right font-medium">参考挂牌</th>
                    <th className="px-4 py-3 text-right font-medium">贷款余额</th>
                    <th className="w-24 px-4 py-3"><span className="sr-only">操作</span></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {items.map((asset) => (
                    <tr key={asset.id} className={selectedID === asset.id ? "bg-blue-50/60" : "bg-white hover:bg-slate-50"}>
                      <td className="px-4 py-3">
                        <button type="button" onClick={() => setSelectedID(asset.id)} className="text-left font-semibold text-slate-900 hover:text-blue-700">
                          {asset.name}
                        </button>
                        <p className="mt-0.5 text-xs text-slate-500">{asset.property.city} · {asset.property.district}</p>
                      </td>
                      <td className="px-4 py-3 text-slate-600">{asset.property.layout} · {formatNumber(asset.property.areaSqm)}㎡</td>
                      <td className="px-4 py-3 text-right font-medium text-slate-900">{asset.property.currentListingPriceWan == null ? "未记录" : `${formatNumber(asset.property.currentListingPriceWan)} 万`}</td>
                      <td className="px-4 py-3 text-right font-medium text-slate-900">{formatNumber(asset.currentLoanBalanceWan)} 万</td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-1">
                          <button type="button" title="编辑资产" aria-label={`编辑 ${asset.name}`} onClick={() => setEditor({ kind: "edit", asset })} className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-white hover:text-blue-700"><Pencil aria-hidden="true" className="h-4 w-4" /></button>
                          <button type="button" title="删除资产" aria-label={`删除 ${asset.name}`} onClick={() => { setDeleteTarget(asset); setDeleteState("idle"); }} className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-white hover:text-rose-700"><Trash2 aria-hidden="true" className="h-4 w-4" /></button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
          {selected ? <AssetDetail asset={selected} /> : null}
        </div>
      ) : null}

      {editor ? <AssetEditor mode={editor} onClose={() => setEditor(undefined)} onSaved={() => { setEditor(undefined); load(); }} /> : null}
      {deleteTarget ? (
        <ConfirmDialog
          title="删除资产档案"
          detail={`${deleteTarget.name} 将从资产列表移除，已有诊断报告仍保留当时快照。`}
          error={deleteState === "failed" ? "删除失败，资产档案仍然保留。" : undefined}
          pending={deleteState === "deleting"}
          onCancel={() => setDeleteTarget(undefined)}
          onConfirm={confirmDelete}
        />
      ) : null}
    </main>
  );
}

function AssetDetail({ asset }: { asset: Asset }) {
  return (
    <aside aria-label="资产详情" className="border-l-4 border-blue-500 bg-white px-5 py-4 shadow-sm">
      <div className="flex items-center gap-2">
        <Building2 aria-hidden="true" className="h-5 w-5 text-blue-700" />
        <h2 className="min-w-0 break-words text-base font-semibold text-slate-900">{asset.name}</h2>
      </div>
      <dl className="mt-4 divide-y divide-slate-100 text-sm">
        <DetailRow label="小区" value={asset.property.neighborhoodName} />
        <DetailRow label="户型面积" value={`${asset.property.layout} · ${formatNumber(asset.property.areaSqm)}㎡`} />
        <DetailRow label="楼层朝向" value={[asset.property.floorDescription || asset.property.floorBand, asset.property.orientation].filter(Boolean).join(" · ") || "未记录"} />
        <DetailRow label="原购入价" value={`${formatNumber(asset.originalPurchasePriceWan)} 万`} />
        <DetailRow label="购入日期" value={asset.purchasedOn} />
        <DetailRow label="当前贷款" value={`${formatNumber(asset.currentLoanBalanceWan)} 万`} />
      </dl>
      <div className="mt-4 border-t border-slate-200 pt-3 text-xs leading-5 text-slate-500">
        {asset.listingSource ? (
          <>
            <p className="font-medium text-slate-700">{asset.listingSource.dataSourceName} · {asset.listingSource.qualityStatus}</p>
            <p>采集于 {formatDateTime(asset.listingSource.collectedAt)}</p>
          </>
        ) : <p>房屋资料由用户手工确认。</p>}
        <p>资产确认于 {formatDateTime(asset.updatedAt)}</p>
      </div>
    </aside>
  );
}

function AssetEditor({ mode, onClose, onSaved }: { mode: EditorMode; onClose: () => void; onSaved: () => void }) {
  const editing = mode.kind === "edit" ? mode.asset : undefined;
  const [form, setForm] = useState<AssetForm>(() => editing ? {
    ...emptyForm,
    name: editing.name,
    originalPurchasePriceWan: String(editing.originalPurchasePriceWan),
    purchasedOn: editing.purchasedOn,
    currentLoanBalanceWan: String(editing.currentLoanBalanceWan),
  } : emptyForm);
  const [sourceMode, setSourceMode] = useState<SourceMode>("market_listing");
  const [query, setQuery] = useState("");
  const [neighborhoods, setNeighborhoods] = useState<Neighborhood[]>([]);
  const [neighborhoodState, setNeighborhoodState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [selectedNeighborhood, setSelectedNeighborhood] = useState<Neighborhood>();
  const [listings, setListings] = useState<MarketListing[]>([]);
  const [listingState, setListingState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [selectedRoomID, setSelectedRoomID] = useState("");
  const [listingDetail, setListingDetail] = useState<MarketListingDetail>();
  const [nameTouched, setNameTouched] = useState(Boolean(editing));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitState, setSubmitState] = useState<"idle" | "submitting" | "failed" | "unavailable">("idle");

  const set = (key: keyof AssetForm, value: string) => {
    setForm((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: "" }));
    setSubmitState("idle");
  };

  const search = async (event?: FormEvent) => {
    event?.preventDefault();
    setNeighborhoodState("loading");
    try {
      const response = await searchNeighborhoods({ q: query, page: 1, pageSize: 100 });
      setNeighborhoods(response.items);
      setNeighborhoodState("ready");
    } catch {
      setNeighborhoodState("failed");
    }
  };

  useEffect(() => {
    if (!editing) void search();
    // Initial catalog load only; subsequent searches are explicit.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!selectedNeighborhood) {
      setListings([]);
      setListingState("idle");
      return;
    }
    const controller = new AbortController();
    setListingState("loading");
    setSelectedRoomID("");
    setListingDetail(undefined);
    getMarketListings(selectedNeighborhood.id, { page: 1, pageSize: 100, sortBy: "date", sortOrder: "desc" }, controller.signal)
      .then((response) => { setListings(response.items); setListingState("ready"); })
      .catch(() => { if (!controller.signal.aborted) setListingState("failed"); });
    return () => controller.abort();
  }, [selectedNeighborhood]);

  useEffect(() => {
    if (!selectedNeighborhood || !selectedRoomID) {
      setListingDetail(undefined);
      return;
    }
    const controller = new AbortController();
    getMarketListingDetail(selectedNeighborhood.id, selectedRoomID, controller.signal)
      .then((detail) => {
        setListingDetail(detail);
        if (!nameTouched) setForm((current) => ({ ...current, name: `${detail.neighborhoodName} ${detail.layout}` }));
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setSelectedRoomID("");
          setSubmitState("unavailable");
        }
      });
    return () => controller.abort();
  }, [nameTouched, selectedNeighborhood, selectedRoomID]);

  const selectedListing = useMemo(() => listings.find((item) => item.roomId === selectedRoomID), [listings, selectedRoomID]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const nextErrors: Record<string, string> = {};
    const purchasePrice = positiveNumber(form.originalPurchasePriceWan, "originalPurchasePriceWan", nextErrors);
    const loan = nonNegativeNumber(form.currentLoanBalanceWan, "currentLoanBalanceWan", nextErrors);
    if (!form.purchasedOn || form.purchasedOn > new Date().toISOString().slice(0, 10)) nextErrors.purchasedOn = "请选择有效的购入日期";
    if (!form.name.trim()) nextErrors.name = "请填写资产名称";

    let input: CreateAssetInput | UpdateAssetInput;
    if (editing) {
      input = {
        name: form.name.trim(),
        originalPurchasePriceWan: purchasePrice,
        purchasedOn: form.purchasedOn,
        currentLoanBalanceWan: loan,
      };
    } else {
      if (!selectedNeighborhood) nextErrors.neighborhood = "请选择小区";
      if (sourceMode === "market_listing" && !selectedRoomID) nextErrors.listing = "请选择具体在售房源";
      const area = sourceMode === "manual" ? positiveNumber(form.areaSqm, "areaSqm", nextErrors) : 0;
      if (sourceMode === "manual" && !form.layout.trim()) nextErrors.layout = "请填写户型";
      const referencePrice = optionalPositiveNumber(form.currentListingPriceWan, "currentListingPriceWan", nextErrors);
      input = {
        name: form.name.trim(),
        neighborhoodId: selectedNeighborhood?.id ?? "",
        originalPurchasePriceWan: purchasePrice,
        purchasedOn: form.purchasedOn,
        currentLoanBalanceWan: loan,
        propertySelection: sourceMode === "market_listing"
          ? { mode: "market_listing", roomId: selectedRoomID }
          : {
              mode: "manual", layout: form.layout.trim(), areaSqm: area,
              floorBand: form.floorBand.trim(), floorDescription: form.floorDescription.trim(),
              orientation: form.orientation.trim(),
              ...(referencePrice == null ? {} : { currentListingPriceWan: referencePrice }),
            },
      };
    }
    setErrors(nextErrors);
    if (Object.values(nextErrors).some(Boolean)) return;

    setSubmitState("submitting");
    try {
      if (editing) await updateAsset(editing.id, input as UpdateAssetInput);
      else await createAsset(input as CreateAssetInput);
      onSaved();
    } catch (error) {
      setSubmitState(error instanceof ApiError && error.code === "listing_unavailable" ? "unavailable" : "failed");
    }
  };

  return (
    <div role="dialog" aria-modal="true" aria-labelledby="asset-editor-title" className="fixed inset-0 z-[70] overflow-y-auto bg-slate-950/45 px-4 py-8">
      <div className="mx-auto w-full max-w-3xl rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-5 py-4">
          <h2 id="asset-editor-title" className="text-base font-semibold text-slate-900">{editing ? "编辑资产" : "新增资产"}</h2>
          <button type="button" aria-label="关闭" title="关闭" onClick={onClose} className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100"><X aria-hidden="true" className="h-4 w-4" /></button>
        </div>
        <form onSubmit={submit} className="space-y-6 p-5" noValidate>
          {editing ? (
            <div className="border-l-4 border-blue-500 bg-blue-50 px-4 py-3 text-sm text-blue-950">
              {editing.property.neighborhoodName} · {editing.property.layout} · {formatNumber(editing.property.areaSqm)}㎡
            </div>
          ) : (
            <>
              <section>
                <h3 className="text-sm font-semibold text-slate-900">小区</h3>
                <div className="mt-3 flex gap-2">
                  <input aria-label="搜索小区" value={query} onChange={(event) => setQuery(event.target.value)} className="h-10 min-w-0 flex-1 rounded-md border border-slate-300 px-3 text-sm outline-none focus:border-blue-500" />
                  <button type="button" onClick={() => void search()} className="inline-flex h-10 items-center gap-2 rounded-md bg-slate-900 px-3 text-sm font-medium text-white"><Search aria-hidden="true" className="h-4 w-4" />搜索</button>
                </div>
                {neighborhoodState === "loading" ? <Loading label="正在读取小区" compact /> : null}
                {neighborhoodState === "failed" ? <InlineError label="小区读取失败" retry={() => void search()} /> : null}
                {neighborhoodState === "ready" ? (
                  <div className="mt-3 grid max-h-40 grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2">
                    {neighborhoods.map((item) => (
                      <button key={item.id} type="button" aria-pressed={selectedNeighborhood?.id === item.id} onClick={() => { setSelectedNeighborhood(item); if (!nameTouched) setForm((current) => ({ ...current, name: item.name })); }} className="min-w-0 rounded-md border border-slate-200 px-3 py-2 text-left text-sm aria-pressed:border-blue-500 aria-pressed:bg-blue-50">
                        <span className="block truncate font-medium text-slate-900">{item.name}</span><span className="text-xs text-slate-500">{item.city} · {item.area}</span>
                      </button>
                    ))}
                  </div>
                ) : null}
                {errors.neighborhood ? <FieldError>{errors.neighborhood}</FieldError> : null}
              </section>

              <section className="border-t border-slate-200 pt-5">
                <h3 className="text-sm font-semibold text-slate-900">房屋资料</h3>
                <div className="mt-3 inline-flex rounded-md border border-slate-300 p-1">
                  <ModeButton selected={sourceMode === "market_listing"} onClick={() => setSourceMode("market_listing")}>当前挂牌</ModeButton>
                  <ModeButton selected={sourceMode === "manual"} onClick={() => setSourceMode("manual")}>手工补充</ModeButton>
                </div>
                {sourceMode === "market_listing" ? (
                  <div className="mt-3">
                    {listingState === "idle" ? <p className="text-sm text-slate-500">选择小区后读取当前在售房源。</p> : null}
                    {listingState === "loading" ? <Loading label="正在读取当前房源" compact /> : null}
                    {listingState === "failed" ? <InlineError label="房源读取失败" retry={() => setSelectedNeighborhood((current) => current ? { ...current } : current)} /> : null}
                    {listingState === "ready" && listings.length === 0 ? <p className="border-l-4 border-amber-400 bg-amber-50 px-4 py-3 text-sm text-amber-950">该小区暂无当前在售房源，可切换为手工补充。</p> : null}
                    {listingState === "ready" && listings.length > 0 ? (
                      <div className="max-h-56 overflow-y-auto border-y border-slate-200">
                        {listings.map((listing) => (
                          <label key={listing.roomId} className="flex cursor-pointer items-center gap-3 border-b border-slate-100 px-3 py-2 last:border-b-0 hover:bg-slate-50">
                            <input type="radio" name="asset-listing" checked={selectedRoomID === listing.roomId} onChange={() => setSelectedRoomID(listing.roomId)} className="h-4 w-4 accent-blue-600" />
                            <span className="min-w-0 flex-1 text-sm text-slate-700">{listing.layout} · {formatNumber(listing.areaSqm)}㎡ · {listing.floorDescription || listing.floorBand || "楼层未标注"} · {listing.orientation || "朝向未标注"}</span>
                            <span className="flex-none text-sm font-semibold text-slate-900">{formatNumber(listing.listingTotalPriceWan)} 万</span>
                          </label>
                        ))}
                      </div>
                    ) : null}
                    {selectedListing && listingDetail ? <p className="mt-2 text-xs text-slate-500">采集于 {formatDateTime(listingDetail.collectedAt)} · {freshnessLabel(listingDetail.freshness)}</p> : null}
                    {errors.listing ? <FieldError>{errors.listing}</FieldError> : null}
                  </div>
                ) : (
                  <div className="mt-4 grid gap-4 sm:grid-cols-2">
                    <TextField label="户型" value={form.layout} error={errors.layout} onChange={(value) => { set("layout", value); if (!nameTouched && selectedNeighborhood) setForm((current) => ({ ...current, name: `${selectedNeighborhood.name} ${value}`.trim() })); }} />
                    <TextField label="建筑面积 (㎡)" value={form.areaSqm} error={errors.areaSqm} inputMode="decimal" onChange={(value) => set("areaSqm", value)} />
                    <TextField label="楼层区间" value={form.floorBand} onChange={(value) => set("floorBand", value)} />
                    <TextField label="楼层描述" value={form.floorDescription} onChange={(value) => set("floorDescription", value)} />
                    <TextField label="朝向" value={form.orientation} onChange={(value) => set("orientation", value)} />
                    <TextField label="当前参考价 (万，可选)" value={form.currentListingPriceWan} error={errors.currentListingPriceWan} inputMode="decimal" onChange={(value) => set("currentListingPriceWan", value)} />
                  </div>
                )}
              </section>
            </>
          )}

          <section className="border-t border-slate-200 pt-5">
            <h3 className="text-sm font-semibold text-slate-900">资产事实</h3>
            <div className="mt-4 grid gap-4 sm:grid-cols-2">
              <TextField label="资产名称" value={form.name} error={errors.name} onChange={(value) => { setNameTouched(true); set("name", value); }} />
              <TextField label="原购入价 (万)" value={form.originalPurchasePriceWan} error={errors.originalPurchasePriceWan} inputMode="decimal" onChange={(value) => set("originalPurchasePriceWan", value)} />
              <DateField label="购入日期" value={form.purchasedOn} error={errors.purchasedOn} onChange={(value) => set("purchasedOn", value)} />
              <TextField label="当前贷款余额 (万)" value={form.currentLoanBalanceWan} error={errors.currentLoanBalanceWan} inputMode="decimal" onChange={(value) => set("currentLoanBalanceWan", value)} />
            </div>
          </section>

          {submitState === "unavailable" ? <p role="alert" className="border-l-4 border-amber-400 bg-amber-50 px-4 py-3 text-sm text-amber-950">所选房源已不在当前在售列表，请重新选择或手工补充。</p> : null}
          {submitState === "failed" ? <p role="alert" className="text-sm text-rose-700">资产保存失败，已填写内容仍保留。</p> : null}
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
            <button type="button" onClick={onClose} className="h-10 rounded-md border border-slate-300 px-4 text-sm font-medium text-slate-700 hover:bg-slate-50">取消</button>
            <button type="submit" disabled={submitState === "submitting"} className="inline-flex h-10 items-center gap-2 rounded-md bg-blue-600 px-4 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {submitState === "submitting" ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : null}
              {editing ? "保存修改" : "建立资产"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function StateBand({ action, detail, icon: Icon, title, tone = "amber" }: { action?: React.ReactNode; detail: string; icon: typeof Building2; title: string; tone?: "amber" | "rose" }) {
  return <section className={`mt-6 border-l-4 px-5 py-5 ${tone === "rose" ? "border-rose-400 bg-rose-50" : "border-amber-400 bg-amber-50"}`}><div className="flex items-start gap-3"><Icon aria-hidden="true" className="mt-0.5 h-5 w-5 flex-none" /><div className="min-w-0 flex-1"><h2 className="font-semibold text-slate-900">{title}</h2>{detail ? <p className="mt-1 text-sm text-slate-600">{detail}</p> : null}{action ? <div className="mt-4">{action}</div> : null}</div></div></section>;
}

function Loading({ compact = false, label }: { compact?: boolean; label: string }) {
  return <div role="status" className={`flex items-center justify-center gap-2 text-sm text-slate-500 ${compact ? "py-4" : "min-h-[18rem]"}`}><LoaderCircle aria-hidden="true" className="h-5 w-5 animate-spin" />{label}</div>;
}

function InlineError({ label, retry }: { label: string; retry: () => void }) {
  return <div role="alert" className="mt-3 flex items-center justify-between gap-3 border-l-4 border-rose-400 bg-rose-50 px-3 py-2 text-sm text-rose-900"><span>{label}</span><button type="button" onClick={retry} className="inline-flex items-center gap-1 font-medium"><RefreshCw aria-hidden="true" className="h-4 w-4" />重试</button></div>;
}

function RetryButton({ onClick }: { onClick: () => void }) {
  return <button type="button" onClick={onClick} className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700"><RefreshCw aria-hidden="true" className="h-4 w-4" />重试</button>;
}

function ModeButton({ children, onClick, selected }: { children: React.ReactNode; onClick: () => void; selected: boolean }) {
  return <button type="button" aria-pressed={selected} onClick={onClick} className="h-8 rounded px-3 text-sm font-medium text-slate-600 aria-pressed:bg-slate-900 aria-pressed:text-white">{children}</button>;
}

function TextField({ error, inputMode, label, onChange, value }: { error?: string; inputMode?: "decimal"; label: string; onChange: (value: string) => void; value: string }) {
  return <label className="block"><span className="mb-1 block text-xs font-medium text-slate-600">{label}</span><input aria-label={label} value={value} inputMode={inputMode} onChange={(event) => onChange(event.target.value)} aria-invalid={Boolean(error)} className="h-10 w-full rounded-md border border-slate-300 px-3 text-sm outline-none focus:border-blue-500 aria-[invalid=true]:border-rose-500" />{error ? <FieldError>{error}</FieldError> : null}</label>;
}

function DateField({ error, label, onChange, value }: { error?: string; label: string; onChange: (value: string) => void; value: string }) {
  return <label className="block"><span className="mb-1 block text-xs font-medium text-slate-600">{label}</span><input aria-label={label} type="date" max={new Date().toISOString().slice(0, 10)} value={value} onChange={(event) => onChange(event.target.value)} aria-invalid={Boolean(error)} className="h-10 w-full rounded-md border border-slate-300 px-3 text-sm outline-none focus:border-blue-500 aria-[invalid=true]:border-rose-500" />{error ? <FieldError>{error}</FieldError> : null}</label>;
}

function FieldError({ children }: { children: React.ReactNode }) { return <span className="mt-1 block text-xs text-rose-600">{children}</span>; }
function DetailRow({ label, value }: { label: string; value: string }) { return <div className="flex justify-between gap-4 py-2"><dt className="text-slate-500">{label}</dt><dd className="break-words text-right font-medium text-slate-900">{value}</dd></div>; }

function ConfirmDialog({ detail, error, onCancel, onConfirm, pending, title }: { detail: string; error?: string; onCancel: () => void; onConfirm: () => void; pending: boolean; title: string }) {
  return <div role="dialog" aria-modal="true" aria-labelledby="confirm-delete-title" className="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/45 px-4"><div className="w-full max-w-md rounded-lg bg-white p-5 shadow-xl"><h2 id="confirm-delete-title" className="font-semibold text-slate-900">{title}</h2><p className="mt-2 text-sm leading-6 text-slate-600">{detail}</p>{error ? <p role="alert" className="mt-3 text-sm text-rose-700">{error}</p> : null}<div className="mt-5 flex justify-end gap-2"><button type="button" onClick={onCancel} disabled={pending} className="h-9 rounded-md border border-slate-300 px-3 text-sm font-medium text-slate-700">取消</button><button type="button" onClick={onConfirm} disabled={pending} className="inline-flex h-9 items-center gap-2 rounded-md bg-rose-600 px-3 text-sm font-medium text-white disabled:opacity-50">{pending ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Trash2 aria-hidden="true" className="h-4 w-4" />}确认删除</button></div></div></div>;
}

function positiveNumber(raw: string, key: string, errors: Record<string, string>): number {
  const value = Number(raw);
  if (!raw.trim() || !Number.isFinite(value) || value <= 0) errors[key] = "请输入大于 0 的数字";
  return value;
}

function nonNegativeNumber(raw: string, key: string, errors: Record<string, string>): number {
  const value = Number(raw);
  if (!raw.trim() || !Number.isFinite(value) || value < 0) errors[key] = "请输入不小于 0 的数字";
  return value;
}

function optionalPositiveNumber(raw: string, key: string, errors: Record<string, string>): number | undefined {
  if (!raw.trim()) return undefined;
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) errors[key] = "请输入大于 0 的数字";
  return value;
}

function freshnessLabel(value: MarketListingDetail["freshness"]) {
  return value === "current" ? "数据较新" : value === "stale" ? "数据已陈旧" : value === "expired" ? "数据已过期" : "新鲜度未知";
}

function formatNumber(value: number) { return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value); }
function formatDateTime(value: string) { return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)); }
