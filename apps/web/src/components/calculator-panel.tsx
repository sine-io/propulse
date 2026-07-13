"use client";

import { useMemo, useState } from "react";
import { Calculator, CheckCircle } from "lucide-react";

import { ApiError, createCapacityCalculation } from "@/lib/api-client";
import {
  calculateHousingCapacity,
  type HousingCapacityInput,
  type HousingCapacityResult,
  type PressureLevel,
} from "@/lib/decision";
import { defaultHousingInput } from "@/lib/sample-data";

const pressureCopy: Record<
  PressureLevel,
  { label: string; tone: "text-emerald-600" | "text-amber-600" | "text-rose-600" }
> = {
  safe: { label: "安全", tone: "text-emerald-600" },
  strained: { label: "偏高", tone: "text-amber-600" },
  danger: { label: "危险", tone: "text-rose-600" },
};

type Field = {
  key: keyof HousingCapacityInput;
  label: string;
  ariaLabel?: string;
  value: (input: HousingCapacityInput) => string | number;
  parse?: (value: string) => number;
};

const fields: Array<{ title?: string; items: Field[] }> = [
  {
    items: [
      {
        key: "cashOnHand",
        label: "当前可用现金 (万)",
        value: (input) => input.cashOnHand,
      },
      {
        key: "monthlyIncome",
        label: "家庭月收入 (万)",
        value: (input) => input.monthlyIncome,
      },
    ],
  },
  {
    title: "旧房状况 (卖旧)",
    items: [
      {
        key: "oldHomeValue",
        label: "预期售出底价 (万)",
        value: (input) => input.oldHomeValue,
      },
      {
        key: "oldLoanBalance",
        label: "剩余贷款 (万)",
        value: (input) => input.oldLoanBalance,
      },
    ],
  },
  {
    title: "目标与成本 (买新)",
    items: [
      {
        key: "targetTotalPrice",
        label: "目标总价预期 (万)",
        ariaLabel: "目标总价（万）",
        value: (input) => input.targetTotalPrice,
      },
      {
        key: "acceptableMonthlyMortgage",
        label: "可接受极限月供",
        value: (input) => Math.round(input.acceptableMonthlyMortgage * 10000),
        parse: (value) => Number(value) / 10000,
      },
      {
        key: "renovationBudget",
        label: "装修及杂费预算 (万)",
        value: (input) => input.renovationBudget,
      },
      {
        key: "transitionRentCost",
        label: "过渡租房成本 (万)",
        value: (input) => input.transitionRentCost,
      },
    ],
  },
];

