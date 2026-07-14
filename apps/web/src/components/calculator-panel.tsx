"use client";

import Link from "next/link";
import { Calculator, CheckCircle, RefreshCw } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";

import {
  ApiError,
  createCapacityCalculation,
  getCapacityAssumptions,
  type CalculationResponse,
  type CapacityAssumptionsResponse,
  type CityPolicyOverride,
  type HousingCapacityInput,
  type LoanParams,
} from "@/lib/api-client";

type FamilyFieldKey = Exclude<
  keyof HousingCapacityInput,
  "loanOverride" | "cityPolicyOverride"
>;
type FamilyForm = Record<FamilyFieldKey, string>;
type FieldErrors = Partial<Record<FamilyFieldKey | "loanRate" | "loanTerm" | "loanMethod" | "policyCity" | "policyName" | "policyRate" | "policyDate" | "policySource", string>>;

const emptyFamilyForm: FamilyForm = {
  cashOnHand: "",
  oldHomeValue: "",
  oldLoanBalance: "",
  monthlyIncome: "",
  currentMonthlyMortgage: "",
  acceptableMonthlyMortgage: "",
  targetTotalPrice: "",
  renovationBudget: "",
  transactionCosts: "",
  transitionRentCost: "",
};

type LoanForm = {
  annualInterestRate: string;
  loanTermMonths: string;
  repaymentMethod: LoanParams["repaymentMethod"];
};

type CityPolicyForm = {
  city: string;
  policyName: string;
  downPaymentRate: string;
  effectiveDate: string;
  source: string;
};

type Field = {
  key: FamilyFieldKey;
  label: string;
  ariaLabel?: string;
  scale?: number;
};

const fields: Array<{ title?: string; items: Field[] }> = [
  {
    items: [
      { key: "cashOnHand", label: "当前可用现金 (万)" },
      { key: "monthlyIncome", label: "家庭月收入 (万)" },
      {
        key: "currentMonthlyMortgage",
        label: "当前月供 (元)",
        ariaLabel: "当前月供（元）",
        scale: 1 / 10000,
      },
    ],
  },
  {
    title: "旧房状况 (卖旧)",
    items: [
      { key: "oldHomeValue", label: "预期售出底价 (万)" },
      { key: "oldLoanBalance", label: "剩余贷款 (万)" },
    ],
  },
  {
    title: "目标与成本 (买新)",
    items: [
      {
        key: "targetTotalPrice",
        label: "目标总价预期 (万)",
        ariaLabel: "目标总价（万）",
      },
      {
        key: "acceptableMonthlyMortgage",
        label: "可接受极限月供 (元)",
        scale: 1 / 10000,
      },
      { key: "renovationBudget", label: "装修及杂费预算 (万)" },
      {
        key: "transactionCosts",
        label: "交易税费 (万)",
        ariaLabel: "交易税费（万）",
      },
      { key: "transitionRentCost", label: "过渡租房成本 (万)" },
    ],
  },
];

const pressureCopy: Record<
  CalculationResponse["result"]["pressureLevel"],
  { label: string; tone: string }
> = {
  safe: { label: "安全", tone: "text-emerald-700" },
  strained: { label: "偏高", tone: "text-amber-700" },
  danger: { label: "危险", tone: "text-rose-700" },
};

