import type { Metadata } from "next";

import { ImportDetailPage } from "@/components/import-detail-page";

export const metadata: Metadata = {
  title: "采集批次详情 | 房脉 propulse",
};

export function generateStaticParams() {
  return [{ id: "_" }];
}

export default function CollectionRunDetailPage() {
  return <ImportDetailPage />;
}
