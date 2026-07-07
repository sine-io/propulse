import Link from "next/link";

const navItems = [
  { label: "换房测算", href: "/calculator" },
  { label: "目标小区", href: "/neighborhoods" },
  { label: "出手窗口", href: "/action-window" },
  { label: "判断方法", href: "/methods" },
  { label: "工具模板", href: "/templates" },
  { label: "我的观察池", href: "/watchlist" },
];

export function AppHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-slate-200/80 bg-white/90 backdrop-blur">
      <div className="mx-auto flex min-h-16 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
        <Link
          href="/"
          className="flex items-center gap-3 text-lg font-bold tracking-tight text-slate-950"
        >
          <span className="grid size-9 place-items-center rounded-2xl bg-slate-950 text-sm font-black text-white shadow-lg shadow-slate-950/10">
            脉
          </span>
          <span>
            房脉 <span className="font-normal text-slate-500">proppulse</span>
          </span>
        </Link>

        <nav
          aria-label="主导航"
          className="hidden items-center gap-1 rounded-full bg-slate-100/80 p-1 lg:flex"
        >
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="rounded-full px-3 py-2 text-sm font-medium text-slate-600 transition hover:bg-white hover:text-slate-950 hover:shadow-sm"
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className="flex items-center gap-2">
          <Link
            href="/calculator"
            className="rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white shadow-lg shadow-slate-950/10 transition hover:-translate-y-0.5 hover:bg-slate-800"
          >
            开始测算
          </Link>
        </div>
      </div>
    </header>
  );
}
