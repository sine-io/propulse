import { CalculatorPanel } from "@/components/calculator-panel";

export default function CalculatorPage() {
  return (
    <main className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      <section className="mb-8">
        <h1 className="text-4xl font-black text-slate-950">
          先算清楚：你现在能不能换，换完压力有多大
        </h1>
        <p className="mt-4 max-w-3xl text-lg leading-8 text-slate-600">
          左侧像计算器，右侧像体检报告。先确定安全总价、危险边界和换房策略，再进入小区判断。
        </p>
      </section>
      <CalculatorPanel />
    </main>
  );
}