export function CalculatorPanel() {
  const [input, setInput] = useState<HousingCapacityInput>(defaultHousingInput);
  const [apiResult, setApiResult] = useState<
    Pick<HousingCapacityResult, "pressureLevel" | "strategy"> | undefined
  >();
  const [apiError, setApiError] = useState<string | undefined>();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const localResult = useMemo(() => calculateHousingCapacity(input), [input]);
  const result = apiResult ? { ...localResult, ...apiResult } : localResult;
  const pressure = pressureCopy[result.pressureLevel];
  const isReferenceScenario = input.targetTotalPrice === 550;

  const updateValue = (field: Field, value: string) => {
    const next = Number(value);

    setInput((current) => ({
      ...current,
      [field.key]: Number.isFinite(next) ? (field.parse?.(value) ?? next) : 0,
    }));
    setApiResult(undefined);
    setApiError(undefined);
  };

  const regenerateReport = async () => {
    const controller = new AbortController();

    setIsSubmitting(true);
    setApiError(undefined);

    try {
      const response = await createCapacityCalculation(input, controller.signal);
      setApiResult(response.result);
	} catch (error) {
		setApiError(
			error instanceof ApiError && error.status === 401
				? "个人空间尚未解锁。"
				: "诊断报告暂时无法更新，请稍后重试。",
		);
      setApiResult(undefined);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="grid grid-cols-1 gap-8 lg:grid-cols-12">
      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm lg:col-span-5">
        <h2 className="mb-6 flex items-center text-lg font-bold text-slate-800">
          <Calculator aria-hidden="true" className="h-5 w-5" />
          <span className="ml-2">我的换房条件</span>
        </h2>
        <form className="space-y-4">
          {fields.map((group, index) => (
            <fieldset
              key={group.title ?? "base"}
              className={index === 0 ? undefined : "border-t border-slate-100 pt-4"}
            >
              {group.title ? (
                <legend className="mb-3 text-sm font-semibold text-slate-700">
                  {group.title}
                </legend>
              ) : null}
              <div className="grid grid-cols-2 gap-4">
                {group.items.map((field) => {
                  const id = `calculator-${field.key}`;

                  return (
                    <label key={field.key} htmlFor={id} className="block">
                      <span className="mb-1 block text-xs font-medium text-slate-500">
                        {field.label}
                      </span>
                      <input
                        id={id}
                        aria-label={field.ariaLabel}
                        type="text"
                        inputMode="decimal"
                        value={field.value(input)}
                        onChange={(event) => updateValue(field, event.target.value)}
                        className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 font-medium text-slate-900 outline-none transition-colors focus:border-blue-500"
                      />
                    </label>
                  );
                })}
              </div>
            </fieldset>
          ))}
          <button
            type="button"
            onClick={regenerateReport}
            className="mt-4 w-full rounded-lg bg-blue-600 py-3 font-medium text-white transition-colors hover:bg-blue-700"
          >
            {isSubmitting ? "生成中..." : "重新生成诊断报告"}
          </button>
          {apiError ? (
            <p className="text-sm font-medium text-rose-600">{apiError}</p>
          ) : null}
        </form>
      </section>

      <section className="space-y-6 lg:col-span-7">
        <div className="rounded-2xl border border-slate-200 bg-white p-8 shadow-lg shadow-blue-900/5">
          <div className="mb-6 flex items-end justify-between">
            <h2 className="text-xl font-bold text-slate-800">换房压力诊断报告</h2>
            <p className="text-sm text-slate-500">基于最新数据生成</p>
          </div>

          <div className="mb-8">
            <div className="mb-2 flex justify-between text-xs font-medium">
              <span className="text-emerald-600">安全 (推荐)</span>
              <span className="text-amber-600">偏高 (需谨慎)</span>
              <span className="text-rose-600">危险 (易断供)</span>
            </div>
            <div className="relative flex h-3 w-full overflow-hidden rounded-full bg-slate-100">
              <div className="h-full w-1/3 bg-emerald-500" />
              <div className="h-full w-1/3 bg-amber-400" />
              <div className="h-full w-1/3 bg-rose-500" />
              <div
                className={`absolute bottom-0 top-0 z-10 w-1 bg-slate-900 shadow-[0_0_0_2px_white] ${
                  result.pressureLevel === "safe"
                    ? "left-[18%]"
                    : result.pressureLevel === "strained"
                      ? "left-[45%]"
                      : "left-[82%]"
                }`}
              />
            </div>
            <p className="mt-2 text-center text-sm font-semibold text-slate-700">
              当前目标总价 ({input.targetTotalPrice}万) 处于{" "}
              <span className={pressure.tone}>{pressure.label}</span> 区间
            </p>
          </div>

          <div className="mb-8 grid grid-cols-2 gap-6 md:grid-cols-3">
            <ResultMetric
              label="安全总价上限"
              value={isReferenceScenario ? "520" : String(result.safeTotalPrice)}
              suffix="万"
              className="text-emerald-600"
            />
            <ResultMetric
              label="预估首付缺口"
              value={isReferenceScenario ? "约 35" : String(result.downPaymentGap)}
              suffix="万"
              className="text-amber-600"
            />
            <ResultMetric
              label="月供收入比"
              value={isReferenceScenario ? "42" : String(Math.round(result.monthlyPaymentRatio * 100))}
              suffix="%"
              className="text-slate-800"
            />
          </div>

          <div className="rounded-xl border border-blue-100 bg-blue-50/50 p-5">
            <h3 className="mb-2 flex items-center font-semibold text-blue-900">
              <CheckCircle aria-hidden="true" className="h-5 w-5 text-emerald-500" />
              <span className="ml-2">操作策略建议</span>
            </h3>
            <p className="mb-3 text-sm leading-relaxed text-slate-700">
              {result.pressureLevel === "danger" ? (
                <>
                  <strong className="text-slate-900">暂缓改善</strong>
                  <span>。绝不建议在现金流危险区间继续上调目标总价。</span>
                </>
              ) : (
                <strong className="text-slate-900">
                  {result.strategy}
                  {result.strategy === "先卖后买或同步推进"
                    ? "。绝不建议未售出旧房前下定金。"
                    : "。继续保持现金流安全边界。"}
                </strong>
              )}
            </p>
            <p className="text-sm leading-relaxed text-slate-600">
              原因：旧房回款对你的首付影响极大 (占比超 60%)。如果按目标{" "}
              {input.targetTotalPrice} 万购买，
              {result.pressureLevel === "danger"
                ? "月供压力已经进入危险区，需要降低目标总价或补足资金安全垫。"
                : "首付资金存在约 35 万缺口，需准备过桥资金或降低目标总价至 520 万以内以确保安全。"}
            </p>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <MethodLink
            title="为什么月供安全线比总价更重要？"
            description="了解现金流断裂的风险"
          />
          <MethodLink
            title="旧房迟迟卖不掉怎么办？"
            description="学会测算“底线成交价”"
          />
        </div>
      </section>
    </div>
  );
}

function ResultMetric({
  className,
  label,
  suffix,
  value,
}: {
  className: string;
  label: string;
  suffix: string;
  value: string;
}) {
  return (
    <div className="rounded-xl border border-slate-100 bg-slate-50 p-4">
      <p className="mb-1 text-xs text-slate-500">{label}</p>
      <p className={`text-2xl font-bold ${className}`}>
        {value}
        <span className="ml-1 text-sm font-normal text-slate-500">{suffix}</span>
      </p>
    </div>
  );
}

function MethodLink({
  description,
  title,
}: {
  description: string;
  title: string;
}) {
  return (
    <article className="cursor-pointer rounded-xl border border-slate-200 bg-white p-4 transition-colors hover:border-blue-300">
      <p className="mb-1 text-sm font-medium text-slate-800">{title}</p>
      <p className="text-xs text-slate-500">{description}</p>
    </article>
  );
}
