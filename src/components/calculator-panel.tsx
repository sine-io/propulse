"use client";

import { useMemo, useState } from "react";

import {
  calculateHousingCapacity,
  type HousingCapacityInput,
  type PressureLevel,
} from "@/lib/decision";
import { defaultHousingInput } from "@/lib/sample-data";

import { StatusBadge } from "./status-badge";

const pressureCopy: Record<PressureLevel, { label: string; tone: "emerald" | "amber" | "rose" }> = {
  safe: { label: "安全", tone: "emerald" },
  strained: { label: "偏高", tone: "amber" },
  danger: { label: "危险", tone: "rose" },
};

const fields: Array<{
  key: keyof HousingCapacityInput;
  label: string;
  suffix: string;
}> = [
  { key: "cashOnHand", label: "当前现金", suffix: "万" },
  { key: "oldHomeValue", label: "旧房估值", suffix: "万" },
  { key: "oldLoanBalance", label: "旧房贷款余额", suffix: "万" },
  { key: "monthlyIncome", label: "家庭月收入", suffix: "万/月" },
  { key: "currentMonthlyMortgage", label: "当前月供", suffix: "万/月" },
  { key: "acceptableMonthlyMortgage", label: "可接受月供", suffix: "万/月" },
  { key: "targetTotalPrice", label: "目标总价", suffix: "万" },
  { key: "renovationBudget", label: "装修预算", suffix: "万" },
  { key: "transactionCosts", label: "税费与中介费", suffix: "万" },
  { key: "transitionRentCost", label: "过渡租房成本", suffix: "万" },
];

export function CalculatorPanel() {
  const [input, setInput] = useState<HousingCapacityInput>(defaultHousingInput);
  const result = useMemo(() => calculateHousingCapacity(input), [input]);
  const pressure = pressureCopy[result.pressureLevel];

  const updateValue = (key: keyof HousingCapacityInput, value: string) => {
    setInput((current) => ({
      ...current,
      [key]: Number.isFinite(Number(value)) ? Number(value) : 0,
    }));
  };

  return (
    <div className="grid gap-6 lg:grid-cols-[0.92fr_1.08fr]">
      <section className="rounded-[1.75rem] border border-slate-200 bg-white p-5 shadow-sm">
        <h2 className="text-xl font-black text-slate-950">我的换房条件</h2>
        <p className="mt-2 text-sm leading-6 text-slate-500">
          所有金额以“万”为单位，月收入和月供以“万/月”为单位。
        </p>
        <div className="mt-5 grid gap-4 sm:grid-cols-2">
          {fields.map((field) => {
            const id = `calculator-${field.key}`;
            return (
              <label key={field.key} htmlFor={id} className="block">
                <span className="text-sm font-semibold text-slate-700">
                  {field.label}（{field.suffix}）
                </span>
                <input
                  id={id}
                  value={input[field.key]}
                  onChange={(event) => updateValue(field.key, event.target.value)}
                  inputMode="decimal"
                  className="mt-2 w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-base font-bold text-slate-950 outline-none transition focus:border-blue-400 focus:bg-white focus:ring-4 focus:ring-blue-100"
                />
              </label>
            );
          })}
        </div>
      </section>

      <section className="space-y-5">
        <div className="rounded-[1.75rem] border border-slate-200 bg-white p-6 shadow-xl shadow-slate-950/5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p className="text-sm font-bold text-slate-500">
                换房压力诊断报告
              </p>
              <h2 className="mt-2 text-3xl font-black text-slate-950">
                月供压力：{pressure.label}
              </h2>
            </div>
            <StatusBadge tone={pressure.tone}>风险等级：{pressure.label}</StatusBadge>
          </div>

          <div className="mt-6 grid gap-3 sm:grid-cols-3">
            {[
              ["安全总价", `${result.safeTotalPrice} 万`, "text-emerald-600"],
              ["勉强总价", `${result.strainedTotalPrice} 万`, "text-amber-600"],
              ["危险总价", `${result.dangerTotalPrice} 万`, "text-rose-600"],
            ].map(([label, value, color]) => (
              <div key={label} className="rounded-2xl bg-slate-50 p-4">
                <p className="text-xs font-semibold text-slate-500">{label}</p>
                <p className={`mt-1 text-2xl font-black ${color}`}>{value}</p>
              </div>
            ))}
          </div>

          <div className="mt-6 grid gap-3 sm:grid-cols-3">
            <Metric
              label="月供收入比"
              value={`${Math.round(result.monthlyPaymentRatio * 100)}%`}
            />
            <Metric label="首付缺口" value={`${result.downPaymentGap} 万`} />
            <Metric
              label="旧房安全成交价"
              value={`${result.minimumSafeOldHomeSalePrice} 万`}
            />
          </div>

          <div className="mt-6 h-3 overflow-hidden rounded-full bg-slate-100">
            <div className="grid h-full grid-cols-3">
              <div className="bg-emerald-500" />
              <div className="bg-amber-400" />
              <div className="bg-rose-500" />
            </div>
          </div>
          <div className="mt-2 flex justify-between text-xs font-semibold text-slate-500">
            <span>安全</span>
            <span>偏高</span>
            <span>危险</span>
          </div>
        </div>

        <div className="rounded-[1.75rem] border border-blue-100 bg-blue-50 p-6">
          <h3 className="text-lg font-black text-blue-950">策略建议</h3>
          <p className="mt-2 text-xl font-black text-blue-900">
            {result.strategy}
          </p>
          <ul className="mt-4 space-y-3 text-sm leading-6 text-blue-900">
            {result.reasons.map((reason) => (
              <li key={reason} className="rounded-2xl bg-white/70 p-3">
                {reason}
              </li>
            ))}
          </ul>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <MethodCard title="为什么月供安全线比总价更重要？" />
          <MethodCard title="旧房迟迟卖不掉怎么办？" />
        </div>
      </section>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-100 bg-white p-4">
      <p className="text-xs font-semibold text-slate-500">{label}</p>
      <p className="mt-1 text-xl font-black text-slate-950">{value}</p>
    </div>
  );
}

function MethodCard({ title }: { title: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-4">
      <p className="font-bold text-slate-900">{title}</p>
      <p className="mt-1 text-sm text-slate-500">查看对应判断逻辑和使用方法。</p>
    </div>
  );
}
