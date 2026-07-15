import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { MethodsPage } from "@/components/methods-page";
import { getMethodArticle, methodArticles } from "@/lib/method-articles";

interface MethodArticleRouteProps {
  params: Promise<{ slug: string }>;
}

export const dynamicParams = false;

export function generateStaticParams() {
  return methodArticles.map(({ slug }) => ({ slug }));
}

export async function generateMetadata({
  params,
}: MethodArticleRouteProps): Promise<Metadata> {
  const { slug } = await params;
  const article = getMethodArticle(slug);
  if (!article) {
    notFound();
  }

  return {
    title: `${article.title} | 房脉 propulse`,
  };
}

export default async function MethodArticleRoute({ params }: MethodArticleRouteProps) {
  const { slug } = await params;
  const article = getMethodArticle(slug);
  if (!article) {
    notFound();
  }

  return <MethodsPage article={article} />;
}