export function CalculatorPanel() {
  const [familyForm, setFamilyForm] = useState<FamilyForm>(emptyFamilyForm);
  const [loanForm, setLoanForm] = useState<LoanForm>();
  const [policyForm, setPolicyForm] = useState<CityPolicyForm>();
  const [assumptions, setAssumptions] = useState<CapacityAssumptionsResponse>();
  const [assumptionsStatus, setAssumptionsStatus] = useState<"loading" | "ready" | "error">("loading");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [report, setReport] = useState<CalculationResponse>();
  const [apiError, setApiError] = useState<string>();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const requestVersion = useRef(0);
  const submitController = useRef<AbortController | undefined>(undefined);

  useEffect(() => {
    const controller = new AbortController();
    setAssumptionsStatus("loading");
    setAssumptions(undefined);
    setLoanForm(undefined);
    setPolicyForm(undefined);

    getCapacityAssumptions(controller.signal)
      .then((next) => {
        setAssumptions(next);
        setLoanForm({
          annualInterestRate: String(next.loan.annualInterestRate * 100),
          loanTermMonths: String(next.loan.loanTermMonths),
          repaymentMethod: next.loan.repaymentMethod,
        });
        setPolicyForm({
          city: next.cityPolicy.city,
          policyName: next.cityPolicy.policyName,
          downPaymentRate: String(next.cityPolicy.downPaymentRate * 100),
          effectiveDate: next.cityPolicy.effectiveDate,
          source: next.cityPolicy.source,
        });
        setAssumptionsStatus("ready");
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setAssumptionsStatus("error");
        }
      });

    return () => controller.abort();
  }, [loadAttempt]);

  useEffect(
    () => () => {
      submitController.current?.abort();
    },
    [],
  );

  const invalidateReport = () => {
    requestVersion.current += 1;
    submitController.current?.abort();
    submitController.current = undefined;
    setIsSubmitting(false);
    setReport(undefined);
    setApiError(undefined);
  };

  const updateFamilyField = (key: FamilyFieldKey, value: string) => {
    setFamilyForm((current) => ({ ...current, [key]: value }));
    setFieldErrors((current) => ({ ...current, [key]: undefined }));
    invalidateReport();
  };

  const updateLoanForm = <K extends keyof LoanForm>(key: K, value: LoanForm[K]) => {
    setLoanForm((current) => (current ? { ...current, [key]: value } : current));
    setFieldErrors((current) => ({
      ...current,
      [key === "annualInterestRate" ? "loanRate" : key === "loanTermMonths" ? "loanTerm" : "loanMethod"]: undefined,
    }));
    invalidateReport();
  };

  const updatePolicyForm = <K extends keyof CityPolicyForm>(key: K, value: CityPolicyForm[K]) => {
    setPolicyForm((current) => (current ? { ...current, [key]: value } : current));
    const errorKey = {
      city: "policyCity",
      policyName: "policyName",
      downPaymentRate: "policyRate",
      effectiveDate: "policyDate",
      source: "policySource",
    }[key] as keyof FieldErrors;
    setFieldErrors((current) => ({ ...current, [errorKey]: undefined }));
    invalidateReport();
  };

  const regenerateReport = async (event: FormEvent) => {
    event.preventDefault();
    if (assumptionsStatus !== "ready" || !loanForm || !policyForm) {
      return;
    }

    const parsed = parseCalculationInput(familyForm, loanForm, policyForm);
    setFieldErrors(parsed.errors);
    setReport(undefined);
    setApiError(undefined);
    if (!parsed.input) {
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
        setReport(response);
      }
    } catch (error) {
      if (!controller.signal.aborted && requestVersion.current === version) {
        setApiError(
          error instanceof ApiError && error.status === 401
            ? "个人空间尚未解锁。"
            : "诊断报告生成失败，请稍后重试。",
        );
        setReport(undefined);
      }
    } finally {
      if (requestVersion.current === version) {
        setIsSubmitting(false);
        submitController.current = undefined;
      }
    }
  };

  return (
    <div className="grid grid-cols-1 gap-8 lg:grid-cols-12">
      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm lg:col-span-5">
        <h2 className="mb-6 flex items-center text-lg font-bold text-slate-800">
          <Calculator aria-hidden="true" className="h-5 w-5" />
          <span className="ml-2">我的换房条件</span>
        </h2>
        <form className="space-y-5" onSubmit={regenerateReport} noValidate>
          {fields.map((group, index) => (
            <fieldset key={group.title ?? "base"} className={index === 0 ? undefined : "border-t border-slate-100 pt-4"}>
              {group.title ? <legend className="mb-3 text-sm font-semibold text-slate-700">{group.title}</legend> : null}
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                {group.items.map((field) => {
                  const id = `calculator-${field.key}`;
                  const error = fieldErrors[field.key];
                  return (
                    <label key={field.key} htmlFor={id} className="block">
                      <span className="mb-1 block text-xs font-medium text-slate-600">{field.label}</span>
                      <input
                        id={id}
                        aria-label={field.ariaLabel}
                        aria-invalid={Boolean(error)}
                        aria-describedby={error ? `${id}-error` : undefined}
                        type="text"
                        inputMode="decimal"
                        placeholder="请输入"
                        value={familyForm[field.key]}
                        onChange={(event) => updateFamilyField(field.key, event.target.value)}
                        className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 font-medium text-slate-900 outline-none transition-colors focus:border-blue-500 aria-[invalid=true]:border-rose-500"
                      />
                      {error ? <span id={`${id}-error`} className="mt-1 block text-xs text-rose-600">{error}</span> : null}
                    </label>
                  );
                })}
              </div>
            </fieldset>
          ))}

          <AssumptionsEditor
            assumptions={assumptions}
            status={assumptionsStatus}
            loanForm={loanForm}
            policyForm={policyForm}
            errors={fieldErrors}
            onLoanChange={updateLoanForm}
            onPolicyChange={updatePolicyForm}
            onRetry={() => {
              invalidateReport();
              setLoadAttempt((attempt) => attempt + 1);
            }}
          />

          <button
            type="submit"
            disabled={assumptionsStatus !== "ready" || isSubmitting}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-3 font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-300"
          >
            <Calculator aria-hidden="true" className="h-4 w-4" />
            {isSubmitting ? "生成中..." : "生成诊断报告"}
          </button>
          {apiError ? <p role="alert" className="text-sm font-medium text-rose-600">{apiError}</p> : null}
        </form>
      </section>

      <section className="space-y-6 lg:col-span-7">
        <div className="min-h-[32rem] rounded-lg border border-slate-200 bg-white p-6 shadow-sm sm:p-8">
          <div className="mb-6 flex flex-wrap items-end justify-between gap-2 border-b border-slate-100 pb-4">
            <h2 className="text-xl font-bold text-slate-800">换房压力诊断报告</h2>
            <p className="text-sm text-slate-500">{report ? `记录 ${report.id}` : "等待 API 测算"}</p>
          </div>
          {report ? (
            <CalculationReport report={report} />
          ) : (
            <p className="border border-dashed border-slate-200 bg-slate-50 p-6 text-center text-sm text-slate-500">
              填写全部家庭、贷款与城市政策参数并提交后，这里会显示已保存的诊断报告。
            </p>
          )}
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <MethodLink href="/methods" title="为什么月供安全线比总价更重要？" description="了解现金流断裂的风险" />
          <MethodLink href="/methods" title="旧房迟迟卖不掉怎么办？" description="学会测算“底线成交价”" />
        </div>
      </section>
    </div>
  );
}

