import { describe, expect, it } from "vitest";

import {
  buildMethodPath,
  defaultMethodArticle,
  getMethodArticle,
  methodArticles,
} from "./method-articles";

const expectedSlugs = [
  "listings-up-transactions-weak",
  "asking-price-vs-transactions",
  "buyer-window",
  "more-price-cuts",
  "upgrade-price-gap",
  "monthly-payment-safety",
  "old-home-sale-delay",
];

describe("method article catalog", () => {
  it("defines the seven unique static article slugs", () => {
    expect(methodArticles.map(({ slug }) => slug)).toEqual(expectedSlugs);
    expect(new Set(methodArticles.map(({ slug }) => slug)).size).toBe(methodArticles.length);
    expect(defaultMethodArticle).toBe(methodArticles[0]);
  });

  it("provides complete decision content for every article", () => {
    for (const article of methodArticles) {
      expect(article.title.length).toBeGreaterThan(8);
      expect(article.realQuestion.length).toBeGreaterThan(20);
      expect(article.commonMistake.length).toBeGreaterThan(20);
      expect(article.correctJudgment.length).toBeGreaterThan(40);
      expect(article.keyMetrics.length).toBeGreaterThanOrEqual(5);
      expect(article.keyMetrics.every(({ name, usage }) => name.length > 0 && usage.length > 15)).toBe(true);
      expect(article.example.situation.length).toBeGreaterThan(20);
      expect(article.example.interpretation.length).toBeGreaterThan(20);
      expect(article.actions.length).toBeGreaterThanOrEqual(3);
      expect(article.actions.every((action) => action.length > 15)).toBe(true);
      expect(article.applicableScope.length).toBeGreaterThan(20);
    }
  });

  it("resolves every catalog entry to its stable path", () => {
    for (const article of methodArticles) {
      expect(buildMethodPath(article.slug)).toBe(`/methods/${article.slug}`);
      expect(getMethodArticle(article.slug)).toBe(article);
    }
    expect(getMethodArticle("not-a-method")).toBeUndefined();
  });

  it("does not prescribe unsupported fixed outcomes or bargaining ratios", () => {
    const decisionContent = methodArticles
      .flatMap((article) => [
        article.correctJudgment,
        article.example.interpretation,
        ...article.actions,
      ])
      .join("\n");

    expect(decisionContent).not.toMatch(/砍价.{0,8}\d+%/u);
    expect(decisionContent).not.toMatch(/保证(?:上涨|下跌)|固定(?:涨跌幅|砍价比例)/u);
  });
});
