import { CalculatorPanel } from "@/components/calculator-panel";

export default function CalculatorPage() {
  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <section className="mb-8">
        <h1 className="text-3xl font-bold text-slate-900">
          先算清楚：你现在能不能换，换完压力有多大
        </h1>
        <p className="mt-2 text-slate-500">
          像体检一样，扫描你的财务健康状况，定位安全预算区间。
        </p>
      </section>
      <CalculatorPanel />
    </main>
  );
}