function AssumptionsEditor({
  assumptions,
  errors,
  loanForm,
  onLoanChange,
  onPolicyChange,
  onRetry,
  policyForm,
  status,
}: {
  assumptions?: CapacityAssumptionsResponse;
  errors: FieldErrors;
  loanForm?: LoanForm;
  policyForm?: CityPolicyForm;
  status: "loading" | "ready" | "error";
  onLoanChange: <K extends keyof LoanForm>(key: K, value: LoanForm[K]) => void;
  onPolicyChange: <K extends keyof CityPolicyForm>(key: K, value: CityPolicyForm[K]) => void;
  onRetry: () => void;
}) {
  if (status === "loading") {
    return <p role="status" className="border-t border-slate-100 pt-4 text-sm text-slate-500">正在加载当前测算假设...</p>;
  }
  if (status === "error" || !assumptions || !loanForm || !policyForm) {
    return (
      <div role="alert" className="border-t border-slate-100 pt-4">
        <p className="text-sm font-medium text-rose-700">当前测算假设加载失败。</p>
        <button type="button" onClick={onRetry} className="mt-3 inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium text-slate-700 hover:border-blue-400">
          <RefreshCw aria-hidden="true" className="h-4 w-4" />
          重试加载
        </button>
      </div>
    );
  }

  return (
    <>
      <fieldset className="border-t border-slate-100 pt-4">
        <legend className="mb-3 text-sm font-semibold text-slate-700">贷款参数</legend>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField id="loan-rate" label="年利率 (%)" ariaLabel="年利率（%）" value={loanForm.annualInterestRate} error={errors.loanRate} onChange={(value) => onLoanChange("annualInterestRate", value)} />
          <TextField id="loan-term" label="贷款期限 (月)" ariaLabel="贷款期限（月）" value={loanForm.loanTermMonths} error={errors.loanTerm} inputMode="numeric" onChange={(value) => onLoanChange("loanTermMonths", value)} />
          <label htmlFor="loan-method" className="block sm:col-span-2">
            <span className="mb-1 block text-xs font-medium text-slate-600">还款方式</span>
            <select id="loan-method" aria-label="还款方式" value={loanForm.repaymentMethod} onChange={(event) => onLoanChange("repaymentMethod", event.target.value as LoanForm["repaymentMethod"])} className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 font-medium text-slate-900 outline-none focus:border-blue-500">
              <option value="equal_installment">等额本息</option>
              <option value="equal_principal">等额本金</option>
            </select>
          </label>
        </div>
        <p className="mt-2 text-xs text-slate-500">{assumptions.loanSource} · 规则 {assumptions.ruleVersion}（生效 {assumptions.effectiveDate}）</p>
      </fieldset>

      <fieldset className="border-t border-slate-100 pt-4">
        <legend className="mb-3 text-sm font-semibold text-slate-700">城市首付政策</legend>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField id="policy-city" label="城市" value={policyForm.city} error={errors.policyCity} inputMode="text" onChange={(value) => onPolicyChange("city", value)} />
          <TextField id="policy-name" label="政策名称" value={policyForm.policyName} error={errors.policyName} inputMode="text" onChange={(value) => onPolicyChange("policyName", value)} />
          <TextField id="policy-rate" label="首付比例 (%)" ariaLabel="首付比例（%）" value={policyForm.downPaymentRate} error={errors.policyRate} onChange={(value) => onPolicyChange("downPaymentRate", value)} />
          <label htmlFor="policy-date" className="block">
            <span className="mb-1 block text-xs font-medium text-slate-600">政策生效日期</span>
            <input id="policy-date" type="date" value={policyForm.effectiveDate} aria-invalid={Boolean(errors.policyDate)} onChange={(event) => onPolicyChange("effectiveDate", event.target.value)} className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 font-medium text-slate-900 outline-none focus:border-blue-500 aria-[invalid=true]:border-rose-500" />
            {errors.policyDate ? <span className="mt-1 block text-xs text-rose-600">{errors.policyDate}</span> : null}
          </label>
          <div className="sm:col-span-2">
            <TextField id="policy-source" label="政策来源" value={policyForm.source} error={errors.policySource} inputMode="text" onChange={(value) => onPolicyChange("source", value)} />
          </div>
        </div>
      </fieldset>
    </>
  );
}

