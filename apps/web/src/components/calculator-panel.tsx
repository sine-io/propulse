"use client";

import Link from "next/link";
import {
  Calculator,
  CheckCircle,
  ExternalLink,
  History,
  LoaderCircle,
  LockKeyhole,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { SearchCombobox } from "@/components/search-combobox";

import { getAccessToken, subscribeToAccessToken } from "@/lib/access-token";
import {
  ApiError,
  createCapacityCalculation,
  getCapacityAssumptions,
  getCapacityCalculation,
  getMarketListingDetail,
  getMarketListings,
  listAssets,
  listCapacityCalculations,
  searchNeighborhoods,
  type Asset,
  type CalculationHistorySummary,
  type CalculationOverrides,
  type CalculationResponse,
  type CapacityAssumptionsResponse,
  type HousingCapacityInput,
  type LoanPlan,
  type MarketListing,
  type MarketListingDetail,
  type Neighborhood,
} from "@/lib/api-client";

type FamilyKey =
  | "cashOnHand"
  | "monthlyIncome"
  | "currentMonthlyMortgage"
  | "acceptableMonthlyMortgage"
  | "renovationBudget"
  | "transitionRentCost";

type FamilyForm = Record<FamilyKey, string>;
type FieldErrors = Partial<Record<FamilyKey | "oldChoice" | "oldSalePrice" | "oldLoanBalance" | "targetNeighborhood" | "targetListing" | "targetPrice" | "combinedTotal" | "commercialAmount" | "providentAmount" | "manual", string>>;

type ManualForm = {
  commercialRate: string;
  providentRate: string;
  downPaymentRate: string;
  deedTax: string;
  vat: string;
  surcharges: string;
  incomeTax: string;
};

const emptyFamily: FamilyForm = {
  cashOnHand: "",
  monthlyIncome: "",
  currentMonthlyMortgage: "",
  acceptableMonthlyMortgage: "",
  renovationBudget: "",
  transitionRentCost: "",
};

const emptyManual: ManualForm = {
  commercialRate: "",
  providentRate: "",
  downPaymentRate: "",
  deedTax: "",
  vat: "",
  surcharges: "",
  incomeTax: "",
};

const familyFields: Array<{ key: FamilyKey; label: string; scale?: number }> = [
  { key: "cashOnHand", label: "当前可用现金 (万)" },
  { key: "monthlyIncome", label: "家庭月收入 (万)" },
  { key: "currentMonthlyMortgage", label: "当前月供 (元)", scale: 1 / 10000 },
  { key: "acceptableMonthlyMortgage", label: "可接受新月供 (元)", scale: 1 / 10000 },
  { key: "renovationBudget", label: "装修预算 (万)" },
  { key: "transitionRentCost", label: "过渡成本 (万)" },
];

const pressureCopy: Record<CalculationResponse["result"]["pressureLevel"], { label: string; tone: string }> = {
  safe: { label: "安全", tone: "text-emerald-700" },
  strained: { label: "偏高", tone: "text-amber-700" },
  danger: { label: "危险", tone: "text-rose-700" },
};

export function CalculatorPanel() {
  const [accessState, setAccessState] = useState<"checking" | "locked" | "unlocked">("checking");
  const [assets, setAssets] = useState<Asset[]>([]);
  const [assetsState, setAssetsState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [assetsVersion, setAssetsVersion] = useState(0);
  const [oldChoice, setOldChoice] = useState<"none" | string>("none");
  const [oldSalePrice, setOldSalePrice] = useState("0");
  const [oldLoanBalance, setOldLoanBalance] = useState("0");
  const [oldPriceConfirmed, setOldPriceConfirmed] = useState(true);
  const [oldHomeOnlyFamilyHome, setOldHomeOnlyFamilyHome] = useState(false);

  const [neighborhoodQuery, setNeighborhoodQuery] = useState("");
  const [neighborhoods, setNeighborhoods] = useState<Neighborhood[]>([]);
  const [neighborhoodsState, setNeighborhoodsState] = useState<"loading" | "ready" | "failed">("loading");
  const [neighborhoodsVersion, setNeighborhoodsVersion] = useState(0);
  const [selectedNeighborhood, setSelectedNeighborhood] = useState<Neighborhood>();
  const [listings, setListings] = useState<MarketListing[]>([]);
  const [listingsState, setListingsState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [listingsVersion, setListingsVersion] = useState(0);
  const [listingQuery, setListingQuery] = useState("");
  const [selectedListing, setSelectedListing] = useState<MarketListing>();
  const [targetListing, setTargetListing] = useState<MarketListingDetail>();
  const [targetDetailState, setTargetDetailState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [targetDetailVersion, setTargetDetailVersion] = useState(0);
  const [targetPrice, setTargetPrice] = useState("");
  const [targetPriceConfirmed, setTargetPriceConfirmed] = useState(false);

  const [family, setFamily] = useState<FamilyForm>(emptyFamily);
  const [homePurchaseOrder, setHomePurchaseOrder] = useState<"first" | "second">("first");
  const [taxBurdenMode, setTaxBurdenMode] = useState<"statutory" | "buyer_all">("statutory");
  const [loanType, setLoanType] = useState<LoanPlan["type"]>("commercial");
  const [loanTermMonths, setLoanTermMonths] = useState(360);
  const [repaymentMethod, setRepaymentMethod] = useState<LoanPlan["repaymentMethod"]>("equal_installment");
  const [combinedTotal, setCombinedTotal] = useState("");
  const [commercialAmount, setCommercialAmount] = useState("");
  const [providentAmount, setProvidentAmount] = useState("");
  const [manual, setManual] = useState<ManualForm>(emptyManual);

  const [assumptions, setAssumptions] = useState<CapacityAssumptionsResponse>();
  const [assumptionsState, setAssumptionsState] = useState<"loading" | "ready" | "failed">("loading");
  const [assumptionsVersion, setAssumptionsVersion] = useState(0);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [report, setReport] = useState<CalculationResponse>();
  const [reportState, setReportState] = useState<"idle" | "loading" | "ready" | "failed" | "missing">("idle");
  const [reportSnapshotLabel, setReportSnapshotLabel] = useState<"刚刚生成" | "历史快照">("历史快照");
  const [apiError, setApiError] = useState<string>();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [historyItems, setHistoryItems] = useState<CalculationHistorySummary[]>([]);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyQuery, setHistoryQuery] = useState("");
  const [historyState, setHistoryState] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [historyVersion, setHistoryVersion] = useState(0);
  const [selectedHistory, setSelectedHistory] = useState<CalculationHistorySummary>();
  const requestVersion = useRef(0);
  const submitController = useRef<AbortController | undefined>(undefined);
  const reportController = useRef<AbortController | undefined>(undefined);
  const historyInitialLoad = useRef(true);

  const selectedAsset = assets.find((asset) => asset.id === oldChoice);
  const selectedOption = assumptions?.loanOptions.find((option) => option.type === loanType);
  const targetNeighborhoodID = selectedNeighborhood?.id ?? "";
  const filteredListings = useMemo(() => {
    const query = normalizeSearch(listingQuery);
    if (!query) return listings;
    return listings.filter((listing) => normalizeSearch([
      listing.layout,
      `${listing.areaSqm}㎡`,
      listing.floorDescription,
      listing.floorBand,
      listing.orientation,
      `${listing.listingTotalPriceWan}万`,
    ].filter(Boolean).join(" ")).includes(query));
  }, [listingQuery, listings]);

  const markDraftChanged = useCallback(() => {
    requestVersion.current += 1;
    submitController.current?.abort();
    submitController.current = undefined;
    setIsSubmitting(false);
    setApiError(undefined);
  }, []);

  const loadHistoricalReport = useCallback((summary: CalculationHistorySummary) => {
    reportController.current?.abort();
    const controller = new AbortController();
    reportController.current = controller;
    setSelectedHistory(summary);
    setReport(undefined);
    setReportState("loading");
    setReportSnapshotLabel("历史快照");
    getCapacityCalculation(summary.id, controller.signal)
      .then((response) => {
        setReport(response);
        setReportState("ready");
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        setReport(undefined);
        if (error instanceof ApiError && error.status === 404) setReportState("missing");
        else {
          if (error instanceof ApiError && error.status === 401) setAccessState("locked");
          setReportState("failed");
        }
      });
  }, []);

  useEffect(() => {
    const sync = () => {
      const unlocked = Boolean(getAccessToken());
      setAccessState(unlocked ? "unlocked" : "locked");
      if (unlocked) setAssetsVersion((value) => value + 1);
      else {
        setAssets([]);
        setAssetsState("idle");
        historyInitialLoad.current = true;
        reportController.current?.abort();
        setHistoryItems([]);
        setHistoryTotal(0);
        setHistoryState("idle");
        setSelectedHistory(undefined);
        setReport(undefined);
        setReportState("idle");
      }
    };
    sync();
    return subscribeToAccessToken(sync);
  }, []);

  useEffect(() => {
    if (accessState !== "unlocked") return;
    const controller = new AbortController();
    setAssetsState("loading");
    listAssets(1, 100, controller.signal)
      .then((response) => { setAssets(response.items); setAssetsState("ready"); })
      .catch((error) => {
        if (controller.signal.aborted) return;
        if (error instanceof ApiError && error.status === 401) setAccessState("locked");
        else setAssetsState("failed");
      });
    return () => controller.abort();
  }, [accessState, assetsVersion]);

  useEffect(() => {
    if (accessState !== "unlocked") return;
    const controller = new AbortController();
    setHistoryState("loading");
    const timeout = window.setTimeout(() => {
      listCapacityCalculations({ q: historyQuery.trim(), page: 1, pageSize: 20 }, controller.signal)
        .then((response) => {
          setHistoryItems(response.items);
          setHistoryTotal(response.total);
          setHistoryState("ready");
          if (!historyInitialLoad.current) return;
          historyInitialLoad.current = false;
          if (response.items[0]) loadHistoricalReport(response.items[0]);
          else {
            setReport(undefined);
            setReportState("idle");
          }
        })
        .catch((error) => {
          if (controller.signal.aborted) return;
          if (error instanceof ApiError && error.status === 401) setAccessState("locked");
          else setHistoryState("failed");
        });
    }, 250);
    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, [accessState, historyQuery, historyVersion, loadHistoricalReport]);

  useEffect(() => {
    const controller = new AbortController();
    setNeighborhoodsState("loading");
    const timeout = window.setTimeout(() => {
      searchNeighborhoods({ q: neighborhoodQuery.trim(), page: 1, pageSize: 100 }, controller.signal)
        .then((response) => { setNeighborhoods(response.items); setNeighborhoodsState("ready"); })
        .catch(() => { if (!controller.signal.aborted) setNeighborhoodsState("failed"); });
    }, 300);
    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, [neighborhoodQuery, neighborhoodsVersion]);

  useEffect(() => {
    if (!targetNeighborhoodID) {
      setListings([]);
      setListingsState("idle");
      return;
    }
    const controller = new AbortController();
    setListingsState("loading");
    getMarketListings(targetNeighborhoodID, { page: 1, pageSize: 100, sortBy: "date", sortOrder: "desc" }, controller.signal)
      .then((response) => { setListings(response.items); setListingsState("ready"); })
      .catch(() => { if (!controller.signal.aborted) setListingsState("failed"); });
    return () => controller.abort();
  }, [listingsVersion, targetNeighborhoodID]);

  useEffect(() => {
    if (!targetNeighborhoodID || !selectedListing) {
      setTargetListing(undefined);
      setTargetDetailState("idle");
      return;
    }
    const controller = new AbortController();
    setTargetDetailState("loading");
    getMarketListingDetail(targetNeighborhoodID, selectedListing.roomId, controller.signal)
      .then((detail) => {
        setTargetListing(detail);
        setTargetPrice(String(detail.listingTotalPriceWan));
        setTargetPriceConfirmed(false);
        setTargetDetailState("ready");
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        setTargetListing(undefined);
        setTargetDetailState("failed");
        setApiError(error instanceof ApiError && error.code === "listing_unavailable" ? "所选目标房已下架，请重新选择。" : "目标房详情读取失败，请重试。");
      });
    return () => controller.abort();
  }, [selectedListing, targetDetailVersion, targetNeighborhoodID]);

  useEffect(() => {
    const controller = new AbortController();
    const city = selectedNeighborhood?.city ?? "天津";
    setAssumptionsState("loading");
    getCapacityAssumptions({ city, homePurchaseOrder, loanTermMonths }, controller.signal)
      .then((response) => { setAssumptions(response); setAssumptionsState("ready"); })
      .catch(() => { if (!controller.signal.aborted) { setAssumptions(undefined); setAssumptionsState("failed"); } });
    return () => controller.abort();
  }, [assumptionsVersion, homePurchaseOrder, loanTermMonths, selectedNeighborhood?.city]);

  useEffect(() => () => {
    submitController.current?.abort();
    reportController.current?.abort();
  }, []);

  const chooseOldHome = (choice: "none" | Asset) => {
    if (choice === "none") {
      setOldChoice("none");
      setOldSalePrice("0");
      setOldLoanBalance("0");
      setOldPriceConfirmed(true);
      setOldHomeOnlyFamilyHome(false);
    } else {
      setOldChoice(choice.id);
      setOldSalePrice(choice.property.currentListingPriceWan == null ? "" : String(choice.property.currentListingPriceWan));
      setOldLoanBalance(String(choice.currentLoanBalanceWan));
      setOldPriceConfirmed(false);
    }
    setFieldErrors((current) => ({ ...current, oldChoice: undefined, oldSalePrice: undefined, oldLoanBalance: undefined }));
    markDraftChanged();
  };

  const chooseNeighborhood = (item?: Neighborhood) => {
    setSelectedNeighborhood(item);
    setSelectedListing(undefined);
    setTargetListing(undefined);
    setTargetDetailState("idle");
    setListingQuery("");
    setTargetPrice("");
    setTargetPriceConfirmed(false);
    setFieldErrors((current) => ({ ...current, targetNeighborhood: undefined, targetListing: undefined, targetPrice: undefined }));
    markDraftChanged();
  };

  const chooseListing = (listing?: MarketListing) => {
    setSelectedListing(listing);
    setTargetListing(undefined);
    setTargetDetailState(listing ? "loading" : "idle");
    setTargetPrice(listing ? String(listing.listingTotalPriceWan) : "");
    setTargetPriceConfirmed(false);
    setFieldErrors((current) => ({ ...current, targetListing: undefined, targetPrice: undefined }));
    markDraftChanged();
  };

  const updateFamily = (key: FamilyKey, value: string) => {
    setFamily((current) => ({ ...current, [key]: value }));
    setFieldErrors((current) => ({ ...current, [key]: undefined }));
    markDraftChanged();
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!assumptions || assumptionsState !== "ready" || !selectedOption) return;
    const parsed = buildCalculationInput({
      assumptions,
      combinedTotal,
      commercialAmount,
      family,
      homePurchaseOrder,
      loanTermMonths,
      loanType,
      manual,
      oldChoice,
      oldHomeOnlyFamilyHome,
      oldLoanBalance,
      oldPriceConfirmed,
      oldSalePrice,
      providentAmount,
      repaymentMethod,
      selectedAsset,
      targetListing,
      targetPrice,
      targetPriceConfirmed,
      taxBurdenMode,
    });
    setFieldErrors(parsed.errors);
    setApiError(undefined);
    if (!parsed.input) return;
    if (accessState !== "unlocked") {
      setApiError("个人空间尚未解锁。");
      return;
    }

    submitController.current?.abort();
    const controller = new AbortController();
    submitController.current = controller;
    const version = ++requestVersion.current;
    setIsSubmitting(true);
    try {
      const response = await createCapacityCalculation(parsed.input, controller.signal);
      if (requestVersion.current === version) {
        const summary = calculationSummaryFromReport(response);
        setReport(response);
        setReportState("ready");
        setReportSnapshotLabel("刚刚生成");
        setSelectedHistory(summary);
        setHistoryQuery("");
        setHistoryState("ready");
        setHistoryItems((current) => {
          const exists = current.some((item) => item.id === summary.id);
          if (!exists) setHistoryTotal((total) => total + 1);
          return [summary, ...current.filter((item) => item.id !== summary.id)].slice(0, 20);
        });
      }
    } catch (error) {
      if (!controller.signal.aborted && requestVersion.current === version) {
        if (error instanceof ApiError && error.status === 401) setApiError("个人空间尚未解锁。");
        else if (error instanceof ApiError && error.code === "listing_unavailable") setApiError("目标房已下架，请重新选择当前房源。");
        else if (error instanceof ApiError && error.code === "selection_not_found") setApiError("资产或目标房已失效，请刷新后重新选择。");
        else setApiError("诊断报告生成失败，请稍后重试。");
      }
    } finally {
      if (requestVersion.current === version) {
        setIsSubmitting(false);
        submitController.current = undefined;
      }
    }
  };

  return (
    <div className="space-y-8">
      <section className="overflow-visible border border-slate-200 bg-slate-200 shadow-sm">
        <form onSubmit={submit} noValidate>
          <div className="grid grid-cols-12 gap-px">
            <StepSection className="col-span-12 bg-white md:col-span-6 xl:col-span-4" number="1" title="选择旧房产">
            <SelectField
              label="选择旧房产"
              value={oldChoice}
              onChange={(value) => {
                if (value === "none") chooseOldHome("none");
                else {
                  const asset = assets.find((item) => item.id === value);
                  if (asset) chooseOldHome(asset);
                }
              }}
              options={[
                { value: "none", label: "无旧房" },
                ...assets.map((asset) => ({ value: asset.id, label: `${asset.name} · ${asset.property.layout}` })),
              ]}
            />
            <div className="min-h-6">
              {accessState === "locked" ? <StatusBand tone="amber">个人资产已锁定，当前按无旧房测算。</StatusBand> : null}
              {assetsState === "loading" ? <InlineLoading label="正在读取资产" /> : null}
              {assetsState === "failed" ? <RetryState label="资产读取失败" onRetry={() => setAssetsVersion((value) => value + 1)} /> : null}
              {assetsState === "ready" && assets.length === 0 ? <StatusBand tone="slate"><span>暂无资产档案。</span> <Link href="/assets" className="font-medium text-blue-700 hover:underline">新增资产</Link></StatusBand> : null}
            </div>
            {fieldErrors.oldChoice ? <FieldError>{fieldErrors.oldChoice}</FieldError> : null}
            {selectedAsset ? (
              <div className="mt-4 grid gap-4 border-l-4 border-blue-500 bg-blue-50 p-4 sm:grid-cols-2 md:grid-cols-1 lg:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                <NumberField label="旧房预期售价 (万)" value={oldSalePrice} error={fieldErrors.oldSalePrice} onChange={(value) => { setOldSalePrice(value); setOldPriceConfirmed(false); markDraftChanged(); }} />
                <NumberField label="当前贷款余额 (万)" value={oldLoanBalance} error={fieldErrors.oldLoanBalance} onChange={(value) => { setOldLoanBalance(value); markDraftChanged(); }} />
                <ConfirmPrice checked={oldPriceConfirmed} onChange={(checked) => { setOldPriceConfirmed(checked); markDraftChanged(); }} label="确认采用该旧房预期售价" />
                <label className="flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" checked={oldHomeOnlyFamilyHome} onChange={(event) => { setOldHomeOnlyFamilyHome(event.target.checked); markDraftChanged(); }} className="h-4 w-4 accent-blue-600" />旧房是家庭唯一住房</label>
                <p className="text-xs text-slate-500 sm:col-span-2 md:col-span-1 lg:col-span-2 xl:col-span-1 2xl:col-span-2">参考挂牌 {selectedAsset.property.currentListingPriceWan == null ? "未记录" : `${formatNumber(selectedAsset.property.currentListingPriceWan)} 万`} · 资产确认于 {formatDateTime(selectedAsset.updatedAt)}</p>
              </div>
            ) : null}
            </StepSection>

            <StepSection className="col-span-12 bg-white md:col-span-6 xl:col-span-4" number="2" title="选择目标房源">
            <div className="grid grid-cols-2 gap-3">
              <SearchCombobox
                label="目标小区"
                placeholder="搜索小区"
                query={neighborhoodQuery}
                onQueryChange={setNeighborhoodQuery}
                items={neighborhoods}
                selectedItem={selectedNeighborhood}
                onSelect={chooseNeighborhood}
                getOptionId={(item) => item.id}
                getOptionLabel={(item) => item.name}
                renderOption={(item) => <><span className="block font-medium text-slate-900">{item.name}</span><span className="block text-xs text-slate-500">{item.area}</span></>}
                state={neighborhoodsState}
                loadingMessage="正在搜索小区"
                emptyMessage="没有匹配小区"
                failureMessage="小区读取失败"
                onRetry={() => setNeighborhoodsVersion((value) => value + 1)}
              />
              <SearchCombobox
                label="目标房源"
                placeholder={selectedNeighborhood ? "筛选房源" : "先选小区"}
                query={listingQuery}
                onQueryChange={setListingQuery}
                items={filteredListings}
                selectedItem={selectedListing}
                onSelect={chooseListing}
                getOptionId={(item) => item.roomId}
                getOptionLabel={listingInputLabel}
                renderOption={(item) => <><span className="block font-medium text-slate-900">{item.layout} · {formatNumber(item.areaSqm)}㎡ · {formatNumber(item.listingTotalPriceWan)} 万</span><span className="block text-xs text-slate-500">{item.floorDescription || item.floorBand || "楼层未标注"} · {item.orientation || "朝向未标注"}</span></>}
                state={targetNeighborhoodID ? listingsState : "idle"}
                disabled={!targetNeighborhoodID}
                loadingMessage="正在读取当前房源"
                emptyMessage={listings.length ? "没有匹配房源" : "该小区暂无当前在售房源"}
                failureMessage="当前房源读取失败"
                onRetry={() => setListingsVersion((value) => value + 1)}
              />
            </div>
            {fieldErrors.targetNeighborhood ? <FieldError>{fieldErrors.targetNeighborhood}</FieldError> : null}
            {fieldErrors.targetListing ? <FieldError>{fieldErrors.targetListing}</FieldError> : null}
            {listingsState === "ready" && listings.length === 0 ? <StatusBand tone="amber">该小区暂无当前在售房源，请选择其他小区。</StatusBand> : null}
            {targetDetailState === "loading" ? <InlineLoading label="正在读取房源详情" /> : null}
            {targetDetailState === "failed" ? <RetryState label="目标房详情读取失败" onRetry={() => setTargetDetailVersion((value) => value + 1)} /> : null}
            {targetListing ? (
              <div className="mt-4 grid gap-4 border-l-4 border-emerald-500 bg-emerald-50 p-4 sm:grid-cols-2 md:grid-cols-1 lg:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                <NumberField label="预计成交价 (万)" value={targetPrice} error={fieldErrors.targetPrice} onChange={(value) => { setTargetPrice(value); setTargetPriceConfirmed(false); markDraftChanged(); }} />
                <div className="text-sm leading-6 text-slate-700"><p className="font-medium text-slate-900">挂牌 {formatNumber(targetListing.listingTotalPriceWan)} 万</p><p>{targetListing.layout} · {formatNumber(targetListing.areaSqm)}㎡ · {targetListing.orientation || "朝向未标注"}</p></div>
                <ConfirmPrice checked={targetPriceConfirmed} onChange={(checked) => { setTargetPriceConfirmed(checked); markDraftChanged(); }} label="确认采用该目标房成交价" />
                <p className={`text-xs ${targetListing.freshness === "current" ? "text-emerald-700" : "text-amber-800"}`}>采集于 {formatDateTime(targetListing.collectedAt)} · {freshnessLabel(targetListing.freshness)}</p>
              </div>
            ) : null}
            </StepSection>

            <StepSection className="col-span-12 bg-white xl:col-span-4" number="3" title="确认家庭资金">
            <div className="grid gap-4 sm:grid-cols-2">
              {familyFields.map((field) => <NumberField key={field.key} label={field.label} value={family[field.key]} error={fieldErrors[field.key]} onChange={(value) => updateFamily(field.key, value)} />)}
            </div>
            <div className="mt-5">
              <span className="mb-2 block text-xs font-medium text-slate-600">本次购房认定</span>
              <div className="inline-flex rounded-md border border-slate-300 p-1">
                <SegmentButton selected={homePurchaseOrder === "first"} onClick={() => { setHomePurchaseOrder("first"); markDraftChanged(); }}>首套</SegmentButton>
                <SegmentButton selected={homePurchaseOrder === "second"} onClick={() => { setHomePurchaseOrder("second"); markDraftChanged(); }}>二套</SegmentButton>
              </div>
            </div>

            <details className="mt-5 border-t border-slate-200 pt-4">
              <summary className="cursor-pointer text-sm font-semibold text-slate-700">贷款与税费设置</summary>
              <div className="mt-4 grid gap-4 sm:grid-cols-2">
                <SelectField label="贷款类型" value={loanType} onChange={(value) => { setLoanType(value as LoanPlan["type"]); markDraftChanged(); }} options={[{ value: "commercial", label: "商业贷款" }, { value: "provident_fund", label: "公积金贷款" }, { value: "combined", label: "组合贷款" }]} />
                <SelectField label="贷款期限" value={String(loanTermMonths)} onChange={(value) => { setLoanTermMonths(Number(value)); markDraftChanged(); }} options={[120, 180, 240, 300, 360].map((value) => ({ value: String(value), label: `${value / 12} 年` }))} />
                <SelectField label="还款方式" value={repaymentMethod} onChange={(value) => { setRepaymentMethod(value as LoanPlan["repaymentMethod"]); markDraftChanged(); }} options={[{ value: "equal_installment", label: "等额本息" }, { value: "equal_principal", label: "等额本金" }]} />
                <SelectField label="税费承担" value={taxBurdenMode} onChange={(value) => { setTaxBurdenMode(value as "statutory" | "buyer_all"); markDraftChanged(); }} options={[{ value: "statutory", label: "依法各自承担" }, { value: "buyer_all", label: "买方承担全部" }]} />
              </div>
              {loanType === "combined" ? (
                <div className="mt-4 grid gap-4 sm:grid-cols-2">
                  <NumberField label="贷款总额 (万)" value={combinedTotal} error={fieldErrors.combinedTotal} onChange={(value) => { setCombinedTotal(value); markDraftChanged(); }} />
                  <NumberField label="商贷金额 (万)" value={commercialAmount} error={fieldErrors.commercialAmount} onChange={(value) => { setCommercialAmount(value); markDraftChanged(); }} />
                  <NumberField label="公积金金额 (万)" value={providentAmount} error={fieldErrors.providentAmount} onChange={(value) => { setProvidentAmount(value); markDraftChanged(); }} />
                </div>
              ) : null}
              <PolicyStatus assumptions={assumptions} option={selectedOption} state={assumptionsState} onRetry={() => setAssumptionsVersion((value) => value + 1)} />
              <ManualOverridesEditor manual={manual} error={fieldErrors.manual} onChange={(value) => { setManual(value); markDraftChanged(); }} />
            </details>
            </StepSection>
          </div>

          <div className="bg-white p-5 sm:p-6">
            <button type="submit" disabled={assumptionsState !== "ready" || isSubmitting} className="mx-auto flex h-12 w-full max-w-xl items-center justify-center gap-2 rounded-md bg-blue-600 px-4 font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-300">
              {isSubmitting ? <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" /> : <Calculator aria-hidden="true" className="h-4 w-4" />}
              {isSubmitting ? "生成中" : "生成诊断报告"}
            </button>
            {apiError ? <p role="alert" className="mt-3 border-l-4 border-rose-400 bg-rose-50 px-3 py-2 text-sm text-rose-800">{apiError}</p> : null}
          </div>
        </form>
      </section>

      <section className="min-h-[32rem] border border-slate-200 bg-white shadow-sm">
        <div className="grid gap-5 border-b border-slate-200 p-5 sm:p-6 md:grid-cols-[minmax(0,1fr)_minmax(18rem,28rem)] md:items-end">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <History aria-hidden="true" className="h-5 w-5 text-blue-700" />
              <h2 className="text-xl font-bold text-slate-800">换房压力诊断报告</h2>
            </div>
            <p className="mt-2 break-words text-sm text-slate-500">
              {report ? `生成于 ${formatDateTime(report.createdAt)} · ${reportSnapshotLabel}` : historyTotal ? `共 ${historyTotal} 份历史报告` : "选择历史报告或生成新报告"}
            </p>
          </div>
          <div className="min-w-0">
            <SearchCombobox
              label="诊断历史"
              placeholder={accessState === "locked" ? "个人空间未解锁" : "按房源或日期搜索"}
              query={historyQuery}
              onQueryChange={setHistoryQuery}
              items={historyItems}
              selectedItem={selectedHistory}
              onSelect={(item) => {
                setSelectedHistory(item);
                if (item) loadHistoricalReport(item);
              }}
              getOptionId={(item) => item.id}
              getOptionLabel={historyInputLabel}
              renderOption={(item) => <HistoryOption item={item} />}
              state={historyState}
              disabled={accessState !== "unlocked"}
              loadingMessage="正在搜索诊断历史"
              emptyMessage={historyQuery ? "没有匹配报告" : "暂无诊断历史"}
              failureMessage="诊断历史读取失败"
              onRetry={() => setHistoryVersion((value) => value + 1)}
            />
            {accessState === "unlocked" && historyState === "ready" ? <p className="mt-1 text-right text-xs text-slate-500">{historyTotal} 份报告</p> : null}
          </div>
        </div>
        <div className="p-5 sm:p-8">
          {accessState === "checking" ? <InlineLoading label="正在确认个人空间" /> : null}
          {accessState === "locked" ? <div className="flex min-h-48 flex-col items-center justify-center border border-dashed border-slate-200 bg-slate-50 p-6 text-center"><LockKeyhole aria-hidden="true" className="mb-3 h-6 w-6 text-slate-500" /><p className="text-sm font-medium text-slate-700">个人空间尚未解锁</p><p className="mt-1 text-xs text-slate-500">解锁后可读取持久化诊断历史。</p></div> : null}
          {accessState === "unlocked" && reportState === "loading" ? <div className="flex min-h-48 items-center justify-center"><InlineLoading label="正在读取报告快照" /></div> : null}
          {accessState === "unlocked" && reportState === "missing" && selectedHistory ? <RetryState label="这条历史记录已失效" onRetry={() => loadHistoricalReport(selectedHistory)} /> : null}
          {accessState === "unlocked" && reportState === "failed" && selectedHistory ? <RetryState label="报告快照读取失败" onRetry={() => loadHistoricalReport(selectedHistory)} /> : null}
          {accessState === "unlocked" && reportState === "ready" && report ? <CalculationReport report={report} snapshotLabel={reportSnapshotLabel} /> : null}
          {accessState === "unlocked" && reportState === "idle" && historyState === "failed" ? <RetryState label="诊断历史读取失败" onRetry={() => setHistoryVersion((value) => value + 1)} /> : null}
          {accessState === "unlocked" && reportState === "idle" && (historyState === "idle" || historyState === "loading") ? <div className="flex min-h-48 items-center justify-center"><InlineLoading label="正在读取最新诊断" /></div> : null}
          {accessState === "unlocked" && reportState === "idle" && historyState === "ready" ? <p className="border border-dashed border-slate-200 bg-slate-50 p-6 text-center text-sm text-slate-500">暂无诊断历史。完成房屋选择、价格确认与家庭资金后生成第一份报告。</p> : null}
        </div>
      </section>
    </div>
  );
}

function StepSection({ children, className, number, title }: { children: React.ReactNode; className?: string; number: string; title: string }) {
  return <fieldset className={`min-w-0 p-5 sm:p-6 ${className ?? ""}`}><legend className="mb-5 flex items-center gap-3 text-base font-semibold text-slate-900"><span className="inline-flex h-7 w-7 flex-none items-center justify-center rounded-full bg-slate-900 text-xs text-white">{number}</span>{title}</legend>{children}</fieldset>;
}

function ConfirmPrice({ checked, label, onChange }: { checked: boolean; label: string; onChange: (checked: boolean) => void }) {
  return <label className="flex items-center gap-2 text-sm font-medium text-slate-800"><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} className="h-4 w-4 accent-blue-600" />{label}</label>;
}

function SegmentButton({ children, onClick, selected }: { children: React.ReactNode; onClick: () => void; selected: boolean }) {
  return <button type="button" aria-pressed={selected} onClick={onClick} className="h-8 rounded px-4 text-sm font-medium text-slate-600 aria-pressed:bg-slate-900 aria-pressed:text-white">{children}</button>;
}

function InlineLoading({ label }: { label: string }) { return <p role="status" className="mt-3 flex items-center gap-2 text-xs text-slate-500"><LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" />{label}</p>; }
function RetryState({ label, onRetry }: { label: string; onRetry: () => void }) { return <div role="alert" className="mt-3 flex items-center justify-between gap-3 border-l-4 border-rose-400 bg-rose-50 px-3 py-2 text-sm text-rose-800"><span>{label}</span><button type="button" onClick={onRetry} className="inline-flex items-center gap-1 font-medium"><RefreshCw aria-hidden="true" className="h-4 w-4" />重试</button></div>; }
function StatusBand({ children, tone }: { children: React.ReactNode; tone: "amber" | "slate" }) { return <div className={`mt-3 border-l-4 px-3 py-2 text-sm ${tone === "amber" ? "border-amber-400 bg-amber-50 text-amber-950" : "border-slate-400 bg-slate-50 text-slate-700"}`}>{children}</div>; }

function NumberField({ error, label, onChange, value }: { error?: string; label: string; onChange: (value: string) => void; value: string }) {
  return <label className="block"><span className="mb-1 block text-xs font-medium text-slate-600">{label}</span><input aria-label={label} aria-invalid={Boolean(error)} type="text" inputMode="decimal" value={value} onChange={(event) => onChange(event.target.value)} className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-900 outline-none focus:border-blue-500 aria-[invalid=true]:border-rose-500" />{error ? <FieldError>{error}</FieldError> : null}</label>;
}

function FieldError({ children }: { children: React.ReactNode }) { return <span className="mt-1 block text-xs text-rose-600">{children}</span>; }

function SelectField({ label, onChange, options, value }: { label: string; onChange: (value: string) => void; options: Array<{ value: string; label: string }>; value: string }) {
  return <label className="block"><span className="mb-1 block text-xs font-medium text-slate-600">{label}</span><select aria-label={label} value={value} onChange={(event) => onChange(event.target.value)} className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-900 outline-none focus:border-blue-500">{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>;
}

function PolicyStatus({ assumptions, onRetry, option, state }: { assumptions?: CapacityAssumptionsResponse; option?: CapacityAssumptionsResponse["loanOptions"][number]; state: "loading" | "ready" | "failed"; onRetry: () => void }) {
  if (state === "loading") return <InlineLoading label="正在读取测算政策" />;
  if (state === "failed" || !assumptions || !option || !assumptions.policyVersion) return <RetryState label="当前测算政策加载失败" onRetry={onRetry} />;
  const rates = [option.commercialAnnualInterestRate, option.providentAnnualInterestRate].filter((value): value is number => value !== undefined).map(formatPercent).join(" / ");
  return <div className="mt-4 border-l-4 border-emerald-500 bg-emerald-50 px-3 py-2 text-xs leading-5 text-slate-700"><p className="font-medium text-slate-900">首付 {formatPercent(option.downPaymentRate)} · 参考利率 {rates}</p><p>{assumptions.policyVersion.version} · {assumptions.policyVersion.effectiveFrom} 生效</p></div>;
}

function ManualOverridesEditor({ error, manual, onChange }: { error?: string; manual: ManualForm; onChange: (value: ManualForm) => void }) {
  const set = (key: keyof ManualForm, value: string) => onChange({ ...manual, [key]: value });
  return <details className="mt-4 border-t border-slate-200 pt-4"><summary className="cursor-pointer text-sm font-semibold text-slate-700">调整政策默认值</summary><div className="mt-4 grid gap-4 sm:grid-cols-2"><OptionalNumber label="商贷年利率 (%)" value={manual.commercialRate} onChange={(value) => set("commercialRate", value)} /><OptionalNumber label="公积金年利率 (%)" value={manual.providentRate} onChange={(value) => set("providentRate", value)} /><OptionalNumber label="首付比例 (%)" value={manual.downPaymentRate} onChange={(value) => set("downPaymentRate", value)} /><OptionalNumber label="契税 (万)" value={manual.deedTax} onChange={(value) => set("deedTax", value)} /><OptionalNumber label="增值税 (万)" value={manual.vat} onChange={(value) => set("vat", value)} /><OptionalNumber label="增值税附加 (万)" value={manual.surcharges} onChange={(value) => set("surcharges", value)} /><OptionalNumber label="个人所得税 (万)" value={manual.incomeTax} onChange={(value) => set("incomeTax", value)} /></div>{error ? <FieldError>{error}</FieldError> : null}</details>;
}

function OptionalNumber({ label, onChange, value }: { label: string; onChange: (value: string) => void; value: string }) { return <label className="block"><span className="mb-1 block text-xs font-medium text-slate-600">{label}</span><input aria-label={label} type="text" inputMode="decimal" placeholder="自动" value={value} onChange={(event) => onChange(event.target.value)} className="h-10 w-full rounded-md border border-slate-300 px-3 text-sm outline-none focus:border-blue-500" /></label>; }

function HistoryOption({ item }: { item: CalculationHistorySummary }) {
  const pressure = pressureCopy[item.pressureLevel];
  const target = [item.targetNeighborhoodName, item.targetLayout].filter(Boolean).join(" · ") || "旧版记录（无房源快照）";
  return <span className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-3"><span className="min-w-0"><span className="block truncate font-medium text-slate-900">{target}</span><span className="mt-0.5 block text-xs text-slate-500">{formatDateTime(item.createdAt)} · {formatNumber(item.targetTotalPrice)} 万</span></span><span className={`self-center text-xs font-semibold ${pressure.tone}`}>{pressure.label}</span></span>;
}

function CalculationReport({ report, snapshotLabel }: { report: CalculationResponse; snapshotLabel: "刚刚生成" | "历史快照" }) {
  const { result } = report;
  const applied = result.appliedAssumptions;
  const pressure = pressureCopy[result.pressureLevel];
  const hasOldHome = report.selectionContext?.oldHome?.mode === "asset";
  return <div className="space-y-8">
    <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-slate-500"><span>报告 ID：{report.id}</span><time dateTime={report.createdAt}>生成时间：{formatDateTime(report.createdAt)}</time><span>快照状态：{snapshotLabel}</span><span>追溯状态：{result.traceabilityStatus}</span></div>
    {report.selectionContext ? <SelectionSnapshotReport context={report.selectionContext} /> : null}
    <div>{applied ? <PressureScale ratio={result.monthlyPaymentRatio} thresholds={applied.pressureThresholds} pressure={pressure} targetTotalPrice={report.input.targetTotalPrice} /> : <p className="border-l-4 border-amber-400 bg-amber-50 p-4 text-sm text-amber-900">该历史记录未保存版本化假设，无法复现压力阈值。</p>}<MonthlyPaymentMethodLink /></div>
    <div className="grid grid-cols-2 border-y border-slate-200 sm:grid-cols-3"><ResultMetric label="旧房净回款" value={formatNumber(result.netOldHomeProceeds)} suffix="万" /><ResultMetric label="可动用现金" value={formatNumber(result.deployableCash)} suffix="万" /><ResultMetric label="安全总价上限" value={formatNumber(result.safeTotalPrice)} suffix="万" tone="text-emerald-700" /><ResultMetric label="首付资金缺口" value={formatNumber(result.downPaymentGap)} suffix="万" tone="text-amber-700" /><ResultMetric label="新房月供" value={formatNumber(result.monthlyPayment * 10000)} suffix="元" /><ResultMetric label="月供收入比" value={formatNumber(result.monthlyPaymentRatio * 100)} suffix="%" /></div>
    <div className="border-l-4 border-blue-500 bg-blue-50 p-5"><h3 className="mb-2 flex items-center font-semibold text-blue-950"><CheckCircle aria-hidden="true" className="h-5 w-5 text-emerald-600" /><span className="ml-2">操作策略建议</span></h3><p className="mb-3 text-sm font-semibold text-slate-900">{result.strategy}</p><ul className="space-y-2 text-sm leading-relaxed text-slate-700">{result.reasons.map((reason) => <li key={reason} className="border-l border-blue-200 pl-3">{reason}</li>)}</ul>{hasOldHome ? <Link href="/methods/old-home-sale-delay" className="mt-4 inline-flex items-center gap-1 text-sm font-medium text-blue-800 hover:underline">旧房迟迟卖不掉怎么办？<ExternalLink aria-hidden="true" className="h-3.5 w-3.5" /></Link> : null}</div>
    {result.loanBreakdown ? <LoanReport breakdown={result.loanBreakdown} /> : null}
    {result.taxBreakdown ? <TaxReport breakdown={result.taxBreakdown} /> : null}
    {result.manualOverrides?.length ? <OverridesReport overrides={result.manualOverrides} /> : null}
    <ReportInputs input={report.input} />
    {result.policyVersion && result.sources ? <PolicySources policy={result.policyVersion} sources={result.sources} /> : null}
    {applied ? <ReportAssumptions assumptions={applied} /> : null}
    {result.disclaimer ? <p className="border-t border-slate-200 pt-4 text-xs leading-5 text-slate-500">{result.disclaimer}</p> : null}
  </div>;
}

function SelectionSnapshotReport({ context }: { context: NonNullable<CalculationResponse["selectionContext"]> }) {
  const oldHome = context.oldHome;
  const target = context.targetHome;
  return <section className="border-y border-slate-200 py-5"><h3 className="text-sm font-semibold text-slate-900">房屋与价格快照</h3><div className="mt-4 grid gap-5 sm:grid-cols-2">
    <div><p className="text-xs font-medium text-slate-500">旧房资产</p>{oldHome?.mode === "asset" && oldHome.property ? <><p className="mt-1 font-semibold text-slate-900">{oldHome.assetName}</p><p className="text-sm text-slate-600">{oldHome.property.layout} · {formatNumber(oldHome.property.areaSqm)}㎡</p><SnapshotPrice reference={oldHome.property.referenceListingPriceWan} confirmed={oldHome.confirmedSalePriceWan} difference={oldHome.priceDifferenceWan} confirmedLabel="确认售价" /><p className="mt-2 text-xs text-slate-500">资产确认 {oldHome.assetUpdatedAt ? formatDateTime(oldHome.assetUpdatedAt) : "未记录"} · 本次确认 {formatDateTime(oldHome.confirmedAt)}</p></> : <p className="mt-1 font-semibold text-slate-900">无旧房</p>}</div>
    <div><p className="text-xs font-medium text-slate-500">目标房源</p>{target ? <><p className="mt-1 font-semibold text-slate-900">{target.property.neighborhoodName} · {target.property.layout}</p><p className="text-sm text-slate-600">{formatNumber(target.property.areaSqm)}㎡ · {target.property.floorDescription || target.property.floorBand || "楼层未标注"} · {target.property.orientation || "朝向未标注"}</p><SnapshotPrice reference={target.property.referenceListingPriceWan} confirmed={target.confirmedPurchasePriceWan} difference={target.priceDifferenceWan} confirmedLabel="确认成交" /><p className="mt-2 text-xs text-slate-500">采集 {formatDateTime(target.marketReference.collectedAt)} · 本次确认 {formatDateTime(target.confirmedAt)}</p></> : <p className="mt-1 text-sm text-slate-500">历史记录未保存目标房快照</p>}</div>
  </div></section>;
}

function SnapshotPrice({ confirmed, confirmedLabel, difference, reference }: { confirmed: number; confirmedLabel: string; difference: number | null; reference: number | null }) { return <dl className="mt-3 space-y-1 text-sm"><div className="flex justify-between gap-3"><dt className="text-slate-500">参考挂牌</dt><dd className="font-medium">{reference == null ? "未记录" : `${formatNumber(reference)} 万`}</dd></div><div className="flex justify-between gap-3"><dt className="text-slate-500">{confirmedLabel}</dt><dd className="font-semibold text-blue-800">{formatNumber(confirmed)} 万</dd></div><div className="flex justify-between gap-3"><dt className="text-slate-500">价差</dt><dd className="font-medium">{difference == null ? "无法计算" : `${difference > 0 ? "+" : ""}${formatNumber(difference)} 万`}</dd></div></dl>; }

function LoanReport({ breakdown }: { breakdown: NonNullable<CalculationResponse["result"]["loanBreakdown"]> }) { return <section className="border-t border-slate-200 pt-5"><h3 className="text-sm font-semibold text-slate-900">贷款明细</h3><dl className="mt-3 divide-y divide-slate-100 text-sm">{breakdown.components.map((component) => <div key={component.type} className="grid grid-cols-[1fr_auto] gap-3 py-2"><dt className="text-slate-600">{component.type === "commercial" ? "商业贷款" : "公积金贷款"} · {formatNumber(component.principal)} 万 · {formatPercent(component.annualInterestRate)}{component.manualOverride ? " · 已调整" : ""}</dt><dd className="font-semibold text-slate-900">{formatNumber(component.monthlyPayment * 10000)} 元/月</dd></div>)}</dl><p className="mt-3 text-right text-sm font-semibold text-slate-900">合计 {formatNumber(breakdown.monthlyPayment * 10000)} 元/月</p></section>; }
function TaxReport({ breakdown }: { breakdown: NonNullable<CalculationResponse["result"]["taxBreakdown"]> }) { return <section className="border-t border-slate-200 pt-5"><h3 className="text-sm font-semibold text-slate-900">交易税费估算</h3><div className="mt-3 overflow-x-auto"><table className="w-full min-w-[560px] text-left text-sm"><thead className="border-b border-slate-200 text-xs text-slate-500"><tr><th className="py-2">税项</th><th>承担方</th><th>计算依据</th><th className="text-right">估算</th></tr></thead><tbody className="divide-y divide-slate-100">{breakdown.items.map((item) => <tr key={item.code}><td className="py-2 font-medium text-slate-900">{item.name}{item.manualOverride ? <span className="ml-2 text-xs text-amber-700">已调整</span> : null}</td><td>{item.paidBy === "buyer" ? "买方" : "卖方"}</td><td className="text-xs text-slate-500">{item.exempt ? "免征" : item.formula}</td><td className="text-right font-semibold">{formatNumber(item.amount)} 万</td></tr>)}</tbody></table></div><p className="mt-3 text-right text-sm font-semibold text-slate-900">合计 {formatNumber(breakdown.total)} 万</p></section>; }
function OverridesReport({ overrides }: { overrides: NonNullable<CalculationResponse["result"]["manualOverrides"]> }) { return <section className="border-l-4 border-amber-400 bg-amber-50 p-4"><h3 className="text-sm font-semibold text-amber-950">用户调整项</h3><ul className="mt-2 space-y-1 text-xs text-amber-900">{overrides.map((item) => <li key={item.field}>{overrideLabel(item.field)}：自动 {formatNumber(item.automaticValue)}，采用 {formatNumber(item.appliedValue)}</li>)}</ul></section>; }
function PolicySources({ policy, sources }: { policy: NonNullable<CalculationResponse["result"]["policyVersion"]>; sources: NonNullable<CalculationResponse["result"]["sources"]> }) { return <section className="border-t border-slate-200 pt-5"><div className="flex items-center gap-2"><ShieldCheck aria-hidden="true" className="h-5 w-5 text-emerald-700" /><h3 className="text-sm font-semibold text-slate-900">政策版本 {policy.version}</h3></div><p className="mt-1 text-xs text-slate-500">{policy.name} · {policy.effectiveFrom} 生效</p><ul className="mt-3 divide-y divide-slate-100">{sources.map((source) => <li key={source.code} className="py-2"><a href={source.url} target="_blank" rel="noreferrer" className="inline-flex items-start gap-1 text-sm font-medium text-blue-700 hover:underline">{source.title}<ExternalLink aria-hidden="true" className="mt-0.5 h-3.5 w-3.5 flex-none" /></a><p className="mt-0.5 text-xs text-slate-500">{source.issuer} · {source.effectiveDate} 生效</p></li>)}</ul></section>; }

function PressureScale({ pressure, ratio, targetTotalPrice, thresholds }: { pressure: { label: string; tone: string }; ratio: number; targetTotalPrice: number; thresholds: NonNullable<CalculationResponse["result"]["appliedAssumptions"]>["pressureThresholds"] }) { const scaleMax = Math.max(thresholds.dangerRatio, thresholds.strainedRatio, ratio, 0.01) * 1.1; const safeWidth = (thresholds.safeRatio / scaleMax) * 100; const strainedWidth = ((thresholds.strainedRatio - thresholds.safeRatio) / scaleMax) * 100; const pointer = Math.min(Math.max((ratio / scaleMax) * 100, 0), 99.5); return <div><div className="mb-2 flex flex-wrap justify-between gap-2 text-xs font-medium"><span className="text-emerald-700">安全 ≤ {formatPercent(thresholds.safeRatio)}</span><span className="text-amber-700">偏高 ≤ {formatPercent(thresholds.strainedRatio)}</span><span className="text-rose-700">危险参考 {formatPercent(thresholds.dangerRatio)}</span></div><div className="relative flex h-3 w-full overflow-hidden bg-slate-100"><div className="h-full bg-emerald-500" style={{ width: `${safeWidth}%` }} /><div className="h-full bg-amber-400" style={{ width: `${strainedWidth}%` }} /><div className="h-full flex-1 bg-rose-500" /><div data-testid="pressure-pointer" className="absolute bottom-0 top-0 z-10 w-1 bg-slate-950 shadow-[0_0_0_2px_white]" style={{ left: `${pointer}%` }} /></div><p className="mt-2 text-center text-sm font-semibold text-slate-700">目标总价 {formatNumber(targetTotalPrice)} 万，月供收入比 {formatPercent(ratio)}，处于 <span className={pressure.tone}>{pressure.label}</span> 区间</p></div>; }
function MonthlyPaymentMethodLink() { return <p className="mt-2 text-center"><Link href="/methods/monthly-payment-safety" className="inline-flex items-center gap-1 text-xs font-medium text-blue-700 hover:underline">了解计算口径<span className="sr-only">：为什么月供安全线比总价更重要？</span><ExternalLink aria-hidden="true" className="h-3.5 w-3.5" /></Link></p>; }
function ReportInputs({ input }: { input: CalculationResponse["input"] }) { const values = [["当前可用现金", `${formatNumber(input.cashOnHand)} 万`], ["旧房预期售价", `${formatNumber(input.oldHomeValue)} 万`], ["家庭月收入", `${formatNumber(input.monthlyIncome)} 万`], ["目标成交价", `${formatNumber(input.targetTotalPrice)} 万`], ["装修预算", `${formatNumber(input.renovationBudget)} 万`], ["过渡成本", `${formatNumber(input.transitionRentCost)} 万`], ["贷款类型", loanTypeLabel(input.loanPlan?.type)], ["购房认定", input.transactionScenario?.homePurchaseOrder === "second" ? "二套" : "首套"]]; return <section><h3 className="mb-3 text-sm font-semibold text-slate-800">最终输入</h3><dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2">{values.map(([label, value]) => <div key={label} className="flex justify-between gap-4 border-b border-slate-100 py-1"><dt className="text-slate-500">{label}</dt><dd className="font-medium text-slate-900">{value}</dd></div>)}</dl></section>; }
function ReportAssumptions({ assumptions }: { assumptions: NonNullable<CalculationResponse["result"]["appliedAssumptions"]> }) { return <section><h3 className="mb-3 text-sm font-semibold text-slate-800">应用假设</h3><dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2"><AssumptionRow label="规则版本" value={`${assumptions.ruleVersion}（${assumptions.effectiveDate} 生效）`} /><AssumptionRow label="规则来源" value={assumptions.ruleSource} /><AssumptionRow label="贷款参数" value={`${formatPercent(assumptions.loan.annualInterestRate)} · ${assumptions.loan.loanTermMonths} 个月 · ${repaymentLabel(assumptions.loan.repaymentMethod)}`} /><AssumptionRow label="现金储备" value={`${formatNumber(assumptions.reserveMonths)} 个月家庭收入`} /></dl></section>; }
function AssumptionRow({ label, value }: { label: string; value: string }) { return <div className="border-b border-slate-100 py-1"><dt className="text-slate-500">{label}</dt><dd className="mt-0.5 break-words font-medium text-slate-900">{value}</dd></div>; }
function ResultMetric({ label, suffix, tone = "text-slate-900", value }: { label: string; suffix: string; tone?: string; value: string }) { return <div className="min-w-0 border-b border-r border-slate-100 px-3 py-4"><p className="mb-1 text-xs text-slate-500">{label}</p><p className={`break-words text-xl font-bold ${tone}`}>{value}<span className="ml-1 text-xs font-normal text-slate-500">{suffix}</span></p></div>; }
function buildCalculationInput(args: {
  assumptions: CapacityAssumptionsResponse;
  combinedTotal: string;
  commercialAmount: string;
  family: FamilyForm;
  homePurchaseOrder: "first" | "second";
  loanTermMonths: number;
  loanType: LoanPlan["type"];
  manual: ManualForm;
  oldChoice: "none" | string;
  oldHomeOnlyFamilyHome: boolean;
  oldLoanBalance: string;
  oldPriceConfirmed: boolean;
  oldSalePrice: string;
  providentAmount: string;
  repaymentMethod: LoanPlan["repaymentMethod"];
  selectedAsset?: Asset;
  targetListing?: MarketListingDetail;
  targetPrice: string;
  targetPriceConfirmed: boolean;
  taxBurdenMode: "statutory" | "buyer_all";
}): { input?: HousingCapacityInput; errors: FieldErrors } {
  const errors: FieldErrors = {};
  const parsedFamily = {} as Record<FamilyKey, number>;
  for (const field of familyFields) {
    const parsed = parseRequiredNonNegative(args.family[field.key], field.key, errors);
    if (field.key === "monthlyIncome" && parsed <= 0) errors[field.key] = "必须大于 0";
    parsedFamily[field.key] = parsed * (field.scale ?? 1);
  }
  let oldHomeValue = 0;
  let oldLoanBalance = 0;
  if (args.oldChoice !== "none") {
    if (!args.selectedAsset) errors.oldChoice = "所选资产已失效";
    oldHomeValue = parseRequiredPositive(args.oldSalePrice, "oldSalePrice", errors);
    oldLoanBalance = parseRequiredNonNegative(args.oldLoanBalance, "oldLoanBalance", errors);
    if (!args.oldPriceConfirmed) errors.oldSalePrice = "请确认旧房预期售价";
  }
  if (!args.targetListing) errors.targetListing = "请选择当前在售目标房";
  const targetTotalPrice = parseRequiredPositive(args.targetPrice, "targetPrice", errors);
  if (!args.targetPriceConfirmed) errors.targetPrice = "请确认目标房成交价";
  const option = args.assumptions.loanOptions.find((item) => item.type === args.loanType);
  if (!option) errors.manual = "当前政策没有该贷款方案";
  const manualOverrides = parseManualOverrides(args.manual, errors);
  const downRate = manualOverrides?.downPaymentRate ?? option?.downPaymentRate ?? 0;
  const maximumLoan = targetTotalPrice * (1 - downRate);
  let totalLoanAmount = maximumLoan;
  let commercialLoanAmount = 0;
  let providentFundLoanAmount = 0;
  if (args.loanType === "combined") {
    totalLoanAmount = parseRequiredPositive(args.combinedTotal, "combinedTotal", errors);
    commercialLoanAmount = parseRequiredPositive(args.commercialAmount, "commercialAmount", errors);
    providentFundLoanAmount = parseRequiredPositive(args.providentAmount, "providentAmount", errors);
    if (Number.isFinite(totalLoanAmount) && Math.abs(commercialLoanAmount + providentFundLoanAmount - totalLoanAmount) > 0.01) errors.combinedTotal = "贷款总额必须等于商贷与公积金金额之和";
    if (totalLoanAmount - maximumLoan > 0.01) errors.combinedTotal = `不得超过 ${formatNumber(maximumLoan)} 万`;
  }
  if (Object.values(errors).some(Boolean) || !args.targetListing) return { errors };

  const loanPlan: LoanPlan = { type: args.loanType, totalLoanAmount: round(totalLoanAmount, 2), loanTermMonths: args.loanTermMonths, repaymentMethod: args.repaymentMethod };
  if (args.loanType === "combined") {
    loanPlan.commercialLoanAmount = commercialLoanAmount;
    loanPlan.providentFundLoanAmount = providentFundLoanAmount;
  }
  const selectedAsset = args.selectedAsset;
  const holdingYears = selectedAsset ? completedYears(selectedAsset.purchasedOn, new Date()) : 0;
  return {
    errors,
    input: {
      cashOnHand: parsedFamily.cashOnHand,
      oldHomeValue,
      oldLoanBalance,
      monthlyIncome: parsedFamily.monthlyIncome,
      currentMonthlyMortgage: parsedFamily.currentMonthlyMortgage,
      acceptableMonthlyMortgage: parsedFamily.acceptableMonthlyMortgage,
      targetTotalPrice,
      renovationBudget: parsedFamily.renovationBudget,
      transitionRentCost: parsedFamily.transitionRentCost,
      transactionScenario: {
        city: args.targetListing.city,
        homePurchaseOrder: args.homePurchaseOrder,
        targetHomeType: "resale",
        targetHomeAreaSqm: args.targetListing.areaSqm,
        oldHomeHoldingYears: holdingYears,
        oldHomeOnlyFamilyHome: selectedAsset ? args.oldHomeOnlyFamilyHome : false,
        oldHomeOriginalPrice: selectedAsset?.originalPurchasePriceWan ?? 0,
        taxBurdenMode: args.taxBurdenMode,
      },
      loanPlan,
      oldHomeSelection: selectedAsset ? {
        mode: "asset", assetId: selectedAsset.id, expectedSalePriceWan: oldHomeValue, priceConfirmed: true,
      } : { mode: "none", priceConfirmed: true },
      targetHomeSelection: {
        neighborhoodId: args.targetListing.neighborhoodId,
        roomId: args.targetListing.roomId,
        expectedPurchasePriceWan: targetTotalPrice,
        priceConfirmed: true,
      },
      ...(manualOverrides ? { manualOverrides } : {}),
    },
  };
}

function calculationSummaryFromReport(report: CalculationResponse): CalculationHistorySummary {
  const oldHome = report.selectionContext?.oldHome;
  const targetHome = report.selectionContext?.targetHome;
  return {
    id: report.id,
    createdAt: report.createdAt,
    pressureLevel: report.result.pressureLevel,
    targetTotalPrice: report.input.targetTotalPrice,
    targetNeighborhoodName: targetHome?.property.neighborhoodName ?? "",
    targetLayout: targetHome?.property.layout ?? "",
    oldHomeName: oldHome?.mode === "asset" ? oldHome.assetName : "",
  };
}

function historyInputLabel(item: CalculationHistorySummary) {
  const target = [item.targetNeighborhoodName, item.targetLayout].filter(Boolean).join(" · ") || "旧版记录";
  return `${formatDateTime(item.createdAt)} · ${target}`;
}

function listingInputLabel(item: MarketListing) {
  return `${item.layout} · ${formatNumber(item.areaSqm)}㎡ · ${formatNumber(item.listingTotalPriceWan)}万`;
}

function normalizeSearch(value: string) {
  return value.toLocaleLowerCase("zh-CN").replace(/\s+/g, "");
}

function parseRequiredNonNegative(raw: string, key: keyof FieldErrors, errors: FieldErrors) { const value = Number(raw); if (!raw.trim()) errors[key] = "请填写此项"; else if (!Number.isFinite(value) || value < 0) errors[key] = "请输入不小于 0 的数字"; return value; }
function parseRequiredPositive(raw: string, key: keyof FieldErrors, errors: FieldErrors) { const value = Number(raw); if (!raw.trim()) errors[key] = "请填写此项"; else if (!Number.isFinite(value) || value <= 0) errors[key] = "必须大于 0"; return value; }
function parseManualOverrides(form: ManualForm, errors: FieldErrors): CalculationOverrides | undefined { const override: CalculationOverrides = {}; let hasValue = false; for (const [key, raw] of [["commercialAnnualInterestRate", form.commercialRate], ["providentAnnualInterestRate", form.providentRate], ["downPaymentRate", form.downPaymentRate]] as const) { if (!raw.trim()) continue; const value = Number(raw); if (!Number.isFinite(value) || value <= 0 || value >= 100) errors.manual = "利率和首付比例须大于 0% 且小于 100%"; else { override[key] = value / 100; hasValue = true; } } const taxAmounts: Record<string, number> = {}; for (const [code, raw] of [["deed_tax", form.deedTax], ["value_added_tax", form.vat], ["vat_surcharges", form.surcharges], ["individual_income_tax", form.incomeTax]] as const) { if (!raw.trim()) continue; const value = Number(raw); if (!Number.isFinite(value) || value < 0) errors.manual = "税费覆盖值须为不小于 0 的数字"; else { taxAmounts[code] = value; hasValue = true; } } if (Object.keys(taxAmounts).length) override.taxAmounts = taxAmounts; return hasValue ? override : undefined; }
function completedYears(date: string, asOf: Date) { const [year, month, day] = date.split("-").map(Number); let years = asOf.getFullYear() - year; if (asOf.getMonth() + 1 < month || (asOf.getMonth() + 1 === month && asOf.getDate() < day)) years -= 1; return Math.max(years, 0); }
function freshnessLabel(value: MarketListingDetail["freshness"]) { return value === "current" ? "数据较新" : value === "stale" ? "数据已陈旧" : value === "expired" ? "数据已过期" : "新鲜度未知"; }
function overrideLabel(field: string) { return ({ downPaymentRate: "首付比例", commercialAnnualInterestRate: "商贷利率", providentAnnualInterestRate: "公积金利率", "taxAmounts.deed_tax": "契税", "taxAmounts.value_added_tax": "增值税", "taxAmounts.vat_surcharges": "增值税附加", "taxAmounts.individual_income_tax": "个人所得税" } as Record<string, string>)[field] ?? field; }
function loanTypeLabel(value?: LoanPlan["type"]) { return value === "provident_fund" ? "公积金贷款" : value === "combined" ? "组合贷款" : "商业贷款"; }
function repaymentLabel(value: "equal_installment" | "equal_principal") { return value === "equal_principal" ? "等额本金" : "等额本息"; }
function formatNumber(value: number) { return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value); }
function formatPercent(value: number) { return `${formatNumber(value * 100)}%`; }
function formatDateTime(value: string) { return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)); }
function round(value: number, digits: number) { const factor = 10 ** digits; return Math.round(value * factor) / factor; }
