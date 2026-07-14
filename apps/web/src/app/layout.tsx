import type { Metadata } from "next";

import { AppHeader } from "@/components/app-header";

import "./globals.css";

export const metadata: Metadata = {
  title: "房脉 propulse - 房产决策工具",
  description: "输入预算和目标小区，判断换房压力、小区信号和出手窗口。",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body className="flex min-h-screen flex-col">
        <AppHeader />
        {children}
        <footer className="mt-auto border-t border-slate-200 bg-white py-6">
          <div className="mx-auto max-w-7xl px-4 text-center text-sm text-slate-400 sm:px-6 lg:px-8">
            © 2026 房脉 propulse. 数据与结论以页面标注的来源、采集时间和质量状态为准。
          </div>
        </footer>
      </body>
    </html>
  );
}