function TextField({ ariaLabel, error, id, inputMode = "decimal", label, onChange, value }: {
  ariaLabel?: string;
  error?: string;
  id: string;
  inputMode?: "decimal" | "numeric" | "text";
  label: string;
  onChange: (value: string) => void;
  value: string;
}) {
  return (
    <label htmlFor={id} className="block">
      <span className="mb-1 block text-xs font-medium text-slate-600">{label}</span>
      <input id={id} aria-label={ariaLabel} aria-invalid={Boolean(error)} aria-describedby={error ? `${id}-error` : undefined} type="text" inputMode={inputMode} value={value} onChange={(event) => onChange(event.target.value)} className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 font-medium text-slate-900 outline-none focus:border-blue-500 aria-[invalid=true]:border-rose-500" />
      {error ? <span id={`${id}-error`} className="mt-1 block text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function CalculationReport({ report }: { report: CalculationResponse }) {
  const result = report.result;
  const applied = result.appliedAssumptions;
  const pressure = pressureCopy[result.pressureLevel];

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-slate-500">
        <span>报告 ID：{report.id}</span>
        <time dateTime={report.createdAt}>生成时间：{report.createdAt}</time>
        <span>追溯状态：{result.traceabilityStatus}</span>
      </div>

      {applied ? <PressureScale ratio={result.monthlyPaymentRatio} thresholds={applied.pressureThresholds} pressure={pressure} targetTotalPrice={report.input.targetTotalPrice} /> : (
        <p className="border-l-4 border-amber-400 bg-amber-50 p-4 text-sm text-amber-900">该历史记录未保存版本化假设，无法复现压力阈值。</p>
      )}

      <div className="grid grid-cols-2 border-y border-slate-200 sm:grid-cols-3">
        <ResultMetric label="旧房净回款" value={formatNumber(result.netOldHomeProceeds)} suffix="万" />
        <ResultMetric label="可动用现金" value={formatNumber(result.deployableCash)} suffix="万" />
        <ResultMetric label="安全总价上限" value={formatNumber(result.safeTotalPrice)} suffix="万" tone="text-emerald-700" />
        <ResultMetric label="偏高总价上限" value={formatNumber(result.strainedTotalPrice)} suffix="万" tone="text-amber-700" />
        <ResultMetric label="危险总价参考" value={formatNumber(result.dangerTotalPrice)} suffix="万" tone="text-rose-700" />
        <ResultMetric label="首付资金缺口" value={formatNumber(result.downPaymentGap)} suffix="万" tone="text-amber-700" />
        <ResultMetric label="新房月供" value={formatNumber(result.monthlyPayment * 10000)} suffix="元" />
        <ResultMetric label="月供收入比" value={formatNumber(result.monthlyPaymentRatio * 100)} suffix="%" />
        <ResultMetric label="旧房安全成交底价" value={formatNumber(result.minimumSafeOldHomeSalePrice)} suffix="万" />
      </div>

      <div className="border-l-4 border-blue-500 bg-blue-50 p-5">
        <h3 className="mb-2 flex items-center font-semibold text-blue-950"><CheckCircle aria-hidden="true" className="h-5 w-5 text-emerald-600" /><span className="ml-2">操作策略建议</span></h3>
        <p className="mb-3 text-sm font-semibold text-slate-900">{result.strategy}</p>
        <ul className="space-y-2 text-sm leading-relaxed text-slate-700">
          {result.reasons.map((reason) => <li key={reason} className="border-l border-blue-200 pl-3">{reason}</li>)}
        </ul>
      </div>

      <ReportInputs input={report.input} />
      {applied ? <ReportAssumptions assumptions={applied} /> : null}
    </div>
  );
}

function PressureScale({ pressure, ratio, targetTotalPrice, thresholds }: {
  pressure: { label: string; tone: string };
  ratio: number;
  targetTotalPrice: number;
  thresholds: NonNullable<CalculationResponse["result"]["appliedAssumptions"]>["pressureThresholds"];
}) {
  const scaleMax = Math.max(thresholds.dangerRatio, thresholds.strainedRatio, ratio, 0.01) * 1.1;
  const safeWidth = (thresholds.safeRatio / scaleMax) * 100;
  const strainedWidth = ((thresholds.strainedRatio - thresholds.safeRatio) / scaleMax) * 100;
  const dangerWidth = Math.max(100 - safeWidth - strainedWidth, 0);
  const pointer = Math.min(Math.max((ratio / scaleMax) * 100, 0), 99.5);

  return (
    <div>
      <div className="mb-2 flex flex-wrap justify-between gap-2 text-xs font-medium">
        <span className="text-emerald-700">安全 ≤ {formatPercent(thresholds.safeRatio)}</span>
        <span className="text-amber-700">偏高 ≤ {formatPercent(thresholds.strainedRatio)}</span>
        <span className="text-rose-700">危险参考 {formatPercent(thresholds.dangerRatio)}</span>
      </div>
      <div className="relative flex h-3 w-full overflow-hidden bg-slate-100">
        <div className="h-full bg-emerald-500" style={{ width: `${safeWidth}%` }} />
        <div className="h-full bg-amber-400" style={{ width: `${strainedWidth}%` }} />
        <div className="h-full bg-rose-500" style={{ width: `${dangerWidth}%` }} />
        <div data-testid="pressure-pointer" className="absolute bottom-0 top-0 z-10 w-1 bg-slate-950 shadow-[0_0_0_2px_white]" style={{ left: `${pointer}%` }} />
      </div>
      <p className="mt-2 text-center text-sm font-semibold text-slate-700">目标总价 {formatNumber(targetTotalPrice)} 万，月供收入比 {formatPercent(ratio)}，处于 <span className={pressure.tone}>{pressure.label}</span> 区间</p>
    </div>
  );
}

function ReportInputs({ input }: { input: CalculationResponse["input"] }) {
  const values = [
    ["当前可用现金", `${formatNumber(input.cashOnHand)} 万`],
    ["旧房预期售价", `${formatNumber(input.oldHomeValue)} 万`],
    ["旧房剩余贷款", `${formatNumber(input.oldLoanBalance)} 万`],
    ["家庭月收入", `${formatNumber(input.monthlyIncome)} 万`],
    ["当前月供", `${formatNumber(input.currentMonthlyMortgage * 10000)} 元`],
    ["可接受新月供", `${formatNumber(input.acceptableMonthlyMortgage * 10000)} 元`],
    ["目标总价", `${formatNumber(input.targetTotalPrice)} 万`],
    ["装修预算", `${formatNumber(input.renovationBudget)} 万`],
    ["交易税费", `${formatNumber(input.transactionCosts)} 万`],
    ["过渡租房成本", `${formatNumber(input.transitionRentCost)} 万`],
  ];
  return (
    <section>
      <h3 className="mb-3 text-sm font-semibold text-slate-800">最终输入</h3>
      <dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
        {values.map(([label, value]) => <div key={label} className="flex justify-between gap-4 border-b border-slate-100 py-1"><dt className="text-slate-500">{label}</dt><dd className="font-medium text-slate-900">{value}</dd></div>)}
      </dl>
    </section>
  );
}

function ReportAssumptions({ assumptions }: { assumptions: NonNullable<CalculationResponse["result"]["appliedAssumptions"]> }) {
  return (
    <section>
      <h3 className="mb-3 text-sm font-semibold text-slate-800">应用假设</h3>
      <dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
        <AssumptionRow label="规则版本" value={`${assumptions.ruleVersion}（${assumptions.effectiveDate} 生效）`} />
        <AssumptionRow label="规则来源" value={assumptions.ruleSource} />
        <AssumptionRow label="贷款参数" value={`${formatPercent(assumptions.loan.annualInterestRate)} · ${assumptions.loan.loanTermMonths} 个月 · ${repaymentLabel(assumptions.loan.repaymentMethod)}`} />
        <AssumptionRow label="贷款来源" value={`${assumptions.loanSource} · ${originLabel(assumptions.loanOrigin)}`} />
        <AssumptionRow label="城市政策" value={`${assumptions.cityPolicy.city} · ${assumptions.cityPolicy.policyName} · 首付 ${formatPercent(assumptions.cityPolicy.downPaymentRate)}`} />
        <AssumptionRow label="政策来源" value={`${assumptions.cityPolicy.source} · ${assumptions.cityPolicy.effectiveDate} 生效 · ${originLabel(assumptions.cityPolicy.origin)}`} />
        <AssumptionRow label="现金储备" value={`${formatNumber(assumptions.reserveMonths)} 个月家庭收入`} />
        <AssumptionRow label="压力阈值" value={`${formatPercent(assumptions.pressureThresholds.safeRatio)} / ${formatPercent(assumptions.pressureThresholds.strainedRatio)} / ${formatPercent(assumptions.pressureThresholds.dangerRatio)}`} />
        <AssumptionRow label="危险额度系数" value={formatNumber(assumptions.pressureThresholds.dangerMultiplier)} />
        <AssumptionRow label="旧房回款占比阈值" value={formatPercent(assumptions.oldHomeShareThreshold)} />
      </dl>
    </section>
  );
}

function AssumptionRow({ label, value }: { label: string; value: string }) {
  return <div className="border-b border-slate-100 py-1"><dt className="text-slate-500">{label}</dt><dd className="mt-0.5 break-words font-medium text-slate-900">{value}</dd></div>;
}

function ResultMetric({ label, suffix, tone = "text-slate-900", value }: { label: string; suffix: string; tone?: string; value: string }) {
  return <div className="min-w-0 border-b border-r border-slate-100 px-3 py-4"><p className="mb-1 text-xs text-slate-500">{label}</p><p className={`break-words text-xl font-bold ${tone}`}>{value}<span className="ml-1 text-xs font-normal text-slate-500">{suffix}</span></p></div>;
}

function MethodLink({ description, href, title }: { description: string; href: string; title: string }) {
  return <Link href={href} className="block rounded-lg border border-slate-200 bg-white p-4 transition-colors hover:border-blue-300 focus-visible:border-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"><p className="mb-1 text-sm font-medium text-slate-800">{title}</p><p className="text-xs text-slate-500">{description}</p></Link>;
}

function parseCalculationInput(familyForm: FamilyForm, loanForm: LoanForm, policyForm: CityPolicyForm): { input?: HousingCapacityInput; errors: FieldErrors } {
  const errors: FieldErrors = {};
  const family = {} as Record<FamilyFieldKey, number>;
  for (const group of fields) {
    for (const field of group.items) {
      const raw = familyForm[field.key].trim();
      const value = Number(raw);
      if (raw === "") {
        errors[field.key] = "请填写此项";
      } else if (!Number.isFinite(value) || value < 0) {
        errors[field.key] = "请输入不小于 0 的数字";
      } else if ((field.key === "monthlyIncome" || field.key === "targetTotalPrice") && value <= 0) {
        errors[field.key] = "必须大于 0";
      } else {
        family[field.key] = value * (field.scale ?? 1);
      }
    }
  }

  const annualRatePercent = Number(loanForm.annualInterestRate);
  const loanTerm = Number(loanForm.loanTermMonths);
  if (loanForm.annualInterestRate.trim() === "" || !Number.isFinite(annualRatePercent) || annualRatePercent < 0 || annualRatePercent > 100) {
    errors.loanRate = "年利率须在 0% 到 100% 之间";
  }
  if (loanForm.loanTermMonths.trim() === "" || !Number.isInteger(loanTerm) || loanTerm < 1 || loanTerm > 600) {
    errors.loanTerm = "期限须为 1 到 600 个月的整数";
  }
  if (loanForm.repaymentMethod !== "equal_installment" && loanForm.repaymentMethod !== "equal_principal") {
    errors.loanMethod = "请选择合法还款方式";
  }

  const policyRatePercent = Number(policyForm.downPaymentRate);
  if (!policyForm.city.trim()) errors.policyCity = "请填写城市";
  if (!policyForm.policyName.trim()) errors.policyName = "请填写政策名称";
  if (policyForm.downPaymentRate.trim() === "" || !Number.isFinite(policyRatePercent) || policyRatePercent <= 0 || policyRatePercent >= 100) errors.policyRate = "首付比例须大于 0% 且小于 100%";
  if (!isValidPastOrPresentISODate(policyForm.effectiveDate)) errors.policyDate = "请输入不晚于今天的有效日期";
  if (!policyForm.source.trim()) errors.policySource = "请填写政策来源";

  if (Object.values(errors).some(Boolean)) return { errors };

  const loanOverride: LoanParams = {
    annualInterestRate: annualRatePercent / 100,
    loanTermMonths: loanTerm,
    repaymentMethod: loanForm.repaymentMethod,
  };
  const cityPolicyOverride: CityPolicyOverride = {
    city: policyForm.city.trim(),
    policyName: policyForm.policyName.trim(),
    downPaymentRate: policyRatePercent / 100,
    effectiveDate: policyForm.effectiveDate,
    source: policyForm.source.trim(),
  };
  return { errors, input: { ...family, loanOverride, cityPolicyOverride } };
}

function isValidPastOrPresentISODate(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const date = new Date(`${value}T00:00:00Z`);
  return !Number.isNaN(date.valueOf()) && date.toISOString().slice(0, 10) === value && value <= localISODate(new Date());
}

function localISODate(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value);
}

function formatPercent(value: number) {
  return `${formatNumber(value * 100)}%`;
}

function repaymentLabel(value: LoanParams["repaymentMethod"]) {
  return value === "equal_principal" ? "等额本金" : "等额本息";
}

function originLabel(value: "configured_default" | "user_override") {
  return value === "user_override" ? "用户覆盖" : "配置默认";
}
