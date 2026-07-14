import type { Metadata } from "next";

import { DataManagementPage } from "@/components/data-management-page";

export const metadata: Metadata = {
  title: "数据管理 | 房脉 propulse",
};

export default function DataPage() {
  return <DataManagementPage />;
}
