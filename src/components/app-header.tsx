import Link from "next/link";
import { Activity, BookOpen, Calculator, Eye, Target } from "lucide-react";

const navItems = [
  { label: "换房测算", href: "/calculator", icon: Calculator },
  { label: "目标小区", href: "/neighborhoods", icon: Target },
  { label: "出手窗口", href: "/action-window", icon: Activity },
  { label: "判断方法", href: "/methods", icon: BookOpen },
  { label: "我的观察池", href: "/watchlist", icon: Eye },
];

export function AppHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-slate-200 bg-white shadow-sm">
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
        <Link
          href="/"
          className="flex cursor-pointer items-center space-x-2"
        >
          <svg
            aria-hidden="true"
            className="h-8 w-8 text-blue-600"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="2"
          >
            <path d="M3 3v18h18" />
            <path d="M18 17V9a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v8" />
            <polyline points="8 17 8 13 12 13 12 17" />
            <polyline points="12 13 16 13 16 17" />
          </svg>
          <span className="text-xl font-bold tracking-tight text-slate-900">
            房脉{" "}
            <span className="font-light text-slate-500">proppulse</span>
          </span>
        </Link>

        <nav
          aria-label="主导航"
          className="hidden items-center space-x-2 md:flex"
        >
          {navItems.map((item) => {
            const Icon = item.icon;

            return (
              <Link
                key={item.href}
                href={item.href}
                className="flex items-center space-x-1.5 rounded-lg px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-50 hover:text-slate-900"
              >
                <Icon aria-hidden="true" className="h-4 w-4" />
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="flex items-center space-x-4">
          <button className="hidden text-sm font-medium text-slate-500 hover:text-slate-900 sm:block">
            登录
          </button>
          <Link
            href="/calculator"
            className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white shadow-md transition-colors hover:bg-slate-800"
          >
            开始测算
          </Link>
        </div>
      </div>
    </header>
  );
}
