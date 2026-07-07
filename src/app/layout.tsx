import type { Metadata } from "next";

import { AppHeader } from "@/components/app-header";

import "./globals.css";

export const metadata: Metadata = {
  title: "房脉 proppulse - 换房决策助手",
  description: "帮准备买房或换房的人算清压力、观察小区信号并判断出手窗口。",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body>
        <AppHeader />
        {children}
        <footer className="border-t border-slate-200 bg-white/80 py-8">
          <div className="mx-auto max-w-7xl px-4 text-center text-sm text-slate-400 sm:px-6 lg:px-8">
            © 2026 房脉 proppulse. 不预测涨跌，只判断信号和压力。
          </div>
        </footer>
      </body>
    </html>
  );
}
